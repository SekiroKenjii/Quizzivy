//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/core/adapters"
	mediadomain "quizzivy/internal/modules/media/domain"
	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pausedGroupMedia struct {
	locked  chan int
	release <-chan struct{}
}

func (p pausedGroupMedia) LockForVersionUse(ctx context.Context, tx pgx.Tx, id string) error {
	if err := mediarepo.LockForVersionUse(ctx, tx, id); err != nil {
		return err
	}
	var pid int
	if err := tx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		return err
	}
	p.locked <- pid
	select {
	case <-p.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func committedGroupAsset(t *testing.T, pool *pgxpool.Pool, author string) string {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	asset := storedGroupAsset(t, tx, author, "audio")
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM app.media_assets WHERE id=$1`, asset)
	})
	return asset
}

func cleanupStoredGroup(t *testing.T, pool *pgxpool.Pool, id string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = tx.Rollback(ctx) }()
		if _, err := tx.Exec(ctx, `DELETE FROM app.questions WHERE context_group_id=$1`, id); err != nil {
			t.Error(err)
			return
		}
		if _, err := tx.Exec(ctx, `DELETE FROM app.question_groups WHERE id=$1`, id); err != nil {
			t.Error(err)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			t.Error(err)
		}
	})
}

func TestGroupMediaLockSerializesCreateBeforeSoftDelete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	pool := newPool(t)
	author := makeAuthor(t, pool)
	asset := committedGroupAsset(t, pool, author)
	bundle := storedGroupFixture(t, asset)
	cleanupStoredGroup(t, pool, bundle.Group.ID)
	release := make(chan struct{})
	locked := make(chan int, 1)
	store := repositories.NewGroupsPostgres(db.NewContext(pool), adapters.GroupQuestions{}, pausedGroupMedia{locked: locked, release: release})
	created := make(chan error, 1)
	go func() {
		_, err := store.Create(ctx, domain.CreateGroupInput{Bundle: bundle, ActorID: author, Now: time.Now()})
		created <- err
	}()
	var pid int
	select {
	case pid = <-locked:
	case err := <-created:
		t.Fatalf("create ended before media lock: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	deleted := make(chan error, 1)
	go func() {
		deleted <- mediarepo.NewPostgres(db.NewContext(pool)).SoftDelete(ctx, mediadomain.DeleteInput{ID: asset, ActorID: author, Now: time.Now()})
	}()
	observed := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&observed); err != nil {
			t.Fatal(err)
		}
		if observed {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(release)
	if !observed {
		t.Fatal("concurrent delete never waited on the group writer's media lock")
	}
	if err := <-created; err != nil {
		t.Fatalf("group create: %v", err)
	}
	if err := <-deleted; !errors.Is(err, mediadomain.ErrReferenced) {
		t.Fatalf("delete missed the committed context binding: %v", err)
	}
}

func TestGroupMediaLockRejectsAssetDeletedBeforeCreate(t *testing.T) {
	ctx := context.Background()
	pool := newPool(t)
	author := makeAuthor(t, pool)
	asset := committedGroupAsset(t, pool, author)
	media := mediarepo.NewPostgres(db.NewContext(pool))
	if err := media.SoftDelete(ctx, mediadomain.DeleteInput{ID: asset, ActorID: author, Now: time.Now()}); err != nil {
		t.Fatal(err)
	}
	bundle := storedGroupFixture(t, asset)
	store := repositories.NewGroupsPostgres(db.NewContext(pool), adapters.GroupQuestions{}, media)
	if _, err := store.Create(ctx, domain.CreateGroupInput{Bundle: bundle, ActorID: author, Now: time.Now()}); !errors.Is(err, mediadomain.ErrNotFound) {
		t.Fatalf("group revived an already deleted asset: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.question_groups WHERE id=$1`, bundle.Group.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed create left a group: %d, %v", count, err)
	}
}

func TestGroupConcurrentEditsHaveOneRevisionWinner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	pool := newPool(t)
	author := makeAuthor(t, pool)
	asset := committedGroupAsset(t, pool, author)
	bundle := storedGroupFixture(t, asset)
	cleanupStoredGroup(t, pool, bundle.Group.ID)
	media := mediarepo.NewPostgres(db.NewContext(pool))
	store := repositories.NewGroupsPostgres(db.NewContext(pool), adapters.GroupQuestions{}, media)
	stored, err := store.Create(ctx, domain.CreateGroupInput{Bundle: bundle, ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	locked := make(chan int, 1)
	paused := repositories.NewGroupsPostgres(db.NewContext(pool), adapters.GroupQuestions{}, pausedGroupMedia{locked: locked, release: release})
	winner, loser := make(chan error, 1), make(chan error, 1)
	first, second := stored.Bundle, stored.Bundle
	first.Group.Title, second.Group.Title = "First edit", "Stale overwrite"
	go func() {
		_, err := paused.Update(ctx, domain.UpdateGroupInput{GroupMutation: groupMutation(stored, author), Bundle: first})
		winner <- err
	}()
	select {
	case <-locked:
	case err := <-winner:
		t.Fatalf("writer ended before aggregate/media locks: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	go func() {
		_, err := store.Update(ctx, domain.UpdateGroupInput{GroupMutation: groupMutation(stored, author), Bundle: second})
		loser <- err
	}()
	close(release)
	if err := <-winner; err != nil {
		t.Fatal(err)
	}
	if err := <-loser; !errors.Is(err, domain.ErrStaleWrite) {
		t.Fatalf("concurrent stale overwrite: %v", err)
	}
	loaded, err := store.Get(ctx, bundle.Group.ID)
	if err != nil || loaded.Revision != 2 || loaded.Bundle.Group.Title != "First edit" {
		t.Fatalf("lost winner: %+v, %v", loaded, err)
	}
	var events int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE entity_id=$1 AND action='question_group.updated'`, bundle.Group.ID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("failed edit leaked audit: %d, %v", events, err)
	}
}

//go:build integration

package repositories_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

type historySnapshotConnection struct {
	db.Conn
	captured chan struct{}
	resume   <-chan struct{}
	once     sync.Once
}

func (c *historySnapshotConnection) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := c.Conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &historySnapshotTx{Tx: tx, owner: c}, nil
}

type historySnapshotTx struct {
	pgx.Tx
	owner *historySnapshotConnection
}

func (tx *historySnapshotTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := tx.Tx.QueryRow(ctx, sql, args...)
	if strings.Contains(sql, "count(*) FILTER") {
		return historySnapshotRow{Row: row, ctx: ctx, owner: tx.owner}
	}
	return row
}

type historySnapshotRow struct {
	pgx.Row
	ctx   context.Context
	owner *historySnapshotConnection
}

func (r historySnapshotRow) Scan(dest ...any) error {
	if err := r.Row.Scan(dest...); err != nil {
		return err
	}
	r.owner.once.Do(func() { close(r.owner.captured) })
	select {
	case <-r.owner.resume:
		return nil
	case <-r.ctx.Done():
		return r.ctx.Err()
	}
}

func TestHistoryFacetsItemsAndCurrentCountsShareOneSnapshot(t *testing.T) {
	h := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	run := h.processed(t, "snapshot")
	resume := make(chan struct{})
	captured := make(chan struct{})
	connection := &historySnapshotConnection{Conn: h.repo.Conn(), captured: captured, resume: resume}
	repo := repositories.NewPostgres(db.NewContext(connection))
	type outcome struct {
		value domain.List
		err   error
	}
	done := make(chan outcome, 1)
	go func() {
		value, err := repo.List(ctx, domain.Filter{Scope: access.Scope{UserID: h.actor.ID}, Status: []string{"needs_review"}})
		done <- outcome{value, err}
	}()
	var release sync.Once
	t.Cleanup(func() { release.Do(func() { close(resume) }); cancel(); <-done })
	select {
	case <-captured:
	case <-ctx.Done():
		t.Fatal("facet snapshot was not captured")
	}
	if _, err := h.pool.Exec(ctx, `UPDATE app.word_imports SET status='failed' WHERE id=$1`, run.ImportID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `UPDATE app.word_import_drafts SET open_action_count=7,open_confirm_count=9 WHERE import_id=$1`, run.ImportID); err != nil {
		t.Fatal(err)
	}
	release.Do(func() { close(resume) })
	var got outcome
	select {
	case got = <-done:
		done <- got
	case <-ctx.Done():
		t.Fatal("snapshot list did not join")
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.value.Facets.NeedsReview != 1 || got.value.Facets.Failed != 0 || got.value.Page.Total != 1 || len(got.value.Items) != 1 || got.value.Items[0].Status != "needs_review" {
		t.Fatalf("mixed import snapshots %+v", got.value)
	}
	counts := got.value.Items[0].ReviewCounts
	if counts == nil || counts.NeedsAction != 1 || counts.ToConfirm != 0 {
		t.Fatalf("mixed draft counts %+v", counts)
	}
	fresh, err := h.repo.List(ctx, domain.Filter{Scope: access.Scope{UserID: h.actor.ID}})
	if err != nil || fresh.Facets.Failed != 1 || len(fresh.Items) != 1 || fresh.Items[0].ReviewCounts == nil || fresh.Items[0].ReviewCounts.NeedsAction != 7 || fresh.Items[0].ReviewCounts.ToConfirm != 9 {
		t.Fatalf("concurrent write not observable after snapshot: %+v %v", fresh, err)
	}
}

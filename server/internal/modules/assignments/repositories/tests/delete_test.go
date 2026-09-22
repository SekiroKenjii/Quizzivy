//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/modules/assignments/repositories"
	"quizzivy/internal/platform/db"
	"testing"
	"time"
)

func TestAssignmentDeletionRequiresDraftOrClosedWithoutAttempts(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, "published")
	repo := repositories.NewPostgres(db.NewContext(pool))
	ctx := context.Background()
	created, err := repo.Create(ctx, request(w), legalInput(w))
	if err != nil {
		t.Fatal(err)
	}
	req := request(w)
	req.ID = created.ID
	if err := repo.Delete(ctx, req, time.Now()); !errors.Is(err, domain.ErrNotArchived) {
		t.Fatalf("live deletion = %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE app.assignments SET closed_at = greatest(opens_at, now()) WHERE id = $1`, created.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, req, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted assignment still accessible: %v", err)
	}
}

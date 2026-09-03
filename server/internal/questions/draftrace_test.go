package questions_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/questions"
)

// TestLockForDraftUseSerialisesAgainstDelete is the point of the lock.
func TestLockForDraftUseSerialisesAgainstDelete(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	for attempt := range 8 {
		q := write(t, svc, author, "Câu hỏi tranh chấp")
		sectionID := newDraftSection(t, pool, author)

		var wg sync.WaitGroup
		var deleteErr, insertErr error

		wg.Add(2)
		go func() {
			defer wg.Done()
			deleteErr = svc.Delete(ctx, questions.WriteRequest{ID: q.ID, ActorID: author})
		}()
		go func() {
			defer wg.Done()
			insertErr = claimForDraft(ctx, pool, sectionID, q.ID)
		}()
		wg.Wait()

		deleted := deleteErr == nil
		claimed := insertErr == nil

		if deleted && claimed {
			t.Fatalf("attempt %d: the question was soft-deleted AND claimed by a draft; "+
				"the outline now points at a deleted question", attempt)
		}
		if !deleted && !claimed {
			t.Logf("attempt %d: both lost (delete=%v insert=%v)", attempt, deleteErr, insertErr)
		}
		// A refused delete must be the documented refusal, not a lock error.
		if !deleted && !errors.Is(deleteErr, questions.ErrReferenced) &&
			!errors.Is(deleteErr, questions.ErrNotFound) {
			t.Errorf("attempt %d: delete failed with %v, want ErrReferenced or ErrNotFound",
				attempt, deleteErr)
		}
	}
}

// TestLockForDraftUseRefusesADeletedQuestion: a soft-deleted question must not
// be addable to a draft at all, however the race resolves.
func TestLockForDraftUseRefusesADeletedQuestion(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	q := write(t, svc, author, "Câu hỏi đã xoá")
	sectionID := newDraftSection(t, pool, author)
	if err := svc.Delete(ctx, questions.WriteRequest{ID: q.ID, ActorID: author}); err != nil {
		t.Fatal(err)
	}

	if err := claimForDraft(ctx, pool, sectionID, q.ID); !errors.Is(err, questions.ErrNotFound) {
		t.Errorf("adding a deleted question to a draft returned %v, want ErrNotFound", err)
	}
}

// claimForDraft is the insert side as T-2.8 must write it: take the question's
// row lock inside the transaction, then insert.
func claimForDraft(ctx context.Context, pool *pgxpool.Pool, sectionID, questionID string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := questions.LockForDraftUse(ctx, tx, questionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO app.test_section_questions (test_section_id, ordinal, question_id)
		 VALUES ($1, 0, $2)`, sectionID, questionID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

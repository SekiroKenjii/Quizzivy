package dashboard_test

import (
	"context"
	"testing"
	"time"

	"quizzivy/internal/dashboard"
)

func TestTheAttemptListFiltersTheGradingQueueAndTheFlaggedOnes(t *testing.T) {
	pool := newPool(t)
	tx := isolated(t, pool)
	store := dashboard.NewStore(tx)
	ctx := context.Background()

	pending := seed(t, tx, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), false)
	flagged := seed(t, tx, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), true)
	if _, err := tx.Exec(ctx, `UPDATE app.attempt_answers SET manual_score = '2.00', graded_at = now() WHERE attempt_id = $1`, flagged.attempt); err != nil {
		t.Fatal(err)
	}

	yes := true
	queue, page, err := store.List(ctx, dashboard.ListInput{PendingGrading: &yes, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !has(queue, pending.attempt) || has(queue, flagged.attempt) {
		t.Errorf("pendingGrading=true: got %v", ids(queue))
	}
	if page.Total < 1 || page.Size != 100 {
		t.Errorf("page %+v", page)
	}

	marked, _, err := store.List(ctx, dashboard.ListInput{Flagged: &yes, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !has(marked, flagged.attempt) || has(marked, pending.attempt) {
		t.Errorf("flagged=true: got %v", ids(marked))
	}
	for _, r := range marked {
		if r.ID == flagged.attempt && (r.StudentName != "Học viên" || r.PendingManual != 0) {
			t.Errorf("row %+v", r)
		}
	}
}

func has(rows []dashboard.Recent, id string) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

func ids(rows []dashboard.Recent) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}

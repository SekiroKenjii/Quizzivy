package attempts_test

import (
	"context"
	"testing"

	"quizzivy/internal/attempts"
)

// Reset voids and permits; it never deletes, so the first attempt stays
// readable and the second is numbered after it (O-08, §6.4).
func TestAfterAResetTheOldAttemptIsReadableAndTheNewOneIsNumberedNext(t *testing.T) {
	pool := newPool(t)
	svc, w, first := started(t, pool)
	answerEverythingRight(t, pool, w, first, svc)
	ctx := context.Background()
	if _, err := svc.Submit(ctx, first.Attempt.ID, w.student, attempts.Manual); err != nil {
		t.Fatal(err)
	}

	// max_attempts is 1, so without the reset the student is done.
	if _, err := svc.StartOrResume(ctx, w.assignment, w.student); err != attempts.ErrLimitReached {
		t.Fatalf("before reset: %v, want ErrLimitReached", err)
	}

	voided, err := svc.Reset(ctx, teacher(w), first.Attempt.ID, "Mất điện giữa giờ, cả lớp xác nhận")
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if voided.Status != attempts.Voided || voided.AttemptNo != 1 {
		t.Errorf("reset returned %+v, want attempt 1 voided", voided)
	}
	if audits := auditRows(t, pool, first.Attempt.ID, "attempt.reset"); len(audits) != 1 {
		t.Errorf("%d audit rows for the reset, want 1", len(audits))
	}

	second, err := svc.StartOrResume(ctx, w.assignment, w.student)
	if err != nil {
		t.Fatalf("start after reset: %v", err)
	}
	if second.Attempt.AttemptNo != 2 {
		t.Errorf("attempt_no %d, want 2", second.Attempt.AttemptNo)
	}
	if second.Attempt.ID == first.Attempt.ID {
		t.Error("the reset handed back the same attempt")
	}
	if len(second.Answers) != 0 {
		t.Errorf("the new attempt started with %d answers, want a clean paper", len(second.Answers))
	}

	if got := count(t, pool, `SELECT count(*) FROM app.attempt_answers WHERE attempt_id = $1::uuid`, first.Attempt.ID); got != 4 {
		t.Errorf("the old attempt has %d answers, want its 4 still readable", got)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status::text FROM app.attempts WHERE id = $1::uuid`, first.Attempt.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "voided" {
		t.Errorf("old attempt is %s, want voided", status)
	}
}

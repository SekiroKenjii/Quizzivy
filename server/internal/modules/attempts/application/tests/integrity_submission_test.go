//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"testing"
	"time"
)

func TestFocusBeaconDoesNotCloseBeforeReloadRecoversPendingAnswers(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE app.assignments SET integrity_max_focus_loss = -1, integrity_on_limit_exceeded = 'auto_submit' WHERE id = $1`, w.assignment); err != nil {
		t.Fatal(err)
	}
	flush := flushOf(w, session, 50, "window_focus")
	flush.Events[0].Meta = []byte(`{"awayMs":4000}`)
	if _, err := svc.Commands.Flush.Handle(ctx, command.Flush{Input: flush}); err != nil {
		t.Fatal(err)
	}
	request := query.Get{AttemptID: session.Attempt.ID, StudentID: w.student}
	reloaded, err := svc.Queries.Get.Handle(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Attempt.Status != domain.InProgress {
		t.Fatal("refetch closed the attempt before pending answers could be recovered")
	}
	answer := domain.Answer{QuestionID: w.essay, Payload: []byte(`{"type":"text","value":"Recovered final answer"}`)}
	if _, err := svc.Commands.Save.Handle(ctx, command.Save{Input: domain.SaveInput{AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID, Answers: []domain.Answer{answer}}}); err != nil {
		t.Fatal(err)
	}
	var status, text string
	if err := pool.QueryRow(ctx, `SELECT a.status::text, aa.payload->>'value'
 FROM app.attempts a JOIN app.attempt_answers aa ON aa.attempt_id = a.id AND aa.question_id = $2
 WHERE a.id = $1`, session.Attempt.ID, w.essay).Scan(&status, &text); err != nil {
		t.Fatal(err)
	}
	if status != "submitted" || text != "Recovered final answer" {
		t.Fatalf("status=%s final answer=%q", status, text)
	}
}

func TestFocusLimitSubmitsAfterSavingFinalAnswersAndRecordsViolation(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE app.assignments SET integrity_max_focus_loss = -1, integrity_on_limit_exceeded = 'auto_submit' WHERE id = $1`, w.assignment); err != nil {
		t.Fatal(err)
	}
	answerEverythingRight(t, pool, w, session, svc)
	in := domain.SaveInput{
		AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
		Answers: []domain.Answer{{QuestionID: w.essay, Payload: []byte(`{"type":"text","value":"Final unsaved answer"}`)}},
		Events:  []domain.Event{{Kind: "window_focus", OccurredAt: time.Now(), ClientSeq: 50, Meta: []byte(`{"awayMs":4000}`)}},
	}
	if _, err := svc.Commands.Save.Handle(ctx, command.Save{Input: in}); err != nil {
		t.Fatal(err)
	}
	var status, answer string
	var flagged bool
	var earned float64
	if err := pool.QueryRow(ctx, `SELECT a.status::text, a.flagged, a.score_earned, aa.payload->>'value'
 FROM app.attempts a JOIN app.attempt_answers aa ON aa.attempt_id = a.id AND aa.question_id = $2
 WHERE a.id = $1`, session.Attempt.ID, w.essay).Scan(&status, &flagged, &earned, &answer); err != nil {
		t.Fatal(err)
	}
	if status != "submitted" || !flagged || earned != 15 || answer != "Final unsaved answer" {
		t.Fatalf("result = %s flagged=%v earned=%v answer=%q", status, flagged, earned, answer)
	}
	if _, err := svc.Commands.Save.Handle(ctx, command.Save{Input: in}); !errors.Is(err, domain.ErrAttemptClosed) {
		t.Fatalf("closed attempt remains editable: %v", err)
	}
	if !containsKind(eventKinds(t, pool, session.Attempt.ID), "auto_submit") {
		t.Fatal("missing auto-submit timeline event")
	}
}

func TestUnlimitedAutoSubmitPolicyNeverClosesForFocusLoss(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE app.assignments SET integrity_max_focus_loss = 0, integrity_on_limit_exceeded = 'auto_submit' WHERE id = $1`, w.assignment); err != nil {
		t.Fatal(err)
	}
	in := domain.SaveInput{AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
		Events: []domain.Event{{Kind: "window_focus", OccurredAt: time.Now(), ClientSeq: 50, Meta: []byte(`{"awayMs":4000}`)}}}
	if _, err := svc.Commands.Save.Handle(ctx, command.Save{Input: in}); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status::text FROM app.attempts WHERE id = $1`, session.Attempt.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "in_progress" {
		t.Fatalf("unlimited policy closed attempt: %s", status)
	}
}

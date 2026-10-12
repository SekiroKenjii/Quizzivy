//go:build integration

package application_test

import (
	"bytes"
	"context"
	"encoding/json"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func takenOver(t *testing.T, pool *pgxpool.Pool) (*application.Application, world, domain.Session, domain.Session) {
	t.Helper()
	svc, w, first := started(t, pool)
	second, err := svc.Commands.StartOrResume.Handle(context.Background(), command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if second.Attempt.ID != first.Attempt.ID || second.SessionID == first.SessionID {
		t.Fatalf("the resume did not take attempt %s over: session %s, then %s on attempt %s",
			first.Attempt.ID, first.SessionID, second.SessionID, second.Attempt.ID)
	}
	return svc, w, first, second
}

func beaconHash(t *testing.T, pool *pgxpool.Pool, attemptID string) []byte {
	t.Helper()
	var hash []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT beacon_token_hash FROM app.attempts WHERE id = $1::uuid`, attemptID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	return hash
}

func wantTheHoldersAnswer(t *testing.T, pool *pgxpool.Pool, svc *application.Application, q query.Get, holder domain.Session) {
	t.Helper()
	before := beaconHash(t, pool, q.AttemptID)

	got, err := svc.Queries.Get.Handle(context.Background(), q)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.Superseded {
		t.Error("the reader was told it is superseded")
	}
	if got.SessionID != holder.SessionID {
		t.Errorf("session %s, want the attempt's, %s", got.SessionID, holder.SessionID)
	}
	if got.BeaconToken == "" {
		t.Error("the reader was handed no beacon token")
	}
	if bytes.Equal(beaconHash(t, pool, q.AttemptID), before) {
		t.Error("the stored beacon token was not replaced")
	}
}

func TestAGetNamingASupersededSessionIsNotHandedTheNewOne(t *testing.T) {
	pool := newPool(t)
	svc, w, first, second := takenOver(t, pool)
	hashBefore := beaconHash(t, pool, first.Attempt.ID)
	eventsBefore := eventKinds(t, pool, first.Attempt.ID)

	got, err := svc.Queries.Get.Handle(context.Background(), query.Get{AttemptID: first.Attempt.ID, StudentID: w.student, HeldSession: first.SessionID})
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if !got.Superseded {
		t.Error("a reader naming the session that was taken over was not told it is superseded")
	}
	if got.SessionID != first.SessionID {
		t.Errorf("session %s, want the one the reader named, %s (the attempt's is %s)", got.SessionID, first.SessionID, second.SessionID)
	}
	if got.BeaconToken != "" {
		t.Error("a superseded reader was handed a beacon token")
	}
	if !bytes.Equal(beaconHash(t, pool, first.Attempt.ID), hashBefore) {
		t.Error("the read replaced the stored beacon token")
	}
	if after := eventKinds(t, pool, first.Attempt.ID); !slices.Equal(after, eventsBefore) {
		t.Errorf("events %v after the read, want %v: the read recorded something", after, eventsBefore)
	}
	held := count(t, pool, `SELECT count(*) FROM app.attempts
	                         WHERE id = $1::uuid AND session_id = $2::uuid`,
		first.Attempt.ID, second.SessionID)
	if held != 1 {
		t.Error("the read took the attempt's session from the tab that holds it")
	}
}

func TestTheTokenOfTheTabThatHoldsTheAttemptSurvivesAnotherTabsRead(t *testing.T) {
	pool := newPool(t)
	svc, w, first, second := takenOver(t, pool)
	ctx := context.Background()

	if _, err := svc.Queries.Get.Handle(ctx, query.Get{AttemptID: first.Attempt.ID, StudentID: w.student, HeldSession: first.SessionID}); err != nil {
		t.Fatalf("get: %v", err)
	}

	closing := domain.FlushInput{
		AttemptID:   second.Attempt.ID,
		SessionID:   second.SessionID,
		BeaconToken: second.BeaconToken,
		Events:      []domain.Event{{Kind: "page_hide", OccurredAt: time.Now(), ClientSeq: 99}},
	}
	if _, err := svc.Commands.Flush.Handle(ctx, command.Flush{Input: closing}); err != nil {
		t.Fatalf("the closing flush of the tab that holds the attempt was refused: %v", err)
	}
	if !containsKind(eventKinds(t, pool, second.Attempt.ID), "page_hide") {
		t.Error("the closing event of the tab that holds the attempt was not stored")
	}
}

func TestAGetNamingTheAttemptsSessionIsHandedIt(t *testing.T) {
	pool := newPool(t)
	svc, w, _, second := takenOver(t, pool)

	wantTheHoldersAnswer(t, pool, svc, query.Get{AttemptID: second.Attempt.ID, StudentID: w.student, HeldSession: second.SessionID}, second)
}

func TestAGetNamingNoSessionIsHandedTheCurrentOne(t *testing.T) {
	pool := newPool(t)
	svc, w, _, second := takenOver(t, pool)

	wantTheHoldersAnswer(t, pool, svc, query.Get{AttemptID: second.Attempt.ID, StudentID: w.student}, second)
}

func TestAnEndedAttemptIsNeverMarkedSuperseded(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	answerEverythingRight(t, pool, w, session, svc)
	ctx := context.Background()
	if _, err := svc.Commands.Submit.Handle(ctx, command.Submit{AttemptID: session.Attempt.ID, StudentID: w.student, Reason: domain.Manual}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	got, err := svc.Queries.Get.Handle(ctx, query.Get{AttemptID: session.Attempt.ID, StudentID: w.student, HeldSession: uuid.NewString()})
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.Attempt.Status == domain.InProgress {
		t.Fatalf("status %q after a submit, want an ended attempt", got.Attempt.Status)
	}
	if got.Superseded {
		t.Error("an ended attempt was marked superseded")
	}
	if got.SessionID != session.SessionID {
		t.Errorf("session %s, want the attempt's own, %s", got.SessionID, session.SessionID)
	}
}

func TestASupersededReadStillSeesWhatTheOtherSessionSaved(t *testing.T) {
	pool := newPool(t)
	svc, w, first, second := takenOver(t, pool)
	ctx := context.Background()
	const written = "Viết ở thiết bị thứ hai."
	payload, err := json.Marshal(map[string]string{"type": "text", "value": written})
	if err != nil {
		t.Fatal(err)
	}
	saved := domain.SaveInput{
		AttemptID: second.Attempt.ID, StudentID: w.student, SessionID: second.SessionID,
		Answers: []domain.Answer{{QuestionID: w.essay, Payload: payload}},
	}
	if _, err := svc.Commands.Save.Handle(ctx, command.Save{Input: saved}); err != nil {
		t.Fatalf("save under the session that holds the attempt: %v", err)
	}

	got, err := svc.Queries.Get.Handle(ctx, query.Get{AttemptID: first.Attempt.ID, StudentID: w.student, HeldSession: first.SessionID})
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if !got.Superseded {
		t.Error("a reader naming the session that was taken over was not told it is superseded")
	}
	var answer struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(got.Answers[w.essay], &answer); err != nil || answer.Value != written {
		t.Errorf("the essay answer reads %q (%v) in %s, want what the other session saved", answer.Value, err, got.Answers[w.essay])
	}
}

package attempts_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/attempts"
)

func started(t *testing.T, pool *pgxpool.Pool) (*attempts.Service, world, attempts.Session) {
	t.Helper()
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	session, err := svc.StartOrResume(context.Background(), w.assignment, w.student)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	return svc, w, session
}

func batch(w world, s attempts.Session, seq int) attempts.SaveInput {
	return attempts.SaveInput{
		AttemptID: s.Attempt.ID,
		StudentID: w.student,
		SessionID: s.SessionID,
		Answers: []attempts.Answer{
			{QuestionID: w.choice, Payload: []byte(`{"type":"choice","optionIds":[]}`)},
			{QuestionID: w.essay, Payload: []byte(`{"type":"text","value":"Tôi dậy lúc 6 giờ."}`)},
		},
		Events: []attempts.Event{
			{Kind: "tab_hidden", OccurredAt: time.Now(), ClientSeq: seq, QuestionID: &w.choice},
			{Kind: "tab_visible", OccurredAt: time.Now(), ClientSeq: seq + 1},
		},
	}
}

func answerCount(t *testing.T, pool *pgxpool.Pool, attemptID string) int {
	t.Helper()
	return count(t, pool, `SELECT count(*) FROM app.attempt_answers WHERE attempt_id = $1::uuid`, attemptID)
}

func TestAutosaveWritesAnswersAndTheEventsBesideThem(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)

	got, err := svc.Save(context.Background(), batch(w, session, 1))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if got.Saved != 2 {
		t.Errorf("saved %d answers, want 2", got.Saved)
	}
	if answerCount(t, pool, session.Attempt.ID) != 2 {
		t.Error("the answers are not in the table")
	}
	if n := len(eventKinds(t, pool, session.Attempt.ID)); n != 2 {
		t.Errorf("%d events recorded, want 2", n)
	}
}

// [D-01] A flush that fails is retried, and a retry must be free. Without the
// conflict clause the second attempt is a duplicate-key error, and a client
// that cannot safely retry is a client that loses events.
func TestReplayingTheIdenticalBatchChangesNothing(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()
	same := batch(w, session, 1)

	if _, err := svc.Save(ctx, same); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if _, err := svc.Save(ctx, same); err != nil {
		t.Fatalf("replay: %v", err)
	}

	if n := answerCount(t, pool, session.Attempt.ID); n != 2 {
		t.Errorf("%d answers after a replay, want 2", n)
	}
	if n := len(eventKinds(t, pool, session.Attempt.ID)); n != 2 {
		t.Errorf("%d events after a replay, want 2 — the batch was recorded twice", n)
	}
}

// The same question answered again overwrites. A student changing their mind
// must not leave two rows for one question with no rule about which counts.
func TestAnsweringTheSameQuestionAgainOverwrites(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	first := batch(w, session, 1)
	if _, err := svc.Save(ctx, first); err != nil {
		t.Fatalf("first: %v", err)
	}
	second := batch(w, session, 3)
	second.Answers = []attempts.Answer{
		{QuestionID: w.essay, Payload: []byte(`{"type":"text","value":"Tôi dậy lúc 5 giờ."}`)},
	}
	if _, err := svc.Save(ctx, second); err != nil {
		t.Fatalf("second: %v", err)
	}

	var payload string
	if err := pool.QueryRow(ctx, `
		SELECT payload::text FROM app.attempt_answers
		 WHERE attempt_id = $1::uuid AND question_id = $2::uuid`,
		session.Attempt.ID, w.essay).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if want := "5 giờ"; !strings.Contains(payload, want) {
		t.Errorf("stored payload %s does not carry the newer answer %q", payload, want)
	}
	if n := answerCount(t, pool, session.Attempt.ID); n != 2 {
		t.Errorf("%d answers, want 2 — answering again added a row instead of replacing one", n)
	}
}

// E2E 7. The tab that lost learns on its next write, and that write must not
// land: it is a stale device typing into a paper another device now owns.
func TestASupersededSessionIsRefusedAndWritesNothing(t *testing.T) {
	pool := newPool(t)
	svc, w, first := started(t, pool)
	ctx := context.Background()

	second, err := svc.StartOrResume(ctx, w.assignment, w.student)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if second.SessionID == first.SessionID {
		t.Fatal("the session did not change; this test proves nothing")
	}

	if _, err := svc.Save(ctx, batch(w, first, 1)); !errors.Is(err, attempts.ErrSessionSuperseded) {
		t.Fatalf("got %v, want ErrSessionSuperseded", err)
	}
	if n := answerCount(t, pool, first.Attempt.ID); n != 0 {
		t.Errorf("%d answers were written by a superseded session, want 0", n)
	}
	// The resume events are the server's own; the losing tab's must not be there.
	for _, kind := range eventKinds(t, pool, first.Attempt.ID) {
		if kind == "tab_hidden" {
			t.Error("a superseded session's events were recorded")
		}
	}
}

func TestWritingAfterTheDeadlineIsRefused(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE app.attempts
		    SET started_at = now() - interval '2 hours', deadline_at = now() - interval '1 hour'
		  WHERE id = $1::uuid`, session.Attempt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Save(ctx, batch(w, session, 1)); !errors.Is(err, attempts.ErrDeadlinePassed) {
		t.Fatalf("got %v, want ErrDeadlinePassed", err)
	}
	if n := answerCount(t, pool, session.Attempt.ID); n != 0 {
		t.Errorf("%d answers landed after the deadline, want 0", n)
	}
}

func TestWritingToASubmittedAttemptIsRefused(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE app.attempts SET status='submitted', submitted_at=now() WHERE id=$1::uuid`,
		session.Attempt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Save(ctx, batch(w, session, 1)); !errors.Is(err, attempts.ErrAttemptClosed) {
		t.Fatalf("got %v, want ErrAttemptClosed", err)
	}
}

// A closed attempt is over; being told to submit is advice the client cannot
// act on, and being told the session moved is worse. Both conditions hold here.
func TestAClosedAttemptOutranksTheDeadline(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `
		UPDATE app.attempts
		   SET status='submitted', submitted_at=now(),
		       started_at = now() - interval '2 hours', deadline_at = now() - interval '1 hour'
		 WHERE id=$1::uuid`, session.Attempt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Save(ctx, batch(w, session, 1)); !errors.Is(err, attempts.ErrAttemptClosed) {
		t.Fatalf("got %v, want ErrAttemptClosed", err)
	}
}

// "Submit" is not something a superseded tab can do, so it must not be what it
// is told when both are true.
func TestASupersededSessionOutranksTheDeadline(t *testing.T) {
	pool := newPool(t)
	svc, w, first := started(t, pool)
	ctx := context.Background()

	if _, err := svc.StartOrResume(ctx, w.assignment, w.student); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE app.attempts
		    SET started_at = now() - interval '2 hours', deadline_at = now() - interval '1 hour'
		  WHERE id=$1::uuid`, first.Attempt.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Save(ctx, batch(w, first, 1)); !errors.Is(err, attempts.ErrSessionSuperseded) {
		t.Fatalf("got %v, want ErrSessionSuperseded", err)
	}
}

func TestAnotherStudentCannotWriteToThisAttempt(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)

	in := batch(w, session, 1)
	in.StudentID = w.outsider
	if _, err := svc.Save(context.Background(), in); !errors.Is(err, attempts.ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
	if n := answerCount(t, pool, session.Attempt.ID); n != 0 {
		t.Error("an outsider's write landed")
	}
}

// [D-19] §7's pendingManual needs a real column to filter on, because
// final_score is VIRTUAL and PG18 refuses to index it. The type is known at
// write time, so it is decided here rather than at grading.
func TestOnlyTheHandGradedTypeIsMarkedForManualGrading(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)

	if _, err := svc.Save(context.Background(), batch(w, session, 1)); err != nil {
		t.Fatalf("save: %v", err)
	}

	for _, c := range []struct {
		question string
		name     string
		want     bool
	}{
		{w.essay, "short_answer", true},
		{w.choice, "single_choice", false},
	} {
		var got bool
		if err := pool.QueryRow(context.Background(), `
			SELECT requires_manual FROM app.attempt_answers
			 WHERE attempt_id = $1::uuid AND question_id = $2::uuid`,
			session.Attempt.ID, c.question).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("%s requires_manual = %v, want %v", c.name, got, c.want)
		}
	}
}

// A question that is not on this paper is dropped, not fatal.
//
// The only ways to send one are a client bug or an attempt to write against
// someone else's paper. Refusing the whole batch would throw away the real
// answers sitting beside it, and losing a student's work is the outcome this
// whole feature exists to prevent.
func TestAQuestionFromAnotherPaperIsIgnoredRatherThanFatal(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	other := seedWorld(t, pool, openAssignment())

	in := batch(w, session, 1)
	in.Answers = append(in.Answers, attempts.Answer{
		QuestionID: other.choice, Payload: []byte(`{"type":"choice","optionIds":[]}`),
	})

	got, err := svc.Save(context.Background(), in)
	if err != nil {
		t.Fatalf("a foreign question id was fatal: %v", err)
	}
	if got.Saved != 2 {
		t.Errorf("saved %d, want 2 — the foreign question should not have been written", got.Saved)
	}
	// Named, so the drop leaves a trace. The handler logs these; without them
	// a client bug that destroys work is invisible on the server.
	if len(got.Dropped) != 1 || got.Dropped[0] != other.choice {
		t.Errorf("dropped %v, want exactly the foreign question %s", got.Dropped, other.choice)
	}
	if n := count(t, pool, `SELECT count(*) FROM app.attempt_answers WHERE question_id = $1::uuid`,
		other.choice); n != 0 {
		t.Error("an answer was written against another paper's question")
	}
}

// The event's question id is context, not a key. An unknown one is stored as
// NULL rather than failing a batch that also carries answers.
func TestAnEventNamingAnUnknownQuestionIsStoredWithoutIt(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	stranger := uuid.NewString()

	in := batch(w, session, 1)
	in.Events = []attempts.Event{
		{Kind: "paste", OccurredAt: time.Now(), ClientSeq: 9, QuestionID: &stranger},
	}
	if _, err := svc.Save(context.Background(), in); err != nil {
		t.Fatalf("save: %v", err)
	}

	var questionID *string
	if err := pool.QueryRow(context.Background(),
		`SELECT question_id::text FROM app.attempt_events
		  WHERE attempt_id = $1::uuid AND kind = 'paste'`,
		session.Attempt.ID).Scan(&questionID); err != nil {
		t.Fatal(err)
	}
	if questionID != nil {
		t.Errorf("question_id = %q, want NULL", *questionID)
	}
}

// The join drops an answer against a question that is not on the paper; this
// is the same rule one level down. A choice naming an id that is not one of
// that question's options -- malformed, or a real option lifted from another
// question -- would grade as nonsense with no visible cause, so it is not
// stored. The answers beside it are (#52).
func TestAChoiceNamingAnOptionThatIsNotTheQuestionsIsDroppedNotStored(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	var choiceOption string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM app.test_version_options
		 WHERE test_version_question_id = $1::uuid AND is_correct`, w.choice).Scan(&choiceOption); err != nil {
		t.Fatal(err)
	}

	got, err := svc.Save(ctx, attempts.SaveInput{
		AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
		Answers: []attempts.Answer{
			{QuestionID: w.choice, Payload: []byte(`{"type":"choice","optionIds":[""]}`)},
			{QuestionID: w.listening, Payload: []byte(`{"type":"choice","optionIds":["` + choiceOption + `"]}`)},
			{QuestionID: w.essay, Payload: []byte(`{"type":"text","value":"Tôi dậy lúc 6 giờ."}`)},
		},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if got.Saved != 1 {
		t.Errorf("saved %d, want 1: the essay, and neither choice", got.Saved)
	}
	if len(got.Dropped) != 2 {
		t.Errorf("dropped %v, want both choice questions named", got.Dropped)
	}
	stored := count(t, pool, `SELECT count(*) FROM app.attempt_answers
		 WHERE attempt_id = $1::uuid AND question_id IN ($2::uuid, $3::uuid)`,
		session.Attempt.ID, w.choice, w.listening)
	if stored != 0 {
		t.Errorf("%d choice rows stored, want 0", stored)
	}

	// The same question with its own option is an ordinary save.
	got, err = svc.Save(ctx, attempts.SaveInput{
		AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
		Answers: []attempts.Answer{
			{QuestionID: w.choice, Payload: []byte(`{"type":"choice","optionIds":["` + choiceOption + `"]}`)},
		},
	})
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if got.Saved != 1 {
		t.Errorf("saved %d, want 1: a real option on its own question", got.Saved)
	}
}

// A batch where everything lands names nothing: the log line this feeds is
// meant to be rare, and a Dropped that is never empty is a log nobody reads.
func TestABatchThatFullyLandsNamesNothingAsDropped(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)

	got, err := svc.Save(context.Background(), batch(w, session, 1))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if got.Saved != 2 || len(got.Dropped) != 0 {
		t.Errorf("saved %d, dropped %v; want 2 saved and nothing dropped", got.Saved, got.Dropped)
	}
}

// Re-sending the same answers is a retry, not a drop. ON CONFLICT DO UPDATE
// returns the row either way, so a client that flushes twice must not produce
// a warning that says its answers went missing.
func TestAReplayedBatchIsNotAdrop(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	if _, err := svc.Save(ctx, batch(w, session, 1)); err != nil {
		t.Fatalf("first save: %v", err)
	}
	again, err := svc.Save(ctx, batch(w, session, 1))
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(again.Dropped) != 0 {
		t.Errorf("a replayed batch reported %v as dropped", again.Dropped)
	}
}

package attempts_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/attempts"
)

// answerEverythingRight fills the fixture's paper with the correct answers, so
// a test can assert on a score rather than on zero.
func answerEverythingRight(t *testing.T, pool *pgxpool.Pool, w world, s attempts.Session, svc *attempts.Service) {
	t.Helper()
	ctx := context.Background()

	var correctOption, listeningOption string
	if err := pool.QueryRow(ctx, `
		SELECT id::text FROM app.test_version_options
		 WHERE test_version_question_id = $1::uuid AND is_correct`, w.choice).Scan(&correctOption); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT id::text FROM app.test_version_options
		 WHERE test_version_question_id = $1::uuid AND is_correct`, w.listening).Scan(&listeningOption); err != nil {
		t.Fatal(err)
	}

	blanks := blankIDs(t, pool, w.blank)
	values := `{"` + blanks[0] + `":"` + secretBlankAnswer + `","` + blanks[1] + `":"` + secretBlankAnswer + `"}`

	in := attempts.SaveInput{
		AttemptID: s.Attempt.ID, StudentID: w.student, SessionID: s.SessionID,
		Answers: []attempts.Answer{
			{QuestionID: w.choice, Payload: []byte(`{"type":"choice","optionIds":["` + correctOption + `"]}`)},
			{QuestionID: w.listening, Payload: []byte(`{"type":"choice","optionIds":["` + listeningOption + `"]}`)},
			{QuestionID: w.blank, Payload: []byte(`{"type":"fill_blank","values":` + values + `}`)},
			{QuestionID: w.essay, Payload: []byte(`{"type":"text","value":"Tôi dậy lúc 6 giờ."}`)},
		},
	}
	if _, err := svc.Save(ctx, in); err != nil {
		t.Fatalf("save answers: %v", err)
	}
}

func blankIDs(t *testing.T, pool *pgxpool.Pool, questionID string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT id::text FROM app.test_version_blanks
		 WHERE test_version_question_id = $1::uuid ORDER BY ordinal`, questionID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	return out
}

func TestSubmittingGradesWhatAMachineCanAndLeavesTheRest(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	answerEverythingRight(t, pool, w, session, svc)

	closed, err := svc.Submit(context.Background(), session.Attempt.ID, w.student, attempts.Manual)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Three objective questions at 5 points each, all right. The essay is the
	// teacher's, so the attempt is not `graded` yet.
	if closed.Status != attempts.Submitted {
		t.Errorf("status %q, want submitted — an essay is still waiting for a person", closed.Status)
	}
	if closed.SubmittedAt == nil {
		t.Error("submittedAt was not set")
	}

	var earned, total float64
	if err := pool.QueryRow(context.Background(),
		`SELECT score_earned, score_total FROM app.attempts WHERE id = $1::uuid`,
		session.Attempt.ID).Scan(&earned, &total); err != nil {
		t.Fatal(err)
	}
	if earned != 15 {
		t.Errorf("earned %v, want 15", earned)
	}
	if total != 10 {
		t.Errorf("total %v, want 10 — it comes from the frozen version, not from summing the paper", total)
	}
}

// §15 calls it idempotent. A manual tap racing the timer's auto-submit must
// yield one submission, not two grading passes over the same answers.
func TestASecondSubmitIsRefusedRatherThanRegrading(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	answerEverythingRight(t, pool, w, session, svc)
	ctx := context.Background()

	if _, err := svc.Submit(ctx, session.Attempt.ID, w.student, attempts.Manual); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if _, err := svc.Submit(ctx, session.Attempt.ID, w.student, attempts.Manual); !errors.Is(err, attempts.ErrAttemptClosed) {
		t.Fatalf("second submit gave %v, want ErrAttemptClosed", err)
	}
}

func TestConcurrentSubmitsProduceOneSubmission(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	answerEverythingRight(t, pool, w, session, svc)

	const racers = 6
	var wg sync.WaitGroup
	errs := make([]error, racers)
	start := make(chan struct{})
	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = svc.Submit(context.Background(), session.Attempt.ID, w.student, attempts.Manual)
		}()
	}
	close(start)
	wg.Wait()

	won := 0
	for i, err := range errs {
		switch {
		case err == nil:
			won++
		case errors.Is(err, attempts.ErrAttemptClosed):
		default:
			t.Fatalf("racer %d: %v", i, err)
		}
	}
	if won != 1 {
		t.Errorf("%d submits succeeded, want exactly 1", won)
	}
}

// `graded` is the teacher's word, not the machine's (§8). A paper with no essay
// is fully scored at submit and still says `submitted`, because that is what
// happened -- and it does not clutter anyone's queue, which counts ANSWERS
// awaiting a person rather than attempts in a status.
func TestAPaperWithNoEssayIsScoredButNotDeclaredGraded(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	var correct string
	if err := pool.QueryRow(ctx, `
		SELECT id::text FROM app.test_version_options
		 WHERE test_version_question_id = $1::uuid AND is_correct`, w.choice).Scan(&correct); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Save(ctx, attempts.SaveInput{
		AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
		Answers: []attempts.Answer{
			{QuestionID: w.choice, Payload: []byte(`{"type":"choice","optionIds":["` + correct + `"]}`)},
		},
	}); err != nil {
		t.Fatal(err)
	}

	closed, err := svc.Submit(ctx, session.Attempt.ID, w.student, attempts.Manual)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if closed.Status != attempts.Submitted {
		t.Errorf("status %q, want submitted", closed.Status)
	}
	if closed.GradedAt != nil {
		t.Error("gradedAt was set by the machine; that word is the teacher's")
	}

	awaiting := count(t, pool, `
		SELECT count(*) FROM app.attempt_answers ans
		  JOIN app.attempts at ON at.id = ans.attempt_id
		 WHERE at.id = $1::uuid AND ans.requires_manual AND ans.manual_score IS NULL
		   AND at.status IN ('submitted','timed_out')`, session.Attempt.ID)
	if awaiting != 0 {
		t.Errorf("%d answers in the queue, want 0 — there is no essay to read", awaiting)
	}
}

func TestAnotherStudentCannotSubmitThisAttempt(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)

	if _, err := svc.Submit(context.Background(), session.Attempt.ID, w.outsider, attempts.Manual); !errors.Is(err, attempts.ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
}

//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"slices"
	"strings"
	"testing"
)

// §13.5's rule, checked the way it actually fails: by value. A key-name walk
// catches a field someone adds; only a value search catches a grading key that
// arrives under an innocent name, or embedded in a string.
func TestThePaperCarriesNoPartOfTheGradingKey(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())

	for _, held := range []struct{ query, id, secret string }{
		{`SELECT count(*) FROM app.test_version_questions WHERE id = $1::uuid AND explanation = $2`, w.choice, secretExplanation},
		{`SELECT count(*) FROM app.test_version_questions WHERE id = $1::uuid AND sample_answer = $2`, w.essay, secretSampleAnswer},
		{`SELECT count(*) FROM app.test_version_questions WHERE id = $1::uuid AND transcript = $2`, w.listening, secretTranscript},
		{`SELECT count(*) FROM app.test_version_blank_answers ba
		    JOIN app.test_version_blanks b ON b.id = ba.test_version_blank_id
		   WHERE b.test_version_question_id = $1::uuid AND ba.answer = $2`, w.blank, secretBlankAnswer},
	} {
		if count(t, pool, held.query, held.id, held.secret) == 0 {
			t.Fatalf("the fixture does not hold %q; this test proves nothing", held.secret)
		}
	}

	session, err := newService(t, pool).Commands.StartOrResume.Handle(context.Background(), command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	encoded, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}

	for _, secret := range []string{
		secretExplanation, secretSampleAnswer, secretBlankAnswer, secretTranscript,
	} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("the payload contains %q", secret)
		}
	}
}

// A guard against a field added later. Nothing in the domain types can carry
// these today, which is the point -- this fails the moment that stops being
// true, wherever in the tree it happens.
func TestNoAnswerBearingFieldAppearsAtAnyDepth(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())

	session, err := newService(t, pool).Commands.StartOrResume.Handle(context.Background(), command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	encoded, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	var tree any
	if err := json.Unmarshal(encoded, &tree); err != nil {
		t.Fatal(err)
	}

	banned := map[string]bool{
		"iscorrect": true, "sampleanswer": true, "acceptedanswers": true,
		"transcript": true, "transcripts": true, "explanation": true, "teachernote": true,
		"shuffleseed": true, "seed": true, "beacontokenhash": true,
	}
	var walk func(node any, path string)
	walk = func(node any, path string) {
		switch v := node.(type) {
		case map[string]any:
			for key, child := range v {
				if banned[strings.ToLower(key)] {
					t.Errorf("%s.%s is in the student payload", path, key)
				}
				walk(child, path+"."+key)
			}
		case []any:
			for i, child := range v {
				walk(child, path+"["+string(rune('0'+i%10))+"]")
			}
		}
	}
	walk(tree, "$")
}

// The blank ids are shown; what may be typed into them is not. Reading the
// answers table directly is the only way to be sure the projection never
// touched it.
func TestBlanksArriveWithoutTheirAcceptedAnswers(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())

	session, err := newService(t, pool).Commands.StartOrResume.Handle(context.Background(), command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	var blanks int
	for _, q := range session.Questions {
		blanks += len(q.Blanks)
		for _, b := range q.Blanks {
			if b.ID == "" || b.Ordinal < 1 {
				t.Errorf("blank %+v is not usable by the client", b)
			}
			if got, want := jsonKeys(t, b), []string{"CaseSensitive", "GapID", "ID", "Ordinal"}; !slices.Equal(got, want) {
				t.Errorf("a blank reaches the student with fields %v, want %v", got, want)
			}
		}
	}
	if blanks != 2 {
		t.Fatalf("%d blanks reached the student, want 2", blanks)
	}

	if got := count(t, pool, `
		SELECT count(*) FROM app.test_version_blank_answers ba
		  JOIN app.test_version_blanks b ON b.id = ba.test_version_blank_id
		 WHERE b.test_version_question_id = $1::uuid AND ba.answer = $2`, w.blank, secretBlankAnswer); got != 2 {
		t.Fatalf("the fixture holds %d accepted answers, want 2; this test proves nothing", got)
	}
}

// Options are what a student picks between. Which of them is right is the whole
// secret, and it is not in the type.
func TestOptionsArriveWithoutTheirCorrectness(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())

	session, err := newService(t, pool).Commands.StartOrResume.Handle(context.Background(), command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	var options int
	for _, q := range session.Questions {
		options += len(q.Options)
		for _, o := range q.Options {
			if got, want := jsonKeys(t, o), []string{"Content", "ID", "Text"}; !slices.Equal(got, want) {
				t.Errorf("an option reaches the student with fields %v, want %v", got, want)
			}
		}
	}
	if options != 6 {
		t.Fatalf("%d options reached the student, want 6", options)
	}
	if got := count(t, pool,
		`SELECT count(*) FROM app.test_version_options WHERE test_version_question_id = $1::uuid AND is_correct`,
		w.choice); got != 1 {
		t.Fatalf("the fixture marks %d options correct, want 1; this test proves nothing", got)
	}
}

func TestAnotherStudentsAttemptIsNotReadable(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	ctx := context.Background()

	mine, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Refused as forbidden rather than missing.
	if _, err := svc.Queries.Get.Handle(ctx, query.Get{AttemptID: mine.Attempt.ID, StudentID: w.outsider}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("got %v, want ErrForbidden", err)
	}
	if _, err := svc.Queries.Get.Handle(ctx, query.Get{AttemptID: "01935000-0000-7000-8000-0000000000ff", StudentID: w.student}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("an attempt that does not exist gave %v, want the same ErrForbidden", err)
	}
}

// Refetching the paper is not taking it over. A student reconciling audio plays
// on the tab they are already sitting in must not supersede themselves.
func TestFetchingThePayloadDoesNotSupersedeTheSession(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	ctx := context.Background()

	started, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	fetched, err := svc.Queries.Get.Handle(ctx, query.Get{AttemptID: started.Attempt.ID, StudentID: w.student})
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if fetched.SessionID != started.SessionID {
		t.Errorf("session changed on a read: %s then %s", started.SessionID, fetched.SessionID)
	}
	if got := eventKinds(t, pool, started.Attempt.ID); len(got) != 1 || got[0] != domain.KindStart {
		t.Errorf("a read wrote to the timeline: %v, want only the start", got)
	}
}

// The paper is dealt from the stored seed, so it survives a refetch intact.
// Answers are keyed by question id and would survive either way; a student
// reading "câu 4" and a teacher reviewing "câu 4" would not.
func TestRefetchingDealsTheSamePaper(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	svc := newService(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE app.assignments SET shuffle_questions = true, shuffle_options = true WHERE id = $1::uuid`,
		w.assignment); err != nil {
		t.Fatal(err)
	}

	started, err := svc.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	for i := range 5 {
		fetched, err := svc.Queries.Get.Handle(ctx, query.Get{AttemptID: started.Attempt.ID, StudentID: w.student})
		if err != nil {
			t.Fatalf("get %d: %v", i, err)
		}
		if got, want := ids(fetched.Questions), ids(started.Questions); got != want {
			t.Fatalf("fetch %d re-dealt the paper\n got %s\nwant %s", i, got, want)
		}
	}
}

func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func ids(qs []domain.Question) string {
	var b strings.Builder
	for _, q := range qs {
		b.WriteString(q.ID)
		for _, o := range q.Options {
			b.WriteString("/" + o.ID)
		}
		b.WriteString(" ")
	}
	return b.String()
}

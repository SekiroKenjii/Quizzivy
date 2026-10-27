//go:build integration

package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
)

func wrongOf(t *testing.T, pool *pgxpool.Pool, scope access.Scope, assignment string) (domain.WrongAnswers, error) {
	t.Helper()
	return newService(t, pool).Queries.WrongAnswers.Handle(context.Background(), query.WrongAnswers{AssignmentID: assignment, Scope: scope})
}

func optionIDs(t *testing.T, pool *pgxpool.Pool, question string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT id::text FROM app.test_version_options WHERE test_version_question_id = $1::uuid ORDER BY ordinal`, question)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	return ids
}

func chose(options ...string) map[string]any {
	return map[string]any{"type": "choice", "optionIds": options}
}

func filled(values map[string]string) map[string]any {
	raw := map[string]any{}
	for id, v := range values {
		raw[id] = v
	}
	return map[string]any{"type": "fill_blank", "values": raw}
}

func TestTheWrongAnswersAreTheDistinctTextsThatDidNotEarnFullPointsWithTheirCounts(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	people := enrol(t, pool, w, 9)
	options, blanks := optionIDs(t, pool, w.choice), blankIDs(t, pool, w.blank)

	paper := func(student string, answers func(attempt string)) {
		answers(handIn(t, pool, w, student, 1, "submitted"))
	}
	paper(people[0], func(a string) {
		saved(t, pool, a, w.choice, chose(options[1]), mark(0), nil, false)
		saved(t, pool, a, w.essay, essay("  Play   Street "), nil, mark(0), true)
		saved(t, pool, a, w.blank, filled(map[string]string{blanks[0]: "Has Lived", blanks[1]: "2019"}), mark(2.5), nil, false)
	})
	paper(people[1], func(a string) {
		saved(t, pool, a, w.choice, chose(options[1]), mark(0), nil, false)
		saved(t, pool, a, w.essay, essay("play street"), nil, mark(0), true)
	})
	paper(people[2], func(a string) {
		saved(t, pool, a, w.choice, chose(options[2], options[1]), mark(0), nil, false)
		saved(t, pool, a, w.essay, essay("playing street"), nil, mark(2.5), true)
	})
	paper(people[3], func(a string) {
		saved(t, pool, a, w.choice, chose(options[0]), mark(5), nil, false)
		saved(t, pool, a, w.essay, essay("play streets"), nil, mark(5), true)
		saved(t, pool, a, w.blank, filled(map[string]string{blanks[0]: secretBlankAnswer, blanks[1]: secretBlankAnswer}), mark(5), nil, false)
	})
	paper(people[4], func(a string) {
		saved(t, pool, a, w.choice, chose(), mark(0), nil, false)
		saved(t, pool, a, w.essay, essay("   "), nil, mark(0), true)
		saved(t, pool, a, w.blank, emptyBlank(), mark(0), nil, false)
	})
	paper(people[5], func(a string) {
		saved(t, pool, a, w.essay, essay("not marked yet"), nil, nil, true)
	})
	first := handIn(t, pool, w, people[6], 1, "submitted")
	saved(t, pool, first, w.choice, chose(options[3]), mark(0), nil, false)
	later := handIn(t, pool, w, people[6], 2, "submitted")
	saved(t, pool, later, w.choice, chose(options[0]), mark(5), nil, false)
	voided := handIn(t, pool, w, people[7], 1, "voided")
	saved(t, pool, voided, w.choice, chose(options[3]), mark(0), nil, false)
	sitting := handIn(t, pool, w, people[8], 1, "in_progress")
	saved(t, pool, sitting, w.choice, chose(options[3]), mark(0), nil, false)

	got, err := wrongOf(t, pool, everyone, w.assignment)
	if err != nil {
		t.Fatal(err)
	}
	if got.HandedIn != 7 {
		t.Errorf("handedIn = %d, want 7: seven students handed in, the voided and the unfinished papers left out", got.HandedIn)
	}
	if len(got.Questions) != 4 {
		t.Fatalf("%d questions, want 4: %+v", len(got.Questions), got.Questions)
	}

	byID := map[string]domain.QuestionWrongAnswers{}
	for _, q := range got.Questions {
		byID[q.QuestionID] = q
	}
	for i, id := range []string{w.choice, w.blank, w.essay, w.listening} {
		if got.Questions[i].QuestionID != id || got.Questions[i].Number != i+1 {
			t.Errorf("question %d is %s #%d, want %s", i, got.Questions[i].QuestionID, got.Questions[i].Number, id)
		}
	}

	choice := byID[w.choice]
	if choice.Key != "A" || choice.Type != "single_choice" {
		t.Errorf("the choice question reads key %q type %q, want A and single_choice", choice.Key, choice.Type)
	}
	wantChoice := []domain.WrongAnswer{{Text: "B", Count: 2}, {Text: "B+C", Count: 1}}
	if fmt.Sprint(choice.Wrong) != fmt.Sprint(wantChoice) {
		t.Errorf("choice wrong answers = %+v, want %+v: letters, an answer that says nothing and a full mark left out, and only the latest paper of the student with two", choice.Wrong, wantChoice)
	}

	essayQ := byID[w.essay]
	if essayQ.Key != secretSampleAnswer {
		t.Errorf("the short answer's key = %q, want its sample answer", essayQ.Key)
	}
	wantEssay := []domain.WrongAnswer{{Text: "play street", Count: 2}, {Text: "playing street", Count: 1}}
	if fmt.Sprint(essayQ.Wrong) != fmt.Sprint(wantEssay) {
		t.Errorf("short answer wrong answers = %+v, want %+v: lower case, spaces collapsed, a blank answer, a full mark and an unmarked answer left out", essayQ.Wrong, wantEssay)
	}

	blank := byID[w.blank]
	wantKey := "1: " + secretBlankAnswer + "; 2: " + secretBlankAnswer
	if blank.Key != wantKey {
		t.Errorf("the fill-in's key = %q, want %q", blank.Key, wantKey)
	}
	if fmt.Sprint(blank.Wrong) != fmt.Sprint([]domain.WrongAnswer{{Text: "has lived | 2019", Count: 1}}) {
		t.Errorf("fill-in wrong answers = %+v, want one partly right answer", blank.Wrong)
	}
	if len(byID[w.listening].Wrong) != 0 {
		t.Errorf("a question nobody answered reports %+v", byID[w.listening].Wrong)
	}
}

func TestOnlyTheEightMostFrequentWrongAnswersAreReported(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	people := enrol(t, pool, w, 12)
	for i, student := range people {
		a := handIn(t, pool, w, student, 1, "submitted")
		text := fmt.Sprintf("answer %02d", i)
		if i >= 10 {
			text = "answer 00"
		}
		saved(t, pool, a, w.essay, essay(text), nil, mark(0), true)
	}
	got, err := wrongOf(t, pool, everyone, w.assignment)
	if err != nil {
		t.Fatal(err)
	}
	var essayQ domain.QuestionWrongAnswers
	for _, q := range got.Questions {
		if q.QuestionID == w.essay {
			essayQ = q
		}
	}
	if len(essayQ.Wrong) != domain.MaxWrongAnswers {
		t.Fatalf("%d wrong answers, want %d", len(essayQ.Wrong), domain.MaxWrongAnswers)
	}
	if essayQ.Wrong[0] != (domain.WrongAnswer{Text: "answer 00", Count: 3}) || essayQ.Wrong[1] != (domain.WrongAnswer{Text: "answer 01", Count: 1}) {
		t.Errorf("the most frequent come first, then by text: %+v", essayQ.Wrong[:2])
	}
}

func TestAnotherTeachersAssignmentHasNoWrongAnswersToRead(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	other := seedWorld(t, pool, openAssignment())

	_, err := wrongOf(t, pool, access.Scope{UserID: other.admin}, w.assignment)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound: another teacher's assignment answers as a missing one", err)
	}
	if _, err := wrongOf(t, pool, access.Scope{UserID: w.admin}, w.assignment); err != nil {
		t.Errorf("the owner's read: %v", err)
	}
}

func TestTheWrongAnswersOfATfngAndAMatchingQuestionReadInTheirOwnWords(t *testing.T) {
	pool := newPool(t)
	w := seedTypedWorld(t, pool)
	session := startTyped(t, pool, w)
	svc := newService(t, pool)
	ctx := context.Background()
	tfng := questionOf(t, session, w.tfng).Options
	if _, err := svc.Commands.Save.Handle(ctx, command.Save{Input: domain.SaveInput{
		AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
		Answers: []domain.Answer{
			{QuestionID: w.tfng, Payload: []byte(fmt.Sprintf(`{"type":"choice","optionIds":[%q]}`, tfng[0].ID))},
			{QuestionID: w.matching, Payload: []byte(fmt.Sprintf(`{"type":"matching","pairs":{%q:%q,%q:%q}}`, w.items[0], w.choices[2], w.items[1], w.choices[1]))},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.Submit.Handle(ctx, command.Submit{AttemptID: session.Attempt.ID, StudentID: w.student, Reason: domain.Manual}); err != nil {
		t.Fatal(err)
	}
	got, err := wrongOf(t, pool, everyone, w.assignment)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct{ key, wrong string }{
		w.tfng:     {"Not given", "True"},
		w.matching: {"1: C; 2: A", "1: C; 2: B"},
	}
	for _, q := range got.Questions {
		expected, ok := want[q.QuestionID]
		if !ok {
			continue
		}
		delete(want, q.QuestionID)
		if q.Key != expected.key {
			t.Errorf("%s: key %q, want %q", q.Type, q.Key, expected.key)
		}
		if len(q.Wrong) != 1 || q.Wrong[0].Text != expected.wrong || q.Wrong[0].Count != 1 {
			t.Errorf("%s: wrong answers %+v, want one %q", q.Type, q.Wrong, expected.wrong)
		}
	}
	if len(want) > 0 {
		t.Errorf("questions not read: %v", want)
	}
}

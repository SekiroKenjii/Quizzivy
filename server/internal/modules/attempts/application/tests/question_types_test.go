//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"fmt"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/domain"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// typedWorld adds one frozen question of each R6 type to a world's version.
type typedWorld struct {
	world
	tfng, chooseTwo, limited, essay, matching string
	items, choices                            []string
}

func seedTypedWorld(t *testing.T, pool *pgxpool.Pool) typedWorld {
	t.Helper()
	ctx := context.Background()
	w := typedWorld{world: seedWorld(t, pool, openAssignment())}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var section string
	must(pool.QueryRow(ctx, `SELECT id::text FROM app.test_version_sections WHERE test_version_id = $1::uuid`, w.versionID).Scan(&section))
	question := func(ordinal int, columns, values string, args ...any) string {
		t.Helper()
		var id string
		must(pool.QueryRow(ctx, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, prompt, points, `+columns+`)
			VALUES ($1::uuid, $2, 'Câu hỏi', '2.00', `+values+`) RETURNING id::text`, append([]any{section, ordinal}, args...)...).Scan(&id))
		return id
	}
	options := func(questionID string, correct ...bool) []string {
		t.Helper()
		ids := make([]string, len(correct))
		for i, isCorrect := range correct {
			must(pool.QueryRow(ctx, `INSERT INTO app.test_version_options (test_version_question_id, ordinal, text, is_correct)
				VALUES ($1::uuid, $2, $3, $4) RETURNING id::text`, questionID, i, fmt.Sprintf("choice %d", i), isCorrect).Scan(&ids[i]))
		}
		return ids
	}
	w.tfng = question(4, "type", "'tfng'")
	options(w.tfng, false, false, true)
	w.chooseTwo = question(5, "type, select_count", "'multiple_choice', 2")
	options(w.chooseTwo, true, false, true, false)
	w.limited = question(6, "type, max_words", "'short_answer', 3")
	w.essay = question(7, "type, sample_answer", "'essay', $3", secretSampleAnswer)
	_, err := pool.Exec(ctx, `INSERT INTO app.test_version_rubric_criteria (test_version_question_id, ordinal, label) VALUES ($1::uuid, 0, 'TIEU-CHI-AN')`, w.essay)
	must(err)
	w.matching = question(8, "type, match_allow_reuse", "'matching', false")
	w.choices = options(w.matching, false, false, false)
	for i, key := range []int{2, 0} {
		var id string
		must(pool.QueryRow(ctx, `INSERT INTO app.test_version_match_items (test_version_question_id, ordinal, text, correct_option_id)
			VALUES ($1::uuid, $2, $3, $4::uuid) RETURNING id::text`, w.matching, i, fmt.Sprintf("item %d", i), w.choices[key]).Scan(&id))
		w.items = append(w.items, id)
	}
	_, err = pool.Exec(ctx, `UPDATE app.assignments SET shuffle_options = true WHERE id = $1::uuid`, w.assignment)
	must(err)
	_, err = pool.Exec(ctx, `UPDATE app.test_versions SET delivery_version = 'group_v1' WHERE id = $1::uuid`, w.versionID)
	must(err)
	return w
}

func startTyped(t *testing.T, pool *pgxpool.Pool, w typedWorld) domain.Session {
	t.Helper()
	session, err := newService(t, pool).Commands.StartOrResume.Handle(context.Background(), command.StartOrResume{AssignmentID: w.assignment, StudentID: w.student})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	return session
}

func questionOf(t *testing.T, s domain.Session, id string) domain.Question {
	t.Helper()
	at := slices.IndexFunc(s.Questions, func(q domain.Question) bool { return q.ID == id })
	if at < 0 {
		t.Fatalf("question %s is not on the paper", id)
	}
	return s.Questions[at]
}

func TestNoPaperTypeHasAFieldNamedForAnAnswer(t *testing.T) {
	banned := []string{"Correct", "Accepted", "Sample", "Transcript", "Rubric"}
	rules := map[string]bool{"ShowTranscriptAfterSubmit": true}
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type, string)
	walk = func(typ reflect.Type, path string) {
		for typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Map {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct || seen[typ] {
			return
		}
		seen[typ] = true
		for i := range typ.NumField() {
			field := typ.Field(i)
			for _, word := range banned {
				if strings.Contains(field.Name, word) && !rules[field.Name] {
					t.Errorf("%s.%s names an answer", path, field.Name)
				}
			}
			walk(field.Type, path+"."+field.Name)
		}
	}
	walk(reflect.TypeFor[domain.Session](), "Session")
	if !seen[reflect.TypeFor[domain.MatchItem]()] {
		t.Fatal("the walk never reached MatchItem, so it proves nothing about the new types")
	}
}

func TestThePaperCarriesTheRulesAndKeysOfTheNewTypes(t *testing.T) {
	pool := newPool(t)
	w := seedTypedWorld(t, pool)
	session := startTyped(t, pool, w)

	if got := questionOf(t, session, w.chooseTwo).SelectCount; got == nil || *got != 2 {
		t.Errorf("selectCount = %v, want 2", got)
	}
	if got := questionOf(t, session, w.limited).MaxWords; got == nil || *got != 3 {
		t.Errorf("maxWords = %v, want 3", got)
	}
	matching := questionOf(t, session, w.matching)
	if matching.MatchAllowReuse == nil || *matching.MatchAllowReuse {
		t.Errorf("matchAllowReuse = %v, want false", matching.MatchAllowReuse)
	}
	var keys []string
	for _, o := range matching.Options {
		keys = append(keys, o.Key+" "+o.Text)
	}
	if !slices.Equal(keys, []string{"i choice 0", "ii choice 1", "iii choice 2"}) {
		t.Errorf("matching choices %v, want i to iii in authored order", keys)
	}
	if got := matching.MatchItems; len(got) != 2 || got[0] != (domain.MatchItem{ID: w.items[0], Key: "A", Text: "item 0"}) || got[1].Key != "B" {
		t.Errorf("match items %+v", got)
	}
	tfng := questionOf(t, session, w.tfng)
	for i, o := range tfng.Options {
		if o.Text != fmt.Sprintf("choice %d", i) || o.Key != "" {
			t.Errorf("tfng option %d is %+v; a tfng keeps its order and has no key", i, o)
		}
	}
	if essay := questionOf(t, session, w.essay); len(essay.Options) != 0 || len(essay.MatchItems) != 0 {
		t.Errorf("an essay arrives with %+v", essay)
	}

	encoded, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{secretSampleAnswer, "TIEU-CHI-AN", w.choices[2]} {
		if secret == w.choices[2] {
			if strings.Count(string(encoded), secret) != 1 {
				t.Errorf("choice %s appears %d times; only as an option, never as an item's key", secret, strings.Count(string(encoded), secret))
			}
			continue
		}
		if strings.Contains(string(encoded), secret) {
			t.Errorf("the paper contains %q", secret)
		}
	}
}

func TestTFNGAndMatchingKeepTheirOrderWhileOtherOptionsShuffle(t *testing.T) {
	pool := newPool(t)
	moved := false
	for range 6 {
		w := seedTypedWorld(t, pool)
		session := startTyped(t, pool, w)
		for _, id := range []string{w.tfng, w.matching} {
			for i, o := range questionOf(t, session, id).Options {
				if o.Text != fmt.Sprintf("choice %d", i) {
					t.Fatalf("question %s option %d is %q; it was shuffled", id, i, o.Text)
				}
			}
		}
		for i, o := range questionOf(t, session, w.chooseTwo).Options {
			if o.Text != fmt.Sprintf("choice %d", i) {
				moved = true
			}
		}
	}
	if !moved {
		t.Fatal("six seeds never shuffled the choose-two options, so the fixed order proves nothing")
	}
}

func TestAutosaveKeepsOnlyPairsThatBelongToTheQuestion(t *testing.T) {
	pool := newPool(t)
	w := seedTypedWorld(t, pool)
	session := startTyped(t, pool, w)
	svc := newService(t, pool)
	ctx := context.Background()
	pairs := func(item, option string) []byte {
		return []byte(fmt.Sprintf(`{"type":"matching","pairs":{%q:%q}}`, item, option))
	}
	cases := []struct {
		name     string
		question string
		payload  []byte
		kept     bool
	}{
		{"its own item and choice", w.matching, pairs(w.items[0], w.choices[1]), true},
		{"no pairs yet", w.matching, []byte(`{"type":"matching","pairs":{}}`), true},
		{"another question's option", w.matching, pairs(w.items[0], w.choice), false},
		{"an option as the item", w.matching, pairs(w.choices[0], w.choices[1]), false},
		{"a matching answer on a single choice", w.choice, []byte(`{"type":"matching","pairs":{}}`), false},
		{"a matching answer on a short answer", w.essay, []byte(`{"type":"matching","pairs":{}}`), false},
		{"a choice answer on a matching", w.matching, []byte(`{"type":"choice","optionIds":[]}`), false},
		{"a text answer on a matching", w.matching, []byte(`{"type":"text","value":"A ii"}`), false},
		{"a true/false answer on a matching", w.matching, []byte(`{"type":"true_false","value":true}`), false},
		{"a fill-in answer on a matching", w.matching, []byte(`{"type":"fill_blank","values":{}}`), false},
		{"three picks on a choose two", w.chooseTwo, []byte(`{"type":"choice","optionIds":[]}`), true},
	}
	options := questionOf(t, session, w.chooseTwo).Options
	cases[len(cases)-1].payload = []byte(fmt.Sprintf(`{"type":"choice","optionIds":[%q,%q,%q]}`, options[0].ID, options[1].ID, options[2].ID))
	for _, c := range cases {
		got, err := svc.Commands.Save.Handle(ctx, command.Save{Input: domain.SaveInput{
			AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
			Answers: []domain.Answer{{QuestionID: c.question, Payload: c.payload}},
		}})
		if err != nil {
			t.Fatalf("%s: save: %v", c.name, err)
		}
		if kept := got.Saved == 1; kept != c.kept {
			t.Errorf("%s: kept %v, want %v (dropped %v)", c.name, kept, c.kept, got.Dropped)
		}
	}
}

func TestAnEssayAnswerAwaitsTheTeacher(t *testing.T) {
	pool := newPool(t)
	w := seedTypedWorld(t, pool)
	session := startTyped(t, pool, w)
	if _, err := newService(t, pool).Commands.Save.Handle(context.Background(), command.Save{Input: domain.SaveInput{
		AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
		Answers: []domain.Answer{
			{QuestionID: w.essay, Payload: []byte(`{"type":"text","value":"An essay."}`)},
			{QuestionID: w.limited, Payload: []byte(`{"type":"text","value":"far too many words here"}`)},
			{QuestionID: w.tfng, Payload: []byte(`{"type":"choice","optionIds":[]}`)},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	if got := count(t, pool, `SELECT count(*) FROM app.attempt_answers WHERE attempt_id = $1::uuid AND requires_manual`, session.Attempt.ID); got != 2 {
		t.Fatalf("%d answers await the teacher, want the essay and the short answer", got)
	}
	if got := count(t, pool, `SELECT count(*) FROM app.attempt_answers WHERE attempt_id = $1::uuid AND payload->>'value' = 'far too many words here'`, session.Attempt.ID); got != 1 {
		t.Fatal("an answer over the word limit was not stored as typed")
	}
}

func TestAnEssayStillAwaitsTheTeacherAfterSubmitAndTheNewTypesAreScored(t *testing.T) {
	pool := newPool(t)
	w := seedTypedWorld(t, pool)
	session := startTyped(t, pool, w)
	svc := newService(t, pool)
	ctx := context.Background()
	chooseTwo := questionOf(t, session, w.chooseTwo).Options
	var right []string
	for _, o := range chooseTwo {
		if o.Text == "choice 0" || o.Text == "choice 2" {
			right = append(right, o.ID)
		}
	}
	tfng := questionOf(t, session, w.tfng).Options
	if _, err := svc.Commands.Save.Handle(ctx, command.Save{Input: domain.SaveInput{
		AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
		Answers: []domain.Answer{
			{QuestionID: w.essay, Payload: []byte(`{"type":"text","value":"An essay."}`)},
			{QuestionID: w.tfng, Payload: []byte(fmt.Sprintf(`{"type":"choice","optionIds":[%q]}`, tfng[2].ID))},
			{QuestionID: w.chooseTwo, Payload: []byte(fmt.Sprintf(`{"type":"choice","optionIds":[%q]}`, right[0]))},
			{QuestionID: w.matching, Payload: []byte(fmt.Sprintf(`{"type":"matching","pairs":{%q:%q,%q:%q}}`, w.items[0], w.choices[2], w.items[1], w.choices[1]))},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.Submit.Handle(ctx, command.Submit{AttemptID: session.Attempt.ID, StudentID: w.student, Reason: domain.Manual}); err != nil {
		t.Fatal(err)
	}
	scored := func(question string) (manual bool, auto *float64) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT requires_manual, auto_score FROM app.attempt_answers WHERE attempt_id = $1::uuid AND question_id = $2::uuid`,
			session.Attempt.ID, question).Scan(&manual, &auto); err != nil {
			t.Fatal(err)
		}
		return manual, auto
	}
	if manual, _ := scored(w.essay); !manual {
		t.Error("the essay was settled at submit; it must wait for the teacher (QC-R6SP-2)")
	}
	for question, want := range map[string]float64{w.tfng: 2, w.chooseTwo: 1, w.matching: 1} {
		if manual, auto := scored(question); manual || auto == nil || *auto != want {
			t.Errorf("question %s scored %v (manual %v), want %v", question, auto, manual, want)
		}
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status::text FROM app.attempts WHERE id = $1::uuid`, session.Attempt.ID).Scan(&status); err != nil || status != "submitted" {
		t.Errorf("status %q (%v): with an essay to grade the paper is not graded yet", status, err)
	}
}

//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"fmt"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/platform/db"
	"slices"
	"strings"
	"testing"
)

var everyPolicy = []struct{ score, correct, explanations bool }{
	{false, false, false}, {true, false, false}, {false, true, false}, {false, false, true},
	{true, true, false}, {true, false, true}, {false, true, true}, {true, true, true},
}

func TestEachNewResultFieldFollowsItsOwnFlagUnderEveryPolicy(t *testing.T) {
	pool := newPool(t)
	w := seedTypedWorld(t, pool)
	session := startTyped(t, pool, w)
	svc := newService(t, pool)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE app.test_version_questions SET skill = 'reading' WHERE id = $1::uuid`, w.tfng); err != nil {
		t.Fatal(err)
	}
	var criterion string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM app.test_version_rubric_criteria WHERE test_version_question_id = $1::uuid`, w.essay).Scan(&criterion); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.Save.Handle(ctx, command.Save{Input: domain.SaveInput{
		AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
		Answers: []domain.Answer{
			{QuestionID: w.essay, Payload: []byte(`{"type":"text","value":"An essay."}`)},
			{QuestionID: w.matching, Payload: []byte(fmt.Sprintf(`{"type":"matching","pairs":{%q:%q}}`, w.items[0], w.choices[2]))},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.Submit.Handle(ctx, command.Submit{AttemptID: session.Attempt.ID, StudentID: w.student, Reason: domain.Manual}); err != nil {
		t.Fatal(err)
	}
	if _, err := repositories.NewReviews(db.NewContext(pool)).Grade(ctx, everyone, session.Attempt.ID, domain.Request{ActorID: w.admin},
		[]domain.GradeItem{{QuestionID: w.essay, Criteria: []domain.CriterionBand{{CriterionID: criterion, Band: 7}}}}); err != nil {
		t.Fatal(err)
	}
	world := w.world
	matches := []domain.MatchPair{{ItemID: w.items[0], OptionID: w.choices[2]}, {ItemID: w.items[1], OptionID: w.choices[0]}}

	for _, tc := range everyPolicy {
		setReview(t, pool, world, tc.score, tc.correct, tc.explanations)
		result, err := svc.Queries.Result.Handle(ctx, query.Result{AttemptID: session.Attempt.ID, StudentID: w.student})
		if err != nil {
			t.Fatalf("%+v: %v", tc, err)
		}
		qs := byID(result)
		matching, essay, tfng := qs[w.matching], qs[w.essay], qs[w.tfng]

		if got := matching.CorrectMatches; (got != nil) != tc.correct || (tc.correct && !slices.Equal(got, matches)) {
			t.Errorf("%+v: correctMatches %v", tc, got)
		}
		if (essay.Band != nil) != tc.score || (tc.score && *essay.Band != 7) {
			t.Errorf("%+v: essay band %v", tc, essay.Band)
		}
		want := []domain.ResultCriterion{{ID: criterion, Label: "TIEU-CHI-AN", Band: 7}}
		if (essay.Criteria != nil) != tc.score || (tc.score && !slices.Equal(essay.Criteria, want)) {
			t.Errorf("%+v: essay criteria %v", tc, essay.Criteria)
		}
		if tfng.Skill == nil || *tfng.Skill != "reading" {
			t.Errorf("%+v: skill %v, want reading under every policy", tc, tfng.Skill)
		}
		if result.Score != nil && result.Score.Band != nil {
			t.Errorf("%+v: a mixed paper has band %v", tc, *result.Score.Band)
		}
		if !tc.score {
			body, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), "TIEU-CHI-AN") || strings.Contains(string(body), `"Band":7`) {
				t.Errorf("%+v: the withheld score left a band or a rubric in %s", tc, body)
			}
		}
	}
}

func TestAnAllEssayPaperShowsItsBandOnlyWithTheScore(t *testing.T) {
	pool := newPool(t)
	p := seedEssayPaper(t, pool)
	ctx := context.Background()
	if _, err := repositories.NewReviews(db.NewContext(pool)).Grade(ctx, everyone, p.attempt, domain.Request{ActorID: p.admin}, []domain.GradeItem{
		{QuestionID: p.plain, Band: pts(7)},
		{QuestionID: p.rubric, Criteria: p.bands(6, 6.5, 6.5, 6)},
	}); err != nil {
		t.Fatal(err)
	}
	var assignment string
	if err := pool.QueryRow(ctx, `SELECT assignment_id::text FROM app.attempts WHERE id = $1::uuid`, p.attempt).Scan(&assignment); err != nil {
		t.Fatal(err)
	}
	svc := newService(t, pool)
	for _, tc := range everyPolicy {
		setReview(t, pool, world{assignment: assignment}, tc.score, tc.correct, tc.explanations)
		result, err := svc.Queries.Result.Handle(ctx, query.Result{AttemptID: p.attempt, StudentID: p.student})
		if err != nil {
			t.Fatalf("%+v: %v", tc, err)
		}
		switch {
		case !tc.score && result.Score != nil:
			t.Errorf("%+v: score %+v while withheld", tc, result.Score)
		case tc.score && (result.Score == nil || !is(result.Score.Band, 7)):
			t.Errorf("%+v: score %+v, want band 7.0", tc, result.Score)
		}
		if rubric := byID(result)[p.rubric]; (rubric.Criteria != nil) != tc.score || (tc.score && len(rubric.Criteria) != 4) {
			t.Errorf("%+v: rubric criteria %+v", tc, rubric.Criteria)
		}
	}
	setReview(t, pool, world{assignment: assignment}, true, true, true)
	for _, phase := range []struct {
		name  string
		set   string
		shown bool
	}{
		{"before an after-close release", `review_release = 'after_close', closes_at = now() + interval '1 hour', closed_at = NULL`, false},
		{"once the close has passed", `review_release = 'after_close', closed_at = now() - interval '1 minute'`, true},
	} {
		if _, err := pool.Exec(ctx, `UPDATE app.assignments SET `+phase.set+` WHERE id = $1::uuid`, assignment); err != nil {
			t.Fatal(err)
		}
		result, err := svc.Queries.Result.Handle(ctx, query.Result{AttemptID: p.attempt, StudentID: p.student})
		if err != nil {
			t.Fatalf("%s: %v", phase.name, err)
		}
		rubric, plain := byID(result)[p.rubric], byID(result)[p.plain]
		got := result.Score != nil && is(result.Score.Band, 7) && is(plain.Band, 7) && len(rubric.Criteria) == 4
		hidden := result.Score == nil && plain.Band == nil && rubric.Band == nil && rubric.Criteria == nil
		if (phase.shown && !got) || (!phase.shown && !hidden) {
			t.Errorf("%s: score %+v, plain band %v, rubric band %v criteria %+v", phase.name, result.Score, plain.Band, rubric.Band, rubric.Criteria)
		}
	}
}

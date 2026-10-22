//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/platform/db"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// essayPaper is seedPaper with both questions made essays: rubric (3 points)
// has four criteria and plain (6 points) none.
type essayPaper struct {
	paper
	rubric, plain string
	criteria      []string
}

func seedEssayPaper(t *testing.T, pool *pgxpool.Pool) essayPaper {
	t.Helper()
	ctx := context.Background()
	p := essayPaper{paper: seedPaper(t, pool, "submitted")}
	p.rubric, p.plain = p.choice, p.essay
	for _, q := range []string{
		`DELETE FROM app.test_version_options WHERE test_version_question_id = $1::uuid`,
		`UPDATE app.test_version_questions SET type = 'essay', points = '3.00' WHERE id = $1::uuid`,
		`UPDATE app.attempt_answers SET requires_manual = true, auto_score = NULL, payload = '{"type":"text","value":"An essay."}' WHERE question_id = $1::uuid`,
	} {
		if _, err := pool.Exec(ctx, q, p.rubric); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE app.test_version_questions SET type = 'essay', points = '6.00' WHERE id = $1::uuid`, p.plain); err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO app.test_version_rubric_criteria (test_version_question_id, ordinal, label)
			VALUES ($1::uuid, $2, 'Tiêu chí') RETURNING id::text`, p.rubric, i).Scan(&id); err != nil {
			t.Fatal(err)
		}
		p.criteria = append(p.criteria, id)
	}
	return p
}

func (p essayPaper) bands(bands ...float64) []domain.CriterionBand {
	out := make([]domain.CriterionBand, len(bands))
	for i, b := range bands {
		out[i] = domain.CriterionBand{CriterionID: p.criteria[i], Band: b}
	}
	return out
}

type storedMark struct {
	manual, band *float64
	criteria     []float64
}

func markOf(t *testing.T, pool *pgxpool.Pool, attempt, question string) storedMark {
	t.Helper()
	var m storedMark
	if err := pool.QueryRow(context.Background(), `
		SELECT a.manual_score, a.band,
		       coalesce((SELECT array_agg(c.band ORDER BY r.ordinal)
		                   FROM app.attempt_answer_criteria c
		                   JOIN app.test_version_rubric_criteria r ON r.id = c.criterion_id
		                  WHERE c.attempt_id = a.attempt_id AND c.question_id = a.question_id), '{}')
		  FROM app.attempt_answers a WHERE a.attempt_id = $1::uuid AND a.question_id = $2::uuid`,
		attempt, question).Scan(&m.manual, &m.band, &m.criteria); err != nil {
		t.Fatal(err)
	}
	return m
}

func is(got *float64, want float64) bool { return got != nil && *got == want }

func TestAnEssayIsGradedOnItsBandOrItsCriteriaAndThePaperOnTheirWeightedMean(t *testing.T) {
	pool := newPool(t)
	p := seedEssayPaper(t, pool)
	store := repositories.NewReviews(db.NewContext(pool))
	ctx := context.Background()
	grader := domain.Request{ActorID: p.admin}

	score, err := store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{{QuestionID: p.plain, Band: pts(7)}})
	if err != nil {
		t.Fatal(err)
	}
	if got := markOf(t, pool, p.attempt, p.plain); !is(got.manual, 4.67) || !is(got.band, 7) || len(got.criteria) != 0 {
		t.Errorf("band 7 of 6 points: %+v, want 4.67 at band 7", got)
	}
	if score.Band != nil || score.PendingManual != 1 {
		t.Errorf("with one essay ungraded the paper has band %v (%d pending); it has none yet", score.Band, score.PendingManual)
	}

	score, err = store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{{QuestionID: p.rubric, Criteria: p.bands(6, 6.5, 6.5, 6)}})
	if err != nil {
		t.Fatal(err)
	}
	if got := markOf(t, pool, p.attempt, p.rubric); !is(got.manual, 2.17) || !is(got.band, 6.5) || !slices.Equal(got.criteria, []float64{6, 6.5, 6.5, 6}) {
		t.Errorf("criteria with mean 6.25 of 3 points: %+v, want 2.17 at band 6.5", got)
	}
	if !is(score.Band, 7) || score.Earned != 6.84 || score.PendingManual != 0 {
		t.Errorf("score %+v: (7 × 6 + 6.5 × 3) ÷ 9 = 6.83 rounds to band 7.0, earned 4.67 + 2.17", score)
	}

	score, err = store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{{QuestionID: p.rubric, Criteria: p.bands(3, 3, 3, 3.5)}})
	if err != nil {
		t.Fatal(err)
	}
	if got := markOf(t, pool, p.attempt, p.rubric); !is(got.manual, 1) || !is(got.band, 3) || !slices.Equal(got.criteria, []float64{3, 3, 3, 3.5}) {
		t.Errorf("a regrade with mean 3.125: %+v, want its criteria to replace the old ones at band 3.0", got)
	}
	if !is(score.Band, 5.5) {
		t.Errorf("paper band %v: (7 × 6 + 3 × 3) ÷ 9 = 5.67 rounds to 5.5", score.Band)
	}
}

func TestAGradeThatDoesNotFitItsEssayIsRefusedAndSavesNothing(t *testing.T) {
	pool := newPool(t)
	p := seedEssayPaper(t, pool)
	store := repositories.NewReviews(db.NewContext(pool))
	ctx := context.Background()
	grader := domain.Request{ActorID: p.admin}
	if _, err := store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{{QuestionID: p.rubric, Criteria: p.bands(6, 6, 6, 6)}}); err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		items []domain.GradeItem
		shape domain.GradeShape
		ids   []string
	}{
		"three of four criteria":    {[]domain.GradeItem{{QuestionID: p.plain, Band: pts(8)}, {QuestionID: p.rubric, Criteria: p.bands(7, 7, 7)}}, domain.CriteriaIncomplete, []string{p.rubric}},
		"another essay's criterion": {[]domain.GradeItem{{QuestionID: p.rubric, Criteria: append(p.bands(7, 7, 7), domain.CriterionBand{CriterionID: uuid.NewString(), Band: 7})}}, domain.CriteriaIncomplete, []string{p.rubric}},
		"points on an essay":        {[]domain.GradeItem{{QuestionID: p.plain, Points: pts(5)}}, domain.NeedsBand, []string{p.plain}},
		"a band on a rubric":        {[]domain.GradeItem{{QuestionID: p.rubric, Band: pts(7)}}, domain.CriteriaIncomplete, []string{p.rubric}},
	} {
		_, err := store.Grade(ctx, everyone, p.attempt, grader, tc.items)
		var shaped *domain.GradeShapeError
		if !errors.As(err, &shaped) || shaped.Shape != tc.shape || !slices.Equal(shaped.QuestionIDs, tc.ids) {
			t.Errorf("%s: %v, want %s naming %v", name, err, tc.shape, tc.ids)
		}
	}
	if got := markOf(t, pool, p.attempt, p.rubric); !is(got.band, 6) || !slices.Equal(got.criteria, []float64{6, 6, 6, 6}) {
		t.Errorf("a refused grade changed the rubric essay: %+v", got)
	}
	if got := markOf(t, pool, p.attempt, p.plain); got.manual != nil || got.band != nil {
		t.Errorf("a refused grade marked the plain essay: %+v", got)
	}
}

func TestTheShapeOfAGradeIsJudgedOnlyOnceTheAttemptIsFound(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	store := repositories.NewReviews(db.NewContext(pool))
	ctx := context.Background()
	if _, err := store.Grade(ctx, everyone, uuid.NewString(), domain.Request{ActorID: p.admin}, []domain.GradeItem{{QuestionID: p.essay, Band: pts(6)}}); !errors.Is(err, domain.ErrPaperNotFound) {
		t.Errorf("a band on a missing attempt: %v, want not found", err)
	}
	live := seedPaper(t, pool, "in_progress")
	if _, err := store.Grade(ctx, everyone, live.attempt, domain.Request{ActorID: live.admin}, []domain.GradeItem{{QuestionID: live.essay}}); !errors.Is(err, domain.ErrPaperInProgress) {
		t.Errorf("no points on a live attempt: %v, want in progress", err)
	}
	var shaped *domain.GradeShapeError
	_, err := store.Grade(ctx, everyone, p.attempt, domain.Request{ActorID: p.admin}, []domain.GradeItem{{QuestionID: p.essay}, {QuestionID: p.choice, Points: pts(1), Band: pts(6)}})
	if !errors.As(err, &shaped) || shaped.Shape != domain.BandNotAllowed || !slices.Equal(shaped.QuestionIDs, []string{p.choice}) {
		t.Errorf("a band beside missing points: %v, want the band refused first", err)
	}
}

func TestAMixedPaperHasNoBand(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE app.test_version_questions SET type = 'essay' WHERE id = $1::uuid`, p.essay); err != nil {
		t.Fatal(err)
	}
	score, err := repositories.NewReviews(db.NewContext(pool)).Grade(ctx, everyone, p.attempt, domain.Request{ActorID: p.admin}, []domain.GradeItem{{QuestionID: p.essay, Band: pts(9)}})
	if err != nil {
		t.Fatal(err)
	}
	if score.Band != nil || score.Earned != 10 {
		t.Errorf("an essay at band 9 beside a choice question: %+v, want 10 points and no band", score)
	}
}

type auditChange[T any] struct{ Old, New T }

type auditedMark struct {
	ManualScore *auditChange[*float64]           `json:"manualScore"`
	Band        *auditChange[*float64]           `json:"band"`
	Criteria    *auditChange[map[string]float64] `json:"criteria"`
}

func gradeEntries(t *testing.T, pool *pgxpool.Pool, attempt string) []map[string]auditedMark {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT diff FROM app.audit_log WHERE action = 'attempt.graded' AND entity = 'attempt' AND entity_id = $1::uuid ORDER BY id`, attempt)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]auditedMark
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "RIENG-TU") || strings.Contains(string(raw), "manual_score") {
			t.Errorf("entry %s holds the comment or a snake_case key", raw)
		}
		var diff struct {
			Questions map[string]auditedMark `json:"questions"`
		}
		if err := json.Unmarshal(raw, &diff); err != nil {
			t.Fatal(err)
		}
		out = append(out, diff.Questions)
	}
	return out
}

func TestAGradeIsAuditedWithWhatItChanged(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	store := repositories.NewReviews(db.NewContext(pool))
	t.Run("points", func(t *testing.T) {
		p := seedPaper(t, pool, "submitted")
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM app.audit_log WHERE entity_id = $1::uuid`, p.attempt)
		})
		grader := domain.Request{ActorID: p.admin, IP: "203.0.113.9", UserAgent: "grader-test"}
		if _, err := store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{{QuestionID: p.essay, Points: pts(4), Comment: comment("RIENG-TU")}}); err != nil {
			t.Fatal(err)
		}
		var actor, ip, agent string
		if err := pool.QueryRow(ctx, `SELECT actor_user_id::text, host(ip), user_agent FROM app.audit_log WHERE action = 'attempt.graded' AND entity_id = $1::uuid`,
			p.attempt).Scan(&actor, &ip, &agent); err != nil {
			t.Fatal(err)
		}
		if actor != p.admin || ip != "203.0.113.9" || agent != "grader-test" {
			t.Errorf("entry by %s from %s %s", actor, ip, agent)
		}
		entries := gradeEntries(t, pool, p.attempt)
		if mark := entries[0][p.essay]; len(entries[0]) != 1 || mark.ManualScore.Old != nil || !is(mark.ManualScore.New, 4) || mark.Band != nil || mark.Criteria != nil {
			t.Errorf("points entry %+v, want only the short answer's score nothing → 4", entries[0])
		}
		for name, items := range map[string][]domain.GradeItem{
			"the same mark again":      {{QuestionID: p.essay, Points: pts(4)}},
			"only the comment changes": {{QuestionID: p.essay, Points: pts(4), Comment: comment("RIENG-TU, sửa lại")}},
		} {
			if _, err := store.Grade(ctx, everyone, p.attempt, grader, items); err != nil {
				t.Fatal(err)
			}
			if n := len(gradeEntries(t, pool, p.attempt)); n != 1 {
				t.Errorf("%s wrote an entry: %d entries", name, n)
			}
		}
		if _, err := store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{{QuestionID: p.essay, Points: pts(4)}, {QuestionID: p.choice, Points: pts(3)}}); err != nil {
			t.Fatal(err)
		}
		entries = gradeEntries(t, pool, p.attempt)
		if last := entries[len(entries)-1]; len(last) != 1 || last[p.choice].ManualScore.Old != nil || !is(last[p.choice].ManualScore.New, 3) {
			t.Errorf("the entry names %+v, want only the choice it changed", last)
		}
	})
	t.Run("bands and criteria", func(t *testing.T) {
		p := seedEssayPaper(t, pool)
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM app.audit_log WHERE entity_id = $1::uuid`, p.attempt)
		})
		grader := domain.Request{ActorID: p.admin}
		if _, err := store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{
			{QuestionID: p.rubric, Criteria: p.bands(6, 6, 6, 6)},
			{QuestionID: p.plain, Band: pts(6.5)},
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{
			{QuestionID: p.rubric, Criteria: p.bands(7, 7, 7, 8), Comment: comment("RIENG-TU")},
			{QuestionID: p.plain, Band: pts(6.5)},
		}); err != nil {
			t.Fatal(err)
		}
		entries := gradeEntries(t, pool, p.attempt)
		if len(entries) != 2 {
			t.Fatalf("%d entries, want one per grade that changed a mark", len(entries))
		}
		first, regrade := entries[0], entries[1]
		if plain := first[p.plain]; plain.ManualScore.Old != nil || !is(plain.ManualScore.New, 4.33) || plain.Band.Old != nil || !is(plain.Band.New, 6.5) || plain.Criteria != nil {
			t.Errorf("plain essay %+v, want nothing → 4.33 at band 6.5 and no criteria", plain)
		}
		if rubric := first[p.rubric]; rubric.Criteria == nil || len(rubric.Criteria.Old) != 0 || rubric.Criteria.New[p.criteria[0]] != 6 || !is(rubric.Band.New, 6) {
			t.Errorf("first rubric grade %+v", rubric)
		}
		if _, present := regrade[p.plain]; present || len(regrade) != 1 {
			t.Errorf("the regrade names %+v; the plain essay did not change", regrade)
		}
		rubric := regrade[p.rubric]
		if !is(rubric.ManualScore.Old, 2) || !is(rubric.ManualScore.New, 2.5) || !is(rubric.Band.Old, 6) || !is(rubric.Band.New, 7.5) {
			t.Errorf("rubric regrade %+v, want 2 → 2.5 and band 6 → 7.5", rubric)
		}
		if rubric.Criteria.Old[p.criteria[3]] != 6 || rubric.Criteria.New[p.criteria[3]] != 8 || len(rubric.Criteria.New) != 4 {
			t.Errorf("rubric criteria %+v", rubric.Criteria)
		}
	})
}

func TestTheTeacherReviewCarriesBandsCriteriaAndThePaperBand(t *testing.T) {
	pool := newPool(t)
	p := seedEssayPaper(t, pool)
	store := repositories.NewReviews(db.NewContext(pool))
	ctx := context.Background()
	if _, err := store.Grade(ctx, everyone, p.attempt, domain.Request{ActorID: p.admin}, []domain.GradeItem{
		{QuestionID: p.rubric, Criteria: p.bands(7, 6.5, 6, 6.5)},
		{QuestionID: p.plain, Band: pts(5.5)},
	}); err != nil {
		t.Fatal(err)
	}

	rv, err := store.Get(ctx, everyone, p.attempt)
	if err != nil {
		t.Fatal(err)
	}
	if !is(rv.Score.Band, 6) {
		t.Errorf("review score %+v: (6.5 × 3 + 5.5 × 6) ÷ 9 = 5.83 rounds to band 6.0", rv.Score)
	}
	rubric := rv.Answers[p.rubric]
	if !is(rubric.Band, 6.5) || !slices.Equal(rubric.Criteria, p.bands(7, 6.5, 6, 6.5)) {
		t.Errorf("rubric answer %+v, want band 6.5 and the criteria in rubric order", rubric)
	}
	if plain := rv.Answers[p.plain]; !is(plain.Band, 5.5) || plain.Criteria != nil {
		t.Errorf("plain answer %+v", plain)
	}
	for _, q := range rv.Questions {
		if q.ID == p.rubric && (len(q.Rubric) != 4 || q.Rubric[0].ID != p.criteria[0] || q.Rubric[3].Ordinal != 3) {
			t.Errorf("rubric question carries %+v", q.Rubric)
		}
		if q.ID == p.plain && len(q.Rubric) != 0 {
			t.Errorf("plain essay carries a rubric %+v", q.Rubric)
		}
	}

	var assignment string
	if err := pool.QueryRow(ctx, `SELECT assignment_id::text FROM app.attempts WHERE id = $1::uuid`, p.attempt).Scan(&assignment); err != nil {
		t.Fatal(err)
	}
	byQ, err := store.AnswersForQuestion(ctx, everyone, assignment, p.rubric)
	if err != nil {
		t.Fatal(err)
	}
	if len(byQ.Items) != 1 || !is(byQ.Items[0].Band, 6.5) || !slices.Equal(byQ.Items[0].Criteria, p.bands(7, 6.5, 6, 6.5)) {
		t.Errorf("by-question rows %+v", byQ.Items)
	}
	if !slices.Equal(byQ.ManualIDs, []string{p.rubric, p.plain}) {
		t.Errorf("manual questions %v, want both essays", byQ.ManualIDs)
	}
	if len(byQ.Question.Rubric) != 4 {
		t.Errorf("by-question question carries rubric %+v", byQ.Question.Rubric)
	}
}

func TestTheTeacherReviewCarriesTheRulesAndKeysOfTheNewTypes(t *testing.T) {
	pool := newPool(t)
	w := seedTypedWorld(t, pool)
	session := startTyped(t, pool, w)
	rv, err := repositories.NewReviews(db.NewContext(pool)).Get(context.Background(), everyone, session.Attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]domain.ReviewQuestion{}
	for _, q := range rv.Questions {
		byID[q.ID] = q
	}
	if got := byID[w.chooseTwo].SelectCount; got == nil || *got != 2 {
		t.Errorf("selectCount %v", got)
	}
	if got := byID[w.limited].MaxWords; got == nil || *got != 3 {
		t.Errorf("maxWords %v", got)
	}
	matching := byID[w.matching]
	if matching.MatchAllowReuse == nil || *matching.MatchAllowReuse {
		t.Errorf("matchAllowReuse %v", matching.MatchAllowReuse)
	}
	want := []domain.ReviewMatchItem{
		{ID: w.items[0], Key: "A", Text: "item 0", CorrectOptionID: w.choices[2], Ordinal: 0},
		{ID: w.items[1], Key: "B", Text: "item 1", CorrectOptionID: w.choices[0], Ordinal: 1},
	}
	if !slices.Equal(matching.MatchItems, want) {
		t.Errorf("match items %+v, want %+v", matching.MatchItems, want)
	}
	var keys []string
	for _, o := range matching.Options {
		keys = append(keys, o.Key)
	}
	if !slices.Equal(keys, []string{"i", "ii", "iii"}) {
		t.Errorf("matching choices keyed %v", keys)
	}
	for _, o := range byID[w.tfng].Options {
		if o.Key != "" {
			t.Errorf("a tfng option has key %q", o.Key)
		}
	}
	if rubric := byID[w.essay].Rubric; len(rubric) != 1 || rubric[0].Label != "TIEU-CHI-AN" {
		t.Errorf("essay rubric %+v", rubric)
	}
}

package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/retention"

	"github.com/jackc/pgx/v5"
)

type gradingSlot struct {
	manual   *float64
	band     *float64
	kind     string
	rubric   []string
	criteria map[string]float64
	ceiling  float64
	answered bool
	removed  bool
}

type gradingPaper map[string]*gradingSlot

func readGradingPaper(ctx context.Context, tx pgx.Tx, attemptID, versionID string) (gradingPaper, error) {
	rows, err := tx.Query(ctx, `
		SELECT q.id::text, q.type, q.points, (aa.attempt_id IS NOT NULL),
		       coalesce(aa.payload = '`+retention.RemovedAnswer+`'::jsonb, false), aa.manual_score, aa.band,
		       coalesce((SELECT array_agg(c.id::text ORDER BY c.ordinal)
		                   FROM app.test_version_rubric_criteria c
		                  WHERE c.test_version_question_id = q.id), '{}')
		  FROM app.test_version_questions q
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		  LEFT JOIN app.attempt_answers aa
		    ON aa.attempt_id = $2::uuid AND aa.question_id = q.id
		 WHERE s.test_version_id = $1::uuid`, versionID, attemptID)
	if err != nil {
		return nil, fmt.Errorf("review: read paper for grading: %w", err)
	}
	defer rows.Close()

	paper := gradingPaper{}
	for rows.Next() {
		var id string
		sl := gradingSlot{criteria: map[string]float64{}}
		if err := rows.Scan(&id, &sl.kind, &sl.ceiling, &sl.answered, &sl.removed, &sl.manual, &sl.band, &sl.rubric); err != nil {
			return nil, fmt.Errorf("review: scan paper: %w", err)
		}
		paper[id] = &sl
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("review: read paper for grading: %w", err)
	}

	given, err := tx.Query(ctx, `
		SELECT question_id::text, criterion_id::text, band
		  FROM app.attempt_answer_criteria WHERE attempt_id = $1::uuid`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("review: read criteria: %w", err)
	}
	defer given.Close()
	for given.Next() {
		var question, criterion string
		var band float64
		if err := given.Scan(&question, &criterion, &band); err != nil {
			return nil, fmt.Errorf("review: scan criteria: %w", err)
		}
		if sl, ok := paper[question]; ok {
			sl.criteria[criterion] = band
		}
	}
	if err := given.Err(); err != nil {
		return nil, fmt.Errorf("review: read criteria: %w", err)
	}
	return paper, nil
}

func (p gradingPaper) check(items []domain.GradeItem) error {
	if removed := p.removed(items); len(removed) > 0 {
		return &domain.AnswerRemovedError{QuestionIDs: removed}
	}
	if err := p.misshapen(items); err != nil {
		return err
	}
	if invalid := p.invalid(items); len(invalid) > 0 {
		return &domain.GradeValidationError{Items: invalid}
	}
	return nil
}

func (p gradingPaper) removed(items []domain.GradeItem) []string {
	var removed []string
	for _, it := range items {
		if sl, ok := p[it.QuestionID]; ok && sl.removed {
			removed = append(removed, it.QuestionID)
		}
	}
	return removed
}

func (p gradingPaper) misshapen(items []domain.GradeItem) error {
	shaped := map[domain.GradeShape][]string{}
	for _, it := range items {
		if sl, ok := p[it.QuestionID]; ok {
			if shape := domain.Grading.ShapeOf(sl.kind, sl.rubric, it); shape != "" {
				shaped[shape] = append(shaped[shape], it.QuestionID)
			}
		}
	}
	for _, shape := range domain.GradeShapes {
		if ids := shaped[shape]; len(ids) > 0 {
			return &domain.GradeShapeError{Shape: shape, QuestionIDs: ids}
		}
	}
	return nil
}

func (p gradingPaper) invalid(items []domain.GradeItem) []domain.GradeItemError {
	var invalid []domain.GradeItemError
	for _, it := range items {
		sl, ok := p[it.QuestionID]
		switch {
		case !ok:
			invalid = append(invalid, domain.GradeItemError{QuestionID: it.QuestionID, Reason: "not_on_paper"})
		case !sl.answered:
			invalid = append(invalid, domain.GradeItemError{QuestionID: it.QuestionID, Reason: "unanswered"})
		case it.Points != nil && *it.Points > sl.ceiling:
			invalid = append(invalid, domain.GradeItemError{QuestionID: it.QuestionID, Reason: "above_ceiling"})
		}
	}
	return invalid
}

type change[T any] struct {
	Old T `json:"old"`
	New T `json:"new"`
}

type markChange struct {
	Band        *change[*float64]           `json:"band,omitempty"`
	Criteria    *change[map[string]float64] `json:"criteria,omitempty"`
	ManualScore change[*float64]            `json:"manual_score"`
}

func (p gradingPaper) diff(items []domain.GradeItem, marked map[string]float64) []byte {
	questions := make(map[string]markChange, len(items))
	for _, it := range items {
		sl := p[it.QuestionID]
		score := marked[it.QuestionID]
		mc := markChange{ManualScore: change[*float64]{Old: sl.manual, New: &score}}
		if sl.kind == domain.Essay {
			mc.Band = &change[*float64]{Old: sl.band, New: domain.Grading.BandOf(it)}
			given := map[string]float64{}
			for _, c := range it.Criteria {
				given[c.CriterionID] = c.Band
			}
			mc.Criteria = &change[map[string]float64]{Old: sl.criteria, New: given}
		}
		questions[it.QuestionID] = mc
	}
	out, _ := json.Marshal(map[string]any{"questions": questions})
	return out
}

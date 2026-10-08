package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/schedule"
	"time"

	"github.com/jackc/pgx/v5"
)

type resultRules struct {
	domain.Rules
	Review      domain.ReviewPolicy
	TestTitle   string
	MaxAttempts int
}

// LoadResult reads a result as the student may read it at now. An assignment
// that releases after close withholds, until its close, what the review policy
// would hide: the score, the marks, the key, the explanations and the grader's
// comments, none of which is selected, and it says when the withholding ends.
func (s *Postgres) LoadResult(ctx context.Context, a domain.AttemptRecord, now time.Time) (domain.Result, error) {
	rules, err := s.resultRules(ctx, a.AssignmentID)
	if err != nil {
		return domain.Result{}, err
	}
	closes := schedule.Close(rules.ClosesAt, rules.ClosedAt, nil)
	withheld := domain.Reviews.Withheld(rules.Review.Release, now, closes)
	stored := rules.Review
	rules.Review = domain.Reviews.Effective(stored, withheld)
	sections, err := s.Sections(ctx, a.TestVersionID)
	if err != nil {
		return domain.Result{}, err
	}
	base, err := s.Questions(ctx, a.TestVersionID)
	if err != nil {
		return domain.Result{}, err
	}
	version, err := s.DeliveryVersion(ctx, a.TestVersionID)
	if err != nil {
		return domain.Result{}, err
	}
	base, err = domain.Deal.PresentVersion(version, a.Seed, rules.ShuffleQuestions, rules.ShuffleOptions, sections, base)
	if err != nil {
		return domain.Result{}, err
	}

	extras, err := s.resultExtras(ctx, a.TestVersionID, rules.Review)
	if err != nil {
		return domain.Result{}, err
	}
	answers, err := s.gradedAnswers(ctx, a.ID)
	if err != nil {
		return domain.Result{}, err
	}
	plays, err := s.AudioPlays(ctx, a.ID)
	if err != nil {
		return domain.Result{}, err
	}

	out := domain.Result{
		Attempt: a.Attempt, Review: rules.Review,
		TestTitle: rules.TestTitle, MaxAttempts: rules.MaxAttempts,
		Sections:  sections,
		Questions: make([]domain.ResultQuestion, len(base)),
	}
	pending := 0
	earned := 0.0
	for i, q := range base {
		rq, value := resultQuestion(q, extras, plays, answers, rules.Review.ShowScore, withheld)
		switch {
		case rq.PendingManual:
			pending++
		case value != nil:
			earned += *value
		}
		out.Questions[i] = rq
	}
	if rules.Review.ShowScore {
		total, err := s.scoreTotal(ctx, a.ID)
		if err != nil {
			return domain.Result{}, err
		}
		out.Score = &domain.Score{Earned: earned, Total: total, PendingManual: pending}
	}
	if withheld {
		out.ReleasesAt = &closes
	}
	if out.ClassAverage, err = s.classAverageOf(ctx, a.AssignmentID, stored, withheld, !now.Before(closes)); err != nil {
		return domain.Result{}, err
	}
	return out, nil
}

func (s *Postgres) classAverageOf(ctx context.Context, assignmentID string, stored domain.ReviewPolicy, withheld, closed bool) (*float64, error) {
	if !stored.ShowClassAverage || withheld || !closed {
		return nil, nil
	}
	var qualifying int
	var mean *float64
	err := s.QueryRow(ctx, `
		WITH roster AS (
		    SELECT m.user_id
		      FROM app.assignment_classes ac
		      JOIN app.class_members m ON m.class_id = ac.class_id
		     WHERE ac.assignment_id = $1::uuid
		    UNION
		    SELECT ast.user_id FROM app.assignment_students ast
		     WHERE ast.assignment_id = $1::uuid
		), best AS (
		    SELECT at.student_id,
		           max(at.score_earned::float8 / at.score_total::float8) AS share
		      FROM app.attempts at
		      JOIN roster r ON r.user_id = at.student_id
		      JOIN app.users u ON u.id = at.student_id AND u.disabled_at IS NULL
		     WHERE at.assignment_id = $1::uuid AND at.status = 'graded'
		       AND at.score_earned IS NOT NULL AND at.score_total IS NOT NULL
		     GROUP BY at.student_id
		)
		SELECT count(share), round((avg(share) * 100)::numeric, 2)::float8 FROM best`,
		assignmentID).Scan(&qualifying, &mean)
	if err != nil {
		return nil, fmt.Errorf("attempts: read class average: %w", err)
	}
	if mean == nil || !domain.Reviews.ShowsAverage(stored, withheld, closed, qualifying) {
		return nil, nil
	}
	return mean, nil
}

// resultQuestion is one line of the paper as the student may see it. The
// second result is what the line adds to the score: nil while the answer is
// unmarked, or while the policy hides scores altogether.
func resultQuestion(q domain.Question, extras map[string]resultExtra, plays map[string]int,
	answers map[string]gradedAnswer, showScore, withheld bool) (domain.ResultQuestion, *float64) {
	rq := domain.ResultQuestion{Question: q}
	if ex, ok := extras[q.ID]; ok {
		rq.Explanation, rq.Transcript = ex.explanation, ex.transcript
		rq.ExplanationContent = ex.explanationContent
		rq.CorrectOptions, rq.CorrectAnswers = ex.correctOptions, ex.correctAnswers
	}
	if q.Audio != nil {
		used := plays[q.ID]
		rq.AudioPlaysUsed = &used
	}
	ans, answered := answers[q.ID]
	if answered {
		rq.Answer = ans.payload
		if !withheld {
			rq.GraderComment = ans.comment
		}
		rq.PendingManual = ans.requiresManual && ans.manual == nil
	}
	if rq.PendingManual || !showScore {
		return rq, nil
	}
	value := 0.0
	if answered && ans.final != nil {
		value = *ans.final
	}
	rq.Earned = &value
	return rq, &value
}

func (s *Postgres) resultRules(ctx context.Context, assignmentID string) (resultRules, error) {
	var r resultRules
	err := s.QueryRow(ctx, `
		SELECT a.shuffle_questions, a.shuffle_options,
		       a.review_show_score, a.review_show_correct_answers, a.review_show_explanations,
		       a.review_release, a.review_show_class_average,
		       a.closes_at, a.closed_at, a.max_attempts, t.title
		  FROM app.assignments a
		  JOIN app.tests t ON t.id = a.test_id
		 WHERE a.id = $1::uuid`, assignmentID).Scan(
		&r.ShuffleQuestions, &r.ShuffleOptions,
		&r.Review.ShowScore, &r.Review.ShowCorrectAnswers, &r.Review.ShowExplanations,
		&r.Review.Release, &r.Review.ShowClassAverage,
		&r.ClosesAt, &r.ClosedAt, &r.MaxAttempts, &r.TestTitle)
	if errors.Is(err, pgx.ErrNoRows) {
		return resultRules{}, domain.ErrNotFound
	}
	if err != nil {
		return resultRules{}, fmt.Errorf("attempts: read review policy: %w", err)
	}
	return r, nil
}

type resultExtra struct {
	explanation        *string
	explanationContent json.RawMessage
	transcript         *string
	correctOptions     []string
	correctAnswers     []domain.BlankAnswer
}

// resultExtras reads the released parts of the key. Each column is gated in
// SQL: when the policy is off the AttemptRecord carries NULL, not a value the Go side
// has to remember to drop (§13.5).
func (s *Postgres) resultExtras(ctx context.Context, versionID string, p domain.ReviewPolicy) (map[string]resultExtra, error) {
	rows, err := s.Query(ctx, `
		SELECT q.id::text,
		       CASE WHEN $2 THEN q.explanation END,
		       CASE WHEN $2 THEN q.explanation_content END,
		       CASE WHEN q.audio_show_transcript_after THEN q.transcript END,
		       CASE WHEN $3 THEN coalesce((SELECT array_agg(o.id::text ORDER BY o.ordinal)
		                                     FROM app.test_version_options o
		                                    WHERE o.test_version_question_id = q.id AND o.is_correct), '{}') END,
		       CASE WHEN $3 THEN coalesce((SELECT array_agg(b.id::text ORDER BY b.ordinal)
		                                     FROM app.test_version_blanks b
		                                    WHERE b.test_version_question_id = q.id), '{}') END,
		       CASE WHEN $3 THEN coalesce((SELECT array_agg(
		                                       (SELECT ba.answer FROM app.test_version_blank_answers ba
		                                         WHERE ba.test_version_blank_id = b.id ORDER BY ba.id LIMIT 1)
		                                       ORDER BY b.ordinal)
		                                     FROM app.test_version_blanks b
		                                    WHERE b.test_version_question_id = q.id), '{}') END
		  FROM app.test_version_questions q
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1::uuid`,
		versionID, p.ShowExplanations, p.ShowCorrectAnswers)
	if err != nil {
		return nil, fmt.Errorf("attempts: read released key: %w", err)
	}
	defer rows.Close()

	out := map[string]resultExtra{}
	for rows.Next() {
		var id string
		var ex resultExtra
		var blankIDs, blankAnswers []*string
		if err := rows.Scan(&id, &ex.explanation, &ex.explanationContent, &ex.transcript, &ex.correctOptions, &blankIDs, &blankAnswers); err != nil {
			return nil, fmt.Errorf("attempts: scan released key: %w", err)
		}
		for i, blankID := range blankIDs {
			if blankID == nil || i >= len(blankAnswers) || blankAnswers[i] == nil {
				continue
			}
			ex.correctAnswers = append(ex.correctAnswers, domain.BlankAnswer{BlankID: *blankID, Answer: *blankAnswers[i]})
		}
		out[id] = ex
	}
	return out, rows.Err()
}

type gradedAnswer struct {
	payload        []byte
	final          *float64
	manual         *float64
	requiresManual bool
	comment        *string
}

func (s *Postgres) gradedAnswers(ctx context.Context, attemptID string) (map[string]gradedAnswer, error) {
	rows, err := s.Query(ctx, `
		SELECT question_id::text, payload, final_score, manual_score, requires_manual, grader_comment
		  FROM app.attempt_answers WHERE attempt_id = $1::uuid`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("attempts: read graded answers: %w", err)
	}
	defer rows.Close()
	out := map[string]gradedAnswer{}
	for rows.Next() {
		var id string
		var g gradedAnswer
		if err := rows.Scan(&id, &g.payload, &g.final, &g.manual, &g.requiresManual, &g.comment); err != nil {
			return nil, fmt.Errorf("attempts: scan graded answer: %w", err)
		}
		out[id] = g
	}
	return out, rows.Err()
}

func (s *Postgres) scoreTotal(ctx context.Context, attemptID string) (float64, error) {
	var total *float64
	if err := s.QueryRow(ctx,
		`SELECT score_total FROM app.attempts WHERE id = $1::uuid`, attemptID).Scan(&total); err != nil {
		return 0, fmt.Errorf("attempts: read score total: %w", err)
	}
	if total == nil {
		return 0, nil
	}
	return *total, nil
}

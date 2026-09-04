package attempts

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var ErrAttemptInProgress = errors.New("attempts: attempt is still in progress")

// ReviewPolicy is what the assignment lets a student see afterwards (§9).
type ReviewPolicy struct {
	ShowScore          bool
	ShowCorrectAnswers bool
	ShowExplanations   bool
}

// BlankAnswer is one canonical accepted answer, never the full list.
type BlankAnswer struct {
	BlankID string
	Answer  string
}

// ResultQuestion is the post-submission view of one question. Every revealing
// field is nil unless the policy released it -- and the query that reads it
// never selected the column when it did not, so there is nothing to strip.
type ResultQuestion struct {
	Question
	Answer         []byte
	Earned         *float64
	PendingManual  bool
	GraderComment  *string
	CorrectOptions []string
	CorrectAnswers []BlankAnswer
	Explanation    *string
	Transcript     *string
	AudioPlaysUsed *int
}

type Result struct {
	Attempt     Attempt
	Score       *Score
	Review      ReviewPolicy
	TestTitle   string
	MaxAttempts int
	Questions   []ResultQuestion
}

// Result is §9's result page, in the order the student saw the paper.
func (s *Service) Result(ctx context.Context, attemptID, studentID string) (Result, error) {
	if err := s.store.ExpireIfDue(ctx, attemptID, s.now()); err != nil {
		return Result{}, err
	}
	a, err := s.store.ByID(ctx, attemptID, studentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Result{}, ErrForbidden
		}
		return Result{}, err
	}
	switch a.Status {
	case InProgress:
		return Result{}, ErrAttemptInProgress
	case Voided:
		return Result{}, ErrAttemptVoided
	}
	return s.store.result(ctx, a)
}

type resultRules struct {
	Rules
	Review      ReviewPolicy
	TestTitle   string
	MaxAttempts int
}

func (s *Store) result(ctx context.Context, a row) (Result, error) {
	rules, err := s.resultRules(ctx, a.AssignmentID)
	if err != nil {
		return Result{}, err
	}
	base, err := s.Questions(ctx, a.TestVersionID)
	if err != nil {
		return Result{}, err
	}
	base = present(a.Seed, rules.ShuffleQuestions, rules.ShuffleOptions, base)

	extras, err := s.resultExtras(ctx, a.TestVersionID, rules.Review)
	if err != nil {
		return Result{}, err
	}
	answers, err := s.gradedAnswers(ctx, a.ID)
	if err != nil {
		return Result{}, err
	}
	plays, err := s.AudioPlays(ctx, a.ID)
	if err != nil {
		return Result{}, err
	}

	out := Result{
		Attempt: a.Attempt, Review: rules.Review,
		TestTitle: rules.TestTitle, MaxAttempts: rules.MaxAttempts,
		Questions: make([]ResultQuestion, len(base)),
	}
	pending := 0
	earned := 0.0
	for i, q := range base {
		rq := ResultQuestion{Question: q}
		if ex, ok := extras[q.ID]; ok {
			rq.Explanation, rq.Transcript = ex.explanation, ex.transcript
			rq.CorrectOptions, rq.CorrectAnswers = ex.correctOptions, ex.correctAnswers
		}
		if q.Audio != nil {
			used := plays[q.ID]
			rq.AudioPlaysUsed = &used
		}
		if ans, ok := answers[q.ID]; ok {
			rq.Answer = ans.payload
			rq.GraderComment = ans.comment
			rq.PendingManual = ans.requiresManual && ans.manual == nil
		}
		if rq.PendingManual {
			pending++
		} else if rules.Review.ShowScore {
			value := 0.0
			if ans, ok := answers[q.ID]; ok && ans.final != nil {
				value = *ans.final
			}
			earned += value
			rq.Earned = &value
		}
		out.Questions[i] = rq
	}
	if rules.Review.ShowScore {
		total, err := s.scoreTotal(ctx, a.ID)
		if err != nil {
			return Result{}, err
		}
		out.Score = &Score{Earned: earned, Total: total, PendingManual: pending}
	}
	return out, nil
}

func (s *Store) resultRules(ctx context.Context, assignmentID string) (resultRules, error) {
	var r resultRules
	err := s.pool.QueryRow(ctx, `
		SELECT a.shuffle_questions, a.shuffle_options,
		       a.review_show_score, a.review_show_correct_answers, a.review_show_explanations,
		       a.max_attempts, t.title
		  FROM app.assignments a
		  JOIN app.tests t ON t.id = a.test_id
		 WHERE a.id = $1::uuid`, assignmentID).Scan(
		&r.ShuffleQuestions, &r.ShuffleOptions,
		&r.Review.ShowScore, &r.Review.ShowCorrectAnswers, &r.Review.ShowExplanations,
		&r.MaxAttempts, &r.TestTitle)
	if errors.Is(err, pgx.ErrNoRows) {
		return resultRules{}, ErrNotFound
	}
	if err != nil {
		return resultRules{}, fmt.Errorf("attempts: read review policy: %w", err)
	}
	return r, nil
}

type resultExtra struct {
	explanation    *string
	transcript     *string
	correctOptions []string
	correctAnswers []BlankAnswer
}

// resultExtras reads the released parts of the key. Each column is gated in
// SQL: when the policy is off the row carries NULL, not a value the Go side
// has to remember to drop (§13.5).
func (s *Store) resultExtras(ctx context.Context, versionID string, p ReviewPolicy) (map[string]resultExtra, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT q.id::text,
		       CASE WHEN $2 THEN q.explanation END,
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
		if err := rows.Scan(&id, &ex.explanation, &ex.transcript, &ex.correctOptions, &blankIDs, &blankAnswers); err != nil {
			return nil, fmt.Errorf("attempts: scan released key: %w", err)
		}
		for i, blankID := range blankIDs {
			if blankID == nil || i >= len(blankAnswers) || blankAnswers[i] == nil {
				continue
			}
			ex.correctAnswers = append(ex.correctAnswers, BlankAnswer{BlankID: *blankID, Answer: *blankAnswers[i]})
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

func (s *Store) gradedAnswers(ctx context.Context, attemptID string) (map[string]gradedAnswer, error) {
	rows, err := s.pool.Query(ctx, `
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

func (s *Store) scoreTotal(ctx context.Context, attemptID string) (float64, error) {
	var total *float64
	if err := s.pool.QueryRow(ctx,
		`SELECT score_total FROM app.attempts WHERE id = $1::uuid`, attemptID).Scan(&total); err != nil {
		return 0, fmt.Errorf("attempts: read score total: %w", err)
	}
	if total == nil {
		return 0, nil
	}
	return *total, nil
}

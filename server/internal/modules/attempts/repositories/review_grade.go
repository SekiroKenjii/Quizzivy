package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/answered"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/bands"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/visibility"
	"time"

	"github.com/jackc/pgx/v5"
)

// Grade writes manual marks for a closed attempt on an assignment the scope
// reaches, audits them as grader, and returns the live score. An essay's band
// is converted to manual_score = round(band × points ÷ 9, 2) here, and its
// criteria's bands replace the ones a previous grade wrote.
//
// It refuses, saving nothing, in this order: an answer removed under the
// retention policy; a mark that does not fit its question's type, which is
// known only once the attempt is found (R6C-5); a question off the paper,
// unanswered, or marked above its points. The audit entry (D-069) holds, per
// question it changed, the manual score, band and criteria that changed, as
// {old, new}, never the comment: that is the teacher's words, not a mark. A
// grade that changes no mark writes no entry.
func (s *Reviews) Grade(ctx context.Context, scope access.Scope, attemptID string, grader domain.Request, items []domain.GradeItem) (domain.Score, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Score{}, fmt.Errorf("review: begin grade: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	versionID, err := lockGradable(ctx, tx, scope, attemptID)
	if err != nil {
		return domain.Score{}, err
	}
	paper, err := readGradingPaper(ctx, tx, attemptID, versionID)
	if err != nil {
		return domain.Score{}, err
	}
	if err := paper.check(items); err != nil {
		return domain.Score{}, err
	}

	marked, err := writeMarks(ctx, tx, attemptID, grader.ActorID, s.now(), items)
	if err != nil {
		return domain.Score{}, err
	}
	if err := replaceCriteria(ctx, tx, attemptID, items); err != nil {
		return domain.Score{}, err
	}
	if diff := paper.diff(items, marked); diff != nil {
		if err := audit.Write(ctx, tx, audit.Entry{
			ActorUserID: opt.String(grader.ActorID), Action: "attempt.graded", Entity: "attempt", EntityID: &attemptID,
			OccurredAt: s.now(), IP: opt.String(grader.IP), UserAgent: opt.String(grader.UserAgent), Diff: diff,
		}); err != nil {
			return domain.Score{}, fmt.Errorf("review: %w", err)
		}
	}

	score, err := recomputeScore(ctx, tx, attemptID)
	if err != nil {
		return domain.Score{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Score{}, fmt.Errorf("review: commit grade: %w", err)
	}
	return score, nil
}

func writeMarks(ctx context.Context, tx pgx.Tx, attemptID, graderID string, now time.Time, items []domain.GradeItem) (map[string]float64, error) {
	ids := make([]string, len(items))
	points := make([]*float64, len(items))
	bands := make([]*float64, len(items))
	comments := make([]*string, len(items))
	for i, it := range items {
		ids[i] = it.QuestionID
		points[i] = it.Points
		bands[i] = domain.Grading.BandOf(it)
		comments[i] = it.Comment
	}
	rows, err := tx.Query(ctx, `
		UPDATE app.attempt_answers a
		   SET manual_score = coalesce(round(marked.band * q.points / 9, 2), marked.points),
		       band = marked.band, grader_comment = marked.comment,
		       graded_by = $2::uuid, graded_at = $3
		  FROM unnest($4::uuid[], $5::numeric[], $6::numeric[], $7::text[]) AS marked(question_id, points, band, comment)
		  JOIN app.test_version_questions q ON q.id = marked.question_id
		 WHERE a.attempt_id = $1::uuid AND a.question_id = marked.question_id
		RETURNING a.question_id::text, a.manual_score`,
		attemptID, graderID, now, ids, points, bands, comments)
	if err != nil {
		return nil, fmt.Errorf("review: write marks: %w", err)
	}
	marked := map[string]float64{}
	for rows.Next() {
		var id string
		var score float64
		if err := rows.Scan(&id, &score); err != nil {
			return nil, fmt.Errorf("review: scan marks: %w", err)
		}
		marked[id] = score
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("review: write marks: %w", err)
	}
	return marked, nil
}

func replaceCriteria(ctx context.Context, tx pgx.Tx, attemptID string, items []domain.GradeItem) error {
	var questions, banded, criteria []string
	var bands []float64
	for _, it := range items {
		questions = append(questions, it.QuestionID)
		for _, c := range it.Criteria {
			banded = append(banded, it.QuestionID)
			criteria = append(criteria, c.CriterionID)
			bands = append(bands, c.Band)
		}
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM app.attempt_answer_criteria
		 WHERE attempt_id = $1::uuid AND question_id = ANY($2::uuid[])`, attemptID, questions); err != nil {
		return fmt.Errorf("review: clear criteria: %w", err)
	}
	if len(criteria) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.attempt_answer_criteria (attempt_id, question_id, criterion_id, band)
		SELECT $1::uuid, c.question_id, c.criterion_id, c.band
		  FROM unnest($2::uuid[], $3::uuid[], $4::numeric[]) AS c(question_id, criterion_id, band)`,
		attemptID, banded, criteria, bands); err != nil {
		return fmt.Errorf("review: write criteria: %w", err)
	}
	return nil
}

// Finish declares a paper of an assignment the scope reaches graded.
// Re-enterable: a graded attempt can be marked again and finished again, and
// the score is recomputed each time.
func (s *Reviews) Finish(ctx context.Context, scope access.Scope, attemptID string) (domain.Attempt, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Attempt{}, fmt.Errorf("review: begin finish: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := lockGradable(ctx, tx, scope, attemptID); err != nil {
		return domain.Attempt{}, err
	}
	score, err := recomputeScore(ctx, tx, attemptID)
	if err != nil {
		return domain.Attempt{}, err
	}
	if score.PendingManual > 0 {
		return domain.Attempt{}, domain.ErrGradingIncomplete
	}

	var a domain.Attempt
	if err := tx.QueryRow(ctx, `
		UPDATE app.attempts
		   SET status = 'graded', graded_at = $2
		 WHERE id = $1::uuid
		RETURNING id::text, assignment_id::text, student_id::text, test_version_id::text,
		          attempt_no, status, started_at, deadline_at, submitted_at, graded_at,
		          focus_loss_count, flagged`, attemptID, s.now()).Scan(
		&a.ID, &a.AssignmentID, &a.StudentID, &a.TestVersionID,
		&a.AttemptNo, &a.Status, &a.StartedAt, &a.DeadlineAt, &a.SubmittedAt, &a.GradedAt,
		&a.FocusLossCount, &a.Flagged); err != nil {
		return domain.Attempt{}, fmt.Errorf("review: finish: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Attempt{}, fmt.Errorf("review: commit finish: %w", err)
	}
	return a, nil
}

func lockGradable(ctx context.Context, tx pgx.Tx, scope access.Scope, attemptID string) (string, error) {
	var status domain.Status
	var versionID string
	err := tx.QueryRow(ctx, `
		SELECT status, test_version_id::text FROM app.attempts
		 WHERE id = $1::uuid AND ($2::boolean OR assignment_id IN `+visibility.AssignmentIDs(3)+`) FOR UPDATE`,
		attemptID, scope.All, opt.String(scope.UserID)).Scan(&status, &versionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrPaperNotFound
	}
	if err != nil {
		return "", fmt.Errorf("review: lock attempt: %w", err)
	}
	switch status {
	case domain.InProgress:
		return "", domain.ErrPaperInProgress
	case domain.Voided:
		return "", domain.ErrPaperVoided
	}
	return versionID, nil
}

func recomputeScore(ctx context.Context, tx pgx.Tx, attemptID string) (domain.Score, error) {
	var out domain.Score
	var total *float64
	err := tx.QueryRow(ctx, `
		UPDATE app.attempts at
		   SET score_earned = (SELECT coalesce(sum(aa.final_score), 0)
		                         FROM app.attempt_answers aa WHERE aa.attempt_id = at.id)
		 WHERE at.id = $1::uuid
		RETURNING at.score_earned, at.score_total,
		          (SELECT count(*) FROM app.attempt_answers aa
		            WHERE aa.attempt_id = at.id AND `+answered.AwaitsGrade("aa")+`),
		          `+bands.OfPaper("at"),
		attemptID).Scan(&out.Earned, &total, &out.PendingManual, &out.Band)
	if err != nil {
		return domain.Score{}, fmt.Errorf("review: recompute score: %w", err)
	}
	if total != nil {
		out.Total = *total
	}
	return out, nil
}

package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/schedule"
	"quizzivy/internal/shared/visibility"
	"time"

	"github.com/jackc/pgx/v5"
)

var briefingQuery = `
	SELECT at.assignment_id::text, at.student_id::text, s.full_name, t.title, at.focus_loss_count,
	       EXISTS (SELECT 1 FROM app.attempt_answers ans
	                WHERE ans.attempt_id = at.id AND ans.requires_manual AND ans.manual_score IS NULL),
	       a.review_show_score
	         AND (a.review_release = 'on_submit' OR $2::timestamptz >= ` + schedule.CloseOf("o") + `),
	       coalesce((SELECT array_agg(r.id::text ORDER BY r.id)
	                   FROM app.users r
	                  WHERE r.disabled_at IS NULL
	                    AND r.id IN ` + visibility.PaperReaders("at.assignment_id", "at.student_id") + `), '{}')
	  FROM app.attempts at
	  JOIN app.assignments a ON a.id = at.assignment_id
	  JOIN app.tests t ON t.id = a.test_id
	  JOIN app.users s ON s.id = at.student_id
	  ` + schedule.OverrideJoin("at.student_id") + `
	 WHERE at.id = $1::uuid AND at.status <> 'voided'`

// Briefing reads what a notice about the attempt says that the attempt does
// not: the student's name, the test's title, whether an answer still waits
// for a mark, whether the review policy shows the student's score at now
// under the student's own close, and which enabled teachers reach the paper.
// A voided attempt, and one that does not exist, answer ErrNotFound.
func (s *Postgres) Briefing(ctx context.Context, attemptID string, now time.Time) (domain.Briefing, error) {
	var b domain.Briefing
	err := s.QueryRow(ctx, briefingQuery, attemptID, now).Scan(
		&b.AssignmentID, &b.StudentID, &b.StudentName, &b.Title, &b.FocusLost, &b.ToGrade, &b.ShowsResult, &b.Readers)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Briefing{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Briefing{}, fmt.Errorf("attempts: read briefing: %w", err)
	}
	return b, nil
}

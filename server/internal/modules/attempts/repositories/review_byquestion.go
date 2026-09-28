package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/visibility"

	"github.com/jackc/pgx/v5"
)

// AnswersForQuestion lists one manual question across the handed-in,
// non-voided attempts of an assignment the scope reaches that visibility.Papers
// shows it, in attempt order -- not by name, since the mode hides names until
// the question is graded.
func (s *Reviews) AnswersForQuestion(ctx context.Context, scope access.Scope, assignmentID, questionID string) (domain.ByQuestion, error) {
	var out domain.ByQuestion
	var versionID string
	err := s.QueryRow(ctx, `
		SELECT a.test_version_id::text, v.published_at
		  FROM app.assignments a
		  JOIN app.test_versions v ON v.id = a.test_version_id
		 WHERE a.id = $1::uuid AND ($2::boolean OR a.id IN `+visibility.AssignmentIDs(3)+`)`,
		assignmentID, scope.All, opt.String(scope.UserID)).Scan(&versionID, &out.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ByQuestion{}, domain.ErrPaperNotFound
	}
	if err != nil {
		return domain.ByQuestion{}, fmt.Errorf("review: read assignment: %w", err)
	}
	out.VersionID = versionID

	questions, err := s.questions(ctx, versionID)
	if err != nil {
		return domain.ByQuestion{}, err
	}
	out.Count = len(questions)
	for i, q := range questions {
		if q.Type == "short_answer" {
			out.ManualIDs = append(out.ManualIDs, q.ID)
		}
		if q.ID == questionID {
			out.Question = q
			out.Number = i + 1
		}
	}
	if out.Number == 0 {
		return domain.ByQuestion{}, domain.ErrQuestionNotOnPaper
	}

	rows, err := s.Query(ctx, `
		SELECT at.id::text, at.student_id::text, u.full_name, at.attempt_no,
		       ans.payload, ans.manual_score, ans.grader_comment
		  FROM app.attempts at
		  JOIN app.users u ON u.id = at.student_id
		  LEFT JOIN app.attempt_answers ans ON ans.attempt_id = at.id AND ans.question_id = $2::uuid
		 WHERE at.assignment_id = $1::uuid
		   AND at.status IN ('submitted', 'timed_out', 'graded')
		   AND `+visibility.Papers(3, 4, "at.assignment_id", "at.student_id")+`
		 ORDER BY at.id`, assignmentID, questionID, scope.All, opt.String(scope.UserID))
	if err != nil {
		return domain.ByQuestion{}, fmt.Errorf("review: read answers by question: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.QuestionAnswer
		if err := rows.Scan(&item.AttemptID, &item.StudentID, &item.StudentName, &item.AttemptNo,
			&item.Payload, &item.ManualScore, &item.GraderComment); err != nil {
			return domain.ByQuestion{}, fmt.Errorf("review: scan answer by question: %w", err)
		}
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.ByQuestion{}, fmt.Errorf("review: read answers by question: %w", err)
	}
	return out, nil
}

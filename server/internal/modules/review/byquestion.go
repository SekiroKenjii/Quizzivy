package review

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrQuestionNotOnPaper = errors.New("review: question is not on this paper")

// QuestionAnswer is one paper's answer to the question being graded.
type QuestionAnswer struct {
	AttemptID     string
	StudentID     string
	StudentName   string
	AttemptNo     int
	Payload       []byte
	ManualScore   *float64
	GraderComment *string
}

// ByQuestion is G-04's read: one question with its place on the paper, the
// paper's other manual questions to walk to, and every handed-in answer.
type ByQuestion struct {
	Question    Question
	PublishedAt time.Time
	Number      int
	Count       int
	ManualIDs   []string
	Items       []QuestionAnswer
}

// AnswersForQuestion lists one manual question across every handed-in,
// non-voided attempt of the assignment, in attempt order -- not by name,
// since the mode hides names until the question is graded.
func (s *Store) AnswersForQuestion(ctx context.Context, assignmentID, questionID string) (ByQuestion, error) {
	var out ByQuestion
	var versionID string
	err := s.pool.QueryRow(ctx, `
		SELECT a.test_version_id::text, v.published_at
		  FROM app.assignments a
		  JOIN app.test_versions v ON v.id = a.test_version_id
		 WHERE a.id = $1::uuid`, assignmentID).Scan(&versionID, &out.PublishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ByQuestion{}, ErrNotFound
	}
	if err != nil {
		return ByQuestion{}, fmt.Errorf("review: read assignment: %w", err)
	}

	questions, err := s.questions(ctx, versionID)
	if err != nil {
		return ByQuestion{}, err
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
		return ByQuestion{}, ErrQuestionNotOnPaper
	}

	rows, err := s.pool.Query(ctx, `
		SELECT at.id::text, at.student_id::text, u.full_name, at.attempt_no,
		       ans.payload, ans.manual_score, ans.grader_comment
		  FROM app.attempts at
		  JOIN app.users u ON u.id = at.student_id
		  LEFT JOIN app.attempt_answers ans ON ans.attempt_id = at.id AND ans.question_id = $2::uuid
		 WHERE at.assignment_id = $1::uuid
		   AND at.status IN ('submitted', 'timed_out', 'graded')
		 ORDER BY at.id`, assignmentID, questionID)
	if err != nil {
		return ByQuestion{}, fmt.Errorf("review: read answers by question: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item QuestionAnswer
		if err := rows.Scan(&item.AttemptID, &item.StudentID, &item.StudentName, &item.AttemptNo,
			&item.Payload, &item.ManualScore, &item.GraderComment); err != nil {
			return ByQuestion{}, fmt.Errorf("review: scan answer by question: %w", err)
		}
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return ByQuestion{}, fmt.Errorf("review: read answers by question: %w", err)
	}
	return out, nil
}

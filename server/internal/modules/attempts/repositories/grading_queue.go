package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/visibility"
	"strconv"
	"time"
)

type queueSelection struct {
	AttemptID       string
	AssignmentID    string
	AssignmentTitle string
	StudentID       string
	StudentName     string
	VersionID       string
	QuestionID      string
	QuestionNumber  int
	PublishedAt     time.Time
	Payload         json.RawMessage
	ManualScore     *float64
	GraderComment   *string
	GroupKey        string
	GroupRemaining  int
}

// GradingQueue reads coherent pending counts and at most 200 ordered saved answers.
func (s *Reviews) GradingQueue(ctx context.Context, q domain.GradingQueueQuery) (domain.GradingQueue, error) {
	if q.Mode == "" {
		q.Mode = "student"
	}
	out := domain.GradingQueue{Groups: []domain.GradingQueueGroup{}, Items: []domain.GradingQueueItem{}}
	var raw []byte
	var assignmentReached, studentReached bool
	err := s.QueryRow(ctx, gradingQueueSQL(), q.Scope.All, opt.String(q.Scope.UserID), q.AssignmentID, q.StudentID, q.Mode).
		Scan(&assignmentReached, &studentReached, &out.AnswersRemaining, &out.StudentsWaiting, &raw)
	if err != nil {
		return domain.GradingQueue{}, fmt.Errorf("grading queue: select: %w", err)
	}
	if !assignmentReached || !studentReached {
		return domain.GradingQueue{}, domain.ErrPaperNotFound
	}
	var selected []queueSelection
	if err := json.Unmarshal(raw, &selected); err != nil {
		return domain.GradingQueue{}, fmt.Errorf("grading queue: decode: %w", err)
	}
	questions := map[string]map[string]domain.ReviewQuestion{}
	groups := map[string]bool{}
	for _, row := range selected {
		if err := s.appendQueueItem(ctx, q.Mode, &out, row, questions, groups); err != nil {
			return domain.GradingQueue{}, err
		}
	}
	return out, nil
}

func (s *Reviews) appendQueueItem(ctx context.Context, mode string, out *domain.GradingQueue, row queueSelection, cache map[string]map[string]domain.ReviewQuestion, groups map[string]bool) error {
	questions, exists := cache[row.VersionID]
	if !exists {
		frozen, err := s.questions(ctx, row.VersionID)
		if err != nil {
			return err
		}
		questions = make(map[string]domain.ReviewQuestion, len(frozen))
		for _, question := range frozen {
			questions[question.ID] = question
		}
		cache[row.VersionID] = questions
	}
	question, exists := questions[row.QuestionID]
	if !exists {
		return domain.ErrQuestionNotOnPaper
	}
	out.Items = append(out.Items, domain.GradingQueueItem{
		AttemptID: row.AttemptID, AssignmentID: row.AssignmentID, AssignmentTitle: row.AssignmentTitle,
		StudentID: row.StudentID, StudentName: row.StudentName, QuestionNumber: row.QuestionNumber,
		VersionID: row.VersionID, PublishedAt: row.PublishedAt, Question: question,
		Answer: domain.ReviewAnswer{Payload: row.Payload, ManualScore: row.ManualScore, RequiresManual: true, GraderComment: row.GraderComment},
	})
	if !groups[row.GroupKey] {
		label := row.StudentName
		if mode == "question" {
			label = strconv.Itoa(row.QuestionNumber)
		}
		out.Groups = append(out.Groups, domain.GradingQueueGroup{Key: row.GroupKey, Kind: mode, Label: label, Sub: row.AssignmentTitle, Remaining: row.GroupRemaining})
		groups[row.GroupKey] = true
	}
	return nil
}

func gradingQueueSQL() string {
	return `WITH reference_flags AS (
 SELECT ($3::uuid IS NULL OR EXISTS (
  SELECT 1 FROM app.assignments a WHERE a.id=$3 AND ($1::boolean OR a.id IN ` + visibility.AssignmentIDs(2) + `)
 )) AS assignment_reached,
 ($4::uuid IS NULL OR EXISTS (
  SELECT 1 FROM app.users u WHERE u.id=$4
   AND u.role_id IN (SELECT r.id FROM app.student_like_roles r)
   AND ($1::boolean OR u.id IN ` + visibility.StudentIDs(2) + `)
 ) OR EXISTS (
  SELECT 1 FROM app.attempts at WHERE at.student_id=$4
   AND ($1::boolean OR at.assignment_id IN ` + visibility.AssignmentIDs(2) + `)
   AND ` + visibility.Papers(1, 2, "at.assignment_id", "at.student_id") + `
 )) AS student_reached
 ), numbered_questions AS (
 SELECT q.id,q.test_version_section_id,s.test_version_id,
  row_number() OVER (PARTITION BY s.test_version_id ORDER BY s.ordinal,q.ordinal,q.id) AS question_number
 FROM app.test_version_questions q JOIN app.test_version_sections s ON s.id=q.test_version_section_id
 ), eligible AS (
 SELECT at.id AS attempt_id,at.assignment_id,at.student_id,at.test_version_id,
  t.title AS assignment_title,u.full_name AS student_name,nq.id AS question_id,nq.question_number,
  at.submitted_at,v.published_at,ans.payload,ans.manual_score,ans.grader_comment,
  CASE WHEN $5::text='question' THEN at.assignment_id::text || ':' || nq.id::text ELSE at.student_id::text END AS group_key,
  CASE WHEN $5::text='question' THEN at.assignment_id ELSE at.student_id END AS group_first_id,
  CASE WHEN $5::text='question' THEN nq.id END AS group_second_id
 FROM app.attempts at
 JOIN app.assignments a ON a.id=at.assignment_id
 JOIN app.tests t ON t.id=a.test_id
 JOIN app.users u ON u.id=at.student_id
 JOIN app.test_versions v ON v.id=at.test_version_id
 JOIN app.attempt_answers ans ON ans.attempt_id=at.id
 JOIN numbered_questions nq ON nq.id=ans.question_id AND nq.test_version_id=at.test_version_id
 WHERE at.status IN ('submitted','timed_out') AND ans.requires_manual AND ans.manual_score IS NULL
  AND ($1::boolean OR at.assignment_id IN ` + visibility.AssignmentIDs(2) + `)
  AND ` + visibility.Papers(1, 2, "at.assignment_id", "at.student_id") + `
  AND ($3::uuid IS NULL OR at.assignment_id=$3)
  AND ($4::uuid IS NULL OR at.student_id=$4)
 ), grouped AS (
 SELECT group_key,group_first_id,group_second_id,count(*) AS remaining,min(submitted_at) AS first_submitted
 FROM eligible GROUP BY group_key,group_first_id,group_second_id
 ), totals AS (
 SELECT count(*) AS answers_remaining,count(DISTINCT student_id) AS students_waiting FROM eligible
 ), prefix AS (
 SELECT e.attempt_id,e.assignment_id,e.student_id,e.test_version_id,e.assignment_title,e.student_name,
  e.question_id,e.question_number,e.published_at,e.payload,e.manual_score,e.grader_comment,e.group_key,g.remaining,
  row_number() OVER (ORDER BY g.first_submitted ASC NULLS LAST,g.group_first_id,g.group_second_id,
   e.submitted_at ASC NULLS LAST,e.attempt_id,e.question_number,e.question_id) AS position
 FROM eligible e JOIN grouped g ON g.group_key=e.group_key
 ORDER BY g.first_submitted ASC NULLS LAST,g.group_first_id,g.group_second_id,
  e.submitted_at ASC NULLS LAST,e.attempt_id,e.question_number,e.question_id LIMIT 200
 )
 SELECT f.assignment_reached,f.student_reached,t.answers_remaining,t.students_waiting,
  coalesce((SELECT jsonb_agg(jsonb_build_object(
   'AttemptID',p.attempt_id,'AssignmentID',p.assignment_id,'AssignmentTitle',p.assignment_title,
   'StudentID',p.student_id,'StudentName',p.student_name,'VersionID',p.test_version_id,
   'QuestionID',p.question_id,'QuestionNumber',p.question_number,'PublishedAt',p.published_at,
   'Payload',p.payload,'ManualScore',p.manual_score,'GraderComment',p.grader_comment,
   'GroupKey',p.group_key,'GroupRemaining',p.remaining) ORDER BY p.position) FROM prefix p),'[]'::jsonb)
 FROM reference_flags f CROSS JOIN totals t`
}

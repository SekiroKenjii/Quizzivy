package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/answered"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/visibility"
)

var wrongAnswersSQL = `
	WITH a AS (
	  SELECT id, test_version_id FROM app.assignments
	   WHERE id = $1::uuid AND ($2::boolean OR id IN ` + visibility.AssignmentIDs(3) + `)
	), papers AS (
	  SELECT DISTINCT ON (at.student_id) at.id
	    FROM app.attempts at
	    JOIN a ON a.id = at.assignment_id
	   WHERE at.status IN ('submitted', 'timed_out', 'graded')
	     AND ` + visibility.Papers(2, 3, "at.assignment_id", "at.student_id") + `
	   ORDER BY at.student_id, at.attempt_no DESC
	), numbered AS (
	  SELECT q.id, q.type::text AS type, q.points, q.sample_answer,
	         row_number() OVER (ORDER BY s.ordinal, q.ordinal, q.id) AS number
	    FROM a
	    JOIN app.test_version_sections s ON s.test_version_id = a.test_version_id
	    JOIN app.test_version_questions q ON q.test_version_section_id = s.id
	), lettered AS (
	  SELECT o.id, o.test_version_question_id AS question_id, o.ordinal, o.is_correct,
	         chr(64 + (row_number() OVER (PARTITION BY o.test_version_question_id ORDER BY o.ordinal))::int) AS letter
	    FROM app.test_version_options o
	    JOIN numbered n ON n.id = o.test_version_question_id
	), keys AS (
	  SELECT n.id AS question_id,
	         CASE n.type
	           WHEN 'true_false' THEN
	             (SELECT string_agg(CASE l.ordinal WHEN 0 THEN 'True' ELSE 'False' END, ', ' ORDER BY l.ordinal)
	                FROM lettered l WHERE l.question_id = n.id AND l.is_correct)
	           WHEN 'single_choice' THEN
	             (SELECT string_agg(l.letter, ', ' ORDER BY l.letter) FROM lettered l WHERE l.question_id = n.id AND l.is_correct)
	           WHEN 'multiple_choice' THEN
	             (SELECT string_agg(l.letter, ', ' ORDER BY l.letter) FROM lettered l WHERE l.question_id = n.id AND l.is_correct)
	           WHEN 'tfng' THEN
	             (SELECT string_agg(CASE l.ordinal WHEN 0 THEN 'True' WHEN 1 THEN 'False' ELSE 'Not given' END, ', ' ORDER BY l.ordinal)
	                FROM lettered l WHERE l.question_id = n.id AND l.is_correct)
	           WHEN 'matching' THEN
	             (SELECT string_agg((mi.ordinal + 1)::text || ': ' || l.letter, '; ' ORDER BY mi.ordinal)
	                FROM app.test_version_match_items mi
	                JOIN lettered l ON l.question_id = n.id AND l.id = mi.correct_option_id
	               WHERE mi.test_version_question_id = n.id)
	           WHEN 'fill_blank' THEN
	             (SELECT string_agg(b.ordinal::text || ': ' ||
	                      (SELECT string_agg(ba.answer, ' / ' ORDER BY ba.answer) FROM app.test_version_blank_answers ba WHERE ba.test_version_blank_id = b.id),
	                    '; ' ORDER BY b.ordinal)
	                FROM app.test_version_blanks b WHERE b.test_version_question_id = n.id)
	           ELSE n.sample_answer
	         END AS key
	    FROM numbered n
	), given AS (
	  SELECT n.id AS question_id,
	         CASE ans.payload->>'type'
	           WHEN 'text' THEN lower(btrim(regexp_replace(coalesce(ans.payload->>'value', ''), '\s+', ' ', 'g')))
	           WHEN 'true_false' THEN
	             CASE WHEN jsonb_typeof(ans.payload->'value') = 'boolean'
	                  THEN CASE WHEN (ans.payload->>'value')::boolean THEN 'True' ELSE 'False' END END
	           WHEN 'choice' THEN
	             CASE WHEN jsonb_typeof(ans.payload->'optionIds') = 'array' THEN
	               (SELECT string_agg(CASE WHEN n.type = 'tfng'
	                                       THEN CASE l.ordinal WHEN 0 THEN 'True' WHEN 1 THEN 'False' ELSE 'Not given' END
	                                       ELSE l.letter END, '+' ORDER BY l.letter)
	                  FROM lettered l
	                 WHERE l.question_id = n.id
	                   AND l.id::text IN (SELECT jsonb_array_elements_text(ans.payload->'optionIds')))
	             END
	           WHEN 'matching' THEN
	             CASE WHEN jsonb_typeof(ans.payload->'pairs') = 'object' THEN
	               (SELECT string_agg((mi.ordinal + 1)::text || ': ' || coalesce(l.letter, '-'), '; ' ORDER BY mi.ordinal)
	                  FROM app.test_version_match_items mi
	                  LEFT JOIN lettered l ON l.question_id = n.id AND l.id::text = lower(ans.payload->'pairs'->>mi.id::text)
	                 WHERE mi.test_version_question_id = n.id)
	             END
	           WHEN 'fill_blank' THEN
	             (SELECT string_agg(lower(btrim(regexp_replace(coalesce(ans.payload->'values'->>b.id::text, ''), '\s+', ' ', 'g'))), ' | ' ORDER BY b.ordinal)
	                FROM app.test_version_blanks b WHERE b.test_version_question_id = n.id)
	         END AS text
	    FROM numbered n
	   CROSS JOIN papers p
	    JOIN app.attempt_answers ans ON ans.attempt_id = p.id AND ans.question_id = n.id
	   WHERE ans.final_score < n.points
	     AND ` + answered.SaysSomething("ans") + `
	), counted AS (
	  SELECT g.question_id, left(g.text, 200) AS text, count(*) AS papers
	    FROM given g
	   WHERE btrim(coalesce(g.text, '')) <> ''
	   GROUP BY g.question_id, left(g.text, 200)
	), ranked AS (
	  SELECT c.question_id, c.text, c.papers, row_number() OVER (PARTITION BY c.question_id ORDER BY c.papers DESC, c.text) AS rank
	    FROM counted c
	)
	SELECT n.id::text, n.number, n.type, coalesce(k.key, ''), (SELECT count(*) FROM papers),
	       coalesce(r.text, ''), coalesce(r.papers, 0)
	  FROM numbered n
	  JOIN keys k ON k.question_id = n.id
	  LEFT JOIN ranked r ON r.question_id = n.id AND r.rank <= ` + fmt.Sprint(domain.MaxWrongAnswers) + `
	 ORDER BY n.number, r.rank`

// WrongAnswers reads, for each question of an assignment the scope reaches, its key and the distinct answer texts that did not earn full points over the papers handed in by the students the scope reaches: the same papers ItemAnalysis counts. Another teacher's assignment answers ErrNotFound, exactly as a missing one does.
func (s *Postgres) WrongAnswers(ctx context.Context, scope access.Scope, assignmentID string) (domain.WrongAnswers, error) {
	reached, err := s.reachesAll(ctx, scope, []string{assignmentID})
	if err != nil {
		return domain.WrongAnswers{}, err
	}
	if !reached {
		return domain.WrongAnswers{}, domain.ErrNotFound
	}
	rows, err := s.Query(ctx, wrongAnswersSQL, assignmentID, scope.All, opt.String(scope.UserID))
	if err != nil {
		return domain.WrongAnswers{}, fmt.Errorf("attempts: wrong answers: %w", err)
	}
	defer rows.Close()

	out := domain.WrongAnswers{Questions: []domain.QuestionWrongAnswers{}}
	for rows.Next() {
		var (
			id, kind, key, text string
			number, count       int
		)
		if err := rows.Scan(&id, &number, &kind, &key, &out.HandedIn, &text, &count); err != nil {
			return domain.WrongAnswers{}, fmt.Errorf("attempts: scan wrong answers: %w", err)
		}
		if n := len(out.Questions); n == 0 || out.Questions[n-1].QuestionID != id {
			out.Questions = append(out.Questions, domain.QuestionWrongAnswers{QuestionID: id, Number: number, Type: kind, Key: key, Wrong: []domain.WrongAnswer{}})
		}
		if text != "" {
			last := &out.Questions[len(out.Questions)-1]
			last.Wrong = append(last.Wrong, domain.WrongAnswer{Text: text, Count: count})
		}
	}
	if err := rows.Err(); err != nil {
		return domain.WrongAnswers{}, fmt.Errorf("attempts: wrong answers: %w", err)
	}
	return out, nil
}

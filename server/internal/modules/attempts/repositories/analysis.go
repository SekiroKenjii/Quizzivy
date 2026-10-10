package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/answered"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/visibility"
	"slices"
	"sort"
)

var itemAnalysisSQL = `
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
	  SELECT q.id, q.type::text AS type, q.points,
	         left(btrim(regexp_replace(q.prompt, '\s+', ' ', 'g')), 140) AS excerpt,
	         row_number() OVER (ORDER BY s.ordinal, q.ordinal, q.id) AS number
	    FROM a
	    JOIN app.test_version_sections s ON s.test_version_id = a.test_version_id
	    JOIN app.test_version_questions q ON q.test_version_section_id = s.id
	), marks AS (
	  SELECT n.id AS question_id,
	         coalesce(` + answered.SaysSomething("ans") + `, false) AS said,
	         coalesce(ans.requires_manual AND ans.manual_score IS NULL, false) AS pending,
	         ans.final_score >= n.points AS full
	    FROM numbered n
	   CROSS JOIN papers p
	    LEFT JOIN app.attempt_answers ans ON ans.attempt_id = p.id AND ans.question_id = n.id
	)
	SELECT n.id::text, n.number, n.type, n.excerpt, (SELECT count(*) FROM papers),
	       count(*) FILTER (WHERE m.said),
	       count(*) FILTER (WHERE NOT m.pending),
	       count(*) FILTER (WHERE NOT m.pending AND m.full)
	  FROM numbered n
	  LEFT JOIN marks m ON m.question_id = n.id
	 GROUP BY n.id, n.number, n.type, n.excerpt
	 ORDER BY n.number`

// ItemAnalysis reads how the students the scope reaches did on each question of
// an assignment the scope reaches; another teacher's assignment answers
// ErrNotFound, exactly as a missing one does.
func (s *Postgres) ItemAnalysis(ctx context.Context, scope access.Scope, assignmentID string) (domain.ItemAnalysis, error) {
	reached, err := s.reachesAll(ctx, scope, []string{assignmentID})
	if err != nil {
		return domain.ItemAnalysis{}, err
	}
	if !reached {
		return domain.ItemAnalysis{}, domain.ErrNotFound
	}
	rows, err := s.Query(ctx, itemAnalysisSQL, assignmentID, scope.All, opt.String(scope.UserID))
	if err != nil {
		return domain.ItemAnalysis{}, fmt.Errorf("attempts: item analysis: %w", err)
	}
	defer rows.Close()

	out := domain.ItemAnalysis{Items: []domain.AnalysisItem{}}
	for rows.Next() {
		var item domain.AnalysisItem
		var counted, correct int
		if err := rows.Scan(&item.QuestionID, &item.Number, &item.Type, &item.PromptExcerpt, &out.HandedIn,
			&item.Answered, &counted, &correct); err != nil {
			return domain.ItemAnalysis{}, fmt.Errorf("attempts: scan item analysis: %w", err)
		}
		if counted > 0 {
			rate := float64(correct) / float64(counted)
			item.CorrectRate = &rate
		}
		out.Items = append(out.Items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.ItemAnalysis{}, fmt.Errorf("attempts: item analysis: %w", err)
	}
	sort.SliceStable(out.Items, func(i, j int) bool { return hardest(out.Items[i], out.Items[j]) })
	return out, nil
}

func hardest(a, b domain.AnalysisItem) bool {
	switch {
	case a.CorrectRate == nil && b.CorrectRate == nil:
		return a.Number < b.Number
	case a.CorrectRate == nil:
		return false
	case b.CorrectRate == nil:
		return true
	case *a.CorrectRate != *b.CorrectRate:
		return *a.CorrectRate < *b.CorrectRate
	default:
		return a.Number < b.Number
	}
}

var resultsSQL = `
	WITH wanted AS (
	  SELECT a.id, a.test_id, a.test_version_id, w.ord
	    FROM unnest($1::uuid[]) WITH ORDINALITY AS w(id, ord)
	    JOIN app.assignments a ON a.id = w.id
	   WHERE $2::boolean OR a.id IN ` + visibility.AssignmentIDs(3) + `
	), roster AS (
	  SELECT wd.id AS assignment_id, m.user_id
	    FROM wanted wd
	    JOIN app.assignment_classes ac ON ac.assignment_id = wd.id
	     AND ($2::boolean OR ac.class_id IN ` + visibility.TaughtClassIDs(3) + `)
	    JOIN app.class_members m ON m.class_id = ac.class_id
	  UNION
	  SELECT wd.id, ast.user_id
	    FROM wanted wd
	    JOIN app.assignment_students ast ON ast.assignment_id = wd.id
	     AND ($2::boolean OR ast.user_id IN ` + visibility.StudentIDs(3) + `)
	), standing AS (
	  SELECT DISTINCT ON (at.assignment_id, at.student_id)
	         at.assignment_id, at.student_id, at.id, at.status::text AS status, at.submitted_at,
	         at.score_earned, at.score_total, at.focus_loss_count, at.flagged
	    FROM app.attempts at
	    JOIN wanted wd ON wd.id = at.assignment_id
	   ORDER BY at.assignment_id, at.student_id, (at.status <> 'voided') DESC, at.attempt_no DESC
	)
	SELECT t.title, v.version,
	       coalesce((SELECT string_agg(c.name, '; ' ORDER BY c.name)
	                   FROM app.assignment_classes ac
	                   JOIN app.classes c ON c.id = ac.class_id
	                   JOIN app.class_members m ON m.class_id = c.id AND m.user_id = r.user_id
	                  WHERE ac.assignment_id = r.assignment_id
	                    AND ($2::boolean OR c.id IN ` + visibility.TaughtClassIDs(3) + `)), ''),
	       u.full_name, u.email, coalesce(s.status, 'not_started'), s.submitted_at,
	       CASE WHEN s.status IN ('submitted', 'timed_out', 'graded') AND p.pending = 0 THEN s.score_earned END,
	       CASE WHEN s.status IN ('submitted', 'timed_out', 'graded') AND p.pending = 0 THEN s.score_total END,
	       s.focus_loss_count, coalesce(s.flagged, false)
	  FROM roster r
	  JOIN wanted wd ON wd.id = r.assignment_id
	  JOIN app.tests t ON t.id = wd.test_id
	  JOIN app.test_versions v ON v.id = wd.test_version_id
	  JOIN app.users u ON u.id = r.user_id AND u.disabled_at IS NULL
	  LEFT JOIN standing s ON s.assignment_id = r.assignment_id AND s.student_id = r.user_id
	  LEFT JOIN LATERAL (
	    SELECT count(*) AS pending FROM app.attempt_answers aa
	     WHERE aa.attempt_id = s.id AND aa.requires_manual AND aa.manual_score IS NULL
	  ) p ON true
	 ORDER BY wd.ord, u.full_name, u.id
	 LIMIT ` + fmt.Sprint(domain.MaxExportRows+1)

// Results reads the roster rows of the assignments named, in the order named,
// for the students the scope reaches. Every assignment must be one the scope
// reaches, or none is read and the answer is ErrNotFound, as for a missing one;
// more than domain.MaxExportRows rows answer ErrExportTooLarge.
func (s *Postgres) Results(ctx context.Context, scope access.Scope, assignmentIDs []string) ([]domain.ResultRow, error) {
	reached, err := s.reachesAll(ctx, scope, assignmentIDs)
	if err != nil {
		return nil, err
	}
	if !reached {
		return nil, domain.ErrNotFound
	}
	rows, err := s.Query(ctx, resultsSQL, assignmentIDs, scope.All, opt.String(scope.UserID))
	if err != nil {
		return nil, fmt.Errorf("attempts: results: %w", err)
	}
	defer rows.Close()

	out := []domain.ResultRow{}
	for rows.Next() {
		var r domain.ResultRow
		if err := rows.Scan(&r.AssignmentTitle, &r.Version, &r.Classes, &r.StudentName, &r.Email, &r.State,
			&r.SubmittedAt, &r.Earned, &r.Total, &r.FocusLoss, &r.Flagged); err != nil {
			return nil, fmt.Errorf("attempts: scan results: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("attempts: results: %w", err)
	}
	if len(out) > domain.MaxExportRows {
		return nil, domain.ErrExportTooLarge
	}
	return out, nil
}

func (s *Postgres) reachesAll(ctx context.Context, scope access.Scope, assignmentIDs []string) (bool, error) {
	wanted := slices.Clone(assignmentIDs)
	slices.Sort(wanted)
	wanted = slices.Compact(wanted)
	var reached int
	err := s.QueryRow(ctx, `
		SELECT count(*) FROM app.assignments
		 WHERE id = ANY($1::uuid[]) AND ($2::boolean OR id IN `+visibility.AssignmentIDs(3)+`)`,
		wanted, scope.All, opt.String(scope.UserID)).Scan(&reached)
	if err != nil {
		return false, fmt.Errorf("attempts: reach assignments: %w", err)
	}
	return reached == len(wanted), nil
}

package repositories

import (
	"context"
	"fmt"

	"quizzivy/internal/shared/stats"
)

var _ stats.ClassSource = (*StudentStats)(nil)

// ClassScores sums, per class, the best graded attempt of each live member on
// each assignment that targets the class: the highest share of the points on
// offer, the later attempt on a tie, the rule StudentStats uses for a student.
// A voided attempt, an attempt not yet graded and a member who has left or is
// disabled count for nothing. A class whose points on offer sum to nothing has
// no entry.
func (s *StudentStats) ClassScores(ctx context.Context, classIDs []string) (map[string]stats.ClassScore, error) {
	out := make(map[string]stats.ClassScore, len(classIDs))
	if len(classIDs) == 0 {
		return out, nil
	}
	rows, err := s.Query(ctx, `
		SELECT c.id::text, sum(b.earned), sum(b.total), coalesce(sum(b.pending_manual), 0)
		  FROM app.classes c
		  JOIN LATERAL (
		    SELECT DISTINCT ON (a.assignment_id, a.student_id)
		           a.score_earned AS earned,
		           coalesce(a.score_total, v.total_points) AS total,
		           (SELECT count(*) FROM app.attempt_answers ans
		             WHERE ans.attempt_id = a.id
		               AND ans.requires_manual AND ans.manual_score IS NULL) AS pending_manual
		      FROM app.assignment_classes ac
		      JOIN app.attempts a ON a.assignment_id = ac.assignment_id
		      JOIN app.class_members m ON m.class_id = c.id AND m.user_id = a.student_id
		      JOIN app.users u ON u.id = m.user_id AND u.disabled_at IS NULL
		      JOIN app.test_versions v ON v.id = a.test_version_id
		     WHERE ac.class_id = c.id
		       AND a.status = 'graded'
		       AND a.score_earned IS NOT NULL
		     ORDER BY a.assignment_id, a.student_id,
		              a.score_earned / nullif(coalesce(a.score_total, v.total_points), 0) DESC,
		              a.attempt_no DESC
		  ) b ON TRUE
		 WHERE c.id = ANY($1::uuid[])
		 GROUP BY c.id`, classIDs)
	if err != nil {
		return nil, fmt.Errorf("class scores: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var score stats.ClassScore
		if err := rows.Scan(&id, &score.Earned, &score.Total, &score.PendingManual); err != nil {
			return nil, fmt.Errorf("class scores: scan: %w", err)
		}
		if score.Total > 0 {
			out[id] = score
		}
	}
	return out, rows.Err()
}

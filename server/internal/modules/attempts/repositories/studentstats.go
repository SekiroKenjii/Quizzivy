package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/shared/stats"
)

type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// StudentStats derives each student's figures from their attempts: the best
// graded attempt per assignment, what is still unmarked, flags, and when they last worked.
type StudentStats struct{ db Querier }

func NewStudentStats(db Querier) *StudentStats { return &StudentStats{db: db} }

var _ stats.Source = (*StudentStats)(nil)

func (s *StudentStats) StudentStats(ctx context.Context, ids []string) (map[string]stats.Student, error) {
	out := make(map[string]stats.Student, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx, `
		SELECT u.id::text,
		       best.submitted_count, best.earned, best.total, best.pending_manual,
		       act.flagged_count, act.live, act.last_attempt_at
		  FROM app.users u
		  LEFT JOIN LATERAL (
		    SELECT (SELECT count(DISTINCT a.assignment_id)
		              FROM app.attempts a
		             WHERE a.student_id = u.id
		               AND a.status IN ('submitted','timed_out','graded')) AS submitted_count,
		           sum(g.score_earned)                AS earned,
		           sum(g.score_total)                 AS total,
		           coalesce(sum(g.pending_manual), 0) AS pending_manual
		      FROM (
		        SELECT DISTINCT ON (a.assignment_id)
		               a.assignment_id,
		               a.score_earned,
		               coalesce(a.score_total, v.total_points) AS score_total,
		               (SELECT count(*) FROM app.attempt_answers ans
		                 WHERE ans.attempt_id = a.id
		                   AND ans.requires_manual AND ans.manual_score IS NULL) AS pending_manual
		          FROM app.attempts a
		          JOIN app.test_versions v ON v.id = a.test_version_id
		         WHERE a.student_id = u.id
		           AND a.status = 'graded'
		           AND a.score_earned IS NOT NULL
		         ORDER BY a.assignment_id,
		                  a.score_earned / coalesce(a.score_total, v.total_points) DESC,
		                  a.attempt_no DESC
		      ) g
		  ) best ON TRUE
		  LEFT JOIN LATERAL (
		    SELECT count(*) FILTER (WHERE a.flagged AND a.status <> 'voided') AS flagged_count,
		           bool_or(a.status = 'in_progress' AND a.deadline_at > now())  AS live,
		           max(greatest(a.submitted_at, a.started_at))                  AS last_attempt_at
		      FROM app.attempts a
		     WHERE a.student_id = u.id AND a.status <> 'voided'
		  ) act ON TRUE
		 WHERE u.id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, fmt.Errorf("student stats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var submitted, pending, flagged *int
		var earned, total *float64
		var live *bool
		var last *time.Time
		if err := rows.Scan(&id, &submitted, &earned, &total, &pending, &flagged, &live, &last); err != nil {
			return nil, fmt.Errorf("student stats: scan: %w", err)
		}
		st := stats.Student{
			SubmittedCount: intOf(submitted),
			ScoreEarned:    earned,
			ScoreTotal:     total,
			PendingManual:  intOf(pending),
			FlaggedCount:   intOf(flagged),
			LiveAttempt:    live != nil && *live,
			LastAttemptAt:  last,
		}
		if st.ScoreTotal != nil && *st.ScoreTotal <= 0 {
			st.ScoreEarned, st.ScoreTotal = nil, nil
		}
		out[id] = st
	}
	return out, rows.Err()
}

func intOf(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

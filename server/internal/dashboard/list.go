package dashboard

import (
	"context"
	"fmt"
	"strings"

	"quizzivy/internal/paging"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// ListInput is §8's cross-assignment attempt list, filtered the way the two
// dashboard queues need.
type ListInput struct {
	Status         *string
	Flagged        *bool
	PendingGrading *bool
	Page           int
	Limit          int
}

// List returns one page of attempts, most recently handed in first.
func (s *Store) List(ctx context.Context, in ListInput) ([]Recent, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	var args []any
	where := []string{"TRUE"}
	if in.Status != nil {
		args = append(args, *in.Status)
		where = append(where, fmt.Sprintf("at.status = $%d::app.attempt_status", len(args)))
	}
	if in.Flagged != nil {
		args = append(args, *in.Flagged)
		where = append(where, fmt.Sprintf("at.flagged = $%d", len(args)))
	}
	if in.PendingGrading != nil {
		args = append(args, *in.PendingGrading)
		// Both closed-but-ungraded states, as the dashboard counts them (00025).
		where = append(where, fmt.Sprintf(`(at.status IN ('submitted', 'timed_out') AND EXISTS (
		     SELECT 1 FROM app.attempt_answers ans
		      WHERE ans.attempt_id = at.id AND ans.requires_manual AND ans.manual_score IS NULL)) = $%d`, len(args)))
	}
	filter := strings.Join(where, "\n		   AND ")

	page := paging.Page{Number: number, Size: limit}
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM app.attempts at WHERE `+filter, args...).
		Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("dashboard: count attempts: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := s.db.Query(ctx, `
		SELECT at.id::text, at.student_id::text, u.full_name,
		       at.assignment_id::text, t.title, at.status::text, at.submitted_at,
		       (SELECT count(*) FROM app.attempt_answers ans
		         WHERE ans.attempt_id = at.id
		           AND ans.requires_manual AND ans.manual_score IS NULL),
		       at.flagged
		  FROM app.attempts at
		  JOIN app.users u ON u.id = at.student_id
		  JOIN app.assignments a ON a.id = at.assignment_id
		  JOIN app.tests t ON t.id = a.test_id
		 WHERE `+filter+fmt.Sprintf(`
		 ORDER BY at.submitted_at DESC NULLS LAST, at.started_at DESC, at.id DESC
		 LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("dashboard: list attempts: %w", err)
	}
	defer rows.Close()

	out := make([]Recent, 0, limit)
	for rows.Next() {
		var r Recent
		if err := rows.Scan(&r.ID, &r.StudentID, &r.StudentName, &r.AssignmentID,
			&r.TestTitle, &r.Status, &r.SubmittedAt, &r.PendingManual, &r.Flagged); err != nil {
			return nil, paging.Page{}, fmt.Errorf("dashboard: scan attempt: %w", err)
		}
		out = append(out, r)
	}
	return out, page, rows.Err()
}

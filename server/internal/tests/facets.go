package tests

import (
	"context"
	"fmt"
	"strings"
)

// StatusFacets is how many tests each status holds for one search.
type StatusFacets struct {
	All       int
	Draft     int
	Published int
	Archived  int
}

// Facets counts tests per status for the given search.
//
// The status filter itself is deliberately NOT applied: A-03 shows every tab's
// count at once, and applying it would zero the tabs the teacher is not on --
// which is the one thing the numbers are there to prevent them having to guess.
//
// One grouped query rather than four counts: the same scan answers all of them.
func (s *Store) Facets(ctx context.Context, in ListInput) (StatusFacets, error) {
	args := []any{}
	where := []string{`t.deleted_at IS NULL`}
	if q := strings.TrimSpace(in.Query); q != "" {
		args = append(args, escapeLike(q))
		where = append(where, fmt.Sprintf(titleSearch, len(args)))
	}

	sql := `SELECT t.status::text, count(*)
		      FROM app.tests t
		     WHERE ` + strings.Join(where, " AND ") + `
		     GROUP BY t.status`

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return StatusFacets{}, fmt.Errorf("tests: facets: %w", err)
	}
	defer rows.Close()

	var out StatusFacets
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return StatusFacets{}, fmt.Errorf("tests: scan facet: %w", err)
		}
		switch Status(status) {
		case Draft:
			out.Draft = n
		case Published:
			out.Published = n
		case Archived:
			out.Archived = n
		}
		out.All += n
	}
	if err := rows.Err(); err != nil {
		return StatusFacets{}, fmt.Errorf("tests: facets: %w", err)
	}
	return out, nil
}

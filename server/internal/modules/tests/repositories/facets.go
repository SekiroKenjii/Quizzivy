package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"strings"
)

// Facets counts tests per status for the given search.
func (s *Postgres) Facets(ctx context.Context, in domain.ListInput) (domain.StatusFacets, error) {
	args := []any{}
	where := []string{liveTests}
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
		return domain.StatusFacets{}, fmt.Errorf("tests: facets: %w", err)
	}
	defer rows.Close()

	var out domain.StatusFacets
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return domain.StatusFacets{}, fmt.Errorf("tests: scan facet: %w", err)
		}
		switch domain.Status(status) {
		case domain.Draft:
			out.Draft = n
		case domain.Published:
			out.Published = n
		case domain.Archived:
			out.Archived = n
		}
		out.All += n
	}
	if err := rows.Err(); err != nil {
		return domain.StatusFacets{}, fmt.Errorf("tests: facets: %w", err)
	}
	return out, nil
}

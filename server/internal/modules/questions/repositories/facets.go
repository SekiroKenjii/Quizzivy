package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/questions/domain"
	"strings"
)

// Facets counts questions per type for the given tag and search.
func (s *Postgres) Facets(ctx context.Context, in domain.ListInput) (domain.TypeFacets, error) {
	args, where := appendFilters(in, filterOpts{tags: true})

	sql := `SELECT q.type::text, count(*)
		      FROM app.questions q
		     WHERE ` + strings.Join(where, " AND ") + `
		     GROUP BY q.type`

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return domain.TypeFacets{}, fmt.Errorf("questions: facets: %w", err)
	}
	defer rows.Close()

	out := domain.TypeFacets{ByType: map[domain.Type]int{}}
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			return domain.TypeFacets{}, fmt.Errorf("questions: scan facet: %w", err)
		}
		out.ByType[domain.Type(kind)] = n
		out.All += n
	}
	if err := rows.Err(); err != nil {
		return domain.TypeFacets{}, fmt.Errorf("questions: facets: %w", err)
	}
	return out, nil
}

// Tags returns every tag reachable through the current type, audio and search
// filters, so A-06's rail cannot offer a chip that returns nothing.
func (s *Postgres) Tags(ctx context.Context, in domain.ListInput) ([]string, error) {

	args, where := appendFilters(in, filterOpts{types: true})

	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT unnest(q.tags)
		  FROM app.questions q
		 WHERE `+strings.Join(where, " AND ")+`
		 ORDER BY 1`, args...)
	if err != nil {
		return nil, fmt.Errorf("questions: tags: %w", err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("questions: scan tag: %w", err)
		}
		out = append(out, tag)
	}
	return out, rows.Err()
}

// Counts returns the bank's size and how much of it the current filters match --
// A-06's "180 câu · đang lọc 41".
func (s *Postgres) Counts(ctx context.Context, in domain.ListInput) (total int, filtered int, err error) {
	args, where := appendFilters(in, allFilters())

	if err := s.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM app.questions q WHERE q.deleted_at IS NULL),
		       (SELECT count(*) FROM app.questions q WHERE `+strings.Join(where, " AND ")+`)`,
		args...).Scan(&total, &filtered); err != nil {
		return 0, 0, fmt.Errorf("questions: counts: %w", err)
	}
	return total, filtered, nil
}

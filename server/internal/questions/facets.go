package questions

import (
	"context"
	"fmt"
	"strings"
)

// TypeFacets is how many bank questions each type holds for one search.
type TypeFacets struct {
	All    int
	ByType map[Type]int
}

// Facets counts questions per type for the given tag and search.
//
// The type filter is deliberately not applied, for the same reason as the tests
// list: A-06 shows every type's count at once, and applying the filter would
// zero the rows the teacher has not selected.
func (s *Store) Facets(ctx context.Context, in ListInput) (TypeFacets, error) {
	// types: false -- applying the type filter would zero every row the teacher
	// has not ticked, which is the whole point of showing all five counts.
	args, where := appendFilters(in, nil, filterOpts{tags: true})

	sql := `SELECT q.type::text, count(*)
		      FROM app.questions q
		     WHERE ` + strings.Join(where, " AND ") + `
		     GROUP BY q.type`

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return TypeFacets{}, fmt.Errorf("questions: facets: %w", err)
	}
	defer rows.Close()

	out := TypeFacets{ByType: map[Type]int{}}
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			return TypeFacets{}, fmt.Errorf("questions: scan facet: %w", err)
		}
		out.ByType[Type(kind)] = n
		out.All += n
	}
	if err := rows.Err(); err != nil {
		return TypeFacets{}, fmt.Errorf("questions: facets: %w", err)
	}
	return out, nil
}

// Tags returns every tag reachable through the current type, audio and search
// filters, so A-06's rail cannot offer a chip that returns nothing.
//
// Server-derived rather than collected from the returned page. A rail built
// from one page can only offer the tags that page happens to carry: with 72
// questions and a page of 50, two of the bank's three tags were invisible, so a
// second chip could not be selected and multi-tag filtering looked unbuilt.
func (s *Store) Tags(ctx context.Context, in ListInput) ([]string, error) {
	// tags: false -- picking one chip must not empty the rail.
	args, where := appendFilters(in, nil, filterOpts{types: true})

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
//
// `filtered` applies EVERY dimension, unlike facets.All which skips the type
// filter so the "Tất cả" row has something to show.
func (s *Store) Counts(ctx context.Context, in ListInput) (total int, filtered int, err error) {
	args, where := appendFilters(in, nil, allFilters())

	if err := s.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM app.questions q WHERE q.deleted_at IS NULL),
		       (SELECT count(*) FROM app.questions q WHERE `+strings.Join(where, " AND ")+`)`,
		args...).Scan(&total, &filtered); err != nil {
		return 0, 0, fmt.Errorf("questions: counts: %w", err)
	}
	return total, filtered, nil
}

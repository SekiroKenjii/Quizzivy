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
	args := []any{}
	where := []string{`q.deleted_at IS NULL`}

	if in.Tag != "" {
		args = append(args, []string{in.Tag})
		where = append(where, fmt.Sprintf(`q.tags @> $%d::text[]`, len(args)))
	}
	if search := strings.TrimSpace(in.Query); search != "" {
		args = append(args, escapeLike(search))
		where = append(where, fmt.Sprintf(searchCondition, len(args)))
	}

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

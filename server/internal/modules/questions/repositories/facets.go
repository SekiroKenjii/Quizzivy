package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/questions/domain"
	"strings"
)

// Facets counts questions per type for the given tag and search.
func (s *Postgres) Facets(ctx context.Context, in domain.ListInput) (domain.TypeFacets, error) {
	args, where := appendFilters(in, filterOpts{tags: true, levels: true, skills: true})

	sql := `SELECT q.type::text, count(*)
		      FROM app.questions q
		     WHERE ` + strings.Join(where, " AND ") + `
		     GROUP BY q.type`

	rows, err := s.Query(ctx, sql, args...)
	if err != nil {
		return domain.TypeFacets{}, fmt.Errorf("questions: facets: %w", err)
	}
	defer rows.Close()

	out := domain.TypeFacets{ByType: map[domain.Type]int{}, ByLevel: map[domain.Level]int{}, BySkill: map[domain.Skill]int{}}
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
	rows.Close()
	levels, err := s.metadataFacetCounts(ctx, in, filterOpts{types: true, tags: true, skills: true}, "level")
	if err != nil {
		return domain.TypeFacets{}, err
	}
	for value, count := range levels {
		out.ByLevel[domain.Level(value)] = count
	}
	skills, err := s.metadataFacetCounts(ctx, in, filterOpts{types: true, tags: true, levels: true}, "skill")
	if err != nil {
		return domain.TypeFacets{}, err
	}
	for value, count := range skills {
		out.BySkill[domain.Skill(value)] = count
	}
	return out, nil
}

// Tags returns every tag reachable through the current type, audio and search
// filters, so A-06's rail cannot offer a chip that returns nothing.
func (s *Postgres) Tags(ctx context.Context, in domain.ListInput) ([]string, error) {

	args, where := appendFilters(in, filterOpts{types: true, levels: true, skills: true})

	rows, err := s.Query(ctx, `
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

// Counts returns the size of the bank the input's scope reaches and how much of
// it the current filters match -- A-06's "180 câu · đang lọc 41".
func (s *Postgres) Counts(ctx context.Context, in domain.ListInput) (total int, filtered int, err error) {
	args, where := appendFilters(in, allFilters())
	_, bank := bankRows(in.Scope)

	if err := s.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM app.questions q WHERE `+strings.Join(bank, " AND ")+`),
		       (SELECT count(*) FROM app.questions q WHERE `+strings.Join(where, " AND ")+`)`,
		args...).Scan(&total, &filtered); err != nil {
		return 0, 0, fmt.Errorf("questions: counts: %w", err)
	}
	return total, filtered, nil
}

func (s *Postgres) metadataFacetCounts(ctx context.Context, in domain.ListInput, opts filterOpts, dimension string) (map[string]int, error) {
	args, where := appendFilters(in, opts)
	values, err := s.Query(ctx, `SELECT q.`+dimension+`, count(*) FROM app.questions q WHERE `+strings.Join(where, " AND ")+` AND q.`+dimension+` IS NOT NULL GROUP BY q.`+dimension, args...)
	if err != nil {
		return nil, fmt.Errorf("questions: metadata facets: %w", err)
	}
	defer values.Close()
	counts := map[string]int{}
	for values.Next() {
		var value string
		var count int
		if err := values.Scan(&value, &count); err != nil {
			return nil, fmt.Errorf("questions: metadata facet: %w", err)
		}
		counts[value] = count
	}
	if err := values.Err(); err != nil {
		return nil, fmt.Errorf("questions: metadata facets: %w", err)
	}
	return counts, nil
}

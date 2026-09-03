package questions

import (
	"context"
	"fmt"
	"quizzivy/internal/paging"
	"strings"
)

// Page-size bounds for the bank listing.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// ListInput selects a page of the bank.
//
// Types and Tags are each OR-ed within themselves and AND-ed with each other,
// which is what A-06's rail of checkboxes and chips means: ticking a second
// type widens the results, adding a tag from the other group narrows them.
type ListInput struct {
	Types    []Type
	Tags     []string
	HasAudio *bool
	Query    string
	Page     int
	Limit    int
}

// TrigramExpression must match questions_prompt_trgm_idx verbatim; Postgres
// uses an expression index only on an identical expression. Shared with the
// EXPLAIN test in search_test.go.
const TrigramExpression = `app.immutable_unaccent(lower(q.prompt))`

// searchCondition ORs word matching against the tsvector index with
// accent-folded substring matching against the trigram index, so both serve one
// query. The fold is explicit because pg_trgm is case- but not
// accent-insensitive.
const searchCondition = `(
		to_tsvector('simple', q.prompt) @@ plainto_tsquery('simple', $%[1]d)
		OR ` + TrigramExpression + ` LIKE '%%' || app.immutable_unaccent(lower($%[1]d)) || '%%' ESCAPE '\'
	)`

// likeEscaper neutralises LIKE wildcards in user input. Backslash first, or it
// would double the escapes the others add.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string { return likeEscaper.Replace(s) }

// buildFilters returns the bound arguments and WHERE clauses for one page.
// appendFilters adds the bank's WHERE clauses onto whatever arguments the
// caller has already bound, so a paged query and a bare count can share one
// definition of "what is being filtered" instead of drifting apart.
//
// Which filters apply is the caller's choice: the type facets deliberately skip
// the type filter, and the tag rail deliberately skips the tag filter, because
// a facet that applies its own dimension zeroes every row the teacher has not
// picked.
func appendFilters(in ListInput, args []any, opts filterOpts) ([]any, []string) {
	where := []string{`q.deleted_at IS NULL`}

	if opts.types && len(in.Types) > 0 {
		types := make([]string, len(in.Types))
		for i, t := range in.Types {
			types[i] = string(t)
		}
		args = append(args, types)
		where = append(where, fmt.Sprintf(`q.type = ANY($%d::app.question_type[])`, len(args)))
	}
	if opts.tags && len(in.Tags) > 0 {
		// Overlap, not containment: two ticked chips mean "either", the same
		// way two ticked types do. `@>` would mean "carries both", which reads
		// as a filter that gets stricter the more of the rail you touch.
		args = append(args, in.Tags)
		where = append(where, fmt.Sprintf(`q.tags && $%d::text[]`, len(args)))
	}
	if in.HasAudio != nil {
		args = append(args, *in.HasAudio)
		where = append(where, fmt.Sprintf(
			`(q.media_asset_kind = 'audio') = $%d::boolean`, len(args)))
	}
	if q := strings.TrimSpace(in.Query); q != "" {
		args = append(args, escapeLike(q))
		where = append(where, fmt.Sprintf(searchCondition, len(args)))
	}
	return args, where
}

type filterOpts struct {
	types bool
	tags  bool
}

// allFilters is every dimension: what the page itself is filtered by.
func allFilters() filterOpts {
	return filterOpts{types: true, tags: true}
}

// List returns one page of live bank questions, newest first, with the
// paging beside it (O-20: OFFSET, so the client can draw numbered pages).
func (s *Store) List(ctx context.Context, in ListInput) ([]Question, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	args, where := appendFilters(in, nil, allFilters())
	from := `
		  FROM app.questions q
		 WHERE ` + strings.Join(where, "\n		   AND ")

	page := paging.Page{Number: number, Size: limit}
	if err := s.pool.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("questions: count: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, `SELECT`+questionColumns+from+fmt.Sprintf(`
		 ORDER BY q.id DESC
		 LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("questions: list: %w", err)
	}
	defer rows.Close()

	questions := make([]Question, 0, limit)
	for rows.Next() {
		question, err := scanQuestion(rows)
		if err != nil {
			return nil, paging.Page{}, err
		}
		questions = append(questions, question)
	}
	if err := rows.Err(); err != nil {
		return nil, paging.Page{}, fmt.Errorf("questions: list: %w", err)
	}

	if err := s.attachChildren(ctx, questions); err != nil {
		return nil, paging.Page{}, err
	}
	return questions, page, nil
}

// attachChildren fills in the options and blanks for a whole page in two
// queries rather than two per row.
//
// The contract's AdminQuestion carries both, and a list that returned a
// different shape from the detail endpoint is something a client works around
// rather than reports -- so the shape stays and the fetching changes. A full
// page of 100 was 201 round trips, which is paid in network latency against
// Neon on the teacher's main browsing screen.
func (s *Store) attachChildren(ctx context.Context, questions []Question) error {
	if len(questions) == 0 {
		return nil
	}

	ids := make([]string, len(questions))
	for i, q := range questions {
		ids[i] = q.ID
	}

	options, err := s.loadOptionsFor(ctx, s.pool, ids)
	if err != nil {
		return err
	}
	blanks, err := s.loadBlanksFor(ctx, s.pool, ids)
	if err != nil {
		return err
	}

	for i := range questions {
		// Non-nil even when absent: the contract's arrays are never null.
		questions[i].Options = []Option{}
		questions[i].Blanks = []Blank{}
		if got, ok := options[questions[i].ID]; ok {
			questions[i].Options = got
		}
		if got, ok := blanks[questions[i].ID]; ok {
			questions[i].Blanks = got
		}
	}
	return nil
}

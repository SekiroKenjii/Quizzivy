package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/paging"
	"strings"
)

// Page-size bounds for the bank listing.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// TrigramExpression must match questions_prompt_trgm_idx verbatim; Postgres
// uses an expression index only on an identical expression. Shared with the
// EXPLAIN test in search_test.go.
const TrigramExpression = `app.immutable_unaccent(lower(q.prompt))`

const searchCondition = `(
		to_tsvector('simple', q.prompt) @@ plainto_tsquery('simple', $%[1]d)
		OR ` + TrigramExpression + ` LIKE '%%' || app.immutable_unaccent(lower($%[1]d)) || '%%' ESCAPE '\'
	)`

func appendFilters(in domain.ListInput, opts filterOpts) ([]any, []string) {
	var args []any
	where := []string{`q.deleted_at IS NULL`, `q.context_group_id IS NULL`}

	if opts.types && len(in.Types) > 0 {
		types := make([]string, len(in.Types))
		for i, t := range in.Types {
			types[i] = string(t)
		}
		args = append(args, types)
		where = append(where, fmt.Sprintf(`q.type = ANY($%d::app.question_type[])`, len(args)))
	}
	if opts.tags && len(in.Tags) > 0 {
		args = append(args, in.Tags)
		where = append(where, fmt.Sprintf(`q.tags && $%d::text[]`, len(args)))
	}
	if in.HasAudio != nil {
		args = append(args, *in.HasAudio)
		where = append(where, fmt.Sprintf(
			`(q.media_asset_kind = 'audio') = $%d::boolean`, len(args)))
	}
	if q := strings.TrimSpace(in.Query); q != "" {
		args = append(args, db.EscapeLike(q))
		where = append(where, fmt.Sprintf(searchCondition, len(args)))
	}
	return args, where
}

type filterOpts struct {
	types bool
	tags  bool
}

func allFilters() filterOpts {
	return filterOpts{types: true, tags: true}
}

// List returns one page of live bank questions, newest first, with the
// paging beside it (O-20: OFFSET, so the client can draw numbered pages).
func (s *Postgres) List(ctx context.Context, in domain.ListInput) ([]domain.Question, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	args, where := appendFilters(in, allFilters())
	from := `
		  FROM app.questions q
		 WHERE ` + strings.Join(where, "\n		   AND ")

	page := paging.Page{Number: number, Size: limit}
	if err := s.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("questions: count: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := s.Query(ctx, `SELECT`+questionColumns+from+fmt.Sprintf(`
		 ORDER BY q.id DESC
		 LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("questions: list: %w", err)
	}
	defer rows.Close()

	questions := make([]domain.Question, 0, limit)
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

func (s *Postgres) attachChildren(ctx context.Context, questions []domain.Question) error {
	if len(questions) == 0 {
		return nil
	}

	ids := make([]string, len(questions))
	for i, q := range questions {
		ids[i] = q.ID
	}

	options, err := s.loadOptionsFor(ctx, s.Conn(), ids)
	if err != nil {
		return err
	}
	blanks, err := s.loadBlanksFor(ctx, s.Conn(), ids)
	if err != nil {
		return err
	}

	for i := range questions {

		questions[i].Options = []domain.Option{}
		questions[i].Blanks = []domain.Blank{}
		if got, ok := options[questions[i].ID]; ok {
			questions[i].Options = got
		}
		if got, ok := blanks[questions[i].ID]; ok {
			questions[i].Blanks = got
		}
	}
	return nil
}

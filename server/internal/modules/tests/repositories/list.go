package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/paging"
	"strings"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

const titleSearch = `app.immutable_unaccent(lower(t.title))` +
	` LIKE '%%' || app.immutable_unaccent(lower($%[1]d)) || '%%' ESCAPE '\'`

const tagCondition = `EXISTS (
		SELECT 1
		  FROM app.test_sections s
		  JOIN app.test_section_questions sq ON sq.test_section_id = s.id
		  JOIN app.questions q ON q.id = sq.question_id AND q.deleted_at IS NULL
		 WHERE s.test_id = t.id AND q.tags && $%d::text[]
	)`

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string { return likeEscaper.Replace(s) }

// List returns one page of live tests, newest first, and the paging that
// goes with it (O-20: OFFSET, so the client can draw numbered pages).
func (s *Postgres) List(ctx context.Context, in domain.ListInput) ([]domain.Test, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)

	var args []any
	where := []string{liveTests}
	if in.Status != nil {
		args = append(args, string(*in.Status))
		where = append(where, fmt.Sprintf(`t.status = $%d::app.test_status`, len(args)))
	}
	if len(in.Tags) > 0 {
		args = append(args, in.Tags)
		where = append(where, fmt.Sprintf(tagCondition, len(args)))
	}
	if q := strings.TrimSpace(in.Query); q != "" {
		args = append(args, escapeLike(q))
		where = append(where, fmt.Sprintf(titleSearch, len(args)))
	}
	from := `
		  FROM app.tests t
		 WHERE ` + strings.Join(where, "\n		   AND ")

	page := paging.Page{Number: number, Size: limit}
	if err := s.pool.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&page.Total); err != nil {
		return nil, paging.Page{}, fmt.Errorf("tests: count: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := s.pool.Query(ctx, `SELECT`+testColumns+from+fmt.Sprintf(`
		 ORDER BY t.id DESC
		 LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, paging.Page{}, fmt.Errorf("tests: list: %w", err)
	}
	defer rows.Close()

	list := make([]domain.Test, 0, limit)
	for rows.Next() {
		t, err := scanTest(rows)
		if err != nil {
			return nil, paging.Page{}, err
		}
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		return nil, paging.Page{}, fmt.Errorf("tests: list: %w", err)
	}

	if err := s.attachSections(ctx, list); err != nil {
		return nil, paging.Page{}, err
	}
	return list, page, nil
}

func (s *Postgres) attachSections(ctx context.Context, list []domain.Test) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]string, len(list))
	for i, t := range list {
		ids[i] = t.ID
	}

	byTest, err := s.sectionsFor(ctx, s.pool, ids)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].Sections = []domain.Section{}
		if got, ok := byTest[list[i].ID]; ok {
			list[i].Sections = got
		}
	}
	return nil
}

// Tags returns every tag reachable through the current status and search, so
// A-03's filter cannot offer a chip that returns nothing.
func (s *Postgres) Tags(ctx context.Context, in domain.ListInput) ([]string, error) {
	args := []any{}
	where := []string{liveTests}
	if in.Status != nil {
		args = append(args, string(*in.Status))
		where = append(where, fmt.Sprintf(`t.status = $%d::app.test_status`, len(args)))
	}
	if q := strings.TrimSpace(in.Query); q != "" {
		args = append(args, escapeLike(q))
		where = append(where, fmt.Sprintf(titleSearch, len(args)))
	}

	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT unnest(q.tags)
		  FROM app.tests t
		  JOIN app.test_sections sec ON sec.test_id = t.id
		  JOIN app.test_section_questions sq ON sq.test_section_id = sec.id
		  JOIN app.questions q ON q.id = sq.question_id AND q.deleted_at IS NULL
		 WHERE `+strings.Join(where, " AND ")+`
		 ORDER BY 1`, args...)
	if err != nil {
		return nil, fmt.Errorf("tests: tags: %w", err)
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("tests: scan tag: %w", err)
		}
		out = append(out, tag)
	}
	return out, rows.Err()
}

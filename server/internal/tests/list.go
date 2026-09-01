package tests

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// ListInput selects a page of tests.
type ListInput struct {
	Status *Status
	// Tags filter by the tags of the questions a test CONTAINS. §7 gives Test no
	// tags of its own, and a test is its questions -- OR-ed, like the bank's.
	Tags   []string
	Query  string
	Cursor string
	Limit  int
}

func encodeCursor(id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id))
}

func decodeCursor(s string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", ErrBadCursor
	}
	if _, err := uuid.Parse(string(raw)); err != nil {
		return "", ErrBadCursor
	}
	return string(raw), nil
}

// titleSearch folds accents on both sides, so a teacher typing without
// diacritics finds a title that has them -- the same rule the Query parameter
// states for the question bank.
//
// There is NO index for this expression, unlike questions_prompt_trgm_idx: the
// data model gives app.tests only tests_status_id_idx, and a teacher's tests
// number in the dozens, so the sequential scan is the cheaper answer. If this
// table ever grows, the fix is a gin_trgm_ops index on the identical
// expression -- Postgres matches an expression index only on an exact match.
const titleSearch = `app.immutable_unaccent(lower(t.title))` +
	` LIKE '%%' || app.immutable_unaccent(lower($%[1]d)) || '%%' ESCAPE '\'`

// tagCondition matches a test through its DRAFT outline, which is the copy the
// list's other numbers already describe -- questionCount and totalPoints are
// computed the same way. Overlap, not containment: two chips widen, as in the
// bank.
const tagCondition = `EXISTS (
		SELECT 1
		  FROM app.test_sections s
		  JOIN app.test_section_questions sq ON sq.test_section_id = s.id
		  JOIN app.questions q ON q.id = sq.question_id AND q.deleted_at IS NULL
		 WHERE s.test_id = t.id AND q.tags && $%d::text[]
	)`

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string { return likeEscaper.Replace(s) }

// List returns one page of live tests, newest first.
func (s *Store) List(ctx context.Context, in ListInput) ([]Test, string, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	var after string
	if in.Cursor != "" {
		id, err := decodeCursor(in.Cursor)
		if err != nil {
			return nil, "", err
		}
		after = id
	}

	args := []any{limit + 1}
	where := []string{`t.deleted_at IS NULL`}
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
	if after != "" {
		args = append(args, after)
		where = append(where, fmt.Sprintf(`t.id < $%d::uuid`, len(args)))
	}

	sql := `SELECT` + testColumns + `
		  FROM app.tests t
		 WHERE ` + strings.Join(where, "\n		   AND ") + `
		 ORDER BY t.id DESC
		 LIMIT $1`

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, "", fmt.Errorf("tests: list: %w", err)
	}
	defer rows.Close()

	list := make([]Test, 0, limit)
	for rows.Next() {
		t, err := scanTest(rows)
		if err != nil {
			return nil, "", err
		}
		list = append(list, t)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("tests: list: %w", err)
	}

	var next string
	if len(list) > limit {
		list = list[:limit]
		next = encodeCursor(list[len(list)-1].ID)
	}

	if err := s.attachSections(ctx, list); err != nil {
		return nil, "", err
	}
	return list, next, nil
}

// attachSections fills the outline for a whole page in one query rather than
// one per row.
func (s *Store) attachSections(ctx context.Context, list []Test) error {
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
		list[i].Sections = []Section{}
		if got, ok := byTest[list[i].ID]; ok {
			list[i].Sections = got
		}
	}
	return nil
}

// Tags returns every tag reachable through the current status and search, so
// A-03's filter cannot offer a chip that returns nothing.
//
// The tag filter itself is NOT applied, for the same reason the status facets
// ignore the status filter: picking one chip must not empty the rail.
func (s *Store) Tags(ctx context.Context, in ListInput) ([]string, error) {
	args := []any{}
	where := []string{`t.deleted_at IS NULL`}
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

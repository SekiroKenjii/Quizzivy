package questions

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Page-size bounds for the bank listing.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// ListInput selects a page of the bank.
type ListInput struct {
	Type   *Type
	Tag    string
	Query  string
	Cursor string
	Limit  int
}

// encodeCursor renders a keyset position opaquely. A uuidv7 id is both
// time-ordered and unique, so id DESC alone is a strict total order.
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

// List returns one page of live bank questions, newest first.
func (s *Store) List(ctx context.Context, in ListInput) ([]Question, string, error) {
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
	where := []string{`q.deleted_at IS NULL`}

	if in.Type != nil {
		args = append(args, string(*in.Type))
		where = append(where, fmt.Sprintf(`q.type = $%d::app.question_type`, len(args)))
	}
	if in.Tag != "" {
		args = append(args, []string{in.Tag})
		where = append(where, fmt.Sprintf(`q.tags @> $%d::text[]`, len(args)))
	}
	if q := strings.TrimSpace(in.Query); q != "" {
		args = append(args, escapeLike(q))
		where = append(where, fmt.Sprintf(searchCondition, len(args)))
	}
	if after != "" {
		args = append(args, after)
		where = append(where, fmt.Sprintf(`q.id < $%d::uuid`, len(args)))
	}

	sql := `SELECT` + questionColumns + `
		  FROM app.questions q
		 WHERE ` + strings.Join(where, "\n		   AND ") + `
		 ORDER BY q.id DESC
		 LIMIT $1`

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, "", fmt.Errorf("questions: list: %w", err)
	}
	defer rows.Close()

	questions := make([]Question, 0, limit)
	for rows.Next() {
		question, err := scanQuestion(rows)
		if err != nil {
			return nil, "", err
		}
		questions = append(questions, question)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("questions: list: %w", err)
	}

	// One row beyond the page detects a next page without a second query.
	var next string
	if len(questions) > limit {
		questions = questions[:limit]
		next = encodeCursor(questions[len(questions)-1].ID)
	}

	for i := range questions {
		if questions[i].Options, err = s.loadOptions(ctx, s.pool, questions[i].ID); err != nil {
			return nil, "", err
		}
		if questions[i].Blanks, err = s.loadBlanks(ctx, s.pool, questions[i].ID); err != nil {
			return nil, "", err
		}
	}
	return questions, next, nil
}

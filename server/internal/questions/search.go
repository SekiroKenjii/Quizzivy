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

// encodeCursor renders a position opaquely.
//
// The id alone is enough here, unlike the media library's (created_at, id).
// `id` is a uuidv7 and therefore time-ordered (§13.2), and it is unique, so
// `id DESC` is both a valid recency order AND a strict total order -- which is
// what keyset pagination needs to avoid serving a row twice at a page boundary.
// It also matches questions_type_id_idx (type, id DESC) exactly.
func encodeCursor(id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(id))
}

func decodeCursor(s string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", ErrBadCursor
	}
	// Parsed, not pattern-matched: a non-uuid here means the cursor did not
	// come from us, whatever it looks like.
	if _, err := uuid.Parse(string(raw)); err != nil {
		return "", ErrBadCursor
	}
	return string(raw), nil
}

// TrigramExpression is the indexed expression, in ONE place.
//
// questions_prompt_trgm_idx is declared on
// `app.immutable_unaccent(lower(prompt))`, and Postgres matches an expression
// index only when the query spells the expression IDENTICALLY. Writing it out
// at the call site is how a later edit -- `lower(app.immutable_unaccent(...))`,
// say, which is not the same expression -- silently drops the query to a
// sequential scan while still returning correct rows. Sharing the constant with
// the EXPLAIN test is what keeps that from being discovered in production.
const TrigramExpression = `app.immutable_unaccent(lower(q.prompt))`

// searchCondition is the D-11 pair, in one query as T-2.6 requires.
//
// Two indexes, ORed, because they answer different questions:
//
//   - `to_tsvector('simple', prompt) @@ plainto_tsquery('simple', $q)` is word
//     matching. 'simple' does no stemming and no diacritic folding, so it finds
//     whole words as written.
//   - the trigram half is substring AND accent-insensitive matching, which is
//     what makes `nghe` find `nghé` and `duong` find `Đường` in a
//     Vietnamese-first product. pg_trgm alone does NOT do this -- it is
//     case-insensitive but not accent-insensitive -- which is why the fold is
//     explicit on both sides.
//
// An OR of two indexed conditions is a BitmapOr, so both indexes can be used
// for one query rather than one of them being decorative.
//
// The pattern side is escaped in Go (see escapeLike) rather than in SQL, so
// that only the LEFT side has to match the index expression verbatim.
const searchCondition = `(
		to_tsvector('simple', q.prompt) @@ plainto_tsquery('simple', $%[1]d)
		OR ` + TrigramExpression + ` LIKE '%%' || app.immutable_unaccent(lower($%[1]d)) || '%%' ESCAPE '\'
	)`

// escapeLike neutralises the LIKE wildcards in user input.
//
// Without it a search for `%` matches every question in the bank and a search
// for `_` matches any single character -- not a security hole, since the value
// is still a bound parameter and never concatenated into SQL, but a search box
// where two ordinary characters mean something surprising.
//
// Backslash first: escaping it after the others would double the escapes they
// just added.
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
		// The GIN index on tags serves containment; combining it with the type
		// filter is a bitmap AND, which is correct.
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

	// One row beyond the page tells us whether there is a next one, without a
	// second query that could disagree under concurrent inserts.
	var next string
	if len(questions) > limit {
		questions = questions[:limit]
		next = encodeCursor(questions[len(questions)-1].ID)
	}

	// Children are loaded per row. The bank list renders prompts and type
	// badges, so this could be skipped -- but the contract's AdminQuestion
	// carries options and blanks, and a list that returns a DIFFERENT shape
	// from the detail endpoint is the kind of thing a client works around
	// rather than reports.
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

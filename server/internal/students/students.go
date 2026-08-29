package students

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

var ErrBadCursor = errors.New("students: malformed cursor")

// Student is §7's User narrowed to the role this listing returns.
type Student struct {
	ID                 string
	Email              string
	FullName           string
	HasPassword        bool
	LinkedProviders    []string
	MustChangePassword bool
	CreatedAt          time.Time
}

type ListInput struct {
	Query string
	// ClassID narrows to one class's roster, which is how the pickers ask "who
	// is not in this class yet" by diffing against it.
	ClassID string
	Cursor  string
	Limit   int
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func encodeCursor(id string) string { return base64.RawURLEncoding.EncodeToString([]byte(id)) }

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

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// List returns one page of students, newest first.
//
// Search folds accents on both sides so "hân" and "han" find the same person --
// the same rule the question bank uses, and the one a Vietnamese-first product
// needs. There is no trigram index behind it: §1.3 caps this table at tens of
// rows, where an index would cost more to maintain than the scan it saves.
func (s *Store) List(ctx context.Context, in ListInput) ([]Student, string, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	args := []any{limit + 1}
	where := []string{`u.role = 'student'`, `u.disabled_at IS NULL`}

	if in.Query != "" {
		args = append(args, likeEscaper.Replace(in.Query))
		where = append(where, fmt.Sprintf(`(
			app.immutable_unaccent(lower(u.full_name))
				LIKE '%%' || app.immutable_unaccent(lower($%[1]d)) || '%%' ESCAPE '\'
			OR lower(u.email) LIKE '%%' || lower($%[1]d) || '%%' ESCAPE '\'
		)`, len(args)))
	}
	if in.ClassID != "" {
		args = append(args, in.ClassID)
		where = append(where, fmt.Sprintf(
			`EXISTS (SELECT 1 FROM app.class_members m
			          WHERE m.user_id = u.id AND m.class_id = $%d::uuid)`, len(args)))
	}
	if in.Cursor != "" {
		id, err := decodeCursor(in.Cursor)
		if err != nil {
			return nil, "", err
		}
		args = append(args, id)
		where = append(where, fmt.Sprintf(`u.id < $%d::uuid`, len(args)))
	}

	rows, err := s.pool.Query(ctx, `
		SELECT u.id::text, u.email, u.full_name,
		       u.password_hash IS NOT NULL,
		       coalesce((SELECT array_agg(i.provider::text)
		                   FROM app.user_identities i WHERE i.user_id = u.id), '{}'),
		       u.must_change_password, u.created_at
		  FROM app.users u
		 WHERE `+strings.Join(where, "\n		   AND ")+`
		 ORDER BY u.id DESC
		 LIMIT $1`, args...)
	if err != nil {
		return nil, "", fmt.Errorf("students: list: %w", err)
	}
	defer rows.Close()

	var out []Student
	for rows.Next() {
		var student Student
		if err := rows.Scan(&student.ID, &student.Email, &student.FullName,
			&student.HasPassword, &student.LinkedProviders,
			&student.MustChangePassword, &student.CreatedAt); err != nil {
			return nil, "", fmt.Errorf("students: scan: %w", err)
		}
		out = append(out, student)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("students: list: %w", err)
	}

	next := ""
	if len(out) > limit {
		out = out[:limit]
		next = encodeCursor(out[len(out)-1].ID)
	}
	return out, next, nil
}

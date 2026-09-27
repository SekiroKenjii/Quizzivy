// Package repositories reads the access model from app.users, app.roles,
// app.role_permissions and app.permissions. The app role may read them and,
// until R5's role commands, nothing else.
package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

// Postgres reads the access model from the database.
type Postgres struct{ db.Repository }

// NewPostgres builds the repository on the database context.
func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

var _ domain.Repository = (*Postgres)(nil)

// Principal reads a user's role, disabled state and session epoch in one
// statement.
func (s *Postgres) Principal(ctx context.Context, userID string) (domain.PrincipalRow, error) {
	var row domain.PrincipalRow
	var builtin *string
	err := s.QueryRow(ctx, `
		SELECT u.role_id::text, r.builtin_key, r.revision, u.disabled_at IS NOT NULL, u.session_epoch
		  FROM app.users u
		  JOIN app.roles r ON r.id = u.role_id
		 WHERE u.id = $1`, userID).Scan(&row.Role.ID, &builtin, &row.Role.Revision, &row.Disabled, &row.Epoch)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PrincipalRow{}, domain.ErrUnknownUser
	}
	if err != nil {
		return domain.PrincipalRow{}, err
	}
	if builtin != nil {
		row.Role.Builtin = access.Builtin(*builtin)
	}
	return row, nil
}

// Grants returns the keys a role stores.
func (s *Postgres) Grants(ctx context.Context, roleID string) (access.Set, error) {
	rows, err := s.Query(ctx, `SELECT permission_key FROM app.role_permissions WHERE role_id = $1`, roleID)
	if err != nil {
		return access.Set{}, err
	}
	keys, err := pgx.CollectRows(rows, pgx.RowTo[access.Key])
	if err != nil {
		return access.Set{}, err
	}
	return access.NewSet(keys...), nil
}

// Catalogue returns app.permissions' keys in matrix order.
func (s *Postgres) Catalogue(ctx context.Context) ([]access.Key, error) {
	rows, err := s.Query(ctx, `
		SELECT key
		  FROM app.permissions
		 ORDER BY array_position(ARRAY['content', 'teaching', 'people', 'system'], group_key), ordinal`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[access.Key])
}

// Roles returns every role with its grants, the built-in roles first in the
// order Admin, Teacher, Assistant, Student, then custom roles oldest first.
func (s *Postgres) Roles(ctx context.Context) ([]domain.RoleRow, error) {
	rows, err := s.Query(ctx, `
		SELECT r.id::text, r.builtin_key, r.name, r.revision,
		       coalesce(array_agg(rp.permission_key) FILTER (WHERE rp.permission_key IS NOT NULL), '{}')
		  FROM app.roles r
		  LEFT JOIN app.role_permissions rp ON rp.role_id = r.id
		 GROUP BY r.id
		 ORDER BY coalesce(array_position(ARRAY['admin', 'teacher', 'assistant', 'student'], r.builtin_key), 5),
		          r.created_at, r.id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.RoleRow, error) {
		var out domain.RoleRow
		var builtin *string
		var grants []access.Key
		if err := row.Scan(&out.ID, &builtin, &out.Name, &out.Revision, &grants); err != nil {
			return domain.RoleRow{}, err
		}
		if builtin != nil {
			out.Builtin = access.Builtin(*builtin)
		}
		out.Grants = access.NewSet(grants...)
		return out, nil
	})
}

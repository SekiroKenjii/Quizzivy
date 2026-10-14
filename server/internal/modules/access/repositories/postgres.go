// Package repositories reads the access model from app.users, app.roles,
// app.role_permissions and app.permissions, and writes roles and their grants.
package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
)

// Postgres reads the access model from the database.
type Postgres struct{ db.Repository }

// NewPostgres builds the repository on the database context.
func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

var (
	_ domain.Repository = (*Postgres)(nil)
	_ domain.RoleStore  = (*Postgres)(nil)
)

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
		SELECT r.id::text, r.builtin_key, r.name, r.description, r.icon, r.color, r.revision,
		       coalesce(array_agg(rp.permission_key) FILTER (WHERE rp.permission_key IS NOT NULL), '{}')
		  FROM app.roles r
		  LEFT JOIN app.role_permissions rp ON rp.role_id = r.id
		 GROUP BY r.id
		 ORDER BY coalesce(array_position(ARRAY['admin', 'teacher', 'assistant', 'student'], r.builtin_key), 5),
		          r.created_at, r.id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanRole)
}

func scanRole(row pgx.CollectableRow) (domain.RoleRow, error) {
	var out domain.RoleRow
	var builtin *string
	var grants []access.Key
	if err := row.Scan(&out.ID, &builtin, &out.Name, &out.Description, &out.Icon, &out.Color, &out.Revision, &grants); err != nil {
		return domain.RoleRow{}, err
	}
	if builtin != nil {
		out.Builtin = access.Builtin(*builtin)
	}
	out.Grants = access.NewSet(grants...)
	return out, nil
}

// Permissions returns app.permissions' rows in matrix order.
func (s *Postgres) Permissions(ctx context.Context) ([]domain.PermissionRow, error) {
	rows, err := s.Query(ctx, `
		SELECT key, group_key, in_matrix
		  FROM app.permissions
		 ORDER BY array_position(ARRAY['content', 'teaching', 'people', 'system'], group_key), ordinal`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.PermissionRow, error) {
		var out domain.PermissionRow
		err := row.Scan(&out.Key, &out.Group, &out.InMatrix)
		return out, err
	})
}

// Headcounts returns the accounts and the attempts in progress of every role
// that has an account, by role id.
func (s *Postgres) Headcounts(ctx context.Context) (map[string]domain.Headcount, error) {
	rows, err := s.Query(ctx, `
		SELECT u.role_id::text, count(DISTINCT u.id), count(a.id)
		  FROM app.users u
		  LEFT JOIN app.attempts a ON a.student_id = u.id AND a.status = 'in_progress'
		 GROUP BY u.role_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]domain.Headcount{}
	for rows.Next() {
		var id string
		var h domain.Headcount
		if err := rows.Scan(&id, &h.Users, &h.InProgressAttempts); err != nil {
			return nil, err
		}
		out[id] = h
	}
	return out, rows.Err()
}

// InTx runs fn in one transaction, committed when it returns nil.
func (s *Postgres) InTx(ctx context.Context, fn func(ctx context.Context, tx domain.RoleTx) error) error {
	return s.Context.InTx(ctx, "access: role write", func(tx pgx.Tx) error {
		return fn(ctx, roleTx{tx: tx})
	})
}

type roleTx struct{ tx pgx.Tx }

const roleColumns = `
		SELECT r.id::text, r.builtin_key, r.name, r.description, r.icon, r.color, r.revision,
		       coalesce(array_agg(rp.permission_key) FILTER (WHERE rp.permission_key IS NOT NULL), '{}')
		  FROM app.roles r
		  LEFT JOIN app.role_permissions rp ON rp.role_id = r.id`

func (t roleTx) Lock(ctx context.Context, ids []string) ([]domain.RoleRow, error) {
	if _, err := t.tx.Exec(ctx, `SELECT 1 FROM app.roles WHERE id = ANY($1::uuid[]) ORDER BY id FOR UPDATE`, ids); err != nil {
		return nil, err
	}
	rows, err := t.tx.Query(ctx, roleColumns+` WHERE r.id = ANY($1::uuid[]) GROUP BY r.id ORDER BY r.id`, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanRole)
}

func (t roleTx) LockShared(ctx context.Context, id string) (domain.RoleRow, error) {
	if _, err := t.tx.Exec(ctx, `SELECT 1 FROM app.roles WHERE id = $1::uuid FOR SHARE`, id); err != nil {
		return domain.RoleRow{}, err
	}
	return t.Row(ctx, id)
}

func (t roleTx) Row(ctx context.Context, id string) (domain.RoleRow, error) {
	rows, err := t.tx.Query(ctx, roleColumns+` WHERE r.id = $1::uuid GROUP BY r.id`, id)
	if err != nil {
		return domain.RoleRow{}, err
	}
	role, err := pgx.CollectOneRow(rows, scanRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RoleRow{}, domain.ErrRoleNotFound
	}
	return role, err
}

func (t roleTx) Insert(ctx context.Context, role domain.NewRole) (string, error) {
	var id string
	err := t.tx.QueryRow(ctx, `
		INSERT INTO app.roles (name, description, icon, color, copied_from, created_by)
		VALUES ($1, $2, $3, $4, $5::uuid, $6::uuid)
		RETURNING id::text`,
		role.Name, role.Description, domain.NewRoleIcon, domain.NewRoleColor, opt.String(role.CopiedFrom), role.By.ID).Scan(&id)
	if db.IsUniqueViolation(err, nameKey) {
		return "", domain.ErrNameTaken
	}
	return id, err
}

const nameKey = "roles_name_lower_key"

func (t roleTx) Update(ctx context.Context, edit domain.RoleEdit) error {
	_, err := t.tx.Exec(ctx, `
		UPDATE app.roles
		   SET name = coalesce($2, name), description = coalesce($3, description),
		       icon = coalesce($4, icon), color = coalesce($5, color), revision = revision + 1
		 WHERE id = $1::uuid`,
		edit.ID, edit.Name, edit.Description, edit.Icon, edit.Color)
	if db.IsUniqueViolation(err, nameKey) {
		return domain.ErrNameTaken
	}
	return err
}

func (t roleTx) Grant(ctx context.Context, roleID string, keys []access.Key) error {
	_, err := t.tx.Exec(ctx, `
		INSERT INTO app.role_permissions (role_id, permission_key)
		SELECT $1::uuid, k FROM unnest($2::text[]) AS k
		ON CONFLICT DO NOTHING`, roleID, keys)
	return err
}

func (t roleTx) Revoke(ctx context.Context, roleID string, keys []access.Key) error {
	_, err := t.tx.Exec(ctx, `DELETE FROM app.role_permissions WHERE role_id = $1::uuid AND permission_key = ANY($2::text[])`, roleID, keys)
	return err
}

func (t roleTx) Audit(ctx context.Context, entry domain.AuditEntry) error {
	return audit.Write(ctx, t.tx, audit.Entry{
		ActorUserID: opt.String(entry.By.ID), Action: entry.Action, Entity: domain.AuditEntity, EntityID: &entry.RoleID,
		OccurredAt: entry.At, IP: entry.By.IPValue(), UserAgent: entry.By.UserAgentValue(), Diff: entry.Diff,
	})
}

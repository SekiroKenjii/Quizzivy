package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"

	"github.com/jackc/pgx/v5"
)

// AdminUsers is the account directory of the Admin console. Its links store
// writes the invite of an account created with an email link on the transaction
// that creates the account; a nil one cannot send an invite.
type AdminUsers struct {
	db.Repository
	links *SetPasswordLinks
}

// NewAdminUsers returns the directory over dbx.
func NewAdminUsers(dbx db.Context, links *SetPasswordLinks) *AdminUsers {
	return &AdminUsers{Repository: db.NewRepository(dbx), links: links}
}

const adminUserSelect = `
	SELECT u.id::text, u.full_name, u.email, u.avatar_key,
	       r.id::text, r.name, coalesce(r.builtin_key, ''), r.icon, r.color,
	       u.password_hash IS NOT NULL,
	       EXISTS (SELECT 1 FROM app.user_identities i WHERE i.user_id = u.id AND i.provider = 'google'),
	       k.total, k.first_name,
	       u.last_active_at,
	       ` + accountStatusSQL + `,
	       u.created_at
	  FROM app.users u
	  JOIN app.roles r ON r.id = u.role_id
	 CROSS JOIN LATERAL (
	       SELECT count(*)::int AS total, (array_agg(c.name ORDER BY c.at, c.id))[1] AS first_name
	         FROM (SELECT c.id, c.name, m.joined_at AS at
	                 FROM app.class_members m
	                 JOIN app.classes c ON c.id = m.class_id
	                WHERE m.user_id = u.id AND c.archived_at IS NULL
	                  AND EXISTS (SELECT 1 FROM app.student_like_roles s WHERE s.id = u.role_id)
	               UNION ALL
	               SELECT c.id, c.name, c.created_at
	                 FROM app.classes c
	                WHERE c.teacher_id = u.id AND c.archived_at IS NULL
	                  AND NOT EXISTS (SELECT 1 FROM app.student_like_roles s WHERE s.id = u.role_id)) c) k`

func scanAdminUser(row pgx.Row) (domain.AdminUser, error) {
	var u domain.AdminUser
	var status string
	err := row.Scan(&u.ID, &u.FullName, &u.Email, &u.AvatarKey,
		&u.Role.ID, &u.Role.Name, &u.Role.BuiltinKey, &u.Role.Icon, &u.Role.Color,
		&u.HasPassword, &u.HasGoogle, &u.ClassCount, &u.FirstClass,
		&u.LastActiveAt, &status, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AdminUser{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.AdminUser{}, fmt.Errorf("users: scan: %w", err)
	}
	u.Status = domain.AccountStatus(status)
	return u, nil
}

// Create adds an account created by req's actor, with the role in, and a
// password only for domain.DeliveryTemporaryPassword; the account must change
// that password at its first sign-in unless req keeps it. A class in must not
// be archived. issue, when not nil, runs on the same transaction.
func (a *AdminUsers) Create(ctx context.Context, req domain.WriteRequest, in domain.NewUser, issue domain.LinkIssue) (domain.AdminUser, error) {
	diff, err := json.Marshal(struct {
		Role     string `json:"role"`
		RoleID   string `json:"roleId"`
		Delivery string `json:"delivery"`
	}{in.RoleName, in.RoleID, string(in.Delivery)})
	if err != nil {
		return domain.AdminUser{}, fmt.Errorf("users: encode audit diff: %w", err)
	}
	if issue != nil && a.links == nil {
		return domain.AdminUser{}, domain.ErrSetPasswordUnavailable
	}

	tx, err := a.Begin(ctx)
	if err != nil {
		return domain.AdminUser{}, fmt.Errorf("users: begin create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockRoleAt(ctx, tx, in.RoleID, in.RoleRevision); err != nil {
		return domain.AdminUser{}, err
	}
	if in.ClassID != "" {
		if err := lockOpenClass(ctx, tx, in.ClassID); err != nil {
			return domain.AdminUser{}, err
		}
	}
	id, err := insertUser(ctx, tx, req, in)
	if err != nil {
		return domain.AdminUser{}, err
	}
	if in.ClassID != "" {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
			VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)`, in.ClassID, id, req.ActorID); err != nil {
			return domain.AdminUser{}, fmt.Errorf("users: enrol: %w", err)
		}
	}
	afterCommit, err := a.invite(ctx, tx, id, issue)
	if err != nil {
		return domain.AdminUser{}, err
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "user.created",
		Entity:      entityUser,
		EntityID:    &id,
		OccurredAt:  in.Now,
		IP:          opt.String(req.IP),
		UserAgent:   opt.String(req.UserAgent),
		Diff:        diff,
	}); err != nil {
		return domain.AdminUser{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AdminUser{}, fmt.Errorf("users: commit create: %w", err)
	}
	afterCommit()
	return scanAdminUser(a.QueryRow(ctx, adminUserSelect+` WHERE u.id = $1::uuid`, id))
}

func (a *AdminUsers) invite(ctx context.Context, tx pgx.Tx, userID string, issue domain.LinkIssue) (func(), error) {
	if issue == nil {
		return func() {}, nil
	}
	writes, afterCommit := a.links.Writes(tx)
	return afterCommit, issue(ctx, writes, userID)
}

func lockRoleAt(ctx context.Context, tx pgx.Tx, roleID string, revision int64) error {
	var found int64
	err := tx.QueryRow(ctx, `SELECT revision FROM app.roles WHERE id = $1::uuid FOR SHARE`, roleID).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrRoleChanged
	}
	if err != nil {
		return fmt.Errorf("users: lock role: %w", err)
	}
	if found != revision {
		return domain.ErrRoleChanged
	}
	return nil
}

func lockOpenClass(ctx context.Context, tx pgx.Tx, classID string) error {
	var found string
	err := tx.QueryRow(ctx, `
		SELECT c.id::text FROM app.classes c
		 WHERE c.id = $1::uuid AND c.archived_at IS NULL FOR SHARE`, classID).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrClassNotFound
	}
	if err != nil {
		return fmt.Errorf("users: lock class: %w", err)
	}
	return nil
}

func insertUser(ctx context.Context, tx pgx.Tx, req domain.WriteRequest, in domain.NewUser) (string, error) {
	var hash *string
	if in.Hash != "" {
		hash = &in.Hash
	}
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO app.users (email, full_name, role_id, password_hash, must_change_password, created_by)
		VALUES ($1, $2, $3::uuid, $4, $5, $6::uuid)
		RETURNING id::text`,
		in.Email, in.FullName, in.RoleID, hash, hash != nil && !req.KeepTemporaryPassword, req.ActorID).Scan(&id)
	if db.IsUniqueViolation(err, "") {
		return "", domain.ErrEmailTaken
	}
	if err != nil {
		return "", fmt.Errorf("users: insert: %w", err)
	}
	return id, nil
}

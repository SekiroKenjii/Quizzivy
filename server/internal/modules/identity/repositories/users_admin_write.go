package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/email"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type lockedAccount struct {
	roleID   string
	roleName string
	admin    bool
	disabled bool
}

func lockAccount(ctx context.Context, tx pgx.Tx, in domain.AccountWrite) (lockedAccount, error) {
	var acct lockedAccount
	err := tx.QueryRow(ctx, `
		SELECT u.role_id::text, u.disabled_at IS NOT NULL
		  FROM app.users u
		 WHERE u.id = $1::uuid
		   FOR NO KEY UPDATE`, in.UserID).Scan(&acct.roleID, &acct.disabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return lockedAccount{}, domain.ErrUserNotFound
	}
	if err != nil {
		return lockedAccount{}, fmt.Errorf("users: lock account: %w", err)
	}
	if acct.roleID != in.RoleID {
		return lockedAccount{}, domain.ErrRoleChanged
	}
	err = tx.QueryRow(ctx, `
		SELECT r.name, COALESCE(r.builtin_key, '') = 'admin'
		  FROM app.roles r
		 WHERE r.id = $1::uuid`, acct.roleID).Scan(&acct.roleName, &acct.admin)
	if err != nil {
		return lockedAccount{}, fmt.Errorf("users: read account role: %w", err)
	}
	return acct, nil
}

func contended(err *error) {
	var pg *pgconn.PgError
	if errors.As(*err, &pg) && (pg.Code == pgerrcode.DeadlockDetected || pg.Code == pgerrcode.SerializationFailure) {
		*err = fmt.Errorf("%w: %w", domain.ErrWriteContended, *err)
	}
}

func lockActiveAdmins(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT u.id::text
		  FROM app.users u
		  JOIN app.roles r ON r.id = u.role_id
		 WHERE r.builtin_key = 'admin' AND u.disabled_at IS NULL
		 ORDER BY u.id
		   FOR NO KEY UPDATE OF u`)
	if err != nil {
		return nil, fmt.Errorf("users: lock active admins: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("users: lock active admins: %w", err)
	}
	return ids, nil
}

func ownedContent(ctx context.Context, q db.Querier, userID string) (domain.OwnedContent, error) {
	var owned domain.OwnedContent
	err := q.QueryRow(ctx, `SELECT `+ownedCountsSQL+` FROM app.users u WHERE u.id = $1::uuid`, userID).
		Scan(&owned.Classes, &owned.Tests, &owned.Questions, &owned.Media, &owned.Assignments)
	if err != nil {
		return domain.OwnedContent{}, fmt.Errorf("users: count owned content: %w", err)
	}
	return owned, nil
}

type roleDiff struct {
	Old string `json:"old"`
	New string `json:"new"`
}

type accountDiff struct {
	BatchID  string    `json:"batchId,omitempty"`
	Delivery string    `json:"delivery,omitempty"`
	Role     *roleDiff `json:"role,omitempty"`
}

func writeAccountAudit(ctx context.Context, tx pgx.Tx, req domain.WriteRequest, action, userID string, at time.Time, diff accountDiff) error {
	diff.BatchID = req.BatchID
	encoded, err := json.Marshal(diff)
	if err != nil {
		return fmt.Errorf("users: encode audit diff: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      action,
		Entity:      entityUser,
		EntityID:    &userID,
		OccurredAt:  at,
		IP:          opt.String(req.IP),
		UserAgent:   opt.String(req.UserAgent),
		Diff:        encoded,
	})
}

func (a *AdminUsers) commit(ctx context.Context, tx pgx.Tx, what string) error {
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("users: commit %s: %w", what, err)
	}
	return nil
}

// Get reads one account as the table lists it.
func (a *AdminUsers) Get(ctx context.Context, id string) (domain.AdminUser, error) {
	return scanAdminUser(a.QueryRow(ctx, adminUserSelect+` WHERE u.id = $1::uuid`, id))
}

// ChangeRole gives an account another role. It locks every active Admin in id
// order, the account and then the new role, the order every account write
// keeps, because two role changes or disables that each leave a third Admin
// must not deadlock, and two that each take one of the last two Admins away
// must not both pass. The role comes last because the users_last_admin trigger
// locks the Admin role when an Admin is taken away, after the user rows. A
// write the database ends as a deadlock or a serialization failure fails with
// ErrWriteContended; so does every other account write here.
func (a *AdminUsers) ChangeRole(ctx context.Context, req domain.WriteRequest, in domain.RoleChange) (err error) {
	defer contended(&err)
	tx, err := a.Begin(ctx)
	if err != nil {
		return fmt.Errorf("users: begin role change: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	admins, err := lockActiveAdmins(ctx, tx)
	if err != nil {
		return err
	}
	acct, err := lockAccount(ctx, tx, in.AccountWrite)
	if err != nil {
		return err
	}
	if err := lockRoleAt(ctx, tx, in.ToRoleID, in.ToRoleRevision); err != nil {
		return err
	}
	if acct.roleID == in.ToRoleID {
		return nil
	}
	if acct.admin && !acct.disabled && !in.ToAdmin && len(admins) == 1 {
		return domain.ErrLastAdmin
	}
	if !in.ToOpensTeacher {
		owned, err := ownedContent(ctx, tx, in.UserID)
		if err != nil {
			return err
		}
		if owned.Any() {
			return &domain.OwnsContentError{Owned: owned}
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE app.users SET role_id = $2::uuid WHERE id = $1::uuid`, in.UserID, in.ToRoleID); err != nil {
		return fmt.Errorf("users: change role: %w", UserWriteError(err))
	}
	if err := endSessions(ctx, tx, in.UserID, in.Now); err != nil {
		return err
	}
	diff := accountDiff{Role: &roleDiff{Old: acct.roleName, New: in.ToRoleName}}
	if err := writeAccountAudit(ctx, tx, req, "user.role_changed", in.UserID, in.Now, diff); err != nil {
		return err
	}
	return a.commit(ctx, tx, "role change")
}

// Disable blocks an account's sign-in and ends every way back in that was
// issued before: its sessions, its unused set-password links and the messages
// still waiting for it.
func (a *AdminUsers) Disable(ctx context.Context, req domain.WriteRequest, in domain.AccountWrite) (err error) {
	defer contended(&err)
	tx, err := a.Begin(ctx)
	if err != nil {
		return fmt.Errorf("users: begin disable: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	admins, err := lockActiveAdmins(ctx, tx)
	if err != nil {
		return err
	}
	acct, err := lockAccount(ctx, tx, in)
	if err != nil {
		return err
	}
	if acct.disabled {
		return nil
	}
	if acct.admin && len(admins) == 1 {
		return domain.ErrLastAdmin
	}
	if _, err := tx.Exec(ctx, `UPDATE app.users SET disabled_at = $2 WHERE id = $1::uuid`, in.UserID, in.Now); err != nil {
		return fmt.Errorf("users: disable: %w", UserWriteError(err))
	}
	if err := endSessions(ctx, tx, in.UserID, in.Now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, revokeUnusedLinks, in.UserID, in.Now); err != nil {
		return fmt.Errorf("users: revoke set-password links: %w", err)
	}
	if _, err := email.CancelPending(ctx, tx, in.UserID, in.Now); err != nil {
		return err
	}
	if err := writeAccountAudit(ctx, tx, req, "user.disabled", in.UserID, in.Now, accountDiff{}); err != nil {
		return err
	}
	return a.commit(ctx, tx, "disable")
}

// Enable lets a disabled account sign in again. What a disable ended stays
// ended.
func (a *AdminUsers) Enable(ctx context.Context, req domain.WriteRequest, in domain.AccountWrite) (err error) {
	defer contended(&err)
	tx, err := a.Begin(ctx)
	if err != nil {
		return fmt.Errorf("users: begin enable: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	acct, err := lockAccount(ctx, tx, in)
	if err != nil {
		return err
	}
	if !acct.disabled {
		return nil
	}
	if _, err := tx.Exec(ctx, `UPDATE app.users SET disabled_at = NULL WHERE id = $1::uuid`, in.UserID); err != nil {
		return fmt.Errorf("users: enable: %w", err)
	}
	if err := writeAccountAudit(ctx, tx, req, "user.enabled", in.UserID, in.Now, accountDiff{}); err != nil {
		return err
	}
	return a.commit(ctx, tx, "enable")
}

// SignOut ends every session the account has.
func (a *AdminUsers) SignOut(ctx context.Context, req domain.WriteRequest, in domain.AccountWrite) (err error) {
	defer contended(&err)
	tx, err := a.Begin(ctx)
	if err != nil {
		return fmt.Errorf("users: begin sign-out: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := lockAccount(ctx, tx, in); err != nil {
		return err
	}
	if err := endSessions(ctx, tx, in.UserID, in.Now); err != nil {
		return err
	}
	if err := writeAccountAudit(ctx, tx, req, "user.signed_out_everywhere", in.UserID, in.Now, accountDiff{}); err != nil {
		return err
	}
	return a.commit(ctx, tx, "sign-out")
}

func replacePassword(ctx context.Context, tx pgx.Tx, in domain.AccountPasswordReset, markForChange bool) error {
	var hash *string
	if in.Hash != "" {
		hash = &in.Hash
	}
	if _, err := tx.Exec(ctx, `
		UPDATE app.users SET password_hash = $2, must_change_password = $3
		 WHERE id = $1::uuid`, in.UserID, hash, hash != nil && markForChange); err != nil {
		return fmt.Errorf("users: reset password: %w", err)
	}
	return nil
}

// ResetPassword sets a temporary password, or clears the password and issues
// a link to choose another, and ends everything the old password opened: the
// account's sessions and the set-password links it held.
func (a *AdminUsers) ResetPassword(ctx context.Context, req domain.WriteRequest, in domain.AccountPasswordReset) (err error) {
	defer contended(&err)
	if in.Delivery == domain.DeliveryEmailLink && (in.Issue == nil || a.links == nil) {
		return domain.ErrSetPasswordUnavailable
	}
	tx, err := a.Begin(ctx)
	if err != nil {
		return fmt.Errorf("users: begin password reset: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	acct, err := lockAccount(ctx, tx, in.AccountWrite)
	if err != nil {
		return err
	}
	if acct.disabled {
		return domain.ErrAccountDisabled
	}
	if err := replacePassword(ctx, tx, in, !req.KeepTemporaryPassword); err != nil {
		return err
	}
	if err := endSessions(ctx, tx, in.UserID, in.Now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, revokeUnusedLinks, in.UserID, in.Now); err != nil {
		return fmt.Errorf("users: revoke set-password links: %w", err)
	}
	afterCommit := func() {}
	if in.Delivery == domain.DeliveryEmailLink {
		if afterCommit, err = a.invite(ctx, tx, in.UserID, in.Issue); err != nil {
			return err
		}
	}
	if err := writeAccountAudit(ctx, tx, req, "user.password_reset", in.UserID, in.Now, accountDiff{Delivery: string(in.Delivery)}); err != nil {
		return err
	}
	if err := a.commit(ctx, tx, "password reset"); err != nil {
		return err
	}
	afterCommit()
	return nil
}

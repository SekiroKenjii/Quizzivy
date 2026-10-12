package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/audit"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Users struct {
	db.Repository
}

func NewUsers(dbx db.Context) *Users { return &Users{Repository: db.NewRepository(dbx)} }

const userProjection = `
	SELECT u.id::text, u.email, u.full_name,
	       u.password_hash, u.must_change_password, u.disabled_at, u.created_at, u.session_epoch,
	       u.display_name, u.avatar_key, u.phone, u.locale, u.time_zone, u.preferences,
	       coalesce(array_agg(i.provider) FILTER (WHERE i.provider IS NOT NULL), '{}')
	  FROM app.users u
	  LEFT JOIN app.user_identities i ON i.user_id = u.id`

func scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	err := row.Scan(
		&u.ID, &u.Email, &u.FullName, &u.PasswordHash,
		&u.MustChangePassword, &u.DisabledAt, &u.CreatedAt, &u.SessionEpoch,
		&u.DisplayName, &u.AvatarKey, &u.Phone, &u.Locale, &u.TimeZone, &u.Preferences, &u.LinkedProviders,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// FindUserByEmail looks a user up case-insensitively, matching the
// users_email_lower_key expression index so the lookup is an index scan rather
// than a seq scan with a filter.
func (s *Users) FindUserByEmail(ctx context.Context, email string) (domain.User, error) {
	q := userProjection + `
		 WHERE lower(u.email) = lower($1)
		 GROUP BY u.id`
	return scanUser(s.QueryRow(ctx, q, email))
}

// FindUserByID is the refresh path's lookup: the token names its owner, and
// refresh must re-read that owner rather than trust the token. A user disabled
// an hour ago still holds a valid refresh token, and it must stop working.
func (s *Users) FindUserByID(ctx context.Context, id string) (domain.User, error) {
	q := userProjection + `
		 WHERE u.id = $1
		 GROUP BY u.id`
	return scanUser(s.QueryRow(ctx, q, id))
}

// UpdateProfile writes supplied fields and compatible actual-change audits atomically.
func (s *Users) UpdateProfile(ctx context.Context, in domain.ProfileRecord) (domain.User, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	p := in.Patch
	const statement = `
 WITH changed AS (
  UPDATE app.users SET
   full_name = CASE WHEN $2::text IS NULL THEN full_name ELSE $2 END,
   display_name = CASE WHEN $3::boolean THEN $4::text ELSE display_name END,
   phone = CASE WHEN $5::boolean THEN $6::text ELSE phone END,
   locale = CASE WHEN $7::text IS NULL THEN locale ELSE $7 END,
   time_zone = CASE WHEN $8::text IS NULL THEN time_zone ELSE $8 END
  WHERE id = $1::uuid AND disabled_at IS NULL
  RETURNING OLD.full_name AS old_name, NEW.full_name AS new_name,
   array_remove(ARRAY[
    CASE WHEN OLD.display_name IS DISTINCT FROM NEW.display_name THEN 'displayName' END,
    CASE WHEN OLD.locale IS DISTINCT FROM NEW.locale THEN 'locale' END,
    CASE WHEN OLD.phone IS DISTINCT FROM NEW.phone THEN 'phone' END,
    CASE WHEN OLD.time_zone IS DISTINCT FROM NEW.time_zone THEN 'timeZone' END
   ], NULL) AS fields
 ), audited AS (
  INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff)
  SELECT $1::uuid, event.action, 'user', $1::uuid, $9, $10::inet, $11, event.diff
  FROM changed CROSS JOIN LATERAL (
   SELECT 'user.renamed' AS action, jsonb_build_object('from', old_name, 'to', new_name) AS diff
    WHERE old_name IS DISTINCT FROM new_name
   UNION ALL
   SELECT 'user.profile_updated', jsonb_build_object('fields', fields)
    WHERE cardinality(fields) > 0
  ) AS event
 ) SELECT EXISTS (SELECT 1 FROM changed)`
	var updated bool
	if err := tx.QueryRow(ctx, statement, in.UserID, p.FullName, p.DisplayNameSet, p.DisplayName, p.PhoneSet, p.Phone, p.Locale, p.TimeZone, in.Now, in.IP, in.UserAgent).Scan(&updated); err != nil {
		return domain.User{}, fmt.Errorf("update profile: %w", err)
	}
	user, err := persistedUser(ctx, tx, in.UserID, updated)
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func persistedUser(ctx context.Context, tx pgx.Tx, userID string, updated bool) (domain.User, error) {
	user, err := scanUser(tx.QueryRow(ctx, userProjection+` WHERE u.id = $1::uuid GROUP BY u.id`, userID))
	if err != nil {
		return domain.User{}, err
	}
	if !updated && user.Disabled() {
		return domain.User{}, domain.ErrAccountDisabled
	}
	return user, nil
}

// CreateRefreshToken stores the hash of a newly minted refresh token, unless
// the account is no longer what the sign-in read: it locks the user FOR
// SHARE, which an update of the user conflicts with, so a reset, a disable
// or a password change either waits for the token and revokes it, or commits
// first and is seen here, which answers ErrAccountChanged and stores nothing.
func (s *Users) CreateRefreshToken(ctx context.Context, in domain.RefreshTokenRecord, basis domain.SessionBasis) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin sign-in: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var locked bool
	err = tx.QueryRow(ctx,
		`SELECT true FROM app.users WHERE id = $1::uuid FOR SHARE`, in.UserID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrAccountChanged
	}
	if err != nil {
		return fmt.Errorf("lock signing-in user: %w", err)
	}

	var unchanged bool
	if err := tx.QueryRow(ctx, `
		SELECT session_epoch = $2 AND password_hash IS NOT DISTINCT FROM $3::text AND disabled_at IS NULL
		  FROM app.users
		 WHERE id = $1::uuid`, in.UserID, basis.Epoch, basis.PasswordHash).Scan(&unchanged); err != nil {
		return fmt.Errorf("re-read signing-in user: %w", err)
	}
	if !unchanged {
		return domain.ErrAccountChanged
	}

	const q = `
		INSERT INTO app.refresh_tokens
		       (user_id, family_id, token_hash, issued_at, expires_at, user_agent, ip, geo_label)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	if _, err := tx.Exec(ctx, q,
		in.UserID, in.FamilyID, in.TokenHash, in.IssuedAt, in.ExpiresAt, in.UserAgent, in.IP, in.GeoLabel); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FindUserByProviderIdentity is §5.3 step 4's first branch: the Google `sub`
// we have seen before. Matching on `sub` rather than email is the whole point --
// a Google account's email can change, its subject cannot.
func (s *Users) FindUserByProviderIdentity(ctx context.Context, provider, providerUserID string) (domain.User, error) {
	q := userProjection + `
		 WHERE u.id = (SELECT user_id FROM app.user_identities
		                WHERE provider = $1 AND provider_user_id = $2)
		 GROUP BY u.id`
	return scanUser(s.QueryRow(ctx, q, provider, providerUserID))
}

// LinkIdentity is step 4's second branch: a verified Google email matching an
// existing account, which becomes a link rather than a new user.
func (s *Users) LinkIdentity(ctx context.Context, userID, provider, providerUserID, emailAtLink string) error {
	const q = `
		INSERT INTO app.user_identities (user_id, provider, provider_user_id, email_at_link)
		VALUES ($1, $2, $3, $4)`
	_, err := s.Exec(ctx, q, userID, provider, providerUserID, emailAtLink)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return domain.ErrIdentityAlreadyLinked
		}
		return err
	}
	return nil
}

// UnlinkIdentity removes a provider identity. Reports whether a row went, so
// the caller can tell "unlinked" from "there was nothing to unlink".
func (s *Users) UnlinkIdentity(ctx context.Context, userID, provider string) (bool, error) {
	const q = `DELETE FROM app.user_identities WHERE user_id = $1 AND provider = $2`
	tag, err := s.Exec(ctx, q, userID, provider)
	if err != nil {
		return false, fmt.Errorf("unlink %s identity: %w", provider, err)
	}
	return tag.RowsAffected() > 0, nil
}

// WriteAudit appends an audit row outside any transaction. Used where the
// audited change is a single statement that has already committed, so there is
// no transaction to join.
func (s *Users) WriteAudit(ctx context.Context, e audit.Entry) error {
	return audit.Write(ctx, s.Conn(), e)
}

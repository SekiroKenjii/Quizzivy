package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/audit"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Users struct {
	pool *pgxpool.Pool
}

func NewUsers(pool *pgxpool.Pool) *Users { return &Users{pool: pool} }

const userProjection = `
	SELECT u.id::text, u.email, u.full_name, u.role::text, u.password_hash,
	       u.must_change_password, u.disabled_at, u.created_at,
	       coalesce(array_agg(i.provider) FILTER (WHERE i.provider IS NOT NULL), '{}')
	  FROM app.users u
	  LEFT JOIN app.user_identities i ON i.user_id = u.id`

func scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	err := row.Scan(
		&u.ID, &u.Email, &u.FullName, &u.Role, &u.PasswordHash,
		&u.MustChangePassword, &u.DisabledAt, &u.CreatedAt, &u.LinkedProviders,
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
	return scanUser(s.pool.QueryRow(ctx, q, email))
}

// FindUserByID is the refresh path's lookup: the token names its owner, and
// refresh must re-read that owner rather than trust the token. A user disabled
// an hour ago still holds a valid refresh token, and it must stop working.
func (s *Users) FindUserByID(ctx context.Context, id string) (domain.User, error) {
	q := userProjection + `
		 WHERE u.id = $1
		 GROUP BY u.id`
	return scanUser(s.pool.QueryRow(ctx, q, id))
}

// CreateRefreshToken stores the hash of a newly minted refresh token.
func (s *Users) CreateRefreshToken(ctx context.Context, in domain.RefreshTokenRecord) error {
	const q = `
		INSERT INTO app.refresh_tokens
		       (user_id, family_id, token_hash, issued_at, expires_at, user_agent, ip)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.pool.Exec(ctx, q,
		in.UserID, in.FamilyID, in.TokenHash, in.IssuedAt, in.ExpiresAt, in.UserAgent, in.IP)
	return err
}

// FindUserByProviderIdentity is §5.3 step 4's first branch: the Google `sub`
// we have seen before. Matching on `sub` rather than email is the whole point --
// a Google account's email can change, its subject cannot.
func (s *Users) FindUserByProviderIdentity(ctx context.Context, provider, providerUserID string) (domain.User, error) {
	q := userProjection + `
		 WHERE u.id = (SELECT user_id FROM app.user_identities
		                WHERE provider = $1 AND provider_user_id = $2)
		 GROUP BY u.id`
	return scanUser(s.pool.QueryRow(ctx, q, provider, providerUserID))
}

// LinkIdentity is step 4's second branch: a verified Google email matching an
// existing account, which becomes a link rather than a new user.
func (s *Users) LinkIdentity(ctx context.Context, userID, provider, providerUserID, emailAtLink string) error {
	const q = `
		INSERT INTO app.user_identities (user_id, provider, provider_user_id, email_at_link)
		VALUES ($1, $2, $3, $4)`
	_, err := s.pool.Exec(ctx, q, userID, provider, providerUserID, emailAtLink)
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
	tag, err := s.pool.Exec(ctx, q, userID, provider)
	if err != nil {
		return false, fmt.Errorf("unlink %s identity: %w", provider, err)
	}
	return tag.RowsAffected() > 0, nil
}

// WriteAudit appends an audit row outside any transaction. Used where the
// audited change is a single statement that has already committed, so there is
// no transaction to join.
func (s *Users) WriteAudit(ctx context.Context, e audit.Entry) error {
	return audit.Write(ctx, s.pool, e)
}

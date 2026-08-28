package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")

// User is the slice of app.users this package needs.
type User struct {
	ID                 string
	Email              string
	FullName           string
	Role               string
	PasswordHash       *string
	MustChangePassword bool
	DisabledAt         *time.Time
	CreatedAt          time.Time
	LinkedProviders    []string
}

// HasPassword reports whether the account can log in with a password at all.
// False for Google-only accounts (§5.1).
func (u User) HasPassword() bool { return u.PasswordHash != nil && *u.PasswordHash != "" }

// Disabled reports whether the account is suspended.
func (u User) Disabled() bool { return u.DisabledAt != nil }

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// FindUserByEmail looks a user up case-insensitively, matching the
// users_email_lower_key expression index so the lookup is an index scan rather
// than a seq scan with a filter.
//
// Explicit column list, never SELECT * (§13.8).
func (s *Store) FindUserByEmail(ctx context.Context, email string) (User, error) {
	const q = `
		SELECT u.id::text, u.email, u.full_name, u.role::text, u.password_hash,
		       u.must_change_password, u.disabled_at, u.created_at,
		       coalesce(array_agg(i.provider) FILTER (WHERE i.provider IS NOT NULL), '{}')
		  FROM app.users u
		  LEFT JOIN app.user_identities i ON i.user_id = u.id
		 WHERE lower(u.email) = lower($1)
		 GROUP BY u.id`

	var u User
	err := s.pool.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.FullName, &u.Role, &u.PasswordHash,
		&u.MustChangePassword, &u.DisabledAt, &u.CreatedAt, &u.LinkedProviders,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// CreateRefreshToken stores the hash of a newly minted refresh token.
//
// The plaintext is never stored (§13.5): a database dump must not hand over
// live sessions. `familyID` chains rotations so §5.2's reuse detection can
// revoke an entire lineage at once — T-1.3 uses it.
func (s *Store) CreateRefreshToken(ctx context.Context, in RefreshTokenRecord) error {
	const q = `
		INSERT INTO app.refresh_tokens (user_id, family_id, token_hash, expires_at, user_agent, ip)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := s.pool.Exec(ctx, q,
		in.UserID, in.FamilyID, in.TokenHash, in.ExpiresAt, in.UserAgent, in.IP)
	return err
}

type RefreshTokenRecord struct {
	UserID    string
	FamilyID  string
	TokenHash []byte
	ExpiresAt time.Time
	UserAgent *string
	IP        *string
}

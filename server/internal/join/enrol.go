package join

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"quizzivy/internal/audit"
)

// ErrEmailTaken is a signup racing another signup for the same address. The
// §5.3 resolution order makes this nearly unreachable -- an existing email is
// matched and linked one branch earlier -- so it means two requests arrived
// inside the same microseconds, and the caller should simply try again.
var ErrEmailTaken = errors.New("join: email already registered")

// NewMember describes an account to create as part of enrolling. Nil when the
// student already has one.
type NewMember struct {
	Email          string
	FullName       string
	Provider       string
	ProviderUserID string
}

type EnrolInput struct {
	RawCode        string
	ExistingUserID string
	NewMember      *NewMember

	Now       time.Time
	IP        *string
	UserAgent *string
}

// EnrolResult reuses PreviewOutcome for its refusals, so /join/preview and the
// enrolment that follows it cannot drift into disagreeing about what a code's
// state means -- or into leaking different amounts about it.
type EnrolResult struct {
	Outcome       PreviewOutcome
	UserID        string
	AlreadyMember bool
	Class         EnrolledClass
}

// EnrolledClass is the §7 Class shape the join endpoints return.
type EnrolledClass struct {
	ID              string
	Name            string
	Description     *string
	StudentCount    int
	SelfJoinEnabled bool
	CreatedAt       time.Time
}

// Enrol validates a join code and enrols a student, creating the account first
// when there is none, in a single transaction.
//
// uses_count increments only when the membership is new, so a student
// re-submitting a code they already used does not exhaust it.
func (s *Store) Enrol(ctx context.Context, in EnrolInput) (EnrolResult, error) {
	normalized := Normalize(in.RawCode)
	if normalized == "" {
		return EnrolResult{Outcome: PreviewInvalid}, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return EnrolResult{}, fmt.Errorf("begin enrol: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	code, err := claimCode(ctx, tx, Hash(normalized))
	if errors.Is(err, pgx.ErrNoRows) {
		return EnrolResult{Outcome: PreviewInvalid}, nil
	}
	if err != nil {
		return EnrolResult{}, fmt.Errorf("claim join code: %w", err)
	}
	if outcome := code.usable(in.Now); outcome != PreviewOK {
		return EnrolResult{Outcome: outcome}, nil
	}

	userID := in.ExistingUserID
	if in.NewMember != nil {
		userID, err = createMember(ctx, tx, *in.NewMember)
		if err != nil {
			return EnrolResult{}, err
		}
	}

	alreadyMember, err := addMember(ctx, tx, code, userID, in.Now)
	if err != nil {
		return EnrolResult{}, err
	}
	if !alreadyMember {
		if err := countUse(ctx, tx, code, userID, in); err != nil {
			return EnrolResult{}, err
		}
	}

	class, err := loadClass(ctx, tx, code.classID)
	if err != nil {
		return EnrolResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return EnrolResult{}, fmt.Errorf("commit enrol: %w", err)
	}
	return EnrolResult{
		Outcome:       PreviewOK,
		UserID:        userID,
		AlreadyMember: alreadyMember,
		Class:         class,
	}, nil
}

// claimedCode is a join code row locked FOR UPDATE, with its class's self-join
// setting.
type claimedCode struct {
	id              string
	classID         string
	revokedAt       *time.Time
	expiresAt       time.Time
	maxUses         *int
	usesCount       int
	selfJoinEnabled bool
}

// usable reports PreviewOK, or the reason the code cannot be redeemed. A class
// with self-join closed is reported as invalid rather than as its own outcome,
// so the endpoint cannot be used to discover which classes exist.
func (c claimedCode) usable(now time.Time) PreviewOutcome {
	switch {
	case !c.selfJoinEnabled:
		return PreviewInvalid
	case c.revokedAt != nil:
		return PreviewRevoked
	case !c.expiresAt.After(now):
		return PreviewExpired
	case c.maxUses != nil && c.usesCount >= *c.maxUses:
		return PreviewExhausted
	}
	return PreviewOK
}

func claimCode(ctx context.Context, tx pgx.Tx, codeHash []byte) (claimedCode, error) {
	const claim = `
		SELECT jc.id::text, jc.class_id::text, jc.revoked_at, jc.expires_at,
		       jc.max_uses, jc.uses_count, c.self_join_enabled AND c.archived_at IS NULL
		  FROM app.class_join_codes jc
		  JOIN app.classes c ON c.id = jc.class_id
		 WHERE jc.code_hash = $1
		   FOR UPDATE OF jc`

	var c claimedCode
	err := tx.QueryRow(ctx, claim, codeHash).Scan(
		&c.id, &c.classID, &c.revokedAt, &c.expiresAt, &c.maxUses, &c.usesCount, &c.selfJoinEnabled)
	return c, err
}

// addMember reports whether the student was already in the class.
func addMember(ctx context.Context, tx pgx.Tx, code claimedCode, userID string, now time.Time) (bool, error) {
	const enrol = `
		INSERT INTO app.class_members (class_id, user_id, joined_via, joined_at, join_code_id)
		VALUES ($1, $2, 'join_code', $3, $4)
		ON CONFLICT (class_id, user_id) DO NOTHING`

	tag, err := tx.Exec(ctx, enrol, code.classID, userID, now, code.id)
	if err != nil {
		return false, fmt.Errorf("enrol member: %w", err)
	}
	return tag.RowsAffected() == 0, nil
}

// countUse spends a seat and audits the enrolment. Only called for a new
// membership, so re-submitting a code does not exhaust it.
func countUse(ctx context.Context, tx pgx.Tx, code claimedCode, userID string, in EnrolInput) error {
	if _, err := tx.Exec(ctx,
		`UPDATE app.class_join_codes SET uses_count = uses_count + 1 WHERE id = $1`,
		code.id); err != nil {
		return fmt.Errorf("increment uses_count: %w", err)
	}

	diff, err := json.Marshal(map[string]string{
		"class_id": code.classID, "user_id": userID, "join_code_id": code.id,
	})
	if err != nil {
		return fmt.Errorf("encode audit diff: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &userID,
		Action:      "class.member_enrolled",
		Entity:      "class_member",
		EntityID:    &code.classID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
		Diff:        diff,
	})
}

func createMember(ctx context.Context, tx pgx.Tx, m NewMember) (string, error) {
	name := strings.TrimSpace(m.FullName)
	if name == "" {
		name, _, _ = strings.Cut(m.Email, "@")
	}

	var userID string
	err := tx.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role)
		 VALUES ($1, $2, 'student') RETURNING id::text`,
		m.Email, name).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return "", ErrEmailTaken
		}
		return "", fmt.Errorf("create member account: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO app.user_identities (user_id, provider, provider_user_id, email_at_link)
		 VALUES ($1, $2, $3, $4)`,
		userID, m.Provider, m.ProviderUserID, m.Email); err != nil {
		return "", fmt.Errorf("link identity for new member: %w", err)
	}
	return userID, nil
}

func loadClass(ctx context.Context, tx pgx.Tx, classID string) (EnrolledClass, error) {
	const q = `
		SELECT c.id::text, c.name, c.description, c.self_join_enabled, c.created_at,
		       (SELECT count(*) FROM app.class_members m WHERE m.class_id = c.id)
		  FROM app.classes c
		 WHERE c.id = $1`

	var c EnrolledClass
	if err := tx.QueryRow(ctx, q, classID).Scan(
		&c.ID, &c.Name, &c.Description, &c.SelfJoinEnabled, &c.CreatedAt, &c.StudentCount,
	); err != nil {
		return EnrolledClass{}, fmt.Errorf("load class %s: %w", classID, err)
	}
	return c, nil
}

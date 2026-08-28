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
	RawCode string
	// Exactly one of these. ExistingUserID is the already-authenticated path
	// (§6.2); NewMember is the Google signup path (§6.3).
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
// if this is a signup -- all in ONE transaction (§6.3).
//
// The transaction opens by locking the CODE row, and that lock is the whole
// mechanism behind max_uses. Two students redeeming the last seat both read
// uses_count, both see room, and both enrol -- unless one waits. With the lock
// the second re-reads after the first commits and finds the code exhausted.
// D-09's `uses_count <= max_uses` CHECK is the backstop: if this logic is ever
// wrong, the database refuses the row rather than silently overselling a class.
//
// uses_count increments only for a NEW membership. Re-submitting is idempotent
// (§6.2), and counting a repeat would let a student burn their own class's code
// by pressing the button twice.
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

	const claim = `
		SELECT jc.id::text, jc.class_id::text, jc.revoked_at, jc.expires_at,
		       jc.max_uses, jc.uses_count, c.self_join_enabled
		  FROM app.class_join_codes jc
		  JOIN app.classes c ON c.id = jc.class_id
		 WHERE jc.code_hash = $1
		   FOR UPDATE OF jc`

	var codeID, classID string
	var revokedAt *time.Time
	var expiresAt time.Time
	var maxUses *int
	var usesCount int
	var selfJoinEnabled bool

	err = tx.QueryRow(ctx, claim, Hash(normalized)).Scan(
		&codeID, &classID, &revokedAt, &expiresAt, &maxUses, &usesCount, &selfJoinEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return EnrolResult{Outcome: PreviewInvalid}, nil
	}
	if err != nil {
		return EnrolResult{}, fmt.Errorf("claim join code: %w", err)
	}

	// Same order as Preview, for the same reason: a closed class must answer
	// exactly as a nonexistent one.
	if !selfJoinEnabled {
		return EnrolResult{Outcome: PreviewInvalid}, nil
	}
	switch {
	case revokedAt != nil:
		return EnrolResult{Outcome: PreviewRevoked}, nil
	case !expiresAt.After(in.Now):
		return EnrolResult{Outcome: PreviewExpired}, nil
	case maxUses != nil && usesCount >= *maxUses:
		return EnrolResult{Outcome: PreviewExhausted}, nil
	}

	// The account. Created only AFTER the code is known good, so a bad code
	// leaves no user behind -- E2E 4's backend half.
	userID := in.ExistingUserID
	if in.NewMember != nil {
		userID, err = createMember(ctx, tx, *in.NewMember)
		if err != nil {
			return EnrolResult{}, err
		}
	}

	// ON CONFLICT DO NOTHING is the idempotency (§6.2): a student who submits
	// twice, or follows a deep link they already used, is already a member and
	// that is a success.
	const enrol = `
		INSERT INTO app.class_members (class_id, user_id, joined_via, joined_at, join_code_id)
		VALUES ($1, $2, 'join_code', $3, $4)
		ON CONFLICT (class_id, user_id) DO NOTHING`
	tag, err := tx.Exec(ctx, enrol, classID, userID, in.Now, codeID)
	if err != nil {
		return EnrolResult{}, fmt.Errorf("enrol member: %w", err)
	}
	alreadyMember := tag.RowsAffected() == 0

	if !alreadyMember {
		if _, err := tx.Exec(ctx,
			`UPDATE app.class_join_codes SET uses_count = uses_count + 1 WHERE id = $1`,
			codeID); err != nil {
			// D-09's CHECK fires here if the seat count was somehow wrong.
			return EnrolResult{}, fmt.Errorf("increment uses_count: %w", err)
		}

		// §6.5 requires class_id, user_id, ip, user_agent and the time. The
		// class is the entity and the student is the actor, which covers the
		// first two; the code is in the diff because after a rotation the
		// teacher needs "joined via the code that leaked" to be answerable.
		diff, err := json.Marshal(map[string]string{
			"class_id": classID, "user_id": userID, "join_code_id": codeID,
		})
		if err != nil {
			return EnrolResult{}, fmt.Errorf("encode audit diff: %w", err)
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			ActorUserID: &userID,
			Action:      "class.member_enrolled",
			Entity:      "class_member",
			EntityID:    &classID,
			OccurredAt:  in.Now,
			IP:          in.IP,
			UserAgent:   in.UserAgent,
			Diff:        diff,
		}); err != nil {
			return EnrolResult{}, err
		}
	}

	class, err := loadClass(ctx, tx, classID)
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

func createMember(ctx context.Context, tx pgx.Tx, m NewMember) (string, error) {
	name := strings.TrimSpace(m.FullName)
	if name == "" {
		// Google can return a profile with no name. An empty full_name would
		// render as a blank row in the teacher's class list, so fall back to
		// the part of the address before the @ -- recognisable, and the
		// student can change it later.
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

	// No password_hash: self-join is Google-only (§6.3), so the identity is
	// the credential and there is nothing to set must_change_password for.
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

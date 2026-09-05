package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/audit"
	"strings"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Enrol validates a join code and enrols a student, creating the account first
// when there is none, in a single transaction.
func (s *Postgres) Enrol(ctx context.Context, in domain.EnrolInput) (domain.EnrolResult, error) {
	normalized := domain.JoinCodes.Normalize(in.RawCode)
	if normalized == "" {
		return domain.EnrolResult{Outcome: domain.PreviewInvalid}, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.EnrolResult{}, fmt.Errorf("begin enrol: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	code, err := claimCode(ctx, tx, domain.JoinCodes.Hash(normalized))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.EnrolResult{Outcome: domain.PreviewInvalid}, nil
	}
	if err != nil {
		return domain.EnrolResult{}, fmt.Errorf("claim join code: %w", err)
	}
	if outcome := code.Usable(in.Now); outcome != domain.PreviewOK {
		return domain.EnrolResult{Outcome: outcome}, nil
	}

	userID := in.ExistingUserID
	if in.NewMember != nil {
		userID, err = createMember(ctx, tx, *in.NewMember)
		if err != nil {
			return domain.EnrolResult{}, err
		}
	}

	alreadyMember, err := addMember(ctx, tx, code, userID, in.Now)
	if err != nil {
		return domain.EnrolResult{}, err
	}
	if !alreadyMember {
		if err := countUse(ctx, tx, code, userID, in); err != nil {
			return domain.EnrolResult{}, err
		}
	}

	class, err := loadClass(ctx, tx, code.classID)
	if err != nil {
		return domain.EnrolResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.EnrolResult{}, fmt.Errorf("commit enrol: %w", err)
	}
	return domain.EnrolResult{
		Outcome:       domain.PreviewOK,
		UserID:        userID,
		AlreadyMember: alreadyMember,
		Class:         class,
	}, nil
}

type claimedCode struct {
	domain.CodeState
	id      string
	classID string
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
		&c.id, &c.classID, &c.RevokedAt, &c.ExpiresAt, &c.MaxUses, &c.UsesCount, &c.SelfJoinEnabled)
	return c, err
}

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

func countUse(ctx context.Context, tx pgx.Tx, code claimedCode, userID string, in domain.EnrolInput) error {
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

func createMember(ctx context.Context, tx pgx.Tx, m domain.NewMember) (string, error) {
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
			return "", domain.ErrEmailTaken
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

func loadClass(ctx context.Context, tx pgx.Tx, classID string) (domain.EnrolledClass, error) {
	const q = `
		SELECT c.id::text, c.name, c.description, c.self_join_enabled, c.created_at,
		       (SELECT count(*) FROM app.class_members m WHERE m.class_id = c.id)
		  FROM app.classes c
		 WHERE c.id = $1`

	var c domain.EnrolledClass
	if err := tx.QueryRow(ctx, q, classID).Scan(
		&c.ID, &c.Name, &c.Description, &c.SelfJoinEnabled, &c.CreatedAt, &c.StudentCount,
	); err != nil {
		return domain.EnrolledClass{}, fmt.Errorf("load class %s: %w", classID, err)
	}
	return c, nil
}

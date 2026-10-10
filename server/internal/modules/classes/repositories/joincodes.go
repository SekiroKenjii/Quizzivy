package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Rotate revokes the active code of a class the actor teaches and issues a
// replacement in one transaction, so a class is never left with two active
// codes or none. Another teacher's class answers ErrClassNotFound. The class
// row is locked for no key update, so a redemption that holds the code row
// finishes while Rotate waits for that row, and the two never deadlock.
func (s *Postgres) Rotate(ctx context.Context, in domain.RotateInput) (domain.IssuedCode, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.IssuedCode{}, fmt.Errorf("begin rotate: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	err = tx.QueryRow(ctx,
		`SELECT true FROM app.classes WHERE id = $1 AND `+taughtClass+` FOR NO KEY UPDATE`,
		in.ClassID, in.All, opt.String(in.ActorUserID)).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.IssuedCode{}, domain.ErrClassNotFound
	}
	if err != nil {
		return domain.IssuedCode{}, fmt.Errorf("lock class: %w", err)
	}

	const revokeActive = `
		UPDATE app.class_join_codes
		   SET revoked_at = $2
		 WHERE class_id = $1 AND revoked_at IS NULL
		RETURNING id::text`
	var replaced *string
	var previousID string
	if err := tx.QueryRow(ctx, revokeActive, in.ClassID, in.Now).Scan(&previousID); err == nil {
		replaced = &previousID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return domain.IssuedCode{}, fmt.Errorf("revoke previous code: %w", err)
	}

	const issue = `
		INSERT INTO app.class_join_codes
		       (id, class_id, code_hash, code_ciphertext, key_id, lookup_scheme,
		        code_hint, expires_at, max_uses, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, 2, $6, $7, $8, $9, $10)
		RETURNING id::text, uses_count`
	out := domain.IssuedCode{
		ClassID:   in.ClassID,
		Hint:      in.Hint,
		ExpiresAt: in.ExpiresAt,
		MaxUses:   in.MaxUses,
	}
	if err := tx.QueryRow(ctx, issue,
		in.CodeID, in.ClassID, in.CodeHash, in.Ciphertext, in.KeyID,
		in.Hint, in.ExpiresAt, in.MaxUses, in.ActorUserID, in.Now,
	).Scan(&out.ID, &out.UsesCount); err != nil {
		return domain.IssuedCode{}, fmt.Errorf("issue code: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE app.classes SET self_join_enabled = true WHERE id = $1`, in.ClassID); err != nil {
		return domain.IssuedCode{}, fmt.Errorf("enable self join: %w", err)
	}

	action := "class.join_code_issued"
	if replaced != nil {
		action = "class.join_code_rotated"
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorUserID,
		Action:      action,
		Entity:      "class_join_code",
		EntityID:    &out.ID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
	}); err != nil {
		return domain.IssuedCode{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.IssuedCode{}, fmt.Errorf("commit rotate: %w", err)
	}
	return out, nil
}

// Revoke ends the active code of a class the actor teaches without issuing a
// replacement, and turns off self-join (§6.4). Another teacher's class answers
// ErrClassNotFound. The class row is locked for no key update, as Rotate's is.
func (s *Postgres) Revoke(ctx context.Context, in domain.RevokeInput) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin revoke: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	err = tx.QueryRow(ctx,
		`SELECT true FROM app.classes WHERE id = $1 AND `+taughtClass+` FOR NO KEY UPDATE`,
		in.ClassID, in.All, opt.String(in.ActorUserID)).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrClassNotFound
	}
	if err != nil {
		return fmt.Errorf("lock class: %w", err)
	}

	var revokedID *string
	var id string
	err = tx.QueryRow(ctx,
		`UPDATE app.class_join_codes SET revoked_at = $2
		  WHERE class_id = $1 AND revoked_at IS NULL
		 RETURNING id::text`, in.ClassID, in.Now).Scan(&id)
	if err == nil {
		revokedID = &id
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("revoke code: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE app.classes SET self_join_enabled = false WHERE id = $1`, in.ClassID); err != nil {
		return fmt.Errorf("disable self join: %w", err)
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorUserID,
		Action:      "class.join_code_revoked",
		Entity:      "class_join_code",
		EntityID:    revokedID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ActiveCode returns the active code of a class the scope reaches, as
// stored. Another teacher's class answers ErrClassNotFound, exactly as a
// missing one does; a reached class without an active code answers
// ErrNoActiveCode.
func (s *Postgres) ActiveCode(ctx context.Context, scope access.Scope, classID string) (domain.StoredCode, error) {
	const q = `
		SELECT jc.id::text, jc.code_hint, jc.expires_at, jc.max_uses, jc.uses_count,
		       jc.lookup_scheme, jc.key_id, jc.code_hash, jc.code_ciphertext
		  FROM app.classes c
		  LEFT JOIN app.class_join_codes jc ON jc.class_id = c.id AND jc.revoked_at IS NULL
		 WHERE c.id = $1 AND ` + taughtClass

	var id, hint *string
	var expiresAt *time.Time
	var usesCount *int
	var scheme *domain.LookupScheme
	c := domain.StoredCode{IssuedCode: domain.IssuedCode{ClassID: classID}}
	err := s.QueryRow(ctx, q, classID, scope.All, opt.String(scope.UserID)).Scan(
		&id, &hint, &expiresAt, &c.MaxUses, &usesCount,
		&scheme, &c.Lookup.KeyID, &c.Lookup.Hash, &c.Ciphertext)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.StoredCode{}, domain.ErrClassNotFound
	}
	if err != nil {
		return domain.StoredCode{}, fmt.Errorf("load active join code: %w", err)
	}
	if id == nil {
		return domain.StoredCode{}, domain.ErrNoActiveCode
	}
	c.ID, c.Hint, c.ExpiresAt, c.UsesCount, c.Lookup.Scheme = *id, *hint, *expiresAt, *usesCount, *scheme
	return c, nil
}

// ActiveCodes reads the active code of each class among classIDs that the
// scope reaches, with the columns and the predicate ActiveCode uses. A class the
// scope does not reach and a class without an active code are absent.
func (s *Postgres) ActiveCodes(ctx context.Context, scope access.Scope, classIDs []string) (map[string]domain.StoredCode, error) {
	out := make(map[string]domain.StoredCode, len(classIDs))
	if len(classIDs) == 0 {
		return out, nil
	}
	const q = `
		SELECT c.id::text, jc.id::text, jc.code_hint, jc.expires_at, jc.max_uses, jc.uses_count,
		       jc.lookup_scheme, jc.key_id, jc.code_hash, jc.code_ciphertext
		  FROM app.classes c
		  JOIN app.class_join_codes jc ON jc.class_id = c.id AND jc.revoked_at IS NULL
		 WHERE c.id = ANY($1::uuid[]) AND ` + taughtClass

	rows, err := s.Query(ctx, q, classIDs, scope.All, opt.String(scope.UserID))
	if err != nil {
		return nil, fmt.Errorf("load active join codes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c domain.StoredCode
		if err := rows.Scan(&c.ClassID, &c.ID, &c.Hint, &c.ExpiresAt, &c.MaxUses, &c.UsesCount,
			&c.Lookup.Scheme, &c.Lookup.KeyID, &c.Lookup.Hash, &c.Ciphertext); err != nil {
			return nil, fmt.Errorf("load active join codes: scan: %w", err)
		}
		out[c.ClassID] = c
	}
	return out, rows.Err()
}

// LegacyCodeClasses lists every class whose active join code is a legacy one
// that has not expired at now, archived classes included, by teacher and then
// by name. It locks nothing: RotateLegacyCode reads each code again.
func (s *Postgres) LegacyCodeClasses(ctx context.Context, now time.Time) ([]domain.LegacyCodeClass, error) {
	const q = `
		SELECT c.id::text, c.name, c.teacher_id::text
		  FROM app.class_join_codes jc
		  JOIN app.classes c ON c.id = jc.class_id
		 WHERE jc.revoked_at IS NULL AND jc.lookup_scheme = 1 AND jc.expires_at > $1
		 ORDER BY c.teacher_id, c.name, c.id`

	classes, err := db.QueryMany(ctx, s, q, []any{now}, func(rows pgx.Rows) (domain.LegacyCodeClass, error) {
		var c domain.LegacyCodeClass
		err := rows.Scan(&c.ClassID, &c.ClassName, &c.TeacherID)
		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("list classes with a legacy join code: %w", err)
	}
	return classes, nil
}

// RotateLegacyCode replaces a class's legacy join code with the sealed one in
// a single transaction, audited as the System, and reports whether it did. The
// transaction takes advisory lock 41, then the class row, and only then reads
// the class's active code again, for update: a code that is revoked, sealed or
// expired at in.Now by then is left alone, nothing is written and the answer
// is false. The class row is locked for no key update, so it excludes Rotate,
// Revoke and every write to the class but not the key-share lock a member
// insert takes: a redemption that holds the code row finishes while the
// rotation waits for that row, and the two never wait for each other. The new
// code keeps the old one's expiry, use cap and creator and starts unused; the
// class's self-join switch is not written. A transaction aborted as a deadlock
// or serialization victim answers domain.ErrRotationContended.
func (s *Postgres) RotateLegacyCode(ctx context.Context, in domain.LegacyRotationInput) (bool, error) {
	wrote := false
	err := s.InTx(ctx, "rotate legacy join code", func(tx pgx.Tx) error {
		old, err := lockLegacyCode(ctx, tx, in)
		if err != nil || old == nil {
			return err
		}
		if err := replaceLegacyCode(ctx, tx, in, *old); err != nil {
			return err
		}
		wrote = true
		return nil
	})
	if err != nil {
		return false, contended(err)
	}
	return wrote, nil
}

const entityJoinCode = "class_join_code"

type legacyCode struct {
	id        string
	hint      string
	expiresAt time.Time
	maxUses   *int
	createdBy string
}

func lockLegacyCode(ctx context.Context, tx pgx.Tx, in domain.LegacyRotationInput) (*legacyCode, error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73819, 41)`); err != nil {
		return nil, fmt.Errorf("lock the legacy rotation: %w", err)
	}
	var exists bool
	err := tx.QueryRow(ctx, `SELECT true FROM app.classes WHERE id = $1 FOR NO KEY UPDATE`, in.ClassID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock class: %w", err)
	}

	const active = `
		SELECT id::text, code_hint, expires_at, max_uses, created_by::text
		  FROM app.class_join_codes
		 WHERE class_id = $1 AND revoked_at IS NULL AND lookup_scheme = 1 AND expires_at > $2
		   FOR UPDATE`
	var old legacyCode
	err = tx.QueryRow(ctx, active, in.ClassID, in.Now).Scan(&old.id, &old.hint, &old.expiresAt, &old.maxUses, &old.createdBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the active legacy code: %w", err)
	}
	return &old, nil
}

func replaceLegacyCode(ctx context.Context, tx pgx.Tx, in domain.LegacyRotationInput, old legacyCode) error {
	if _, err := tx.Exec(ctx,
		`UPDATE app.class_join_codes SET revoked_at = $2 WHERE id = $1`, old.id, in.Now); err != nil {
		return fmt.Errorf("revoke the legacy code: %w", err)
	}

	const issue = `
		INSERT INTO app.class_join_codes
		       (id, class_id, code_hash, code_ciphertext, key_id, lookup_scheme,
		        code_hint, expires_at, max_uses, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, 2, $6, $7, $8, $9, $10)`
	if _, err := tx.Exec(ctx, issue,
		in.CodeID, in.ClassID, in.CodeHash, in.Ciphertext, in.KeyID,
		in.Hint, old.expiresAt, old.maxUses, old.createdBy, in.Now); err != nil {
		return fmt.Errorf("issue the sealed code: %w", err)
	}

	diff, err := json.Marshal(struct {
		Reason       string `json:"reason"`
		PreviousHint string `json:"previousHint"`
		Hint         string `json:"hint"`
	}{Reason: "legacy_rotation", PreviousHint: old.hint, Hint: in.Hint})
	if err != nil {
		return fmt.Errorf("encode audit diff: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{
		Action:     "class.join_code_rotated",
		Entity:     entityJoinCode,
		EntityID:   &in.CodeID,
		OccurredAt: in.Now,
		Diff:       diff,
	})
}

func contended(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && (pg.Code == pgerrcode.DeadlockDetected || pg.Code == pgerrcode.SerializationFailure) {
		return fmt.Errorf("%w: %w", domain.ErrRotationContended, err)
	}
	return err
}

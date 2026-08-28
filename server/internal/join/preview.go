package join

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// PreviewOutcome is why a code was or was not accepted. Each maps to a distinct
// error code so the student gets an accurate message -- "ask your teacher for a
// new code" is useful where "wrong code" is not -- without any of them naming a
// class (§6.5).
type PreviewOutcome int

const (
	PreviewOK PreviewOutcome = iota
	// PreviewInvalid covers an unrecognised code AND a class whose self-join is
	// switched off. The two are deliberately indistinguishable: a teacher who
	// closed their class should not have that fact confirmed to strangers.
	PreviewInvalid
	PreviewRevoked
	PreviewExpired
	PreviewExhausted
)

// PreviewResult carries exactly the three fields §6.5 permits, and the outcome.
type PreviewResult struct {
	Outcome     PreviewOutcome
	ClassID     string
	ClassName   string
	TeacherName string
}

// ErrNoTeacher means the install has no active admin, so no class can name one.
// An operational fault, not a bad request.
var ErrNoTeacher = errors.New("join: no active teacher account")

// Preview backs the /join/:code/confirm step (§6.2), which exists so a student
// sees WHICH class they are joining before authenticating.
//
// The order of the checks is a leak decision, not an implementation detail.
// self_join_enabled is tested before the code's own state, so a closed class
// answers exactly as a nonexistent one does -- checking revocation or expiry
// first would confirm that a code, and therefore a class, exists.
func (s *Service) Preview(ctx context.Context, rawCode string) (PreviewResult, error) {
	normalized := Normalize(rawCode)
	if normalized == "" {
		return PreviewResult{Outcome: PreviewInvalid}, nil
	}

	row, err := s.store.LookupByCodeHash(ctx, Hash(normalized))
	if err != nil {
		return PreviewResult{}, err
	}
	if row == nil {
		return PreviewResult{Outcome: PreviewInvalid}, nil
	}

	// §6.5 names a constant-time comparison. The b-tree probe above is the step
	// that actually finds the row and is not constant-time, so this does not
	// make the lookup constant-time -- nothing short of scanning every code
	// would. What it does is keep the Go-side comparison constant-time, which
	// is the part a future refactor could quietly turn into `bytes.Equal`.
	if !Equal(row.CodeHash, Hash(normalized)) {
		return PreviewResult{Outcome: PreviewInvalid}, nil
	}

	if !row.SelfJoinEnabled {
		return PreviewResult{Outcome: PreviewInvalid}, nil
	}
	switch {
	case row.RevokedAt != nil:
		return PreviewResult{Outcome: PreviewRevoked}, nil
	case !row.ExpiresAt.After(s.now()):
		return PreviewResult{Outcome: PreviewExpired}, nil
	case row.MaxUses != nil && row.UsesCount >= *row.MaxUses:
		return PreviewResult{Outcome: PreviewExhausted}, nil
	}

	if row.TeacherName == nil || *row.TeacherName == "" {
		return PreviewResult{}, ErrNoTeacher
	}

	return PreviewResult{
		Outcome:     PreviewOK,
		ClassID:     row.ClassID,
		ClassName:   row.ClassName,
		TeacherName: *row.TeacherName,
	}, nil
}

// codeRow is everything a preview decision needs, in one round trip.
type codeRow struct {
	ClassID         string
	ClassName       string
	SelfJoinEnabled bool
	CodeHash        []byte
	RevokedAt       *time.Time
	ExpiresAt       time.Time
	MaxUses         *int
	UsesCount       int
	TeacherName     *string
}

// LookupByCodeHash finds a code by its hash, revoked or not.
//
// Revoked rows are deliberately included. code_hash is UNIQUE, so keeping them
// is what lets "revoked" be distinguished from "never existed" -- and telling a
// student their code was cancelled is the difference between them asking for a
// new one and concluding the app is broken.
func (s *Store) LookupByCodeHash(ctx context.Context, hash []byte) (*codeRow, error) {
	// The teacher name comes from the single admin account (§1). app.classes
	// has no teacher column because there is one teacher, so recording which
	// one owns a class would be a column with one value in it forever. If a
	// second teacher is ever added, this is the query that has to change, and
	// `classes.teacher_id` is the change.
	const q = `
		SELECT c.id::text, c.name, c.self_join_enabled,
		       jc.code_hash, jc.revoked_at, jc.expires_at, jc.max_uses, jc.uses_count,
		       (SELECT u.full_name
		          FROM app.users u
		         WHERE u.role = 'admin' AND u.disabled_at IS NULL
		         ORDER BY u.created_at
		         LIMIT 1)
		  FROM app.class_join_codes jc
		  JOIN app.classes c ON c.id = jc.class_id
		 WHERE jc.code_hash = $1`

	var r codeRow
	err := s.pool.QueryRow(ctx, q, hash).Scan(
		&r.ClassID, &r.ClassName, &r.SelfJoinEnabled,
		&r.CodeHash, &r.RevokedAt, &r.ExpiresAt, &r.MaxUses, &r.UsesCount,
		&r.TeacherName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("look up join code: %w", err)
	}
	return &r, nil
}

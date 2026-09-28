//go:build integration

package application_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/classes/repositories"
	"quizzivy/internal/platform/db"
)

type storedCode struct {
	id         string
	scheme     int16
	keyID      *int16
	hash       []byte
	ciphertext []byte
}

func activeRow(t *testing.T, pool *pgxpool.Pool, classID string) storedCode {
	t.Helper()
	var c storedCode
	if err := pool.QueryRow(context.Background(), `
		SELECT id::text, lookup_scheme, key_id, code_hash, code_ciphertext
		  FROM app.class_join_codes
		 WHERE class_id = $1 AND revoked_at IS NULL`, classID).Scan(&c.id, &c.scheme, &c.keyID, &c.hash, &c.ciphertext); err != nil {
		t.Fatalf("active code row: %v", err)
	}
	return c
}

func withKeys(pool *pgxpool.Pool, keys domain.JoinCodeKeys) *application.Application {
	return application.New(repositories.NewPostgres(db.NewContext(pool)), nil, keys)
}

func legacyCode(t *testing.T, pool *pgxpool.Pool, classID, teacherID string) string {
	t.Helper()
	code, err := domain.JoinCodes.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, max_uses, created_by, created_at)
		VALUES ($1, $2, $3, now() + interval '30 days', 40, $4, now())`,
		classID, domain.JoinCodes.Hash(code), domain.JoinCodes.Hint(code), teacherID); err != nil {
		t.Fatalf("the v0.7.0 insert: %v", err)
	}
	return domain.JoinCodes.Format(code)
}

func previewOf(t *testing.T, svc *application.Application, code string) domain.PreviewResult {
	t.Helper()
	got, err := svc.Queries.Preview.Handle(context.Background(), query.Preview{Code: code})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	return got
}

func joinsAll(t *testing.T, pool *pgxpool.Pool, svc *application.Application, classID, code string) {
	t.Helper()
	ctx := context.Background()
	if got := previewOf(t, svc, code); got.Outcome != domain.PreviewOK || got.ClassID != classID {
		t.Fatalf("preview: %+v", got)
	}
	_, _, student := makeClassRow(t, pool)
	joined, err := svc.Commands.EnrolExisting.Handle(ctx, command.EnrolExisting{UserID: student, Code: code})
	if err != nil || joined.Outcome != domain.PreviewOK || joined.Class.ID != classID {
		t.Fatalf("joining: %+v (%v)", joined, err)
	}
	m := newMember(t)
	dropUser(t, pool, m.Email)
	google, err := svc.Commands.EnrolNewMember.Handle(ctx, command.EnrolNewMember{Member: m, Code: code})
	if err != nil || google.Outcome != domain.PreviewOK || google.Class.ID != classID {
		t.Fatalf("the Google join: %+v (%v)", google, err)
	}
}

func refusesAll(t *testing.T, pool *pgxpool.Pool, svc *application.Application, code string) {
	t.Helper()
	if got := previewOf(t, svc, code); got.Outcome != domain.PreviewInvalid || got.ClassID != "" {
		t.Errorf("preview: %+v, want PreviewInvalid", got)
	}
	_, _, student := makeClassRow(t, pool)
	joined, err := svc.Commands.EnrolExisting.Handle(context.Background(), command.EnrolExisting{UserID: student, Code: code})
	if err != nil || joined.Outcome != domain.PreviewInvalid {
		t.Errorf("joining: %+v (%v), want PreviewInvalid", joined, err)
	}
}

func TestANewCodeIsStoredSealedUnderTheKeyedScheme(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	canonical := domain.JoinCodes.Normalize(issueCode(t, svc, classID, teacherID))

	row := activeRow(t, pool, classID)
	if row.scheme != int16(domain.LookupKeyed) || row.keyID == nil || *row.keyID != joinKeys.CurrentID() {
		t.Fatalf("stored as scheme %d under key %v, want 2 under %d", row.scheme, row.keyID, joinKeys.CurrentID())
	}
	if len(row.ciphertext) != 36 {
		t.Errorf("ciphertext is %d bytes, want 36", len(row.ciphertext))
	}
	legacy := sha256.Sum256([]byte(canonical))
	if bytes.Equal(row.hash, legacy[:]) || !bytes.Equal(row.hash, joinKeys.Hash(canonical)) {
		t.Error("code_hash is not the keyed hash: a dump could be hashed through the code space offline")
	}
	opened, err := joinKeys.Open(classID, row.id, *row.keyID, row.ciphertext)
	if err != nil || opened != canonical {
		t.Fatalf("the row opens to %q (%v), want %q", opened, err, canonical)
	}
}

func TestALegacyCodePreviewsJoinsAndGoogleJoins(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	code := legacyCode(t, pool, classID, teacherID)

	row := activeRow(t, pool, classID)
	if row.scheme != int16(domain.LookupLegacy) || row.keyID != nil || row.ciphertext != nil {
		t.Fatalf("the v0.7.0 insert is scheme %d, key %v, ciphertext %x", row.scheme, row.keyID, row.ciphertext)
	}
	joinsAll(t, pool, svc, classID, code)
	if n := usesCount(t, pool, classID); n != 2 {
		t.Errorf("uses_count = %d, want 2", n)
	}
}

func TestACodeIssuedUnderTheOldKeyRedeemsWhileRotating(t *testing.T) {
	pool := newPool(t)
	classID, teacherID, _ := makeClassRow(t, pool)
	code := issueCode(t, withKeys(pool, mustJoinCodeKeys(joinKeyA, nil)), classID, teacherID)

	refusesAll(t, pool, withKeys(pool, mustJoinCodeKeys(joinKeyB, nil)), code)
	rotating := withKeys(pool, mustJoinCodeKeys(joinKeyB, joinKeyA))
	joinsAll(t, pool, rotating, classID, code)

	otherClass, otherTeacher, _ := makeClassRow(t, pool)
	issueCode(t, rotating, otherClass, otherTeacher)
	newKey := mustJoinCodeKeys(joinKeyB, nil).CurrentID()
	if row := activeRow(t, pool, otherClass); row.keyID == nil || *row.keyID != newKey || newKey == joinKeys.CurrentID() {
		t.Errorf("a code issued while rotating is under key %v, want the new key %d", row.keyID, newKey)
	}
}

func TestAKeyedRowHoldingALegacyHashIsRefused(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	code, err := domain.JoinCodes.Generate()
	if err != nil {
		t.Fatal(err)
	}
	junk := make([]byte, 36)
	if _, err := rand.Read(junk); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO app.class_join_codes
		       (class_id, code_hash, code_ciphertext, key_id, lookup_scheme, code_hint, expires_at, max_uses, created_by)
		VALUES ($1, $2, $3, $4, 2, $5, now() + interval '30 days', 40, $6)`,
		classID, domain.JoinCodes.Hash(code), junk, joinKeys.CurrentID(), domain.JoinCodes.Hint(code), teacherID); err != nil {
		t.Fatal(err)
	}
	refusesAll(t, pool, svc, code)
	if n := usesCount(t, pool, classID); n != 0 {
		t.Errorf("uses_count = %d, want 0", n)
	}
}

func TestTheNewestRowWinsWhenTwoRowsHoldTheSameCode(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	oldClass, oldTeacher, _ := makeClassRow(t, pool)
	newClass, newTeacher, _ := makeClassRow(t, pool)
	code, err := domain.JoinCodes.Generate()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, revoked_at, created_by, created_at)
		VALUES ($1, $2, $3, now() + interval '30 days', now(), $4, now() - interval '1 hour')`,
		oldClass, domain.JoinCodes.Hash(code), domain.JoinCodes.Hint(code), oldTeacher); err != nil {
		t.Fatal(err)
	}
	id := uuid.Must(uuid.NewV7()).String()
	sealed, err := joinKeys.Seal(newClass, id, code)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO app.class_join_codes
		       (id, class_id, code_hash, code_ciphertext, key_id, lookup_scheme, code_hint, expires_at, max_uses, created_by)
		VALUES ($1, $2, $3, $4, $5, 2, $6, now() + interval '30 days', 40, $7)`,
		id, newClass, joinKeys.Hash(code), sealed, joinKeys.CurrentID(), domain.JoinCodes.Hint(code), newTeacher); err != nil {
		t.Fatal(err)
	}
	joinsAll(t, pool, svc, newClass, code)
}

func TestNoAuditRowCarriesTheCodeOrItsCiphertext(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	joinsAll(t, pool, svc, classID, code)
	row := activeRow(t, pool, classID)

	var leaks int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM app.audit_log
		 WHERE (entity_id = $1::uuid OR entity_id = $2::uuid OR diff->>'class_id' = $1::text)
		   AND (coalesce(diff::text, '') ILIKE '%' || $3 || '%'
		     OR coalesce(diff::text, '') ILIKE '%' || $4 || '%'
		     OR coalesce(diff::text, '') ILIKE '%' || $5 || '%')`,
		classID, row.id, domain.JoinCodes.Normalize(code), hex.EncodeToString(row.ciphertext), hex.EncodeToString(row.hash)).Scan(&leaks); err != nil {
		t.Fatal(err)
	}
	if leaks != 0 {
		t.Errorf("%d audit row(s) carry the code, its ciphertext or its hash", leaks)
	}
}

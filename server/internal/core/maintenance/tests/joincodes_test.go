//go:build integration

package maintenance_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/core/maintenance"
	classesapp "quizzivy/internal/modules/classes/application"
	classescmd "quizzivy/internal/modules/classes/application/command"
	classesquery "quizzivy/internal/modules/classes/application/query"
	classesdomain "quizzivy/internal/modules/classes/domain"
	classesrepo "quizzivy/internal/modules/classes/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

var (
	rekeyOld   = bytes.Repeat([]byte{0x3c}, classesdomain.JoinCodeKeySize)
	rekeyNew   = bytes.Repeat([]byte{0x4d}, classesdomain.JoinCodeKeySize)
	rekeyStray = bytes.Repeat([]byte{0x5e}, classesdomain.JoinCodeKeySize)
)

type rekeyWorld struct {
	ctx     context.Context
	tx      pgx.Tx
	teacher string
	student string
	classes []string
	codes   []string
	legacy  string
}

func keysFor(t *testing.T, current, previous []byte) classesdomain.JoinCodeKeys {
	t.Helper()
	keys, err := classesdomain.NewJoinCodeKeys(current, previous)
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

func (w *rekeyWorld) classesWith(keys classesdomain.JoinCodeKeys) *classesapp.Application {
	return classesapp.New(classesrepo.NewPostgres(db.NewContext(w.tx)), nil, keys)
}

func (w *rekeyWorld) id(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := w.tx.QueryRow(w.ctx, sql, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func newRekeyWorld(t *testing.T) *rekeyWorld {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	w := &rekeyWorld{ctx: ctx, tx: tx}
	n := uuid.NewString()
	w.teacher = w.id(t, `INSERT INTO app.users (email, full_name, role_id) VALUES ($1, 'Giáo viên', (SELECT id FROM app.roles WHERE builtin_key = 'teacher')) RETURNING id::text`, "rekey-teacher-"+n+"@example.com")
	w.student = w.id(t, `INSERT INTO app.users (email, full_name, role_id) VALUES ($1, 'Học viên', (SELECT id FROM app.roles WHERE builtin_key = 'student')) RETURNING id::text`, "rekey-student-"+n+"@example.com")
	old := w.classesWith(keysFor(t, rekeyOld, nil))
	for i := range 3 {
		class := w.id(t, `INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2) RETURNING id::text`, "Lớp xoay khoá "+n+string(rune('A'+i)), w.teacher)
		rotated, err := old.Commands.Rotate.Handle(ctx, classescmd.Rotate{Request: classesdomain.RotateRequest{ClassID: class, ActorUserID: w.teacher}})
		if err != nil {
			t.Fatal(err)
		}
		w.classes = append(w.classes, class)
		w.codes = append(w.codes, rotated.Code)
	}
	if _, err := old.Commands.Rotate.Handle(ctx, classescmd.Rotate{Request: classesdomain.RotateRequest{ClassID: w.classes[2], ActorUserID: w.teacher}}); err != nil {
		t.Fatal(err)
	}
	legacyClass := w.id(t, `INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2) RETURNING id::text`, "Lớp mã cũ "+n, w.teacher)
	w.legacy = w.id(t, `INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by)
		VALUES ($1, sha256($2::bytea), 'EGAC', now() + interval '30 days', $3) RETURNING id::text`, legacyClass, n, w.teacher)
	return w
}

func (w *rekeyWorld) isolate(t *testing.T) {
	t.Helper()
	if _, err := w.tx.Exec(w.ctx, `LOCK TABLE app.class_join_codes IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	if _, err := w.tx.Exec(w.ctx, `
		UPDATE app.class_join_codes SET revoked_at = now()
		 WHERE revoked_at IS NULL AND lookup_scheme = 2 AND key_id <> ALL($1::smallint[])`,
		[]int16{keysFor(t, rekeyOld, nil).CurrentID(), keysFor(t, rekeyNew, nil).CurrentID()}); err != nil {
		t.Fatal(err)
	}
}

func (w *rekeyWorld) snapshot(t *testing.T) string {
	t.Helper()
	var s string
	if err := w.tx.QueryRow(w.ctx, `
		SELECT string_agg(id::text || encode(code_hash, 'hex') || coalesce(encode(code_ciphertext, 'hex'), '') || coalesce(key_id::text, '') || lookup_scheme, ',' ORDER BY id)
		  FROM app.class_join_codes WHERE class_id = ANY($1::uuid[]) OR id = $2::uuid`, w.classes, w.legacy).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func (w *rekeyWorld) rekey(t *testing.T, apply bool, batch int) maintenance.RekeyReport {
	t.Helper()
	report, err := maintenance.RekeyJoinCodes(w.ctx, w.tx, keysFor(t, rekeyNew, rekeyOld), apply, batch)
	if err != nil {
		t.Fatalf("rekey (apply %v): %v", apply, err)
	}
	return report
}

func TestTheDryRunCountsAndChangesNothing(t *testing.T) {
	w := newRekeyWorld(t)
	before := w.snapshot(t)
	report := w.rekey(t, false, 500)
	oldID, newID := keysFor(t, rekeyOld, nil).CurrentID(), keysFor(t, rekeyNew, nil).CurrentID()
	if report.Applied || report.Moved != 0 || report.Pending != 4 || report.SealedByKey[oldID] != 4 || report.SealedByKey[newID] != 0 || report.Legacy < 1 || len(report.Unopened) != 0 {
		t.Errorf("dry run reported %+v, want 4 pending under %d and at least one legacy code", report, oldID)
	}
	if report.CurrentKeyID != newID || report.PreviousKeyID != oldID {
		t.Errorf("key ids %d and %d, want %d and %d", report.CurrentKeyID, report.PreviousKeyID, newID, oldID)
	}
	if w.snapshot(t) != before {
		t.Error("the dry run changed a join code row")
	}
}

func TestEveryCodeUnderTheOldKeyMovesAndStillRedeems(t *testing.T) {
	w := newRekeyWorld(t)
	w.isolate(t)
	before := w.snapshot(t)
	report := w.rekey(t, true, 3)
	oldID, newID := keysFor(t, rekeyOld, nil).CurrentID(), keysFor(t, rekeyNew, nil).CurrentID()
	if !report.Applied || report.Moved != 4 || report.Pending != 0 || report.SealedByKey[oldID] != 0 || report.SealedByKey[newID] != 4 {
		t.Fatalf("apply reported %+v, want 4 moved in two batches and none left under %d", report, oldID)
	}
	var legacyScheme int
	var legacyKey *int16
	if err := w.tx.QueryRow(w.ctx, `SELECT lookup_scheme, key_id FROM app.class_join_codes WHERE id = $1`, w.legacy).Scan(&legacyScheme, &legacyKey); err != nil || legacyScheme != 1 || legacyKey != nil {
		t.Errorf("the legacy code became scheme %d under %v (%v)", legacyScheme, legacyKey, err)
	}
	if w.snapshot(t) == before {
		t.Error("nothing changed")
	}

	onlyNew := w.classesWith(keysFor(t, rekeyNew, nil))
	for i, code := range w.codes[:2] {
		preview, err := onlyNew.Queries.Preview.Handle(w.ctx, classesquery.Preview{Code: code})
		if err != nil || preview.Outcome != classesdomain.PreviewOK || preview.ClassID != w.classes[i] {
			t.Errorf("code %d with only the new key: %+v (%v)", i, preview, err)
		}
		read, err := onlyNew.Queries.ActiveCode.Handle(w.ctx, classesquery.ActiveCode{Scope: access.Scope{UserID: w.teacher}, ClassID: w.classes[i]})
		if err != nil || read.Code != classesdomain.JoinCodes.Normalize(code) {
			t.Errorf("code %d read back with only the new key: %+v (%v)", i, read, err)
		}
	}
	joined, err := onlyNew.Commands.EnrolExisting.Handle(w.ctx, classescmd.EnrolExisting{UserID: w.student, Code: w.codes[0]})
	if err != nil || joined.Outcome != classesdomain.PreviewOK || joined.Class.ID != w.classes[0] {
		t.Errorf("joining with only the new key: %+v (%v)", joined, err)
	}
	if again := w.rekey(t, true, 500); again.Moved != 0 || again.Pending != 0 {
		t.Errorf("a second run moved %d, pending %d; want nothing", again.Moved, again.Pending)
	}
}

func TestACodeThatDoesNotOpenIsListedAndTheRestMove(t *testing.T) {
	w := newRekeyWorld(t)
	w.isolate(t)
	var broken string
	if err := w.tx.QueryRow(w.ctx, `
		UPDATE app.class_join_codes
		   SET code_ciphertext = set_byte(code_ciphertext, 20, get_byte(code_ciphertext, 20) # 1)
		 WHERE class_id = $1 AND revoked_at IS NULL RETURNING id::text`, w.classes[1]).Scan(&broken); err != nil {
		t.Fatal(err)
	}
	listed := func(report maintenance.RekeyReport, revoked bool) bool {
		return len(report.Unopened) == 1 && report.Unopened[0] == maintenance.UnopenedCode{ID: broken, ClassID: w.classes[1], Revoked: revoked}
	}
	dry := w.rekey(t, false, 3)
	if !listed(dry, false) || dry.Pending != 3 {
		t.Errorf("the dry run reported %+v, want the broken code listed and three to move", dry)
	}
	report := w.rekey(t, true, 3)
	if !listed(report, false) || report.Moved != 3 || report.Pending != 0 {
		t.Fatalf("apply reported %+v, want three moved past the broken code, which stays listed", report)
	}
	for _, code := range []string{w.codes[0]} {
		preview, err := w.classesWith(keysFor(t, rekeyNew, nil)).Queries.Preview.Handle(w.ctx, classesquery.Preview{Code: code})
		if err != nil || preview.Outcome != classesdomain.PreviewOK {
			t.Errorf("a code after the broken one did not move: %+v (%v)", preview, err)
		}
	}
	if _, err := w.classesWith(keysFor(t, rekeyNew, rekeyOld)).Commands.Rotate.Handle(w.ctx, classescmd.Rotate{Request: classesdomain.RotateRequest{ClassID: w.classes[1], ActorUserID: w.teacher}}); err != nil {
		t.Fatal(err)
	}
	again := w.rekey(t, true, 3)
	if !listed(again, true) || again.Moved != 0 || again.Pending != 0 {
		t.Errorf("after the class rotated, the run reported %+v, want the broken code listed as revoked and nothing else to do", again)
	}
}

func TestAKeyPairTheAPIDoesNotRunWithIsRefused(t *testing.T) {
	w := newRekeyWorld(t)
	w.isolate(t)
	stray := w.id(t, `INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp khoá lạ', $1) RETURNING id::text`, w.teacher)
	for range 2 {
		if _, err := w.classesWith(keysFor(t, rekeyStray, nil)).Commands.Rotate.Handle(w.ctx, classescmd.Rotate{Request: classesdomain.RotateRequest{ClassID: stray, ActorUserID: w.teacher}}); err != nil {
			t.Fatal(err)
		}
	}
	before := w.snapshot(t)
	if dry := w.rekey(t, false, 500); dry.Unknown != 1 {
		t.Errorf("the dry run reported %d active codes under an unknown key, want 1", dry.Unknown)
	}
	_, err := maintenance.RekeyJoinCodes(w.ctx, w.tx, keysFor(t, rekeyNew, rekeyOld), true, 500)
	if err == nil || !strings.Contains(err.Error(), "neither JOIN_CODE_KEY") {
		t.Fatalf("apply with a code under a third key: %v, want a refusal", err)
	}
	if w.snapshot(t) != before {
		t.Error("a refused apply changed rows")
	}
}

func TestTheRekeyNeedsAPreviousKey(t *testing.T) {
	w := newRekeyWorld(t)
	if _, err := maintenance.RekeyJoinCodes(w.ctx, w.tx, keysFor(t, rekeyNew, nil), true, 500); err == nil || !strings.Contains(err.Error(), "JOIN_CODE_KEY_PREVIOUS") {
		t.Errorf("rekey without a previous key: %v", err)
	}
}

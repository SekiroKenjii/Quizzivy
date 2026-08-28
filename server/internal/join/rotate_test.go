package join_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/join"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping join integration tests")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func nonce(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// makeClass creates a teacher, a class, and one enrolled student, and removes
// them afterwards. The student is the point of several of these tests: §6.1
// says rotating a code leaves existing members alone.
func makeClass(t *testing.T, pool *pgxpool.Pool) (classID, teacherID, studentID string) {
	t.Helper()
	ctx := context.Background()
	n := nonce(t)

	if err := pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role)
		 VALUES ($1, 'Giáo viên', 'admin') RETURNING id::text`,
		"teacher-"+n+"@example.com").Scan(&teacherID); err != nil {
		t.Fatalf("insert teacher: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role)
		 VALUES ($1, 'Học viên', 'student') RETURNING id::text`,
		"student-"+n+"@example.com").Scan(&studentID); err != nil {
		t.Fatalf("insert student: %v", err)
	}
	// app.classes has no created_by: there is one teacher (§1), so recording
	// which one made a class would be a column with one value in it forever.
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.classes (name) VALUES ($1) RETURNING id::text`,
		"Lớp "+n).Scan(&classID); err != nil {
		t.Fatalf("insert class: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
		 VALUES ($1, $2, 'admin', $3)`, classID, studentID, teacherID); err != nil {
		t.Fatalf("enrol student: %v", err)
	}

	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE actor_user_id IN ($1, $2)`, teacherID, studentID)
		_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE class_id = $1`, classID)
		_, _ = pool.Exec(c, `DELETE FROM app.class_join_codes WHERE class_id = $1`, classID)
		_, _ = pool.Exec(c, `DELETE FROM app.classes WHERE id = $1`, classID)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id IN ($1, $2)`, teacherID, studentID)
	})
	return classID, teacherID, studentID
}

func newSvc(t *testing.T, pool *pgxpool.Pool) *join.Service {
	t.Helper()
	return join.NewService(join.NewStore(pool))
}

func activeCodeCount(t *testing.T, pool *pgxpool.Pool, classID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.class_join_codes WHERE class_id = $1 AND revoked_at IS NULL`,
		classID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func selfJoinEnabled(t *testing.T, pool *pgxpool.Pool, classID string) bool {
	t.Helper()
	var on bool
	if err := pool.QueryRow(context.Background(),
		`SELECT self_join_enabled FROM app.classes WHERE id = $1`, classID).Scan(&on); err != nil {
		t.Fatal(err)
	}
	return on
}

func TestRotationRetiresTheOldCodeAndLeavesMembersAlone(t *testing.T) {
	// §6.1's promise. A teacher rotates because a code leaked; if that also
	// unenrolled the class, nobody would ever rotate.
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, studentID := makeClass(t, pool)
	ctx := context.Background()

	first, err := svc.Rotate(ctx, join.RotateRequest{ClassID: classID, ActorUserID: teacherID})
	if err != nil {
		t.Fatalf("first rotate: %v", err)
	}
	second, err := svc.Rotate(ctx, join.RotateRequest{ClassID: classID, ActorUserID: teacherID})
	if err != nil {
		t.Fatalf("second rotate: %v", err)
	}
	if first.Code == second.Code {
		t.Fatal("rotation returned the same code")
	}

	// The old code is no longer the active row...
	var revoked bool
	if err := pool.QueryRow(ctx,
		`SELECT revoked_at IS NOT NULL FROM app.class_join_codes WHERE code_hash = $1`,
		join.Hash(join.Normalize(first.Code))).Scan(&revoked); err != nil {
		t.Fatalf("old code row: %v", err)
	}
	if !revoked {
		t.Error("the previous code was not revoked")
	}
	// ...and exactly one is.
	if n := activeCodeCount(t, pool, classID); n != 1 {
		t.Errorf("active codes = %d, want 1", n)
	}

	// The enrolled student is untouched.
	var stillMember bool
	if err := pool.QueryRow(ctx,
		`SELECT true FROM app.class_members WHERE class_id = $1 AND user_id = $2`,
		classID, studentID).Scan(&stillMember); err != nil {
		t.Fatalf("the student was unenrolled by a rotation: %v", err)
	}
}

func TestOnlyTheHashAndAHintAreStored(t *testing.T) {
	// §13.3. A database dump must not hand over class access.
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)

	rotated, err := svc.Rotate(context.Background(), join.RotateRequest{
		ClassID: classID, ActorUserID: teacherID})
	if err != nil {
		t.Fatal(err)
	}
	canonical := join.Normalize(rotated.Code)

	var hint string
	var hash []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT code_hint, code_hash FROM app.class_join_codes
		  WHERE class_id = $1 AND revoked_at IS NULL`, classID).Scan(&hint, &hash); err != nil {
		t.Fatal(err)
	}
	if hint != canonical[len(canonical)-4:] {
		t.Errorf("hint = %q, want the last four of %q", hint, canonical)
	}
	if !join.Equal(hash, join.Hash(canonical)) {
		t.Error("the stored hash does not match the issued code")
	}

	// Nothing anywhere in the row holds the plaintext.
	var plaintextRows int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.class_join_codes
		  WHERE class_id = $1 AND (code_hint = $2 OR encode(code_hash,'escape') LIKE '%' || $2 || '%')`,
		classID, canonical).Scan(&plaintextRows); err != nil {
		t.Fatal(err)
	}
	if plaintextRows != 0 {
		t.Error("the plaintext code is recoverable from the row")
	}
}

func TestTheDefaultsAreThirtyDaysAndFortyUses(t *testing.T) {
	// §6.1 for the expiry; O-06 for the cap, which deliberately departs from
	// §6.1's `null = unlimited`.
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)

	rotated, err := svc.Rotate(context.Background(), join.RotateRequest{
		ClassID: classID, ActorUserID: teacherID})
	if err != nil {
		t.Fatal(err)
	}
	if rotated.MaxUses == nil || *rotated.MaxUses != join.DefaultMaxUses {
		t.Errorf("maxUses = %v, want %d", rotated.MaxUses, join.DefaultMaxUses)
	}
	days := rotated.ExpiresAt.Sub(time.Now()).Hours() / 24
	if days < 29 || days > 31 {
		t.Errorf("expiry is %.1f days away, want about %d", days, join.DefaultExpiryDays)
	}
}

func TestRevokingClosesTheClassCompletely(t *testing.T) {
	// §6.4: both halves. A revoked code with self-join still on advertises a
	// flow that cannot succeed; a cleared flag without the revocation leaves a
	// live bearer secret the teacher believes they cancelled.
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	ctx := context.Background()

	if _, err := svc.Rotate(ctx, join.RotateRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
		t.Fatal(err)
	}
	if !selfJoinEnabled(t, pool, classID) {
		t.Fatal("issuing a code did not enable self-join")
	}

	if err := svc.Revoke(ctx, join.RevokeRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if n := activeCodeCount(t, pool, classID); n != 0 {
		t.Errorf("active codes after revoke = %d, want 0", n)
	}
	if selfJoinEnabled(t, pool, classID) {
		t.Error("self-join is still enabled after a revoke")
	}
}

func TestRotatingAfterARevokeReopensTheClass(t *testing.T) {
	// Otherwise the teacher is handed a code that silently does nothing --
	// §6.4 turned the flag off, and issuing a code is the action that means
	// "let students in again".
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	ctx := context.Background()

	if err := svc.Revoke(ctx, join.RevokeRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rotate(ctx, join.RotateRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
		t.Fatal(err)
	}
	if !selfJoinEnabled(t, pool, classID) {
		t.Error("rotating after a revoke left self-join off; the new code cannot be used")
	}
}

func TestRevokingIsIdempotent(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	ctx := context.Background()

	for i := range 3 {
		if err := svc.Revoke(ctx, join.RevokeRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
			t.Fatalf("revoke %d: %v", i+1, err)
		}
	}
}

func TestAMissingClassIsReportedRatherThanCreatingAnOrphanCode(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	_, teacherID, _ := makeClass(t, pool)
	const ghost = "01935000-0000-7000-8000-00000000ffff"

	if _, err := svc.Rotate(context.Background(), join.RotateRequest{
		ClassID: ghost, ActorUserID: teacherID}); !errors.Is(err, join.ErrClassNotFound) {
		t.Errorf("rotate: error = %v, want ErrClassNotFound", err)
	}
	if err := svc.Revoke(context.Background(), join.RevokeRequest{
		ClassID: ghost, ActorUserID: teacherID}); !errors.Is(err, join.ErrClassNotFound) {
		t.Errorf("revoke: error = %v, want ErrClassNotFound", err)
	}
}

func TestConcurrentRotationsNeverLeaveTwoActiveCodes(t *testing.T) {
	// class_join_codes_one_active is a partial unique index. Inserting before
	// revoking violates it; revoking before inserting leaves a window with no
	// code at all. The transaction plus the row lock on the class is what makes
	// neither state observable -- without them this test produces either a
	// constraint violation or two live bearer secrets for one class.
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)

	const racers = 6
	var wg sync.WaitGroup
	errs := make([]error, racers)
	release := make(chan struct{})
	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-release
			_, errs[i] = svc.Rotate(context.Background(), join.RotateRequest{
				ClassID: classID, ActorUserID: teacherID})
		}()
	}
	close(release)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("racer %d failed: %v", i, err)
		}
	}
	if n := activeCodeCount(t, pool, classID); n != 1 {
		t.Fatalf("active codes = %d, want exactly 1", n)
	}
}

func TestIssuingRotatingAndRevokingAreAudited(t *testing.T) {
	// §13.4. A join code is a bearer secret; who issued one, and when it was
	// cancelled, is exactly what an audit trail is for.
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	ctx := context.Background()

	if _, err := svc.Rotate(ctx, join.RotateRequest{
		ClassID: classID, ActorUserID: teacherID, IP: "203.0.113.9", UserAgent: "go-test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rotate(ctx, join.RotateRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Revoke(ctx, join.RevokeRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
		t.Fatal(err)
	}

	rows, err := pool.Query(ctx,
		`SELECT action FROM app.audit_log WHERE actor_user_id = $1 ORDER BY id`, teacherID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var actions []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, a)
	}

	// The first issue and the rotation are distinguished: "there was no code
	// before" and "a live code was retired" are different events.
	want := []string{"class.join_code_issued", "class.join_code_rotated", "class.join_code_revoked"}
	if len(actions) != len(want) {
		t.Fatalf("audit actions = %v, want %v", actions, want)
	}
	for i, a := range actions {
		if a != want[i] {
			t.Errorf("audit[%d] = %q, want %q", i, a, want[i])
		}
	}
}

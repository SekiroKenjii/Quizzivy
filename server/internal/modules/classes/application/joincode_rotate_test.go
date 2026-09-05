//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/classes/domain/joincode"
	"quizzivy/internal/modules/classes/repositories"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// makeClassRow creates a teacher, a class, and one enrolled student, and removes
// them afterwards. The student is the point of several of these tests: §6.1
// says rotating a code leaves existing members alone.
func makeClassRow(t *testing.T, pool *pgxpool.Pool) (classID, teacherID, studentID string) {
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
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE actor_user_id IN ($1, $2)`, teacherID, studentID)
		_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE class_id = $1`, classID)
		_, _ = pool.Exec(c, `DELETE FROM app.class_join_codes WHERE class_id = $1`, classID)
		_, _ = pool.Exec(c, `DELETE FROM app.classes WHERE id = $1`, classID)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id IN ($1, $2)`, teacherID, studentID)
	})
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

	return classID, teacherID, studentID
}

func newSvc(t *testing.T, pool *pgxpool.Pool) *application.Enrolment {
	t.Helper()
	return application.NewEnrolment(repositories.NewPostgres(pool))
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
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, studentID := makeClassRow(t, pool)
	ctx := context.Background()

	first, err := svc.Rotate(ctx, domain.RotateRequest{ClassID: classID, ActorUserID: teacherID})
	if err != nil {
		t.Fatalf("first rotate: %v", err)
	}
	second, err := svc.Rotate(ctx, domain.RotateRequest{ClassID: classID, ActorUserID: teacherID})
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
		joincode.Hash(joincode.Normalize(first.Code))).Scan(&revoked); err != nil {
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
	classID, teacherID, _ := makeClassRow(t, pool)

	rotated, err := svc.Rotate(context.Background(), domain.RotateRequest{
		ClassID: classID, ActorUserID: teacherID})
	if err != nil {
		t.Fatal(err)
	}
	canonical := joincode.Normalize(rotated.Code)

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
	if !joincode.Equal(hash, joincode.Hash(canonical)) {
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
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)

	rotated, err := svc.Rotate(context.Background(), domain.RotateRequest{
		ClassID: classID, ActorUserID: teacherID})
	if err != nil {
		t.Fatal(err)
	}
	if rotated.MaxUses == nil || *rotated.MaxUses != domain.DefaultMaxUses {
		t.Errorf("maxUses = %v, want %d", rotated.MaxUses, domain.DefaultMaxUses)
	}
	days := time.Until(rotated.ExpiresAt).Hours() / 24
	if days < 29 || days > 31 {
		t.Errorf("expiry is %.1f days away, want about %d", days, domain.DefaultExpiryDays)
	}
}

func TestRevokingClosesTheClassCompletely(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	ctx := context.Background()

	if _, err := svc.Rotate(ctx, domain.RotateRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
		t.Fatal(err)
	}
	if !selfJoinEnabled(t, pool, classID) {
		t.Fatal("issuing a code did not enable self-join")
	}

	if err := svc.Revoke(ctx, domain.RevokeRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
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
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	ctx := context.Background()

	if err := svc.Revoke(ctx, domain.RevokeRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rotate(ctx, domain.RotateRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
		t.Fatal(err)
	}
	if !selfJoinEnabled(t, pool, classID) {
		t.Error("rotating after a revoke left self-join off; the new code cannot be used")
	}
}

func TestRevokingIsIdempotent(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	ctx := context.Background()

	for i := range 3 {
		if err := svc.Revoke(ctx, domain.RevokeRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
			t.Fatalf("revoke %d: %v", i+1, err)
		}
	}
}

func TestAMissingClassIsReportedRatherThanCreatingAnOrphanCode(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	_, teacherID, _ := makeClassRow(t, pool)
	const ghost = "01935000-0000-7000-8000-00000000ffff"

	if _, err := svc.Rotate(context.Background(), domain.RotateRequest{
		ClassID: ghost, ActorUserID: teacherID}); !errors.Is(err, domain.ErrClassNotFound) {
		t.Errorf("rotate: error = %v, want ErrClassNotFound", err)
	}
	if err := svc.Revoke(context.Background(), domain.RevokeRequest{
		ClassID: ghost, ActorUserID: teacherID}); !errors.Is(err, domain.ErrClassNotFound) {
		t.Errorf("revoke: error = %v, want ErrClassNotFound", err)
	}
}

func TestConcurrentRotationsNeverLeaveTwoActiveCodes(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)

	const racers = 6
	var wg sync.WaitGroup
	errs := make([]error, racers)
	release := make(chan struct{})
	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-release
			_, errs[i] = svc.Rotate(context.Background(), domain.RotateRequest{
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
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClassRow(t, pool)
	ctx := context.Background()

	if _, err := svc.Rotate(ctx, domain.RotateRequest{
		ClassID: classID, ActorUserID: teacherID, IP: "203.0.113.9", UserAgent: "go-test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rotate(ctx, domain.RotateRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Revoke(ctx, domain.RevokeRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
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

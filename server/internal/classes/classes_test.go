package classes_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/classes"
	"quizzivy/internal/join"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
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

func makeClass(t *testing.T, pool *pgxpool.Pool) (classID, teacherID, studentID string) {
	t.Helper()
	ctx := context.Background()
	n := nonce(t)

	if err := pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'Giáo viên','admin') RETURNING id::text`,
		"t-"+n+"@example.com").Scan(&teacherID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'Nguyễn Văn A','student') RETURNING id::text`,
		"s-"+n+"@example.com").Scan(&studentID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.classes (name) VALUES ($1) RETURNING id::text`, "Lớp "+n).Scan(&classID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE actor_user_id IN ($1,$2)`, teacherID, studentID)
		_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE class_id = $1`, classID)
		_, _ = pool.Exec(c, `DELETE FROM app.class_join_codes WHERE class_id = $1`, classID)
		_, _ = pool.Exec(c, `DELETE FROM app.classes WHERE id = $1`, classID)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id IN ($1,$2)`, teacherID, studentID)
	})
	return classID, teacherID, studentID
}

func TestAClassCarriesItsCodesMetadataAndNeverTheCode(t *testing.T) {
	pool := newPool(t)
	classID, teacherID, _ := makeClass(t, pool)
	svc := classes.NewService(classes.NewStore(pool))
	joins := join.NewService(join.NewStore(pool))

	rotated, err := joins.Rotate(context.Background(), join.RotateRequest{
		ClassID: classID, ActorUserID: teacherID})
	if err != nil {
		t.Fatal(err)
	}

	got, err := svc.Get(context.Background(), classID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.JoinCode == nil {
		t.Fatal("no join code metadata on a class that has an active code")
	}
	canonical := join.Normalize(rotated.Code)
	if got.JoinCode.Hint != canonical[len(canonical)-4:] {
		t.Errorf("hint = %q", got.JoinCode.Hint)
	}
	if got.JoinCode.MaxUses == nil || *got.JoinCode.MaxUses != join.DefaultMaxUses {
		t.Errorf("maxUses = %v", got.JoinCode.MaxUses)
	}
	if got.JoinCode.UsesCount != 0 {
		t.Errorf("usesCount = %d, want 0", got.JoinCode.UsesCount)
	}
	if !got.SelfJoinEnabled {
		t.Error("issuing a code left self-join off")
	}
}

func TestAClassWithNoActiveCodeReportsNone(t *testing.T) {
	// A closed class is a normal state, not a missing row.
	pool := newPool(t)
	classID, _, _ := makeClass(t, pool)
	svc := classes.NewService(classes.NewStore(pool))

	got, err := svc.Get(context.Background(), classID)
	if err != nil {
		t.Fatal(err)
	}
	if got.JoinCode != nil {
		t.Errorf("joinCode = %+v, want nil", got.JoinCode)
	}
}

func TestMembersShowHowEachOneGotIn(t *testing.T) {
	pool := newPool(t)
	classID, teacherID, studentID := makeClass(t, pool)
	svc := classes.NewService(classes.NewStore(pool))
	joins := join.NewService(join.NewStore(pool))
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
		 VALUES ($1,$2,'admin',$3)`, classID, teacherID, teacherID); err != nil {
		t.Fatal(err)
	}
	rotated, err := joins.Rotate(ctx, join.RotateRequest{ClassID: classID, ActorUserID: teacherID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := joins.EnrolExisting(ctx, studentID, rotated.Code, join.Meta{}); err != nil {
		t.Fatal(err)
	}

	members, err := svc.Members(ctx, classID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("members = %d, want 2", len(members))
	}

	bySource := map[string]classes.Member{}
	for _, m := range members {
		bySource[m.JoinedVia] = m
	}
	if _, ok := bySource["admin"]; !ok {
		t.Error("no admin-added member reported")
	}
	viaCode, ok := bySource["join_code"]
	if !ok {
		t.Fatal("no join_code member reported")
	}
	if viaCode.JoinCodeHint == nil {
		t.Error("a code-joined member has no code hint; a rotation could not be traced")
	}
	if bySource["admin"].JoinCodeHint != nil {
		t.Error("an admin-added member has a code hint")
	}
}

func TestRemovingAMemberRevokesAccessAndKeepsTheirWork(t *testing.T) {
	pool := newPool(t)
	classID, teacherID, studentID := makeClass(t, pool)
	svc := classes.NewService(classes.NewStore(pool))
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
		 VALUES ($1,$2,'admin',$3)`, classID, studentID, teacherID); err != nil {
		t.Fatal(err)
	}

	if err := svc.RemoveMember(ctx, classID, studentID, teacherID, "203.0.113.9", "go-test"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	members, err := svc.Members(ctx, classID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 0 {
		t.Errorf("members = %d, want 0", len(members))
	}
	// The account survives: removing someone from a class is not deleting them.
	var stillExists bool
	if err := pool.QueryRow(ctx, `SELECT true FROM app.users WHERE id = $1`, studentID).Scan(&stillExists); err != nil {
		t.Fatalf("the student's account was deleted along with their membership: %v", err)
	}

	var action string
	if err := pool.QueryRow(ctx,
		`SELECT action FROM app.audit_log WHERE actor_user_id = $1 ORDER BY id DESC LIMIT 1`,
		teacherID).Scan(&action); err != nil {
		t.Fatalf("the removal was not audited: %v", err)
	}
	if action != "class.member_removed" {
		t.Errorf("audit action = %q", action)
	}
}

func TestRemovingSomeoneWhoIsNotAMemberSucceedsButAMissingClassDoesNot(t *testing.T) {
	pool := newPool(t)
	classID, teacherID, studentID := makeClass(t, pool)
	svc := classes.NewService(classes.NewStore(pool))
	ctx := context.Background()

	// Idempotent: the class ends up in the requested state either way.
	if err := svc.RemoveMember(ctx, classID, studentID, teacherID, "", ""); err != nil {
		t.Errorf("removing a non-member: %v", err)
	}
	err := svc.RemoveMember(ctx, "01935000-0000-7000-8000-00000000ffff", studentID, teacherID, "", "")
	if !errors.Is(err, classes.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestAddingAStudentRecordsThatAnAdminDidIt(t *testing.T) {
	pool := newPool(t)
	classID, teacherID, studentID := makeClass(t, pool)
	svc := classes.NewService(classes.NewStore(pool))
	ctx := context.Background()

	m, err := svc.AddMember(ctx, classID, studentID, teacherID, "203.0.113.9", "go-test")
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	// The distinction §6.4 relies on: an admin enrolment is not a code join.
	if m.JoinedVia != "admin" {
		t.Errorf("joinedVia = %q, want admin", m.JoinedVia)
	}
	if m.JoinCodeHint != nil {
		t.Errorf("joinCodeHint = %v, want nil", m.JoinCodeHint)
	}
	if m.UserID != studentID {
		t.Errorf("userId = %s, want %s", m.UserID, studentID)
	}

	members, err := svc.Members(ctx, classID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 {
		t.Fatalf("members = %d, want 1", len(members))
	}
}

// Clicking "Thêm" twice on a slow connection is the ordinary way this happens.
func TestAddingSomebodyTwiceIsNotAnError(t *testing.T) {
	pool := newPool(t)
	classID, teacherID, studentID := makeClass(t, pool)
	svc := classes.NewService(classes.NewStore(pool))
	ctx := context.Background()

	if _, err := svc.AddMember(ctx, classID, studentID, teacherID, "", ""); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if _, err := svc.AddMember(ctx, classID, studentID, teacherID, "", ""); err != nil {
		t.Fatalf("second add: %v", err)
	}

	members, err := svc.Members(ctx, classID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 {
		t.Errorf("members = %d, want 1", len(members))
	}

	var added int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.audit_log
		  WHERE action = 'class.member_added' AND entity_id = $1::uuid`, classID).Scan(&added); err != nil {
		t.Fatal(err)
	}
	if added != 1 {
		t.Errorf("audit rows = %d, want 1: the second add changed nothing", added)
	}
}

func TestOnlyAStudentCanBeEnrolled(t *testing.T) {
	pool := newPool(t)
	classID, teacherID, _ := makeClass(t, pool)
	svc := classes.NewService(classes.NewStore(pool))

	_, err := svc.AddMember(context.Background(), classID, teacherID, teacherID, "", "")
	if !errors.Is(err, classes.ErrNotAStudent) {
		t.Fatalf("enrolling an admin: want ErrNotAStudent, got %v", err)
	}
}

func TestAddingToAClassThatIsNotThereIsNotFound(t *testing.T) {
	pool := newPool(t)
	_, teacherID, studentID := makeClass(t, pool)
	svc := classes.NewService(classes.NewStore(pool))

	_, err := svc.AddMember(context.Background(),
		"00000000-0000-7000-8000-0000000000cc", studentID, teacherID, "", "")
	if !errors.Is(err, classes.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

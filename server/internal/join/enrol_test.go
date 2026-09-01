package join_test

import (
	"context"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/join"
)

func newMember(t *testing.T) join.NewMember {
	t.Helper()
	n := nonce(t)
	return join.NewMember{
		Email:          "self-join-" + n + "@example.com",
		FullName:       "Trần Thị B",
		Provider:       "google",
		ProviderUserID: "google-sub-" + n,
	}
}

// dropUser removes an account the enrolment created, which no fixture registered.
func dropUser(t *testing.T, pool *pgxpool.Pool, email string) {
	t.Helper()
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE user_id IN (SELECT id FROM app.users WHERE email = $1)`, email)
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE actor_user_id IN (SELECT id FROM app.users WHERE email = $1)`, email)
		_, _ = pool.Exec(c, `DELETE FROM app.user_identities WHERE user_id IN (SELECT id FROM app.users WHERE email = $1)`, email)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE email = $1`, email)
	})
}

func memberCount(t *testing.T, pool *pgxpool.Pool, classID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.class_members WHERE class_id = $1`, classID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func usesCount(t *testing.T, pool *pgxpool.Pool, classID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT uses_count FROM app.class_join_codes WHERE class_id = $1 AND revoked_at IS NULL`,
		classID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSelfJoinCreatesTheAccountAndEnrolsIt(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	m := newMember(t)
	dropUser(t, pool, m.Email)

	before := memberCount(t, pool, classID)
	result, err := svc.EnrolNewMember(context.Background(), m, code, join.Meta{IP: "203.0.113.44", UserAgent: "go-test"})
	if err != nil {
		t.Fatalf("EnrolNewMember: %v", err)
	}
	if result.Outcome != join.PreviewOK {
		t.Fatalf("outcome = %v, want PreviewOK", result.Outcome)
	}
	if result.AlreadyMember {
		t.Error("a brand-new account was reported as already a member")
	}
	if memberCount(t, pool, classID) != before+1 {
		t.Error("the member was not enrolled")
	}
	if n := usesCount(t, pool, classID); n != 1 {
		t.Errorf("uses_count = %d, want 1", n)
	}
	if result.Class.ID != classID || result.Class.Name == "" {
		t.Errorf("class = %+v", result.Class)
	}
	if result.Class.StudentCount != before+1 {
		t.Errorf("studentCount = %d, want %d", result.Class.StudentCount, before+1)
	}
	var joinedVia string
	var codeID *string
	if err := pool.QueryRow(context.Background(),
		`SELECT joined_via::text, join_code_id::text FROM app.class_members
		  WHERE class_id = $1 AND user_id = $2`, classID, result.UserID).Scan(&joinedVia, &codeID); err != nil {
		t.Fatal(err)
	}
	if joinedVia != "join_code" {
		t.Errorf("joined_via = %q, want join_code", joinedVia)
	}
	if codeID == nil {
		t.Error("join_code_id is null; the enrolment cannot be traced to a code")
	}
}

func TestConcurrentEnrolmentsAgainstALastSeatProduceExactlyOneMember(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)

	one := 1
	rotated, err := svc.Rotate(context.Background(), join.RotateRequest{
		ClassID: classID, ActorUserID: teacherID, MaxUses: &one})
	if err != nil {
		t.Fatal(err)
	}

	const racers = 6
	members := make([]join.NewMember, racers)
	for i := range members {
		members[i] = newMember(t)
		dropUser(t, pool, members[i].Email)
	}

	before := memberCount(t, pool, classID)
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		enrolled  int
		exhausted int
		other     []error
		release   = make(chan struct{})
	)
	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-release
			res, err := svc.EnrolNewMember(context.Background(), members[i], rotated.Code, join.Meta{})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err != nil:
				other = append(other, err)
			case res.Outcome == join.PreviewOK:
				enrolled++
			case res.Outcome == join.PreviewExhausted:
				exhausted++
			default:
				other = append(other, nil)
			}
		}()
	}
	close(release)
	wg.Wait()

	if len(other) > 0 {
		t.Fatalf("unexpected results from concurrent enrolment: %v", other)
	}
	if enrolled != 1 {
		t.Fatalf("%d of %d concurrent enrolments succeeded against max_uses=1, want exactly 1", enrolled, racers)
	}
	if exhausted != racers-1 {
		t.Errorf("exhausted = %d, want %d", exhausted, racers-1)
	}
	if got := memberCount(t, pool, classID); got != before+1 {
		t.Errorf("members = %d, want %d", got, before+1)
	}
	if n := usesCount(t, pool, classID); n != 1 {
		t.Errorf("uses_count = %d, want 1", n)
	}
}

func TestAnExpiredCodeCreatesNoUser(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	ctx := context.Background()

	if _, err := pool.Exec(ctx,
		`UPDATE app.class_join_codes
		    SET created_at = now() - interval '2 days', expires_at = now() - interval '1 day'
		  WHERE class_id = $1 AND revoked_at IS NULL`, classID); err != nil {
		t.Fatal(err)
	}

	m := newMember(t)
	dropUser(t, pool, m.Email)
	result, err := svc.EnrolNewMember(ctx, m, code, join.Meta{})
	if err != nil {
		t.Fatalf("EnrolNewMember: %v", err)
	}
	if result.Outcome != join.PreviewExpired {
		t.Fatalf("outcome = %v, want PreviewExpired", result.Outcome)
	}

	var users, identities int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE email = $1`, m.Email).Scan(&users); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.user_identities WHERE provider_user_id = $1`, m.ProviderUserID).Scan(&identities); err != nil {
		t.Fatal(err)
	}
	if users != 0 || identities != 0 {
		t.Errorf("an expired code left %d users and %d identities behind", users, identities)
	}
}

func TestEnrollingTwiceIsIdempotentAndDoesNotBurnAUse(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, studentID := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	ctx := context.Background()

	// The fixture's student joined via 'admin'; enrol a fresh one by code.
	m := newMember(t)
	dropUser(t, pool, m.Email)
	first, err := svc.EnrolNewMember(ctx, m, code, join.Meta{})
	if err != nil {
		t.Fatal(err)
	}

	for i := range 3 {
		again, err := svc.EnrolExisting(ctx, first.UserID, code, join.Meta{})
		if err != nil {
			t.Fatalf("repeat %d: %v", i+1, err)
		}
		if again.Outcome != join.PreviewOK {
			t.Fatalf("repeat %d: outcome = %v, want PreviewOK", i+1, again.Outcome)
		}
		if !again.AlreadyMember {
			t.Errorf("repeat %d: alreadyMember = false", i+1)
		}
	}
	if n := usesCount(t, pool, classID); n != 1 {
		t.Errorf("uses_count = %d after three repeats, want 1", n)
	}
	_ = studentID
}

func TestAnAlreadySignedInStudentCanEnrol(t *testing.T) {
	// §6.2's other half: no account is created, only a membership.
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	ctx := context.Background()

	// A student who exists but is not in this class.
	other, _, otherStudent := makeClass(t, pool)
	_ = other

	result, err := svc.EnrolExisting(ctx, otherStudent, code, join.Meta{IP: "203.0.113.77"})
	if err != nil {
		t.Fatalf("EnrolExisting: %v", err)
	}
	if result.Outcome != join.PreviewOK || result.AlreadyMember {
		t.Fatalf("outcome = %v alreadyMember = %v", result.Outcome, result.AlreadyMember)
	}
	if result.UserID != otherStudent {
		t.Errorf("enrolled %s, want %s", result.UserID, otherStudent)
	}
	if n := usesCount(t, pool, classID); n != 1 {
		t.Errorf("uses_count = %d, want 1", n)
	}
}

func TestABadCodeRefusesEveryEnrolmentPathAlike(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	ctx := context.Background()

	if err := svc.Revoke(ctx, join.RevokeRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
		t.Fatal(err)
	}

	m := newMember(t)
	dropUser(t, pool, m.Email)
	signup, err := svc.EnrolNewMember(ctx, m, code, join.Meta{})
	if err != nil {
		t.Fatal(err)
	}
	existing, err := svc.EnrolExisting(ctx, teacherID, code, join.Meta{})
	if err != nil {
		t.Fatal(err)
	}
	if signup.Outcome != existing.Outcome {
		t.Errorf("signup got %v, already-signed-in got %v; they must agree",
			signup.Outcome, existing.Outcome)
	}
	if signup.Outcome == join.PreviewOK {
		t.Error("a revoked code enrolled someone")
	}
}

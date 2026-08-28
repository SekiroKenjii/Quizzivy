package join_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/join"
)

// §6.5: "Log every enrolment (class_id, user_id, ip, user_agent, at) to the
// audit table." A leaked code lets a stranger into the class, and the audit row
// is how the teacher finds out who came in and through which code.

func TestAnEnrolmentWritesExactlyOneAuditRowWithANonNullIp(t *testing.T) {
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	ctx := context.Background()

	m := newMember(t)
	dropUser(t, pool, m.Email)
	result, err := svc.EnrolNewMember(ctx, m, code, join.Meta{
		IP: "203.0.113.201", UserAgent: "Mozilla/5.0 (iPhone)"})
	if err != nil {
		t.Fatalf("EnrolNewMember: %v", err)
	}

	rows, err := pool.Query(ctx,
		`SELECT action, entity, entity_id::text, host(ip), user_agent, occurred_at IS NOT NULL, diff
		   FROM app.audit_log WHERE actor_user_id = $1`, result.UserID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type entry struct {
		action, entity, entityID, ip, userAgent string
		hasTime                                 bool
		diff                                    []byte
	}
	var entries []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.action, &e.entity, &e.entityID, &e.ip, &e.userAgent, &e.hasTime, &e.diff); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("%d audit rows for one enrolment, want exactly 1", len(entries))
	}
	e := entries[0]
	if e.action != "class.member_enrolled" || e.entity != "class_member" {
		t.Errorf("action/entity = %s/%s", e.action, e.entity)
	}
	if e.entityID != classID {
		t.Errorf("entity_id = %s, want the class %s", e.entityID, classID)
	}
	// §6.5 names ip and user_agent specifically: an enrolment with neither is
	// a record that somebody joined and nothing about who.
	if e.ip != "203.0.113.201" {
		t.Errorf("ip = %q, want 203.0.113.201", e.ip)
	}
	if e.userAgent != "Mozilla/5.0 (iPhone)" {
		t.Errorf("user_agent = %q", e.userAgent)
	}
	if !e.hasTime {
		t.Error("occurred_at is null")
	}

	// The code is in the diff because after a rotation the teacher needs
	// "joined through the code that leaked" to be answerable.
	var diff map[string]string
	if err := json.Unmarshal(e.diff, &diff); err != nil {
		t.Fatalf("diff is not an object: %v", err)
	}
	if diff["class_id"] != classID || diff["user_id"] != result.UserID {
		t.Errorf("diff = %v", diff)
	}
	if diff["join_code_id"] == "" {
		t.Error("the audit row does not say which code was used")
	}
}

func TestARepeatedEnrolmentDoesNotWriteASecondAuditRow(t *testing.T) {
	// The membership did not change, so nothing happened to audit. Logging the
	// repeat would make a student refreshing a page look like an enrolment
	// event in the teacher's trail.
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	ctx := context.Background()

	m := newMember(t)
	dropUser(t, pool, m.Email)
	first, err := svc.EnrolNewMember(ctx, m, code, join.Meta{IP: "203.0.113.202"})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := svc.EnrolExisting(ctx, first.UserID, code, join.Meta{IP: "203.0.113.202"}); err != nil {
			t.Fatal(err)
		}
	}

	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.audit_log WHERE actor_user_id = $1`, first.UserID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("%d audit rows after one enrolment and three repeats, want 1", n)
	}
}

func TestARefusedEnrolmentIsNotAudited(t *testing.T) {
	// Nothing happened, so nothing is recorded. An audit trail that logs
	// attempts alongside events makes the events harder to find, and §6.5 asks
	// for enrolments.
	pool := newPool(t)
	svc := newSvc(t, pool)
	classID, teacherID, _ := makeClass(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	ctx := context.Background()

	if err := svc.Revoke(ctx, join.RevokeRequest{ClassID: classID, ActorUserID: teacherID}); err != nil {
		t.Fatal(err)
	}
	before := auditRowsFor(t, pool, teacherID)

	if _, err := svc.EnrolExisting(ctx, teacherID, code, join.Meta{IP: "203.0.113.203"}); err != nil {
		t.Fatal(err)
	}
	if after := auditRowsFor(t, pool, teacherID); after != before {
		t.Errorf("a refused enrolment wrote %d audit rows", after-before)
	}
}

func auditRowsFor(t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.audit_log WHERE actor_user_id = $1`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

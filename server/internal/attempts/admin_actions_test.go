package attempts_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/attempts"
)

func teacher(w world) attempts.Request {
	return attempts.Request{ActorID: w.admin, IP: "203.0.113.7", UserAgent: "go test"}
}

type auditRow struct {
	actor string
	diff  map[string]any
}

func auditRows(t *testing.T, pool *pgxpool.Pool, attemptID, action string) []auditRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT actor_user_id::text, diff FROM app.audit_log
		 WHERE entity = 'attempt' AND entity_id = $1::uuid AND action = $2
		 ORDER BY id`, attemptID, action)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var r auditRow
		if err := rows.Scan(&r.actor, &r.diff); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM app.audit_log WHERE entity = 'attempt' AND entity_id = $1::uuid`, attemptID)
	})
	return out
}

func oldNew(t *testing.T, diff map[string]any, field string) (any, any) {
	t.Helper()
	change, ok := diff[field].(map[string]any)
	if !ok {
		t.Fatalf("diff has no %s: %v", field, diff)
	}
	return change["old"], change["new"]
}

func TestExtendingMovesTheDeadlineAndAuditsBothValuesInOneStatement(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)

	extended, err := svc.Extend(context.Background(), teacher(w), session.Attempt.ID, 10, "  Mất mạng 8 phút, đã xác nhận ")
	if err != nil {
		t.Fatalf("extend: %v", err)
	}
	if got := extended.DeadlineAt.Sub(session.Attempt.DeadlineAt); got != 10*time.Minute {
		t.Errorf("deadline moved by %v, want 10m", got)
	}

	audits := auditRows(t, pool, session.Attempt.ID, "attempt.extended")
	if len(audits) != 1 {
		t.Fatalf("%d audit rows, want 1", len(audits))
	}
	if audits[0].actor != w.admin {
		t.Errorf("actor %s, want the teacher", audits[0].actor)
	}
	before, after := oldNew(t, audits[0].diff, "deadline_at")
	if before == nil || after == nil || before == after {
		t.Errorf("diff.deadline_at = %v -> %v, want the old and the new value", before, after)
	}
	if audits[0].diff["reason"] != "Mất mạng 8 phút, đã xác nhận" {
		t.Errorf("reason %v, want it trimmed and kept", audits[0].diff["reason"])
	}
}

// An accommodation is allowed to outlive the window (T-4.2).
func TestExtendingMayPushTheDeadlinePastTheAssignmentsClose(t *testing.T) {
	pool := newPool(t)
	o := openAssignment()
	o.closesAt = time.Now().Add(5 * time.Minute)
	w := seedWorld(t, pool, o)
	svc := newService(t, pool)
	session, err := svc.StartOrResume(context.Background(), w.assignment, w.student)
	if err != nil {
		t.Fatal(err)
	}

	extended, err := svc.Extend(context.Background(), teacher(w), session.Attempt.ID, 30, "Nộp muộn có lý do")
	if err != nil {
		t.Fatalf("extend: %v", err)
	}
	if !extended.DeadlineAt.After(o.closesAt) {
		t.Errorf("deadline %v did not pass closes_at %v", extended.DeadlineAt, o.closesAt)
	}
	auditRows(t, pool, session.Attempt.ID, "attempt.extended")
}

func TestABlankReasonIsRefusedBeforeAnythingChanges(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	for _, reason := range []string{"", "   ", "\n\t"} {
		if _, err := svc.Extend(ctx, teacher(w), session.Attempt.ID, 5, reason); !errors.Is(err, attempts.ErrBlankReason) {
			t.Errorf("extend with %q: %v, want ErrBlankReason", reason, err)
		}
		if _, err := svc.Void(ctx, teacher(w), session.Attempt.ID, reason); !errors.Is(err, attempts.ErrBlankReason) {
			t.Errorf("void with %q: %v, want ErrBlankReason", reason, err)
		}
	}
	if got := count(t, pool, `SELECT count(*) FROM app.audit_log WHERE entity = 'attempt' AND entity_id = $1::uuid`,
		session.Attempt.ID); got != 0 {
		t.Errorf("%d audit rows after refused actions, want 0", got)
	}
	var deadline time.Time
	if err := pool.QueryRow(ctx, `SELECT deadline_at FROM app.attempts WHERE id = $1::uuid`, session.Attempt.ID).
		Scan(&deadline); err != nil {
		t.Fatal(err)
	}
	if !deadline.Equal(session.Attempt.DeadlineAt) {
		t.Error("the deadline moved without a reason")
	}
}

func TestVoidingKeepsTheRecordAndAuditsTheStatusChange(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	answerEverythingRight(t, pool, w, session, svc)
	ctx := context.Background()

	voided, err := svc.Void(ctx, teacher(w), session.Attempt.ID, "Làm nhầm đề của lớp khác")
	if err != nil {
		t.Fatalf("void: %v", err)
	}
	if voided.Status != attempts.Voided {
		t.Errorf("status %s, want voided", voided.Status)
	}
	if got := count(t, pool, `SELECT count(*) FROM app.attempt_answers WHERE attempt_id = $1::uuid`, session.Attempt.ID); got != 4 {
		t.Errorf("%d answers survive the void, want 4: nothing is deleted (§6.4)", got)
	}

	audits := auditRows(t, pool, session.Attempt.ID, "attempt.voided")
	if len(audits) != 1 {
		t.Fatalf("%d audit rows, want 1", len(audits))
	}
	before, after := oldNew(t, audits[0].diff, "status")
	if before != "in_progress" || after != "voided" {
		t.Errorf("diff.status = %v -> %v", before, after)
	}
	_, reason := oldNew(t, audits[0].diff, "void_reason")
	if reason != "Làm nhầm đề của lớp khác" {
		t.Errorf("void_reason.new = %v", reason)
	}

	// The student's tab finds out on its next save.
	_, err = svc.Save(ctx, attempts.SaveInput{
		AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
	})
	if !errors.Is(err, attempts.ErrAttemptClosed) {
		t.Errorf("save after void: %v, want ErrAttemptClosed", err)
	}
	if _, err := svc.Void(ctx, teacher(w), session.Attempt.ID, "lại"); !errors.Is(err, attempts.ErrAttemptVoided) {
		t.Errorf("voiding twice: %v, want ErrAttemptVoided", err)
	}
}

func TestOnlyALiveAttemptCanBeExtended(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()
	if _, err := svc.Submit(ctx, session.Attempt.ID, w.student, attempts.Manual); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Extend(ctx, teacher(w), session.Attempt.ID, 5, "Thử")
	if !errors.Is(err, attempts.ErrAttemptClosed) {
		t.Errorf("extend after submit: %v, want ErrAttemptClosed", err)
	}
	_, err = svc.Extend(ctx, teacher(w), "01935000-0000-7000-8000-00000000dead", 5, "Thử")
	if !errors.Is(err, attempts.ErrNotFound) {
		t.Errorf("extend unknown: %v, want ErrNotFound", err)
	}
}

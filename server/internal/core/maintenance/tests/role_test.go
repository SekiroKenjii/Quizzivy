//go:build integration

package maintenance_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"quizzivy/internal/core/maintenance"
	classesapp "quizzivy/internal/modules/classes/application"
	classescmd "quizzivy/internal/modules/classes/application/command"
	classesquery "quizzivy/internal/modules/classes/application/query"
	classesdomain "quizzivy/internal/modules/classes/domain"
	classesrepo "quizzivy/internal/modules/classes/repositories"
	"quizzivy/internal/platform/db"
)

func TestTheWindowCommandsRunAsTheMaintenanceRole(t *testing.T) {
	iso := newIsolated(t)
	var now time.Time
	if err := iso.owner.QueryRow(iso.ctx, `SELECT now()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	w := world{ctx: iso.ctx, tx: iso.owner, now: now}
	start := now.Add(-30 * time.Second).Truncate(time.Second)
	end := start.Add(90 * time.Minute)
	assignment := w.assignment(t, start.Add(20*time.Minute))
	attempt := w.attempt(t, assignment, start.Add(10*time.Minute))
	student := w.override(t, assignment, start.Add(15*time.Minute))
	wantAttempt, wantAssignment, wantOverride := w.deadline(t, attempt).Add(90*time.Minute), w.closes(t, assignment).Add(90*time.Minute), w.overrideClose(t, assignment, student).Add(90*time.Minute)

	dry, err := maintenance.ScheduleWindow(iso.ctx, iso.role, start, end, false)
	if err != nil || dry.Applied || dry.Attempts != 1 || dry.Assignments != 1 || dry.Overrides != 1 {
		t.Fatalf("window-schedule dry run = %+v, %v", dry, err)
	}
	report, err := maintenance.ScheduleWindow(iso.ctx, iso.role, start, end, true)
	if err != nil || !report.Applied || report.Attempts != 1 || report.Assignments != 1 || report.Overrides != 1 {
		t.Fatalf("window-schedule = %+v, %v", report, err)
	}
	if got := w.deadline(t, attempt); !got.Equal(wantAttempt) {
		t.Errorf("attempt deadline %v, want %v", got, wantAttempt)
	}
	if got := w.closes(t, assignment); !got.Equal(wantAssignment) {
		t.Errorf("assignment close %v, want %v", got, wantAssignment)
	}
	if got := w.overrideClose(t, assignment, student); !got.Equal(wantOverride) {
		t.Errorf("override close %v, want %v", got, wantOverride)
	}
	var audits, byRole int
	if err := iso.owner.QueryRow(iso.ctx, `SELECT count(*), count(*) FILTER (WHERE diff->>'databaseRole' = $1 AND actor_user_id IS NULL)
		FROM app.audit_log WHERE action IN ('attempt.extended', 'assignment.extended', 'assignment.override_extended', 'maintenance.scheduled')`,
		maintenanceRole).Scan(&audits, &byRole); err != nil || audits != 4 || byRole != 4 {
		t.Errorf("%d window audit entries, %d written by the role (%v)", audits, byRole, err)
	}

	listed, err := maintenance.ListWindows(iso.ctx, iso.role)
	if err != nil || len(listed) != 1 || listed[0].ID != report.WindowID || !listed[0].Active {
		t.Fatalf("window-list = %+v, %v", listed, err)
	}
	if ended, err := maintenance.EndWindow(iso.ctx, iso.role, report.WindowID, false); err != nil || ended.Applied {
		t.Errorf("window-end dry run = %+v, %v", ended, err)
	}
	ended, err := maintenance.EndWindow(iso.ctx, iso.role, report.WindowID, true)
	if err != nil || !ended.Applied || !ended.Window.EndsAt.Before(end) {
		t.Fatalf("window-end = %+v, %v", ended, err)
	}

	later, err := maintenance.ScheduleWindow(iso.ctx, iso.role, now.Add(24*time.Hour), now.Add(25*time.Hour), true)
	if err != nil || !later.Applied {
		t.Fatalf("scheduling a later window: %+v, %v", later, err)
	}
	cancelled, err := maintenance.CancelWindow(iso.ctx, iso.role, later.WindowID, true)
	if err != nil || !cancelled.Applied || cancelled.Window.CancelledAt == nil {
		t.Errorf("window-cancel = %+v, %v", cancelled, err)
	}
	if err := iso.owner.QueryRow(iso.ctx, `SELECT count(*) FROM app.audit_log WHERE action IN ('maintenance.ended', 'maintenance.cancelled')
		AND diff->>'databaseRole' = $1`, maintenanceRole).Scan(&audits); err != nil || audits != 2 {
		t.Errorf("%d end and cancel entries written by the role (%v)", audits, err)
	}
}

func TestRekeyJoinCodesRunsAsTheMaintenanceRole(t *testing.T) {
	iso := newIsolated(t)
	owner := db.NewContext(iso.owner)
	var teacher, class string
	if err := iso.owner.QueryRow(iso.ctx, `INSERT INTO app.users (email, full_name, role_id)
		VALUES (gen_random_uuid()::text || '@example.test', 'Giáo viên', (SELECT id FROM app.roles WHERE builtin_key = 'teacher')) RETURNING id::text`).Scan(&teacher); err != nil {
		t.Fatal(err)
	}
	if err := iso.owner.QueryRow(iso.ctx, `INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp xoay khoá', $1) RETURNING id::text`, teacher).Scan(&class); err != nil {
		t.Fatal(err)
	}
	sealed := classesapp.New(classesrepo.NewPostgres(owner), nil, keysFor(t, rekeyOld, nil))
	rotated, err := sealed.Commands.Rotate.Handle(iso.ctx, classescmd.Rotate{Request: classesdomain.RotateRequest{ClassID: class, ActorUserID: teacher}})
	if err != nil {
		t.Fatal(err)
	}
	keys := keysFor(t, rekeyNew, rekeyOld)

	dry, err := maintenance.RekeyJoinCodes(iso.ctx, iso.role, keys, false, 500)
	if err != nil || dry.Applied || dry.Pending != 1 {
		t.Fatalf("rekey-join-codes dry run = %+v, %v", dry, err)
	}
	run, err := maintenance.RekeyJoinCodes(iso.ctx, iso.role, keys, true, 500)
	if err != nil || !run.Applied || run.Moved != 1 || run.Pending != 0 {
		t.Fatalf("rekey-join-codes = %+v, %v", run, err)
	}
	reader := classesapp.New(classesrepo.NewPostgres(owner), nil, keysFor(t, rekeyNew, nil))
	preview, err := reader.Queries.Preview.Handle(iso.ctx, classesquery.Preview{Code: rotated.Code})
	if err != nil || preview.Outcome != classesdomain.PreviewOK || preview.ClassID != class {
		t.Errorf("the code with only the new key: %+v (%v)", preview, err)
	}
}

func TestTheAICommandsRunAsTheMaintenanceRole(t *testing.T) {
	iso := newIsolated(t)
	w := &aiWorld{t: t, ctx: iso.ctx, tx: iso.owner, conn: iso.role, month: aiMonth(t)}
	teacher := w.user("teacher")

	if status := w.run("ai-credits", "status"); status["command"] != "ai-credits status" || status["result"] == nil {
		t.Errorf("status = %v", status)
	}
	w.run("ai-credits", "set-allowance", "-user", teacher, "-credits", "5000", "-operator", "thuong", "-apply")
	w.run("ai-credits", "pause", "-user", teacher, "-operator", "thuong", "-apply")
	w.run("ai-credits", "resume", "-user", teacher, "-operator", "thuong", "-apply")
	w.run("ai-credits", "add-credits", "-credits", "10000", "-operator", "thuong", "-apply")
	w.run("ai-credits", "set-rules", "-warn-percent", "70", "-on-limit", "request", "-operator", "thuong", "-apply")

	var approve, decline string
	if err := iso.owner.QueryRow(iso.ctx, `INSERT INTO app.ai_credit_requests (user_id, month, amount_credits, reason)
		VALUES ($1::uuid, $2::date, 7000, 'Cần thêm') RETURNING id::text`, teacher, w.month).Scan(&approve); err != nil {
		t.Fatal(err)
	}
	if err := iso.owner.QueryRow(iso.ctx, `INSERT INTO app.ai_credit_requests (user_id, month, amount_credits, reason)
		VALUES ($1::uuid, ($2::date + interval '1 month')::date, 7000, 'Cần thêm tháng sau') RETURNING id::text`, teacher, w.month).Scan(&decline); err != nil {
		t.Fatal(err)
	}
	w.run("ai-credits", "approve", "-request", approve, "-operator", "thuong", "-apply")
	w.run("ai-credits", "decline", "-request", decline, "-operator", "thuong", "-apply")

	for _, step := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE app.ai_teacher_credits SET reserved_credits = 40 WHERE month = $1::date AND user_id = $2::uuid`, []any{w.month, teacher}},
		{`UPDATE app.ai_credit_periods SET reserved_credits = reserved_credits + 40 WHERE month = $1::date`, []any{w.month}},
		{`INSERT INTO app.ai_reservations (month, user_id, feature, credits, created_at)
		  SELECT $1::date, $2::uuid, 'score', 40, now() - interval '30 minutes'`, []any{w.month, teacher}},
	} {
		if _, err := iso.owner.Exec(iso.ctx, step.sql, step.args...); err != nil {
			t.Fatal(err)
		}
	}
	if swept := w.run("ai-reservations-sweep", "-apply"); swept["reservations"] != float64(1) || swept["applied"] != true {
		t.Errorf("ai-reservations-sweep = %v", swept)
	}
	if n := w.count(`SELECT count(*) FROM app.notifications WHERE user_id = $1::uuid`, teacher); n < 1 {
		t.Errorf("the teacher was told of no decision (%d notifications)", n)
	}
	if n := w.count(`SELECT count(*) FROM app.audit_log WHERE action LIKE 'ai.%'`); n < 6 {
		t.Errorf("%d AI audit entries", n)
	}
	if n := w.count(`SELECT count(*) FROM app.ai_usage_ledger`); n != 0 {
		t.Errorf("%d ledger rows", n)
	}
}

func TestRetainIntegrityRunsAsTheMaintenanceRole(t *testing.T) {
	w := newRoleWorld(t)
	w.retention(nil, 6, nil)
	cutoff := w.cutoff(6)
	attempt := w.attempt(w.closedAt(cutoff.Add(-w.margin)), w.user("Học viên", "student"), "submitted", cutoff.Add(-w.margin))
	w.event(attempt, cutoff.Add(-w.margin))
	w.event(attempt, cutoff.Add(w.margin))

	command := parse(t, "retain-integrity", "-apply", "-batch", "10")
	report, err := command.Run(w.ctx, w.conn)
	if err != nil {
		t.Fatal(err)
	}
	if got := report.(maintenance.RetentionReport); got.Rows != 1 || !got.Applied {
		t.Errorf("retain-integrity = %+v", got)
	}
	if n := w.count(`SELECT count(*) FROM app.attempt_events`); n != 1 {
		t.Errorf("%d events remain, want 1", n)
	}
	if n := w.count(`SELECT count(*) FROM app.audit_log WHERE action = 'integrity.retained' AND diff->>'databaseRole' = $1`, maintenanceRole); n != 1 {
		t.Errorf("%d retention entries written by the role", n)
	}
}

func TestTheMaintenanceRoleCannotSetACredentialOrReopenAnAccount(t *testing.T) {
	w := newRoleWorld(t)
	staff := w.user("Quản trị đã khoá", "admin")
	w.disable(staff, time.Now().Add(-24*time.Hour))
	active := w.user("Giáo viên đang dùng", "teacher")
	const known = `'$argon2id$v=19$m=65536,t=3,p=4$known'`
	anonymous := func(id string) string { return `'anonymous-' || '` + id + `' || '@anonymous.invalid'` }
	row := func(id string) string {
		var s string
		if err := w.q.QueryRow(w.ctx, `SELECT to_jsonb(u)::text FROM app.users u WHERE u.id = $1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	refuse := func(name, id, set string) {
		t.Helper()
		before := row(id)
		_, err := w.conn.Exec(w.ctx, `UPDATE app.users SET `+set+` WHERE id = $1`, id)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != "42501" || !strings.Contains(pg.Message, "anonymised") {
			t.Errorf("%s: err = %v, want the guard's 42501", name, err)
		}
		if after := row(id); after != before {
			t.Errorf("%s changed the account:\n%s\n%s", name, before, after)
		}
	}

	refuse("a new email", staff, `email = 'taken-over@example.test'`)
	refuse("a known password hash", staff, `password_hash = `+known)
	refuse("enabling the account again", staff, `disabled_at = NULL`)
	refuse("a known password hash on an active account", active, `password_hash = `+known)
	refuse("the three together", staff, `email = 'taken-over@example.test', password_hash = `+known+`, disabled_at = NULL`)
	refuse("the reserved address with the hash left", staff, `email = `+anonymous(staff))
	refuse("everything but the account still enabled", active, `email = `+anonymous(active)+`, display_name = NULL, phone = NULL,
		avatar_key = NULL, password_hash = NULL, must_change_password = false, disabled_at = NULL`)
	refuse("a name alone", active, `full_name = 'Người lạ'`)
	other := w.user("Người khác", "teacher")
	withEmail := func(email string) string {
		return `email = ` + email + `, password_hash = NULL, must_change_password = false, disabled_at = now(), phone = NULL, avatar_key = NULL, display_name = NULL`
	}
	refuse("another account's reserved address", active, withEmail(anonymous(other)))
	refuse("the reserved address under another domain", active, withEmail(`'anonymous-' || id::text || '@anonymous.invalid.example'`))
	refuse("the reserved address with text after the id", active, withEmail(`'anonymous-' || id::text || 'x@anonymous.invalid'`))
	refuse("the id without the prefix", active, withEmail(`id::text || '@anonymous.invalid'`))
	refuse("the reserved address in capitals", active, withEmail(`'ANONYMOUS-' || id::text || '@anonymous.invalid'`))
	pinned := map[string]string{
		"the reserved address": `email = ` + anonymous(active),
		"the password hash":    `password_hash = NULL`,
		"must_change_password": `must_change_password = false`,
		"the disabled mark":    `disabled_at = now()`,
		"the phone":            `phone = NULL`,
		"the photo reference":  `avatar_key = NULL`,
		"the display name":     `display_name = NULL`,
	}
	for left := range pinned {
		var set []string
		for name, clause := range pinned {
			if name != left {
				set = append(set, clause)
			}
		}
		slices.Sort(set)
		refuse("anonymising everything but "+left, active, strings.Join(set, ", "))
	}

	if _, err := maintenance.AnonymizeUser(w.ctx, w.conn, staff, true); err != nil {
		t.Fatalf("anonymising a disabled account: %v", err)
	}
	if _, err := maintenance.AnonymizeUser(w.ctx, w.conn, active, true); err != nil {
		t.Fatalf("anonymising an active account on request: %v", err)
	}
	for _, id := range []string{staff, active} {
		if n := w.count(`SELECT count(*) FROM app.users WHERE id = $1 AND email = 'anonymous-' || id::text || '@anonymous.invalid'
			AND password_hash IS NULL AND disabled_at IS NOT NULL AND phone IS NULL AND avatar_key IS NULL AND display_name IS NULL`, id); n != 1 {
			t.Errorf("account %s is not anonymised", id)
		}
		refuse("a new email after anonymising", id, `email = 'taken-over@example.test'`)
		refuse("a known password hash after anonymising", id, `password_hash = `+known)
		refuse("enabling the account again after anonymising", id, `disabled_at = NULL`)
	}

	w.exec(`UPDATE app.users SET email = 'owner-may@example.test', password_hash = 'hash', disabled_at = NULL WHERE id = $1`, staff)
	if n := w.count(`SELECT count(*) FROM app.users WHERE id = $1 AND email = 'owner-may@example.test'`, staff); n != 1 {
		t.Error("the guard also stopped the owner role from updating an account")
	}
}

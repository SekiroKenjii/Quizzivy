//go:build integration

package maintenance_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"quizzivy/internal/core/maintenance"
	"quizzivy/internal/platform/db"
)

const maintenanceRole = "quizzivy_maintenance"

const (
	studentPlaceholder = "Học viên đã ẩn danh"
	userPlaceholder    = "Người dùng đã ẩn danh"
	removedPayload     = `{"removed": true}`
	answerPayload      = `{"text": "câu trả lời"}`
)

type sweepWorld struct {
	t         *testing.T
	ctx       context.Context
	q         db.Conn
	conn      db.Conn
	exact     bool
	margin    time.Duration
	role      string
	teacher   string
	version   string
	questions [3]string
}

func newSweepWorld(t *testing.T) *sweepWorld {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, sharedScratch(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	w := &sweepWorld{t: t, ctx: ctx, q: tx, conn: tx, exact: true, margin: time.Second}
	w.seed()
	return w
}

func newRoleWorld(t *testing.T) *sweepWorld {
	t.Helper()
	iso := newIsolated(t)
	w := &sweepWorld{t: t, ctx: iso.ctx, q: iso.owner, conn: iso.role, margin: time.Minute, role: maintenanceRole}
	w.seed()
	return w
}

func eachWorld(t *testing.T, test func(t *testing.T, w *sweepWorld)) {
	t.Helper()
	t.Run("owner in a rolled-back transaction", func(t *testing.T) { test(t, newSweepWorld(t)) })
	t.Run("maintenance role", func(t *testing.T) { test(t, newRoleWorld(t)) })
}

func (w *sweepWorld) seed() {
	w.teacher = w.user("Giáo viên Lan", "admin")
	test := w.id(`INSERT INTO app.tests(title, status, current_version, created_by, owner_id)
		VALUES ('Bài kiểm tra', 'published', 1, $1, $1) RETURNING id::text`, w.teacher)
	w.version = w.id(`INSERT INTO app.test_versions(test_id, version, total_points, published_by)
		VALUES ($1, 1, 3, $2) RETURNING id::text`, test, w.teacher)
	section := w.id(`INSERT INTO app.test_version_sections(test_version_id, ordinal, title)
		VALUES ($1, 0, 'Phần 1') RETURNING id::text`, w.version)
	for i := range w.questions {
		w.questions[i] = w.id(`INSERT INTO app.test_version_questions(test_version_section_id, ordinal, type, prompt, points)
			VALUES ($1, $2, 'short_answer', 'Câu hỏi', 1) RETURNING id::text`, section, i)
	}
}

func (w *sweepWorld) sameInstant(got, want time.Time) bool {
	if w.exact {
		return got.Equal(want)
	}
	return got.Sub(want).Abs() < 10*time.Second
}

func (w *sweepWorld) exec(sql string, args ...any) {
	w.t.Helper()
	if _, err := w.q.Exec(w.ctx, sql, args...); err != nil {
		w.t.Fatalf("%v\n%s", err, sql)
	}
}

func (w *sweepWorld) id(sql string, args ...any) string {
	w.t.Helper()
	var id string
	if err := w.q.QueryRow(w.ctx, sql, args...).Scan(&id); err != nil {
		w.t.Fatalf("%v\n%s", err, sql)
	}
	return id
}

func (w *sweepWorld) count(sql string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.q.QueryRow(w.ctx, sql, args...).Scan(&n); err != nil {
		w.t.Fatalf("%v\n%s", err, sql)
	}
	return n
}

func (w *sweepWorld) cutoff(months int) time.Time {
	w.t.Helper()
	var at time.Time
	err := w.q.QueryRow(w.ctx,
		`SELECT (CURRENT_TIMESTAMP AT TIME ZONE 'UTC' - make_interval(months => $1)) AT TIME ZONE 'UTC'`, months).Scan(&at)
	if err != nil {
		w.t.Fatal(err)
	}
	return at
}

func (w *sweepWorld) retention(answers *int, events int, accounts *int) {
	w.t.Helper()
	w.exec(`UPDATE app.organization_settings
		   SET keep_answers_months = $1, keep_focus_events_months = $2, anonymise_disabled_after_months = $3`, answers, events, accounts)
}

func months(n int) *int { return &n }

func (w *sweepWorld) user(name, role string) string {
	return w.id(`INSERT INTO app.users(email, full_name, display_name, phone, avatar_key, role_id, password_hash, must_change_password)
		VALUES (gen_random_uuid()::text || '@example.test', $1, $1, '0901234567', 'avatars/mau.png',
		        (SELECT id FROM app.roles WHERE builtin_key = $2), 'hash', true) RETURNING id::text`, name, role)
}

func (w *sweepWorld) disable(user string, at time.Time) {
	w.t.Helper()
	w.exec(`UPDATE app.users SET disabled_at = $2 WHERE id = $1`, user, at)
}

func (w *sweepWorld) assignment(opens, closes time.Time, closed *time.Time) string {
	w.t.Helper()
	return w.id(`INSERT INTO app.assignments(test_id, test_version_id, opens_at, closes_at, closed_at, duration_minutes, created_by, published_at)
		SELECT test_id, id, $2, $3, $4, 45, $5, $2 FROM app.test_versions WHERE id = $1
		RETURNING id::text`, w.version, opens, closes, closed, w.teacher)
}

func (w *sweepWorld) closedAt(at time.Time) string {
	return w.assignment(at.Add(-48*time.Hour), at, nil)
}

func (w *sweepWorld) attempt(assignment, student, status string, at time.Time) string {
	w.t.Helper()
	return w.id(`INSERT INTO app.attempts(assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed,
			beacon_token_hash, started_at, deadline_at, submitted_at, graded_at, void_reason)
		SELECT $1::uuid, a.test_version_id, $2::uuid, 1, $3::text::app.attempt_status, uuidv7(), 1,
		       sha256('beacon'::bytea), $4::timestamptz - interval '2 hours',
		       CASE WHEN $3::text = 'timed_out' THEN $4::timestamptz ELSE $4::timestamptz + interval '1 hour' END,
		       CASE WHEN $3::text IN ('submitted', 'graded', 'voided') THEN $4::timestamptz END,
		       CASE WHEN $3::text = 'graded' THEN $4::timestamptz END,
		       CASE WHEN $3::text = 'voided' THEN 'đề lỗi' END
		  FROM app.assignments a WHERE a.id = $1::uuid
		RETURNING id::text`, assignment, student, status, at)
}

func (w *sweepWorld) answers(attempt string) {
	w.t.Helper()
	for _, q := range w.questions[:2] {
		w.exec(`INSERT INTO app.attempt_answers(attempt_id, question_id, payload, requires_manual, auto_score, manual_score,
				graded_by, graded_at, grader_comment)
			VALUES ($1, $2, $3::text::jsonb, true, 1, 2, $4, now(), 'Làm tốt')`, attempt, q, answerPayload, w.teacher)
	}
}

func (w *sweepWorld) payloads(attempt string) []string {
	w.t.Helper()
	rows, err := w.q.Query(w.ctx, `SELECT payload::text FROM app.attempt_answers WHERE attempt_id = $1 ORDER BY question_id`, attempt)
	if err != nil {
		w.t.Fatal(err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		w.t.Fatal(err)
	}
	return out
}

func (w *sweepWorld) removed(attempt string) bool {
	w.t.Helper()
	payloads := w.payloads(attempt)
	if len(payloads) == 0 {
		w.t.Fatalf("attempt %s has no answers", attempt)
	}
	for _, p := range payloads {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(p), &parsed); err != nil {
			w.t.Fatal(err)
		}
		switch {
		case len(parsed) == 1 && parsed["removed"] == true:
		case parsed["text"] == "câu trả lời":
			return false
		default:
			w.t.Fatalf("attempt %s holds %s", attempt, p)
		}
	}
	return true
}

func (w *sweepWorld) event(attempt string, received time.Time) {
	w.t.Helper()
	w.exec(`INSERT INTO app.attempt_events(attempt_id, session_id, kind, occurred_at, received_at)
		VALUES ($1, uuidv7(), 'window_blur', $2, $2)`, attempt, received)
}

func (w *sweepWorld) audits(action string) int {
	w.t.Helper()
	return w.count(`SELECT count(*) FROM app.audit_log WHERE action = $1`, action)
}

func (w *sweepWorld) sweep(opts maintenance.SweepOptions) maintenance.SweepReport {
	w.t.Helper()
	if opts.Batch == 0 {
		opts.Batch = 100
	}
	if opts.Budget == 0 {
		opts.Budget = time.Minute
	}
	report, err := maintenance.RetentionSweep(w.ctx, w.conn, opts)
	if err != nil {
		w.t.Fatalf("sweep: %v (%+v)", err, report)
	}
	if w.role != "" && report.DatabaseRole != w.role {
		w.t.Fatalf("the sweep ran as %q, want %q", report.DatabaseRole, w.role)
	}
	return report
}

func (w *sweepWorld) noActorOnAnyAudit() {
	w.t.Helper()
	if n := w.count(`SELECT count(*) FROM app.audit_log WHERE actor_user_id IS NOT NULL AND action IN
		('retention.answers_removed', 'integrity.retained', 'user.anonymized')`); n != 0 {
		w.t.Fatalf("%d sweep audit rows name an actor", n)
	}
}

type answerCase struct {
	name     string
	attempt  string
	wantGone bool
}

func TestTheAnswersStepRemovesAnswersOncePastTheCutoffOnBothClocks(t *testing.T) {
	eachWorld(t, func(t *testing.T, w *sweepWorld) {
		w.retention(months(12), 13, nil)
		cutoff := w.cutoff(12)
		before, after := cutoff.Add(-w.margin), cutoff.Add(w.margin)
		old := w.closedAt(before)

		var cases []answerCase
		add := func(name, assignment, status string, at time.Time, wantGone bool) {
			attempt := w.attempt(assignment, w.user("Học viên", "student"), status, at)
			w.answers(attempt)
			cases = append(cases, answerCase{name, attempt, wantGone})
		}
		addUnsubmitted := func(name, assignment string, deadline time.Time, wantGone bool) {
			attempt := w.attempt(assignment, w.user("Học viên", "student"), "voided", deadline.Add(-time.Hour))
			w.exec(`UPDATE app.attempts SET submitted_at = NULL WHERE id = $1`, attempt)
			w.answers(attempt)
			cases = append(cases, answerCase{name, attempt, wantGone})
		}
		add("submitted just before the cutoff", old, "submitted", before, true)
		add("graded just before the cutoff", old, "graded", before, true)
		add("voided after it was submitted, just before the cutoff", old, "voided", before, true)
		add("timed out without a submission, by its deadline", old, "timed_out", before, true)
		addUnsubmitted("voided before it was submitted, by its deadline", old, before, true)
		add("submitted just after the cutoff", old, "submitted", after, false)
		add("graded just after the cutoff", old, "graded", after, false)
		add("voided after it was submitted, just after the cutoff", old, "voided", after, false)
		add("timed out, deadline just after the cutoff", old, "timed_out", after, false)
		addUnsubmitted("voided before it was submitted, deadline just after the cutoff", old, after, false)
		add("in progress, however old and overdue", old, "in_progress", before.Add(-time.Hour), false)
		add("in progress, deadline just after the cutoff", old, "in_progress", after.Add(-time.Hour), false)
		add("in progress, its assignment closing just after the cutoff", w.closedAt(after), "in_progress", before.Add(-time.Hour), false)
		add("old attempt, assignment closed just after the cutoff", w.closedAt(after), "submitted", before.Add(-time.Hour), false)
		add("old attempt, assignment still to close", w.assignment(before.Add(-48*time.Hour), cutoff.AddDate(0, 6, 0), nil), "submitted", before.Add(-time.Hour), false)
		closedBefore := before
		add("old attempt, assignment closed by hand before the cutoff", w.assignment(before.Add(-48*time.Hour), cutoff.AddDate(0, 6, 0), &closedBefore), "submitted", before.Add(-time.Hour), true)
		closedAfter := after
		add("old attempt, assignment closed by hand after the scheduled close, which was before the cutoff", w.assignment(before.Add(-48*time.Hour), before.Add(-time.Hour), &closedAfter), "submitted", before.Add(-time.Hour), true)
		if w.exact {
			add("submitted exactly at the cutoff", old, "submitted", cutoff, false)
			add("old attempt, assignment closing exactly at the cutoff", w.closedAt(cutoff), "submitted", before.Add(-time.Hour), false)
		}

		var due int
		for _, c := range cases {
			if c.wantGone {
				due++
			}
		}
		scoresBefore := w.count(`SELECT count(*) FROM app.attempt_answers WHERE auto_score = 1 AND manual_score = 2 AND final_score = 2`)

		report := w.sweep(maintenance.SweepOptions{Apply: true})
		for _, c := range cases {
			if got := w.removed(c.attempt); got != c.wantGone {
				t.Errorf("%s: removed = %v, want %v", c.name, got, c.wantGone)
			}
		}
		if !report.Answers.Enabled || report.Answers.Cutoff == nil || !w.sameInstant(*report.Answers.Cutoff, cutoff) {
			t.Errorf("answers step = %+v, want enabled with cutoff %s", report.Answers, cutoff)
		}
		if report.Answers.OverdueInProgress != 1 {
			t.Errorf("overdueInProgress = %d, want the one attempt in progress past its deadline on a closed assignment", report.Answers.OverdueInProgress)
		}
		if report.Answers.Attempts != int64(due) || report.Answers.Answers != int64(2*due) {
			t.Errorf("answers step counted %d attempts and %d answers, want %d and %d", report.Answers.Attempts, report.Answers.Answers, due, 2*due)
		}
		if got := w.count(`SELECT count(*) FROM app.attempt_answers WHERE auto_score = 1 AND manual_score = 2 AND final_score = 2
			AND grader_comment = 'Làm tốt' AND graded_by IS NOT NULL AND requires_manual`); got != scoresBefore {
			t.Errorf("scores, comments and the grader survive on %d of %d answers", got, scoresBefore)
		}
		if !report.Complete || !report.Applied || report.DatabaseRole == "" {
			t.Errorf("report = %+v", report)
		}

		if got := w.audits("retention.answers_removed"); got != 1 {
			t.Fatalf("%d answers_removed entries for one batch, want 1", got)
		}
		var diff map[string]any
		var entity string
		if err := w.q.QueryRow(w.ctx, `SELECT entity, diff FROM app.audit_log WHERE action = 'retention.answers_removed'`).Scan(&entity, &diff); err != nil {
			t.Fatal(err)
		}
		if entity != "attempt_answers" || diff["attempts"] != float64(due) || diff["answers"] != float64(2*due) || diff["databaseRole"] == nil || diff["cutoff"] == nil {
			t.Errorf("audit entry = %s %v", entity, diff)
		}
		w.noActorOnAnyAudit()

		again := w.sweep(maintenance.SweepOptions{Apply: true})
		if again.Answers.Attempts != 0 || again.Answers.Answers != 0 || w.audits("retention.answers_removed") != 1 {
			t.Errorf("a second run did work: %+v, %d entries", again.Answers, w.audits("retention.answers_removed"))
		}
	})
}

func TestEveryAttemptStatusIsDecidedByTheAnswersRule(t *testing.T) {
	w := newSweepWorld(t)
	rows, err := w.q.Query(w.ctx, `SELECT unnest(enum_range(NULL::app.attempt_status))::text ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	statuses, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"graded", "in_progress", "submitted", "timed_out", "voided"}
	if !slices.Equal(statuses, want) {
		t.Errorf("attempt statuses are %v, the retention rule (shared/retention) was written for %v: decide whether an answer of the new one is removed", statuses, want)
	}
}

func TestDefaultRetentionKeepsEveryAnswerAndEveryAccount(t *testing.T) {
	eachWorld(t, func(t *testing.T, w *sweepWorld) {
		w.retention(nil, 13, nil)
		old := w.cutoff(240)
		attempt := w.attempt(w.closedAt(old), w.user("Học viên", "student"), "submitted", old)
		w.answers(attempt)
		gone := w.user("Học viên cũ", "student")
		w.disable(gone, old)

		report := w.sweep(maintenance.SweepOptions{Apply: true})
		if report.Answers.Enabled || report.Accounts.Enabled || report.Answers.Cutoff != nil || report.Accounts.Cutoff != nil {
			t.Errorf("steps = %+v, %+v", report.Answers, report.Accounts)
		}
		if w.removed(attempt) {
			t.Error("an answer was removed with keep_answers_months unset")
		}
		if got := w.count(`SELECT count(*) FROM app.users WHERE id = $1 AND full_name = 'Học viên cũ'`, gone); got != 1 {
			t.Error("an account was anonymised with anonymise_disabled_after_months unset")
		}
		if n := w.audits("retention.answers_removed") + w.audits("user.anonymized"); n != 0 {
			t.Errorf("%d audit entries for a sweep that did nothing", n)
		}
	})
}

func TestADryRunCountsWhatARunWouldDoAndWritesNothing(t *testing.T) {
	eachWorld(t, func(t *testing.T, w *sweepWorld) {
		w.retention(months(12), 6, months(12))
		cutoff := w.cutoff(12)
		old := w.closedAt(cutoff.Add(-24 * time.Hour))
		attempt := w.attempt(old, w.user("Học viên", "student"), "submitted", cutoff.Add(-24*time.Hour))
		w.answers(attempt)
		w.event(attempt, w.cutoff(6).Add(-time.Hour))
		gone := w.user("Học viên đã khoá", "student")
		w.disable(gone, cutoff.Add(-time.Hour))
		audits := w.count(`SELECT count(*) FROM app.audit_log`)

		dry := w.sweep(maintenance.SweepOptions{})
		if dry.Applied {
			t.Error("a dry run reports itself applied")
		}
		if dry.Answers.Attempts != 1 || dry.Answers.Answers != 2 || dry.FocusEvents.Events != 1 || dry.Accounts.Anonymise != 1 {
			t.Errorf("dry run counted %+v %+v %+v", dry.Answers, dry.FocusEvents, dry.Accounts)
		}
		if w.removed(attempt) || w.count(`SELECT count(*) FROM app.attempt_events WHERE attempt_id = $1`, attempt) != 1 ||
			w.count(`SELECT count(*) FROM app.users WHERE id = $1 AND full_name = 'Học viên đã khoá'`, gone) != 1 {
			t.Error("a dry run changed data")
		}
		if got := w.count(`SELECT count(*) FROM app.audit_log`); got != audits {
			t.Errorf("a dry run wrote %d audit entries", got-audits)
		}

		run := w.sweep(maintenance.SweepOptions{Apply: true})
		if run.Answers.Attempts != dry.Answers.Attempts || run.Answers.Answers != dry.Answers.Answers ||
			run.FocusEvents.Events != dry.FocusEvents.Events || run.Accounts.Anonymise != dry.Accounts.Anonymise {
			t.Errorf("the run did %+v %+v %+v, the dry run counted %+v %+v %+v",
				run.Answers, run.FocusEvents, run.Accounts, dry.Answers, dry.FocusEvents, dry.Accounts)
		}
	})
}

func TestTheAnswersStepWorksInBatchesAndKeepsAnAuditEntryForEach(t *testing.T) {
	w := newSweepWorld(t)
	w.retention(months(12), 13, nil)
	at := w.cutoff(12).Add(-time.Hour)
	assignment := w.closedAt(at)
	var attempts []string
	for range 5 {
		attempt := w.attempt(assignment, w.user("Học viên", "student"), "submitted", at)
		w.answers(attempt)
		attempts = append(attempts, attempt)
	}
	report := w.sweep(maintenance.SweepOptions{Apply: true, Batch: 2})
	if report.Answers.Batches != 3 || report.Answers.Attempts != 5 || report.Answers.Answers != 10 {
		t.Errorf("answers step = %+v, want 3 batches of 5 attempts and 10 answers", report.Answers)
	}
	for _, attempt := range attempts {
		if !w.removed(attempt) {
			t.Errorf("attempt %s kept its answers", attempt)
		}
	}
	if got := w.audits("retention.answers_removed"); got != 3 {
		t.Errorf("%d entries for 3 batches", got)
	}
}

func TestARunWhoseBudgetHasEndedDoesNothingAndTheNextRunFinishes(t *testing.T) {
	w := newSweepWorld(t)
	w.retention(months(12), 13, nil)
	at := w.cutoff(12).Add(-time.Hour)
	attempt := w.attempt(w.closedAt(at), w.user("Học viên", "student"), "submitted", at)
	w.answers(attempt)

	spent, err := maintenance.RetentionSweep(w.ctx, w.conn, maintenance.SweepOptions{Apply: true, Batch: 10, Budget: time.Nanosecond})
	if err != nil {
		t.Fatalf("an ended budget is not a failure: %v", err)
	}
	if spent.Complete || len(spent.Failed) != 0 || w.removed(attempt) {
		t.Errorf("report = %+v, removed = %v", spent, w.removed(attempt))
	}
	if next := w.sweep(maintenance.SweepOptions{Apply: true}); !next.Complete || !w.removed(attempt) {
		t.Errorf("the next run did not finish: %+v", next)
	}
}

func TestTheEventsStepUsesTheConfiguredMonthsOnReceiptAndClose(t *testing.T) {
	eachWorld(t, func(t *testing.T, w *sweepWorld) {
		w.retention(nil, 6, nil)
		cutoff := w.cutoff(6)
		before, after := cutoff.Add(-w.margin), cutoff.Add(w.margin)
		student := w.user("Học viên", "student")
		closed := w.attempt(w.closedAt(before), student, "submitted", before)
		open := w.attempt(w.assignment(before.Add(-48*time.Hour), cutoff.AddDate(0, 3, 0), nil), w.user("Học viên khác", "student"), "submitted", before)
		closedLate := w.attempt(w.closedAt(after), w.user("Học viên nữa", "student"), "submitted", before)

		w.event(closed, before)
		remain := 3
		if w.exact {
			w.event(closed, cutoff)
			remain++
		}
		w.event(closed, after)
		w.event(open, before)
		w.event(closedLate, before)

		report := w.sweep(maintenance.SweepOptions{Apply: true})
		if report.FocusEvents.Events != 1 || !w.sameInstant(report.FocusEvents.Cutoff, cutoff) {
			t.Errorf("events step = %+v, want one event at cutoff %s", report.FocusEvents, cutoff)
		}
		if got := w.count(`SELECT count(*) FROM app.attempt_events`); got != remain {
			t.Errorf("%d events remain, want %d", got, remain)
		}
		if got := w.count(`SELECT count(*) FROM app.attempts`); got != 3 {
			t.Errorf("%d attempts remain, want 3", got)
		}
		if w.audits("integrity.retained") != 1 {
			t.Errorf("%d integrity.retained entries", w.audits("integrity.retained"))
		}
		w.noActorOnAnyAudit()
	})
}

type accountFixture struct {
	pupil, staff, active, recent, owner, sitting, boundary string
	attempt                                                string
}

func (w *sweepWorld) accounts(cutoff time.Time) accountFixture {
	w.t.Helper()
	before := cutoff.Add(-w.margin)
	var f accountFixture
	f.pupil = w.user("Nguyễn Văn Nam", "student")
	f.staff = w.user("Trần Thị Hoa", "teacher")
	f.active = w.user("Lê Văn Bình", "student")
	f.recent = w.user("Phạm Thị Mai", "student")
	f.owner = w.user("Đỗ Văn Sơn", "teacher")
	f.sitting = w.user("Vũ Thị Lan", "student")
	for _, u := range []string{f.pupil, f.staff, f.owner, f.sitting} {
		w.disable(u, before)
	}
	w.disable(f.recent, cutoff.Add(w.margin))
	if w.exact {
		f.boundary = w.user("Ngô Văn Tâm", "student")
		w.disable(f.boundary, cutoff)
	}

	assignment := w.closedAt(cutoff.AddDate(0, -1, 0))
	f.attempt = w.attempt(assignment, f.pupil, "graded", cutoff.AddDate(0, -1, 0))
	w.answers(f.attempt)
	w.attempt(assignment, f.sitting, "in_progress", before)
	w.id(`INSERT INTO app.classes(name, teacher_id) VALUES ('Lớp của thầy Sơn', $1) RETURNING id::text`, f.owner)

	for _, u := range []string{f.pupil, f.staff, f.active} {
		w.exec(`INSERT INTO app.user_identities(user_id, provider, provider_user_id, email_at_link)
			VALUES ($1::uuid, 'google', $1::uuid::text, $1::uuid::text || '@example.test')`, u)
		w.exec(`INSERT INTO app.refresh_tokens(user_id, family_id, token_hash, expires_at)
			VALUES ($1::uuid, gen_random_uuid(), sha256($1::uuid::text::bytea), now() + interval '1 day')`, u)
		w.exec(`INSERT INTO app.set_password_tokens(user_id, token_hash, purpose, expires_at)
			VALUES ($1::uuid, sha256('unused'::bytea || $1::uuid::text::bytea), 'reset', now() + interval '1 day')`, u)
		w.exec(`INSERT INTO app.set_password_tokens(user_id, token_hash, purpose, expires_at, used_at)
			VALUES ($1::uuid, sha256('used'::bytea || $1::uuid::text::bytea), 'reset', now() + interval '1 day', now())`, u)
	}
	return f
}

func (w *sweepWorld) notification(params string) string {
	w.t.Helper()
	return w.id(`INSERT INTO app.notifications(user_id, kind, params, dedupe_key)
		VALUES ($1, 'attempt.submitted', $2::text::jsonb, gen_random_uuid()::text) RETURNING id::text`, w.teacher, params)
}

func TestTheAccountsStepAnonymisesEveryRoleOnceDisabledPastTheCutoff(t *testing.T) {
	eachWorld(t, func(t *testing.T, w *sweepWorld) {
		w.retention(nil, 13, months(12))
		cutoff := w.cutoff(12)
		f := w.accounts(cutoff)
		byName := w.notification(`{"studentName": "Nguyễn Văn Nam"}`)
		byTeacher := w.notification(`{"teacherName": "Trần Thị Hoa", "count": 2}`)
		nested := w.notification(`{"classNames": ["Lớp 1", "Nguyễn Văn Nam"]}`)
		other := w.notification(`{"studentName": "Lê Văn Bình"}`)
		plain := w.notification(`{"count": 3, "title": "Bài kiểm tra"}`)

		report := w.sweep(maintenance.SweepOptions{Apply: true})
		step := report.Accounts
		if !step.Enabled || step.Cutoff == nil || !w.sameInstant(*step.Cutoff, cutoff) {
			t.Errorf("accounts step = %+v", step)
		}
		if step.Anonymise != 2 || step.SkippedOwners != 1 || step.SkippedInProgress != 1 || step.Notices != 3 {
			t.Errorf("accounts step = %+v, want 2 anonymised, 1 owner and 1 in progress skipped, 3 notices", step)
		}

		for who, want := range map[string]string{f.pupil: studentPlaceholder, f.staff: userPlaceholder} {
			var name, email string
			var display, phone, avatar, password *string
			var mustChange bool
			var disabled *time.Time
			err := w.q.QueryRow(w.ctx, `SELECT full_name, email, display_name, phone, avatar_key, password_hash, must_change_password, disabled_at
				FROM app.users WHERE id = $1`, who).Scan(&name, &email, &display, &phone, &avatar, &password, &mustChange, &disabled)
			if err != nil {
				t.Fatal(err)
			}
			if name != want || email != "anonymous-"+who+"@anonymous.invalid" || display != nil || phone != nil || avatar != nil ||
				password != nil || mustChange || disabled == nil || !disabled.Before(cutoff) {
				t.Errorf("%s holds %q %q %v %v %v %v %v %v", who, name, email, display, phone, avatar, password, mustChange, disabled)
			}
			if w.count(`SELECT count(*) FROM app.user_identities WHERE user_id = $1`, who) != 0 ||
				w.count(`SELECT count(*) FROM app.refresh_tokens WHERE user_id = $1`, who) != 0 {
				t.Errorf("%s keeps a sign-in", who)
			}
			if w.count(`SELECT count(*) FROM app.set_password_tokens WHERE user_id = $1 AND used_at IS NULL AND revoked_at IS NULL`, who) != 0 ||
				w.count(`SELECT count(*) FROM app.set_password_tokens WHERE user_id = $1`, who) != 2 {
				t.Errorf("%s: unused links are not revoked, or a link was deleted", who)
			}
			var diff map[string]any
			if err := w.q.QueryRow(w.ctx, `SELECT diff FROM app.audit_log WHERE action = 'user.anonymized' AND entity = 'users' AND entity_id = $1`, who).Scan(&diff); err != nil {
				t.Fatal(err)
			}
			if diff["reason"] != "retention" || diff["cutoff"] == nil || diff["databaseRole"] == nil || diff["structuredIdentityRemoved"] != true {
				t.Errorf("%s audit diff = %v", who, diff)
			}
		}
		if w.audits("user.anonymized") != 2 || w.audits("student.anonymized") != 0 {
			t.Errorf("%d user.anonymized entries, %d student.anonymized", w.audits("user.anonymized"), w.audits("student.anonymized"))
		}
		w.noActorOnAnyAudit()

		for name, user := range map[string]string{"active": f.active, "disabled after the cutoff": f.recent, "disabled at the cutoff": f.boundary, "owner": f.owner, "sitting": f.sitting} {
			if user == "" {
				continue
			}
			if w.count(`SELECT count(*) FROM app.users WHERE id = $1 AND email NOT LIKE 'anonymous-%'`, user) != 1 {
				t.Errorf("the %s account was anonymised", name)
			}
		}
		if w.count(`SELECT count(*) FROM app.user_identities WHERE user_id = $1`, f.active) != 1 {
			t.Error("an active account lost its Google link")
		}
		if w.count(`SELECT count(*) FROM app.attempts WHERE student_id = $1 AND status = 'graded'`, f.pupil) != 1 || w.removed(f.attempt) {
			t.Error("an anonymised student's attempt or answers changed")
		}
		for id, want := range map[string]int{byName: 0, byTeacher: 0, nested: 0, other: 1, plain: 1} {
			if got := w.count(`SELECT count(*) FROM app.notifications WHERE id = $1`, id); got != want {
				t.Errorf("notification %s: %d rows, want %d", id, got, want)
			}
		}

		again := w.sweep(maintenance.SweepOptions{Apply: true})
		if again.Accounts.Anonymise != 0 || again.Accounts.SkippedOwners != 1 || again.Accounts.SkippedInProgress != 1 || w.audits("user.anonymized") != 2 {
			t.Errorf("a second run: %+v, %d entries", again.Accounts, w.audits("user.anonymized"))
		}
	})
}

func TestAnAccountSittingATestIsAnonymisedOnceItsAttemptEnds(t *testing.T) {
	eachWorld(t, func(t *testing.T, w *sweepWorld) {
		w.retention(nil, 13, months(12))
		cutoff := w.cutoff(12)
		sitting := w.user("Vũ Thị Lan", "student")
		w.disable(sitting, cutoff.Add(-time.Hour))
		attempt := w.attempt(w.closedAt(cutoff.Add(-time.Hour)), sitting, "in_progress", cutoff.Add(-time.Hour))

		if report := w.sweep(maintenance.SweepOptions{Apply: true}); report.Accounts.SkippedInProgress != 1 || report.Accounts.Anonymise != 0 {
			t.Fatalf("accounts step = %+v", report.Accounts)
		}
		w.exec(`UPDATE app.attempts SET status = 'timed_out' WHERE id = $1`, attempt)
		if report := w.sweep(maintenance.SweepOptions{Apply: true}); report.Accounts.Anonymise != 1 || report.Accounts.SkippedInProgress != 0 {
			t.Fatalf("accounts step = %+v", report.Accounts)
		}
	})
}

func TestTheAccountsStepWorksInBatches(t *testing.T) {
	w := newSweepWorld(t)
	w.retention(nil, 13, months(12))
	at := w.cutoff(12).Add(-time.Hour)
	for i := range 5 {
		w.disable(w.user(fmt.Sprintf("Học viên %d", i), "student"), at)
	}
	report := w.sweep(maintenance.SweepOptions{Apply: true, Batch: 2})
	if report.Accounts.Anonymise != 5 || report.Accounts.Batches != 3 {
		t.Errorf("accounts step = %+v, want 5 accounts in 3 batches", report.Accounts)
	}
}

func TestAnAdminDisabledLongAgoIsAnonymisedAndTheLastActiveAdminIsNotTouched(t *testing.T) {
	eachWorld(t, func(t *testing.T, w *sweepWorld) {
		w.retention(nil, 13, months(12))
		at := w.cutoff(12).Add(-time.Hour)
		retired := w.user("Quản trị đã nghỉ", "admin")
		w.disable(retired, at)
		report := w.sweep(maintenance.SweepOptions{Apply: true})
		if report.Accounts.Anonymise != 1 {
			t.Fatalf("accounts step = %+v", report.Accounts)
		}
		var name string
		if err := w.q.QueryRow(w.ctx, `SELECT full_name FROM app.users WHERE id = $1`, retired).Scan(&name); err != nil || name != userPlaceholder {
			t.Errorf("the retired Admin is %q (%v)", name, err)
		}
		if w.count(`SELECT count(*) FROM app.users WHERE id = $1 AND disabled_at IS NULL AND email NOT LIKE 'anonymous-%'`, w.teacher) != 1 {
			t.Error("the active Admin changed")
		}
	})
}

func TestEveryStepIsReadOnlyForTheDryRunAndItsReportCarriesTheRole(t *testing.T) {
	w := newSweepWorld(t)
	w.retention(months(24), 24, months(24))
	report := w.sweep(maintenance.SweepOptions{})
	if report.DatabaseRole == "" || report.Retention.KeepAnswersMonths == nil || *report.Retention.KeepAnswersMonths != 24 ||
		report.Retention.KeepFocusEventsMonths != 24 || *report.Retention.AnonymiseDisabledAfterMonths != 24 {
		t.Errorf("report = %+v", report)
	}
}

func TestSweepOptionsAreCheckedBeforeAnythingIsRead(t *testing.T) {
	ctx := context.Background()
	for name, opts := range map[string]maintenance.SweepOptions{
		"no batch":      {Budget: time.Minute},
		"batch too big": {Batch: 10001, Budget: time.Minute},
		"no budget":     {Batch: 10},
	} {
		if _, err := maintenance.RetentionSweep(ctx, nil, opts); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestAnAccountOwnerIsNeverAnonymisedBySweepOrRequest(t *testing.T) {
	w := newSweepWorld(t)
	w.retention(nil, 13, months(12))
	other := w.user("Quản trị khác", "admin")
	w.user("Quản trị còn lại", "admin")
	w.disable(w.teacher, w.cutoff(12).Add(-time.Hour))
	if report := w.sweep(maintenance.SweepOptions{Apply: true}); report.Accounts.SkippedOwners != 1 || report.Accounts.Anonymise != 0 {
		t.Errorf("accounts step = %+v", report.Accounts)
	}
	if _, err := maintenance.AnonymizeUser(w.ctx, w.conn, w.teacher, true); err == nil {
		t.Error("anonymize-user accepted an owner")
	}
	if _, err := maintenance.AnonymizeUser(w.ctx, w.conn, other, true); err != nil {
		t.Errorf("anonymize-user refused an Admin who owns nothing: %v", err)
	}
}

func TestAnonymizeUserTakesAnyRoleAndAnActiveAccount(t *testing.T) {
	eachWorld(t, func(t *testing.T, w *sweepWorld) {
		for role, want := range map[string]string{"student": studentPlaceholder, "teacher": userPlaceholder, "assistant": userPlaceholder} {
			user := w.user("Họ tên thật", role)
			w.exec(`INSERT INTO app.refresh_tokens(user_id, family_id, token_hash, expires_at)
				VALUES ($1::uuid, gen_random_uuid(), sha256($1::uuid::text::bytea), now() + interval '1 day')`, user)
			w.notification(fmt.Sprintf(`{"studentName": %q}`, "Họ tên thật"))

			preview, err := maintenance.AnonymizeUser(w.ctx, w.conn, user, false)
			if err != nil || preview.Applied || preview.UserID != user {
				t.Fatalf("%s preview = %+v, %v", role, preview, err)
			}
			if w.count(`SELECT count(*) FROM app.users WHERE id = $1 AND full_name = 'Họ tên thật' AND disabled_at IS NULL`, user) != 1 {
				t.Fatalf("%s: the dry run changed the account", role)
			}
			for range 2 {
				if _, err := maintenance.AnonymizeUser(w.ctx, w.conn, user, true); err != nil {
					t.Fatalf("%s: %v", role, err)
				}
			}
			var name, email string
			var disabled *time.Time
			if err := w.q.QueryRow(w.ctx, `SELECT full_name, email, disabled_at FROM app.users WHERE id = $1`, user).Scan(&name, &email, &disabled); err != nil {
				t.Fatal(err)
			}
			if name != want || email != "anonymous-"+user+"@anonymous.invalid" || disabled == nil {
				t.Errorf("%s: %q %q %v", role, name, email, disabled)
			}
			if w.count(`SELECT count(*) FROM app.refresh_tokens WHERE user_id = $1`, user) != 0 ||
				w.count(`SELECT count(*) FROM app.notifications WHERE params ->> 'studentName' = 'Họ tên thật'`) != 0 {
				t.Errorf("%s: a session or a notice naming them survives", role)
			}
			var reason string
			if err := w.q.QueryRow(w.ctx, `SELECT diff ->> 'reason' FROM app.audit_log WHERE action = 'user.anonymized' AND entity_id = $1`, user).Scan(&reason); err != nil || reason != "request" {
				t.Errorf("%s: audit reason %q (%v)", role, reason, err)
			}
			if got := w.count(`SELECT count(*) FROM app.audit_log WHERE action = 'user.anonymized' AND entity_id = $1`, user); got != 1 {
				t.Errorf("%s: %d entries after two runs", role, got)
			}
		}
	})
}

func TestAnonymizeUserRefusesWhatItCannotFind(t *testing.T) {
	w := newSweepWorld(t)
	for name, id := range map[string]string{"not a UUID": "nam", "no such account": "00000000-0000-4000-8000-000000000000"} {
		if _, err := maintenance.AnonymizeUser(w.ctx, w.conn, id, true); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	sitting := w.user("Học viên", "student")
	w.attempt(w.closedAt(time.Now().Add(time.Hour)), sitting, "in_progress", time.Now())
	if _, err := maintenance.AnonymizeUser(w.ctx, w.conn, sitting, true); err == nil {
		t.Error("anonymize-user accepted a student sitting a test")
	}
}

func TestAnonymizeUserLeavesTheLastActiveAdminAlone(t *testing.T) {
	w := newSweepWorld(t)
	var id string
	if err := w.q.QueryRow(w.ctx, `SELECT id::text FROM app.users WHERE role_id = (SELECT id FROM app.roles WHERE builtin_key = 'admin') AND disabled_at IS NULL AND id <> $1`, w.teacher).Scan(&id); err == nil {
		t.Fatalf("the scratch database holds another active Admin %s", id)
	}
	spare := w.user("Quản trị không sở hữu", "admin")
	w.exec(`UPDATE app.tests SET owner_id = $2, created_by = $2 WHERE owner_id = $1`, w.teacher, spare)
	w.exec(`UPDATE app.test_versions SET published_by = $2 WHERE published_by = $1`, w.teacher, spare)
	if _, err := maintenance.AnonymizeUser(w.ctx, w.conn, w.teacher, true); err != nil {
		t.Fatalf("with a second active Admin: %v", err)
	}
	if _, err := maintenance.AnonymizeUser(w.ctx, w.conn, spare, true); err == nil {
		t.Error("anonymize-user disabled the last active Admin")
	}
}

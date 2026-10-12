//go:build integration

package maintenance_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
)

type aiWorld struct {
	t     *testing.T
	ctx   context.Context
	tx    pgx.Tx
	month string
}

func newAIWorld(t *testing.T) *aiWorld {
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
	zone, err := config.AppLocation()
	if err != nil {
		t.Fatal(err)
	}
	local := time.Now().In(zone)
	return &aiWorld{t: t, ctx: ctx, tx: tx, month: time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.UTC).Format(time.DateOnly)}
}

func (w *aiWorld) run(args ...string) map[string]any {
	w.t.Helper()
	command := parse(w.t, args...)
	result, err := command.Run(w.ctx, w.tx)
	if err != nil {
		w.t.Fatalf("%v: %v", args, err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		w.t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		w.t.Fatal(err)
	}
	return out
}

func (w *aiWorld) fails(args ...string) error {
	w.t.Helper()
	_, err := parse(w.t, args...).Run(w.ctx, w.tx)
	if err == nil {
		w.t.Fatalf("%v succeeded", args)
	}
	return err
}

func (w *aiWorld) user(role string) string {
	w.t.Helper()
	var id string
	if err := w.tx.QueryRow(w.ctx, `
		INSERT INTO app.users (email, full_name, role_id)
		VALUES (gen_random_uuid()::text || '@example.test', 'Giáo viên console', (SELECT id FROM app.roles WHERE builtin_key = $1))
		RETURNING id::text`, role).Scan(&id); err != nil {
		w.t.Fatal(err)
	}
	return id
}

func (w *aiWorld) count(sql string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.tx.QueryRow(w.ctx, sql, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func result(t *testing.T, report map[string]any) map[string]any {
	t.Helper()
	inner, ok := report["result"].(map[string]any)
	if !ok {
		t.Fatalf("report %v has no result object", report)
	}
	return inner
}

func TestAStatusReadsTheMonthAndWritesNothing(t *testing.T) {
	w := newAIWorld(t)
	teacher := w.user("teacher")
	w.run("ai-credits", "set-allowance", "-user", teacher, "-credits", "30000", "-operator", "thuong", "-apply")
	before := w.count(`SELECT count(*) FROM app.audit_log WHERE action LIKE 'ai.%'`)

	report := w.run("ai-credits", "status")
	status := result(t, report)
	if report["command"] != "ai-credits status" || report["applied"] != false {
		t.Errorf("report %v", report)
	}
	if status["month"] != w.month {
		t.Errorf("month %v, want %s", status["month"], w.month)
	}
	pool, _ := status["pool"].(map[string]any)
	if pool["plan"].(float64) <= 0 || pool["total"].(float64) < pool["plan"].(float64) {
		t.Errorf("pool %v", pool)
	}
	listed := false
	for _, entry := range status["teachers"].([]any) {
		if row := entry.(map[string]any); row["userId"] == teacher {
			listed = row["allowance"] == float64(30000) && row["custom"] == true && row["status"] == "on_track"
		}
	}
	if !listed {
		t.Errorf("the teacher with a custom 30,000 is not listed as such: %v", status["teachers"])
	}
	if rules, _ := status["rules"].(map[string]any); rules["warnPercent"] == nil || rules["onTeacherLimit"] == nil {
		t.Errorf("rules %v", rules)
	}
	if after := w.count(`SELECT count(*) FROM app.audit_log WHERE action LIKE 'ai.%'`); after != before {
		t.Errorf("a status wrote %d audit rows", after-before)
	}
}

func TestADryRunReportsTheChangeAndRollsItBack(t *testing.T) {
	w := newAIWorld(t)
	teacher := w.user("teacher")

	report := w.run("ai-credits", "pause", "-user", teacher, "-operator", "thuong")
	if report["applied"] != false || report["operator"] != "thuong" {
		t.Errorf("report %v", report)
	}
	if row := result(t, report); row["paused"] != true || row["userId"] != teacher {
		t.Errorf("the dry run reports %v, want the pause it would make", row)
	}
	if n := w.count(`SELECT count(*) FROM app.ai_teacher_credits WHERE user_id = $1::uuid`, teacher); n != 0 {
		t.Errorf("a dry run left %d teacher rows", n)
	}
	if n := w.count(`SELECT count(*) FROM app.audit_log WHERE action = 'ai.allowance.updated' AND entity_id = $1::uuid`, teacher); n != 0 {
		t.Errorf("a dry run left %d audit rows", n)
	}

	applied := w.run("ai-credits", "pause", "-user", teacher, "-operator", "thuong", "-apply")
	if applied["applied"] != true {
		t.Errorf("report %v", applied)
	}
	var paused bool
	if err := w.tx.QueryRow(w.ctx, `SELECT paused FROM app.ai_teacher_credits WHERE month = $1::date AND user_id = $2::uuid`, w.month, teacher).Scan(&paused); err != nil || !paused {
		t.Fatalf("paused %v, %v", paused, err)
	}
	var actor *string
	var diff []byte
	if err := w.tx.QueryRow(w.ctx, `SELECT actor_user_id::text, diff FROM app.audit_log WHERE action = 'ai.allowance.updated' AND entity_id = $1::uuid`, teacher).Scan(&actor, &diff); err != nil {
		t.Fatal(err)
	}
	var details map[string]any
	if err := json.Unmarshal(diff, &details); err != nil {
		t.Fatal(err)
	}
	if actor != nil || details["operator"] != "thuong" || details["reason"] != "maintenance" {
		t.Errorf("audit row actor %v, diff %v; want System with the operator named", actor, details)
	}

	resumed := w.run("ai-credits", "resume", "-user", teacher, "-operator", "thuong", "-apply")
	if result(t, resumed)["paused"] != false {
		t.Errorf("resume: %v", resumed)
	}
}

func TestSetAllowanceTakesACustomFigureOrTheDefault(t *testing.T) {
	w := newAIWorld(t)
	teacher := w.user("teacher")

	custom := result(t, w.run("ai-credits", "set-allowance", "-user", teacher, "-credits", "45000", "-operator", "thuong", "-apply"))
	if custom["allowance"] != float64(45000) || custom["custom"] != true {
		t.Errorf("custom: %v", custom)
	}
	back := result(t, w.run("ai-credits", "set-allowance", "-user", teacher, "-default", "-operator", "thuong", "-apply"))
	if back["custom"] != false || back["allowance"] == float64(45000) {
		t.Errorf("default: %v", back)
	}
	if err := w.fails("ai-credits", "set-allowance", "-user", teacher, "-credits", "999", "-operator", "thuong", "-apply"); err == nil || err.Error() == "" {
		t.Error("an allowance below the least was accepted")
	}
	if err := w.fails("ai-credits", "pause", "-user", w.user("student"), "-operator", "thuong", "-apply"); err == nil {
		t.Error("a student was paused")
	}
}

func TestAddCreditsReachesThePoolOnlyWithApply(t *testing.T) {
	w := newAIWorld(t)
	read := func(report map[string]any) float64 { return result(t, report)["added"].(float64) }

	dry := read(w.run("ai-credits", "add-credits", "-credits", "50000", "-operator", "thuong"))
	var persisted int
	if err := w.tx.QueryRow(w.ctx, `SELECT coalesce((SELECT added_credits FROM app.ai_credit_periods WHERE month = $1::date), 0)`, w.month).Scan(&persisted); err != nil || persisted != 0 {
		t.Fatalf("a dry run left %d added (%v)", persisted, err)
	}
	applied := read(w.run("ai-credits", "add-credits", "-credits", "50000", "-operator", "thuong", "-apply"))
	if dry != 50000 || applied != 50000 {
		t.Errorf("dry %v, applied %v, want 50,000", dry, applied)
	}
	if n := w.count(`SELECT count(*) FROM app.audit_log WHERE action = 'ai.credits.added' AND diff->>'operator' = 'thuong' AND diff->>'month' = $1`, w.month); n != 1 {
		t.Errorf("%d audit rows, want one", n)
	}
}

func TestApproveRaisesTheAllowanceAndTellsTheTeacherOnlyWhenApplied(t *testing.T) {
	w := newAIWorld(t)
	teacher := w.user("teacher")
	w.run("ai-credits", "set-allowance", "-user", teacher, "-credits", "5000", "-operator", "thuong", "-apply")
	var request string
	if err := w.tx.QueryRow(w.ctx, `
		INSERT INTO app.ai_credit_requests (user_id, month, amount_credits, reason) VALUES ($1::uuid, $2::date, 7000, 'Cần thêm')
		RETURNING id::text`, teacher, w.month).Scan(&request); err != nil {
		t.Fatal(err)
	}

	dry := result(t, w.run("ai-credits", "approve", "-request", request, "-operator", "thuong"))
	if req, _ := dry["request"].(map[string]any); req["status"] != "approved" || dry["allowance"] != float64(12000) {
		t.Errorf("dry run: %v", dry)
	}
	if n := w.count(`SELECT count(*) FROM app.notifications WHERE user_id = $1::uuid AND kind = 'ai.credit_request.decided'`, teacher); n != 0 {
		t.Errorf("a dry run told the teacher (%d notices)", n)
	}
	var status string
	if err := w.tx.QueryRow(w.ctx, `SELECT status FROM app.ai_credit_requests WHERE id = $1::uuid`, request).Scan(&status); err != nil || status != "pending" {
		t.Fatalf("status after the dry run %q (%v)", status, err)
	}

	w.run("ai-credits", "approve", "-request", request, "-operator", "thuong", "-apply")
	var allowance, granted int
	if err := w.tx.QueryRow(w.ctx, `SELECT allowance_credits, granted_credits FROM app.ai_teacher_credits WHERE month = $1::date AND user_id = $2::uuid`, w.month, teacher).Scan(&allowance, &granted); err != nil || allowance != 12000 || granted != 7000 {
		t.Errorf("allowance %d, granted %d (%v)", allowance, granted, err)
	}
	if n := w.count(`SELECT count(*) FROM app.notifications WHERE user_id = $1::uuid AND kind = 'ai.credit_request.decided'`, teacher); n != 1 {
		t.Errorf("%d notices after applying, want one", n)
	}
	if err := w.fails("ai-credits", "decline", "-request", request, "-operator", "thuong", "-apply"); err == nil {
		t.Error("a request was decided twice")
	}
}

func TestDeclineLeavesTheAllowanceAlone(t *testing.T) {
	w := newAIWorld(t)
	teacher := w.user("teacher")
	w.run("ai-credits", "set-allowance", "-user", teacher, "-credits", "5000", "-operator", "thuong", "-apply")
	var request string
	if err := w.tx.QueryRow(w.ctx, `
		INSERT INTO app.ai_credit_requests (user_id, month, amount_credits, reason) VALUES ($1::uuid, $2::date, 7000, 'Cần thêm')
		RETURNING id::text`, teacher, w.month).Scan(&request); err != nil {
		t.Fatal(err)
	}
	w.run("ai-credits", "decline", "-request", request, "-operator", "thuong", "-apply")
	var allowance int
	if err := w.tx.QueryRow(w.ctx, `SELECT allowance_credits FROM app.ai_teacher_credits WHERE month = $1::date AND user_id = $2::uuid`, w.month, teacher).Scan(&allowance); err != nil || allowance != 5000 {
		t.Errorf("allowance %d (%v)", allowance, err)
	}
}

func TestSetRulesChangesOnlyWhatIsNamedAndOnlyWithApply(t *testing.T) {
	w := newAIWorld(t)
	var warn int
	var limit string
	read := func() (int, string) {
		if err := w.tx.QueryRow(w.ctx, `SELECT warn_percent, on_teacher_limit FROM app.ai_settings`).Scan(&warn, &limit); err != nil {
			t.Fatal(err)
		}
		return warn, limit
	}
	beforeWarn, beforeLimit := read()
	newWarn := 90
	if beforeWarn == 90 {
		newWarn = 85
	}
	newLimit := "pause"
	if beforeLimit == "pause" {
		newLimit = "request"
	}

	dry := result(t, w.run("ai-credits", "set-rules", "-warn-percent", strconv.Itoa(newWarn), "-on-limit", newLimit, "-operator", "thuong"))
	rules, _ := dry["rules"].(map[string]any)
	if rules["warnPercent"] != float64(newWarn) || rules["onTeacherLimit"] != newLimit {
		t.Errorf("dry run reports %v", dry)
	}
	if warn, limit := read(); warn != beforeWarn || limit != beforeLimit {
		t.Fatalf("a dry run left %d and %s", warn, limit)
	}
	w.run("ai-credits", "set-rules", "-warn-percent", strconv.Itoa(newWarn), "-on-limit", newLimit, "-operator", "thuong", "-apply")
	if warn, limit := read(); warn != newWarn || limit != newLimit {
		t.Errorf("applied: %d and %s, want %d and %s", warn, limit, newWarn, newLimit)
	}
	if err := w.fails("ai-credits", "set-rules", "-operator", "thuong", "-apply"); err == nil {
		t.Error("set-rules with nothing to change ran")
	}
	if err := w.fails("ai-credits", "set-rules", "-on-limit", "ignore", "-operator", "thuong", "-apply"); err == nil {
		t.Error("an unknown limit rule was accepted")
	}
}

func TestTheSweepCountsWhatItWouldReleaseAndReleasesItOnlyWithApply(t *testing.T) {
	w := newAIWorld(t)
	teacher := w.user("teacher")
	w.run("ai-credits", "set-allowance", "-user", teacher, "-credits", "5000", "-operator", "thuong", "-apply")
	for _, step := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE app.ai_teacher_credits SET reserved_credits = 40 WHERE month = $1::date AND user_id = $2::uuid`, []any{w.month, teacher}},
		{`UPDATE app.ai_credit_periods SET reserved_credits = reserved_credits + 40 WHERE month = $1::date`, []any{w.month}},
		{`INSERT INTO app.ai_reservations (month, user_id, feature, credits, created_at)
		  SELECT $1::date, $2::uuid, 'score', 40, now() - interval '30 minutes'`, []any{w.month, teacher}},
	} {
		if _, err := w.tx.Exec(w.ctx, step.sql, step.args...); err != nil {
			t.Fatal(err)
		}
	}

	dry := w.run("ai-reservations-sweep")
	if dry["applied"] != false || dry["reservations"].(float64) < 1 || dry["credits"].(float64) < 40 {
		t.Errorf("dry run: %v", dry)
	}
	if n := w.count(`SELECT count(*) FROM app.ai_reservations WHERE user_id = $1::uuid`, teacher); n != 1 {
		t.Fatalf("a dry run left %d reservations", n)
	}
	applied := w.run("ai-reservations-sweep", "-apply")
	if applied["applied"] != true || applied["reservations"].(float64) < 1 {
		t.Errorf("applied: %v", applied)
	}
	if n := w.count(`SELECT count(*) FROM app.ai_reservations WHERE user_id = $1::uuid`, teacher); n != 0 {
		t.Errorf("%d reservations left", n)
	}
	var held int
	if err := w.tx.QueryRow(w.ctx, `SELECT reserved_credits FROM app.ai_teacher_credits WHERE month = $1::date AND user_id = $2::uuid`, w.month, teacher).Scan(&held); err != nil || held != 0 {
		t.Errorf("the teacher still holds %d (%v)", held, err)
	}
}

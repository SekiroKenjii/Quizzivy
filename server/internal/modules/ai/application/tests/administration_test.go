//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/ai/application/command"
	"quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/ai/domain"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
)

type auditRow struct {
	actor    *string
	action   string
	entity   string
	entityID *string
	diff     map[string]any
}

func (w *world) audit(action string, month time.Time) []auditRow {
	w.t.Helper()
	rows, err := w.pool.Query(context.Background(), `
		SELECT actor_user_id::text, action, entity, entity_id::text, diff FROM app.audit_log
		 WHERE action = $1 AND diff->>'month' = $2 ORDER BY id`, action, month.Format(time.DateOnly))
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var r auditRow
		var raw []byte
		if err := rows.Scan(&r.actor, &r.action, &r.entity, &r.entityID, &raw); err != nil {
			w.t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &r.diff); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		w.t.Fatal(err)
	}
	return out
}

func (w *world) notice(userID, key string) (kind string, params map[string]any, found bool) {
	w.t.Helper()
	var raw []byte
	err := w.pool.QueryRow(context.Background(), `SELECT kind, params FROM app.notifications WHERE user_id = $1::uuid AND dedupe_key = $2`, userID, key).Scan(&kind, &raw)
	if err != nil {
		return "", nil, false
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		w.t.Fatal(err)
	}
	return kind, params, true
}

func TestARuleChangeRewritesTheMonthsNonCustomAllowancesAndNoOthers(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	other := month.AddDate(0, 1, 0)
	w.track(other)
	admin := domain.Author{UserID: w.teacher(), IP: "203.0.113.7", UserAgent: "tests"}
	rules := w.rules()
	next := rules.DefaultAllowance + 10_000

	plain, custom, heavy, later := w.teacher(), w.teacher(), w.teacher(), w.teacher()
	w.allowance(month, plain, rules.DefaultAllowance)
	w.allowance(month, custom, 50_000)
	w.exec(`UPDATE app.ai_teacher_credits SET custom = true WHERE month = $1::date AND user_id = $2::uuid`, month, custom)
	w.allowance(month, heavy, rules.DefaultAllowance+25_000)
	w.exec(`UPDATE app.ai_teacher_credits SET used_credits = $3 WHERE month = $1::date AND user_id = $2::uuid`, month, heavy, rules.DefaultAllowance+20_000)
	w.allowance(other, later, rules.DefaultAllowance)
	w.exec(`UPDATE app.ai_teacher_credits SET granted_credits = 2_000, allowance_credits = allowance_credits + 2_000 WHERE month = $1::date AND user_id = $2::uuid`, month, plain)

	change, err := w.app.Commands.SetRules.Handle(context.Background(), command.SetRules{By: admin, DefaultAllowance: &next})
	if err != nil {
		t.Fatal(err)
	}
	if change.Rules.DefaultAllowance != next || change.Rewritten < 2 {
		t.Fatalf("change %+v", change)
	}
	if got := w.teacherRow(month, plain); got.allowance != next+2_000 || got.granted != 2_000 {
		t.Errorf("a teacher on the default is at %+v, want the new default and the grant they already had", got)
	}
	if got := w.teacherRow(month, custom); got.allowance != 50_000 {
		t.Errorf("a custom allowance was rewritten to %d", got.allowance)
	}
	used := rules.DefaultAllowance + 20_000
	if got := w.teacherRow(month, heavy); got.allowance != used {
		t.Errorf("a teacher who has used more than the new default has an allowance of %d, want the %d they have used", got.allowance, used)
	}
	if got := w.teacherRow(other, later); got.allowance != rules.DefaultAllowance {
		t.Errorf("another month's allowance was rewritten to %d", got.allowance)
	}

	lower := rules.DefaultAllowance - 5_000
	if _, err := w.app.Commands.SetRules.Handle(context.Background(), command.SetRules{By: admin, DefaultAllowance: &lower}); err != nil {
		t.Fatal(err)
	}
	if got := w.teacherRow(month, heavy); got.allowance != used {
		t.Errorf("a lowered default took back from %+v", got)
	}
	w.exec(`UPDATE app.ai_teacher_credits SET allowance_credits = $3 WHERE month = $1::date AND user_id = $2::uuid`, month, heavy, used+100)
	lowest := domain.MinAllowance
	if _, err := w.app.Commands.SetRules.Handle(context.Background(), command.SetRules{By: admin, DefaultAllowance: &lowest}); err != nil {
		t.Fatal(err)
	}
	if got := w.teacherRow(month, heavy); got.allowance != used {
		t.Errorf("a default below the usage left %d, want the allowance held at the %d already used", got.allowance, used)
	}
	if got := w.teacherRow(month, plain); got.allowance != lowest+2_000 {
		t.Errorf("the teacher on the default is at %d", got.allowance)
	}

	rows := w.audit("ai.rules.updated", month)
	if len(rows) != 3 {
		t.Fatalf("%d audit rows for three changes", len(rows))
	}
	first := rows[0]
	if first.actor == nil || *first.actor != admin.UserID || first.entity != "ai_settings" || first.entityID != nil {
		t.Errorf("audit row %+v", first)
	}
	after, _ := first.diff["after"].(map[string]any)
	before, _ := first.diff["before"].(map[string]any)
	if after["defaultAllowanceCredits"] != float64(next) || before["defaultAllowanceCredits"] != float64(rules.DefaultAllowance) || first.diff["rewritten"].(float64) < 2 {
		t.Errorf("diff %+v", first.diff)
	}
}

func TestOtherRulesChangeWithoutTouchingAnyAllowance(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	admin := domain.Author{UserID: w.teacher()}
	teacher := w.teacher()
	rules := w.rules()
	w.allowance(month, teacher, rules.DefaultAllowance+1)
	pause, warn := domain.LimitPause, 90

	change, err := w.app.Commands.SetRules.Handle(context.Background(), command.SetRules{By: admin, OnTeacherLimit: &pause, WarnPercent: &warn})
	if err != nil {
		t.Fatal(err)
	}
	if change.Rules.OnTeacherLimit != domain.LimitPause || change.Rules.WarnPercent != 90 || change.Rules.DefaultAllowance != rules.DefaultAllowance || change.Rewritten != 0 {
		t.Errorf("change %+v", change)
	}
	if got := w.teacherRow(month, teacher); got.allowance != rules.DefaultAllowance+1 {
		t.Errorf("allowance %d", got.allowance)
	}
}

func TestARequestIsApprovedRaisingTheAllowanceTellingBothSidesAndWritingTheAuditRow(t *testing.T) {
	w := newTxWorld(t)
	month := w.newMonth()
	teacher, admin := w.teacher(), w.user("admin")
	rules := w.rules()
	if rules.OnTeacherLimit != domain.LimitRequest {
		t.Skip("the centre's rule is not to take requests")
	}
	w.allowance(month, teacher, 2_000)

	request, err := w.app.Commands.RequestCredits.Handle(context.Background(), command.RequestCredits{Actor: asActor(teacher), Amount: 5_000, Reason: "  Ba lớp thi cuối kỳ.  "})
	if err != nil {
		t.Fatal(err)
	}
	w.watch("ai_request:"+request.ID, "ai_decision:"+request.ID)
	if request.Status != domain.RequestPending || request.Reason != "Ba lớp thi cuối kỳ." || request.Amount != 5_000 || !request.Month.Equal(month) {
		t.Fatalf("request %+v", request)
	}
	kind, params, ok := w.notice(admin, "ai_request:"+request.ID)
	if !ok || kind != string(notificationsdomain.AICreditRequest) || params["credits"] != float64(5_000) || params["teacherName"] != "Giáo viên thử credit" {
		t.Errorf("the admin was told %q %v (found %v)", kind, params, ok)
	}

	if _, err := w.app.Commands.RequestCredits.Handle(context.Background(), command.RequestCredits{Actor: asActor(teacher), Amount: 5_000, Reason: "Lại"}); !errors.Is(err, domain.ErrRequestPending) {
		t.Errorf("a second request in the month: %v", err)
	}
	usage, err := w.app.Queries.Usage.Handle(context.Background(), query.Usage{UserID: teacher})
	if err != nil || usage.Pending == nil || usage.Pending.ID != request.ID || !usage.Requests {
		t.Errorf("usage %+v, %v; want the pending request and requests on", usage, err)
	}
	view, err := w.app.Queries.AdminCredits.Handle(context.Background(), query.AdminCredits{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range view.Requests {
		if r.ID == request.ID && r.TeacherName == "Giáo viên thử credit" && r.Allowance == 2_000 {
			found = true
		}
	}
	if !found {
		t.Errorf("the Admin's view lists %+v, want the request with the teacher's standing", view.Requests)
	}

	decided, err := w.app.Commands.DecideRequest.Handle(context.Background(), command.DecideRequest{
		By: domain.Author{UserID: admin, IP: "203.0.113.5", UserAgent: "tests"}, ID: request.ID, Approve: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if decided.Request.Status != domain.RequestApproved || decided.Allowance != 7_000 || decided.Request.DecidedBy != admin || decided.Request.DecidedAt == nil {
		t.Fatalf("decided %+v", decided)
	}
	if got := w.teacherRow(month, teacher); got.allowance != 7_000 || got.granted != 5_000 || got.custom {
		t.Errorf("teacher %+v, want 7,000 with 5,000 granted and still on the default", got)
	}
	kind, params, ok = w.notice(teacher, "ai_decision:"+request.ID)
	if !ok || kind != string(notificationsdomain.AICreditRequestDecided) || params["decision"] != "approved" || params["credits"] != float64(5_000) {
		t.Errorf("the teacher was told %q %v (found %v)", kind, params, ok)
	}

	rows := w.audit("ai.credit_request.approved", month)
	if len(rows) != 1 {
		t.Fatalf("%d audit rows", len(rows))
	}
	row := rows[0]
	if row.actor == nil || *row.actor != admin || row.entity != "ai_credit_requests" || row.entityID == nil || *row.entityID != request.ID {
		t.Errorf("audit row %+v", row)
	}
	if row.diff["credits"] != float64(5_000) || row.diff["allowanceBefore"] != float64(2_000) || row.diff["allowanceAfter"] != float64(7_000) || row.diff["userId"] != teacher {
		t.Errorf("diff %+v", row.diff)
	}

	if _, err := w.app.Commands.DecideRequest.Handle(context.Background(), command.DecideRequest{By: domain.Author{UserID: admin}, ID: request.ID}); !errors.Is(err, domain.ErrRequestDecided) {
		t.Errorf("a second decision: %v", err)
	}
	if got := w.teacherRow(month, teacher); got.allowance != 7_000 {
		t.Errorf("a refused decision moved the allowance to %d", got.allowance)
	}
	renewed, err := w.app.Commands.RequestCredits.Handle(context.Background(), command.RequestCredits{Actor: asActor(teacher), Amount: 1_000, Reason: "Thêm"})
	if err != nil {
		t.Fatalf("a new request after the first was decided: %v", err)
	}
	w.watch("ai_request:"+renewed.ID, "ai_decision:"+renewed.ID)
}

func TestADeclineChangesNothingButTheStatusAndAnEarlierMonthCanOnlyBeDeclined(t *testing.T) {
	w := newWorld(t)
	earlier := w.newMonth()
	later := earlier.AddDate(0, 1, 0)
	w.track(later)
	teacher, admin := w.teacher(), domain.Author{UserID: w.teacher()}
	w.allowance(earlier, teacher, 3_000)

	w.at(earlier.Add(24 * time.Hour))
	old, err := w.app.Commands.RequestCredits.Handle(context.Background(), command.RequestCredits{Actor: asActor(teacher), Amount: 4_000, Reason: "Tháng trước"})
	if err != nil {
		t.Fatal(err)
	}
	w.watch("ai_request:"+old.ID, "ai_decision:"+old.ID)

	w.at(later.Add(24 * time.Hour))
	if _, err := w.app.Commands.DecideRequest.Handle(context.Background(), command.DecideRequest{By: admin, ID: old.ID, Approve: true}); !errors.Is(err, domain.ErrRequestExpired) {
		t.Fatalf("approving last month's request: %v", err)
	}
	if got := w.teacherRow(earlier, teacher); got.allowance != 3_000 || got.granted != 0 {
		t.Errorf("a refused approval changed %+v", got)
	}
	decided, err := w.app.Commands.DecideRequest.Handle(context.Background(), command.DecideRequest{By: admin, ID: old.ID})
	if err != nil || decided.Request.Status != domain.RequestDeclined || decided.Allowance != 0 {
		t.Fatalf("declining it: %+v, %v", decided, err)
	}
	if _, params, ok := w.notice(teacher, "ai_decision:"+old.ID); !ok || params["decision"] != "declined" {
		t.Errorf("the teacher was told %v (found %v)", params, ok)
	}
	if rows := w.audit("ai.credit_request.declined", earlier); len(rows) != 1 || rows[0].diff["allowanceAfter"] != nil {
		t.Errorf("audit rows %+v", rows)
	}
	if _, err := w.app.Commands.DecideRequest.Handle(context.Background(), command.DecideRequest{By: admin, ID: "0198f5f0-0000-7000-8000-0000000000ff"}); !errors.Is(err, domain.ErrRequestNotFound) {
		t.Errorf("an id that is no request: %v", err)
	}
}

func TestRequestsAreRefusedWhileTheRuleIsToPauseAndForPeopleWhoAreNotTeachers(t *testing.T) {
	w := newWorld(t)
	w.newMonth()
	admin := domain.Author{UserID: w.teacher()}
	teacher := w.teacher()
	pause, ask := domain.LimitPause, domain.LimitRequest

	if _, err := w.app.Commands.SetRules.Handle(context.Background(), command.SetRules{By: admin, OnTeacherLimit: &pause}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.app.Commands.RequestCredits.Handle(context.Background(), command.RequestCredits{Actor: asActor(teacher), Amount: 2_000, Reason: "x"}); !errors.Is(err, domain.ErrRequestsOff) {
		t.Errorf("a request while teachers are paused when they run out: %v", err)
	}
	if usage, err := w.app.Queries.Usage.Handle(context.Background(), query.Usage{UserID: teacher}); err != nil || usage.Requests {
		t.Errorf("usage %+v, %v; want requests off", usage, err)
	}
	if _, err := w.app.Commands.SetRules.Handle(context.Background(), command.SetRules{By: admin, OnTeacherLimit: &ask}); err != nil {
		t.Fatal(err)
	}
	student := w.user("student")
	if _, err := w.app.Commands.RequestCredits.Handle(context.Background(), command.RequestCredits{Actor: asActor(student), Amount: 2_000, Reason: "x"}); !errors.Is(err, domain.ErrNotATeacher) {
		t.Errorf("a student's request: %v", err)
	}
}

func TestSetAllowanceChangesTheAllowanceAndThePauseAndAuditsEach(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	admin := domain.Author{UserID: w.teacher(), IP: "203.0.113.8", UserAgent: "tests"}
	teacher := w.teacher()
	rules := w.rules()
	yes, no := true, false

	set, err := w.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: admin, UserID: teacher, Custom: &yes, Credits: 40_000})
	if err != nil {
		t.Fatal(err)
	}
	if set.Allowance != 40_000 || !set.Custom || set.Paused || set.Status != domain.StatusOnTrack || set.Name != "Giáo viên thử credit" {
		t.Errorf("set %+v", set)
	}
	w.exec(`UPDATE app.ai_teacher_credits SET granted_credits = 1_000, allowance_credits = allowance_credits + 1_000, used_credits = 30_000 WHERE month = $1::date AND user_id = $2::uuid`, month, teacher)

	if _, err := w.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: admin, UserID: teacher, Custom: &yes, Credits: 20_000}); !errors.Is(err, domain.ErrAllowanceBelowUse) {
		t.Fatalf("an allowance below the 30,000 used: %v", err)
	}
	if got := w.teacherRow(month, teacher); got.allowance != 41_000 || !got.custom {
		t.Errorf("a refused change left %+v", got)
	}
	back, err := w.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: admin, UserID: teacher, Custom: &no, Paused: &yes})
	if err != nil && !errors.Is(err, domain.ErrAllowanceBelowUse) {
		t.Fatal(err)
	}
	if rules.DefaultAllowance+1_000 >= 30_000 {
		if err != nil || back.Allowance != rules.DefaultAllowance+1_000 || back.Custom || !back.Paused {
			t.Errorf("back to the default: %+v, %v", back, err)
		}
	} else if !errors.Is(err, domain.ErrAllowanceBelowUse) {
		t.Errorf("the default is below the usage and was applied: %+v", back)
	}

	w.exec(`UPDATE app.ai_teacher_credits SET used_credits = 0 WHERE month = $1::date AND user_id = $2::uuid`, month, teacher)
	back, err = w.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: admin, UserID: teacher, Custom: &no, Paused: &yes})
	if err != nil || back.Allowance != rules.DefaultAllowance+1_000 || back.Custom || !back.Paused || back.Status != domain.StatusPaused {
		t.Fatalf("back to the default and paused: %+v, %v", back, err)
	}

	rows := w.audit("ai.allowance.updated", month)
	if len(rows) != 2 {
		t.Fatalf("%d audit rows for two changes", len(rows))
	}
	first := rows[0]
	if first.actor == nil || *first.actor != admin.UserID || first.entity != "ai_teacher_credits" || first.entityID == nil || *first.entityID != teacher {
		t.Errorf("audit row %+v", first)
	}
	if after, _ := first.diff["after"].(map[string]any); after["allowanceCredits"] != float64(40_000) || after["custom"] != true {
		t.Errorf("diff %+v", first.diff)
	}
}

func TestOnlyAnActiveNonStudentUserHasAnAllowance(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	admin := domain.Author{UserID: w.teacher()}
	yes := true
	student, disabled := w.user("student"), w.teacher()
	w.exec(`UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, disabled)

	for name, id := range map[string]string{
		"a student":      student,
		"a disabled one": disabled,
		"nobody":         "0198f5f0-0000-7000-8000-0000000000ee",
	} {
		if _, err := w.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: admin, UserID: id, Paused: &yes}); !errors.Is(err, domain.ErrNotATeacher) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if n := w.count(`SELECT count(*) FROM app.ai_teacher_credits WHERE month = $1::date`, month); n != 0 {
		t.Errorf("%d rows were created for people who are not teachers", n)
	}
}

func TestSystemActsWithTheOperatorInTheAuditRowAndNoActor(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	teacher := w.teacher()
	yes := true
	console := domain.Author{Operator: "thuong"}

	if _, err := w.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: console, UserID: teacher, Paused: &yes}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.app.Commands.AddCredits.Handle(context.Background(), command.AddCredits{By: console, Credits: 10_000}); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"ai.allowance.updated", "ai.credits.added"} {
		rows := w.audit(action, month)
		if len(rows) != 1 || rows[0].actor != nil || rows[0].diff["reason"] != "maintenance" || rows[0].diff["operator"] != "thuong" {
			t.Errorf("%s: audit rows %+v", action, rows)
		}
	}
}

func TestAddedCreditsAreTheApprovalToSpendBeyondThePlan(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	w.period(month, 300)
	teacher, admin := w.teacher(), domain.Author{UserID: w.teacher(), IP: "203.0.113.6"}

	w.mustReserve(teacher, domain.FeatureGenerate, 300)
	if _, err := w.reserve(teacher, domain.FeatureGenerate, 100); !isScope(err, domain.ScopePool) {
		t.Fatalf("a pool at its plan: %v", err)
	}
	pool, err := w.app.Commands.AddCredits.Handle(context.Background(), command.AddCredits{By: admin, Credits: 150})
	if err != nil || pool.Plan != 300 || pool.Added != 150 || pool.Total() != 450 {
		t.Fatalf("pool %+v, %v", pool, err)
	}
	w.mustReserve(teacher, domain.FeatureGenerate, 100)
	if _, err := w.reserve(teacher, domain.FeatureGenerate, 100); !isScope(err, domain.ScopePool) {
		t.Fatalf("a pool at its total: %v", err)
	}
	if _, err := w.app.Commands.AddCredits.Handle(context.Background(), command.AddCredits{By: admin, Credits: 50}); err != nil {
		t.Fatal(err)
	}
	if p := w.periodRow(month); p.added != 200 || p.reserved != 400 {
		t.Errorf("period %+v", p)
	}

	rows := w.audit("ai.credits.added", month)
	if len(rows) != 2 || rows[0].actor == nil || *rows[0].actor != admin.UserID || rows[0].diff["credits"] != float64(150) || rows[0].diff["addedBefore"] != float64(0) || rows[0].diff["addedAfter"] != float64(150) {
		t.Errorf("audit rows %+v", rows)
	}
}

func TestAddCreditsOnAMonthNobodyHasUsedYetCreatesItsPool(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	admin := domain.Author{UserID: w.teacher()}
	pool, err := w.app.Commands.AddCredits.Handle(context.Background(), command.AddCredits{By: admin, Credits: 5_000})
	if err != nil || pool.Added != 5_000 || pool.Plan != w.rules().PoolCredits {
		t.Fatalf("pool %+v, %v", pool, err)
	}
	if p := w.periodRow(month); p.added != 5_000 {
		t.Errorf("period %+v", p)
	}
}

func TestTheWarningIsRaisedOncePerMonthAndEachMonthWarnsAgain(t *testing.T) {
	w := newWorld(t)
	first := w.newMonth()
	second := first.AddDate(0, 1, 0)
	w.track(second)
	teacher := w.teacher()
	rules := w.rules()
	w.allowance(first, teacher, 1_000)
	w.allowance(second, teacher, 1_000)
	w.watch("ai_warning:"+first.Format(time.DateOnly), "ai_warning:"+second.Format(time.DateOnly))
	crossing := int64(1_000*rules.WarnPercent/100) + 1

	below := w.mustReserve(teacher, domain.FeatureScore, 10)
	if settled := w.settle(below, 5, 0); settled.Warning != nil {
		t.Fatalf("5 of 1,000 warned: %+v", settled.Warning)
	}
	big := w.mustReserve(teacher, domain.FeatureGenerate, 900)
	settled := w.settle(big, crossing, 0)
	if settled.Warning == nil || settled.Warning.Used != crossing+5 || settled.Warning.Allowance != 1_000 || !settled.Warning.Month.Equal(first) {
		t.Fatalf("settled %+v, want the warning at %d of 1,000", settled, crossing+5)
	}
	if tr := w.teacherRow(first, teacher); !tr.warned {
		t.Error("the row does not remember it warned")
	}
	kind, params, ok := w.notice(teacher, "ai_warning:"+first.Format(time.DateOnly))
	if !ok || kind != string(notificationsdomain.AIAllowanceWarning) || params["used"] != float64(crossing+5) || params["allowance"] != float64(1_000) {
		t.Errorf("notice %q %v (found %v)", kind, params, ok)
	}

	again := w.mustReserve(teacher, domain.FeatureScore, 10)
	if settled := w.settle(again, 5, 0); settled.Warning != nil {
		t.Errorf("a second crossing in the month warned: %+v", settled.Warning)
	}
	if n := w.count(`SELECT count(*) FROM app.notifications WHERE user_id = $1::uuid AND kind = 'ai.allowance_warning'`, teacher); n != 1 {
		t.Errorf("%d warnings in the month, want one", n)
	}

	w.at(second.Add(24 * time.Hour))
	next := w.mustReserve(teacher, domain.FeatureGenerate, 900)
	if settled := w.settle(next, crossing, 0); settled.Warning == nil || !settled.Warning.Month.Equal(second) {
		t.Errorf("the next month did not warn: %+v", settled.Warning)
	}
	if n := w.count(`SELECT count(*) FROM app.notifications WHERE user_id = $1::uuid AND kind = 'ai.allowance_warning'`, teacher); n != 2 {
		t.Errorf("%d warnings after two months, want two", n)
	}
}

func TestTheUsageCardSumsTheLedgerByTheFiveCategoriesAndLeavesWhatIsLeft(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	w.period(month, 1_000)
	teacher, other := w.teacher(), w.teacher()
	w.allowance(month, teacher, 2_000)

	w.settle(w.mustReserve(teacher, domain.FeatureGenerate, 200), 100, 0)
	w.settle(w.mustReserve(teacher, domain.FeatureVariant, 300), 50, 0)
	w.settle(w.mustReserve(teacher, domain.FeatureScore, 30), 5, 0)
	w.settle(w.mustReserve(teacher, domain.FeatureExplanation, 20), 3, 0)
	w.settle(w.mustReserve(teacher, domain.FeatureDistractors, 20), 2, 0)
	w.settle(w.mustReserve(teacher, domain.FeatureCheck, 300), 40, 0)
	w.settle(w.mustReserve(teacher, domain.FeatureInsights, 60), 10, 0)
	w.settle(w.mustReserve(other, domain.FeatureImport, 60), 25, 0)
	held := w.mustReserve(teacher, domain.FeatureCleanup, 200)

	usage, err := w.app.Queries.Usage.Handle(context.Background(), query.Usage{UserID: teacher})
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.CategorySum{
		{Category: domain.CategoryGeneration, Credits: 150},
		{Category: domain.CategoryGrading, Credits: 5},
		{Category: domain.CategoryExplanations, Credits: 5},
		{Category: domain.CategoryImports, Credits: 0},
		{Category: domain.CategoryChecks, Credits: 50},
	}
	if len(usage.Categories) != len(want) {
		t.Fatalf("categories %+v", usage.Categories)
	}
	for i, c := range want {
		if usage.Categories[i] != c {
			t.Errorf("row %d: %+v, want %+v", i, usage.Categories[i], c)
		}
	}
	if usage.Used != 210 || usage.Allowance != 2_000 {
		t.Errorf("used %d of %d", usage.Used, usage.Allowance)
	}
	if poolLeft := int64(1_000 - 210 - 25 - 200); usage.Left != poolLeft {
		t.Errorf("left %d, want the pool's %d since it is lower than the allowance's", usage.Left, poolLeft)
	}
	if _, err := w.app.Commands.Release.Handle(context.Background(), command.Release{Reservation: held}); err != nil {
		t.Fatal(err)
	}

	fresh := w.teacher()
	cold, err := w.app.Queries.Usage.Handle(context.Background(), query.Usage{UserID: fresh})
	if err != nil || cold.Used != 0 || cold.Paused || cold.Allowance != w.rules().DefaultAllowance || len(cold.Categories) != 5 {
		t.Errorf("a teacher with no row: %+v, %v", cold, err)
	}
	if w.hasTeacherRow(month, fresh) {
		t.Error("a read created the teacher's row")
	}
	if cold.Left != min(w.rules().DefaultAllowance, 1_000-210-25) {
		t.Errorf("left %d", cold.Left)
	}
}

func TestTheAdminViewHoldsThePoolTheTeachersTheirStandingAndTheRules(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	w.period(month, 10_000)
	idle, busy, near, gone, stopped := w.teacher(), w.teacher(), w.teacher(), w.teacher(), w.teacher()
	rules := w.rules()
	for _, id := range []string{busy, near, gone, stopped} {
		w.allowance(month, id, 1_000)
	}
	w.exec(`UPDATE app.ai_teacher_credits SET used_credits = 100 WHERE month = $1::date AND user_id = $2::uuid`, month, busy)
	w.exec(`UPDATE app.ai_teacher_credits SET used_credits = $3 WHERE month = $1::date AND user_id = $2::uuid`, month, near, 1_000*int64(rules.WarnPercent)/100)
	w.exec(`UPDATE app.ai_teacher_credits SET used_credits = 1000 WHERE month = $1::date AND user_id = $2::uuid`, month, gone)
	w.exec(`UPDATE app.ai_teacher_credits SET used_credits = 1000, paused = true WHERE month = $1::date AND user_id = $2::uuid`, month, stopped)
	w.exec(`UPDATE app.ai_credit_periods SET used_credits = 2_200, added_credits = 500 WHERE month = $1::date`, month)
	w.exec(`UPDATE app.users SET role_id = (SELECT id FROM app.roles WHERE builtin_key = 'teacher') WHERE id = $1::uuid`, idle)

	w.at(month.Add(14*24*time.Hour - 7*time.Hour))
	view, err := w.app.Queries.AdminCredits.Handle(context.Background(), query.AdminCredits{})
	if err != nil {
		t.Fatal(err)
	}
	if view.Pool.Plan != 10_000 || view.Pool.Added != 500 || view.Pool.Used != 2_200 || view.Pool.Left() != 8_300 {
		t.Errorf("pool %+v", view.Pool)
	}
	days := int64(domain.DaysIn(month))
	if want := (int64(2_200)*days + 14) / 15; view.OnPace != want || !view.WithinPool {
		t.Errorf("on pace %d (within pool %v), want %d", view.OnPace, view.WithinPool, want)
	}
	if view.DaysToGo != int(days)-15 {
		t.Errorf("days to go %d", view.DaysToGo)
	}
	if view.Rules != rules {
		t.Errorf("rules %+v, want %+v", view.Rules, rules)
	}

	standing := map[string]domain.TeacherMonth{}
	order := map[string]int{}
	for i, teacher := range view.Teachers {
		standing[teacher.UserID] = teacher
		order[teacher.UserID] = i
	}
	for id, want := range map[string]domain.TeacherStatus{
		busy: domain.StatusOnTrack, near: domain.StatusNearLimit, gone: domain.StatusAtLimit, stopped: domain.StatusPaused,
	} {
		if standing[id].Status != want || standing[id].Allowance != 1_000 {
			t.Errorf("teacher %s: %+v, want %s", id, standing[id], want)
		}
	}
	if order[busy] <= order[near] || order[near] <= order[gone] || order[stopped] > order[busy] {
		t.Errorf("the order is busy %d, near %d, at the limit %d, paused %d; want the most used share first", order[busy], order[near], order[gone], order[stopped])
	}
	if view.Allowances < 4_000 || view.Active < 4 {
		t.Errorf("allowances %d, active %d", view.Allowances, view.Active)
	}
	if _, listed := standing[idle]; listed {
		t.Error("a teacher with no ai.use grant and no use was listed")
	}
}

func TestTheLedgerAndTheAuditLogAreAppendOnlyForTheAppRole(t *testing.T) {
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		t.Skip("APP_DATABASE_URL is not set")
	}
	w := newWorld(t)
	month := w.newMonth()
	teacher := w.teacher()
	w.settle(w.mustReserve(teacher, domain.FeatureScore, 30), 7, 0)

	app, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	var role string
	if err := app.QueryRow(context.Background(), `SELECT current_user`).Scan(&role); err != nil || role != "quizzivy_app" {
		t.Fatalf("connected as %q (%v), want the app role", role, err)
	}
	if n := func() int {
		var n int
		if err := app.QueryRow(context.Background(), `SELECT count(*) FROM app.ai_usage_ledger WHERE month = $1::date`, month).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}(); n != 1 {
		t.Fatalf("the app role reads %d ledger rows, want 1", n)
	}
	for _, statement := range []string{
		`UPDATE app.ai_usage_ledger SET credits = 0 WHERE month = $1::date`,
		`UPDATE app.ai_usage_ledger SET user_id = NULL WHERE month = $1::date`,
		`DELETE FROM app.ai_usage_ledger WHERE month = $1::date`,
	} {
		_, err := app.Exec(context.Background(), statement, month)
		var refused *pgconn.PgError
		if !errors.As(err, &refused) || refused.Code != "42501" {
			t.Errorf("%s: %v, want permission denied", statement, err)
		}
	}
	if _, err := app.Exec(context.Background(), `
		INSERT INTO app.ai_usage_ledger (user_id, month, feature, tier, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, credits)
		VALUES ($1::uuid, $2::date, 'score', 'fast', 1, 1, 0, 0, 1)`, teacher, month); err != nil {
		t.Errorf("the app role cannot append: %v", err)
	}
	if n := w.count(`SELECT count(*) FROM app.ai_usage_ledger WHERE month = $1::date`, month); n != 2 {
		t.Errorf("%d ledger rows, want the settled one and the appended one", n)
	}
	for _, statement := range []string{
		`UPDATE app.ai_reservations SET credits = credits WHERE month = $1::date`,
		`DELETE FROM app.ai_reservations WHERE month = $1::date`,
		`UPDATE app.ai_teacher_credits SET paused = paused WHERE month = $1::date`,
		`UPDATE app.ai_credit_periods SET added_credits = added_credits WHERE month = $1::date`,
	} {
		if _, err := app.Exec(context.Background(), statement, month); err != nil {
			t.Errorf("%s: %v; the balances are the app role's to move", statement, err)
		}
	}
}

package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/ai/application/command"
	"quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/ai/domain"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/validation"
)

const teacherID = "0198f5f0-0000-7000-8000-000000000001"

func refusedFields(t *testing.T, err error) []string {
	t.Helper()
	var refused *validation.Error
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a validation error", err)
	}
	var out []string
	for _, f := range refused.Fields {
		out = append(out, f.Field)
	}
	return out
}

func TestReserveHoldsTheCeilingForTheActorInTheCurrentMonth(t *testing.T) {
	f := newFixture(t)
	got, err := f.app.Commands.Reserve.Handle(context.Background(), command.Reserve{
		Actor: actor.Actor{ID: teacherID}, Feature: domain.FeatureCheck, Ceiling: domain.FeatureCheck.Ceiling(),
	})
	if err != nil || got.Credits != 300 || got.UserID != teacherID {
		t.Fatalf("reservation %+v, %v", got, err)
	}
	if len(f.repo.reserved) != 1 || f.repo.reserved[0].Month.Format(time.DateOnly) != "2090-09-01" {
		t.Errorf("reserved %+v, want the month of the clock in the centre's zone", f.repo.reserved)
	}
}

func TestReserveRefusesWhatNoCallCouldMeanBeforeItAsksTheStore(t *testing.T) {
	f := newFixture(t)
	for name, cmd := range map[string]struct {
		cmd  command.Reserve
		want error
	}{
		"nobody":                {command.Reserve{Feature: domain.FeatureScore, Ceiling: 30}, domain.ErrNotATeacher},
		"a feature no one has":  {command.Reserve{Actor: actor.Actor{ID: teacherID}, Feature: "words", Ceiling: 30}, domain.ErrUnknownFeature},
		"no credits":            {command.Reserve{Actor: actor.Actor{ID: teacherID}, Feature: domain.FeatureScore}, domain.ErrInvalidCeiling},
		"negative credits":      {command.Reserve{Actor: actor.Actor{ID: teacherID}, Feature: domain.FeatureScore, Ceiling: -5}, domain.ErrInvalidCeiling},
		"more than may be held": {command.Reserve{Actor: actor.Actor{ID: teacherID}, Feature: domain.FeatureScore, Ceiling: domain.MaxCeiling + 1}, domain.ErrInvalidCeiling},
	} {
		if _, err := f.app.Commands.Reserve.Handle(context.Background(), cmd.cmd); !errors.Is(err, cmd.want) {
			t.Errorf("%s: err = %v, want %v", name, err, cmd.want)
		}
	}
	if len(f.repo.reserved) != 0 {
		t.Errorf("the store was asked %d times", len(f.repo.reserved))
	}
}

func TestTheStoresRefusalsReachTheCallerAsTheyAre(t *testing.T) {
	f := newFixture(t)
	for _, want := range []error{domain.ErrPaused, &domain.Exhausted{Scope: domain.ScopeTeacher}, &domain.Exhausted{Scope: domain.ScopePool}} {
		f.repo.reserveErr = want
		_, err := f.app.Commands.Reserve.Handle(context.Background(), command.Reserve{Actor: actor.Actor{ID: teacherID}, Feature: domain.FeatureScore, Ceiling: 30})
		if err != want {
			t.Errorf("err = %v, want %v", err, want)
		}
	}
}

func TestReleaseStaleReturnsWhatIsOlderThanTenMinutes(t *testing.T) {
	f := newFixture(t)
	released, err := f.app.Commands.ReleaseStale.Handle(context.Background(), command.ReleaseStale{})
	if err != nil || released.Reservations != 2 || released.Credits != 300 {
		t.Fatalf("released %+v, %v", released, err)
	}
	if want := september.Add(-10 * time.Minute); !f.repo.staleSince[0].Equal(want) {
		t.Errorf("cutoff %v, want %v", f.repo.staleSince[0], want)
	}
}

func TestARequestIsTrimmedBoundedAndTellsEveryManager(t *testing.T) {
	f := newFixture(t)
	f.repo.filedOut = domain.Filed{
		Request:     domain.Request{ID: "q-1", UserID: teacherID, Amount: 20_000, Status: domain.RequestPending},
		TeacherName: "Nguyễn Thị Lan",
		Managers:    []string{"m-1", "m-2"},
	}
	request, err := f.app.Commands.RequestCredits.Handle(context.Background(), command.RequestCredits{
		Actor: actor.Actor{ID: teacherID}, Amount: 20_000, Reason: "  Chuẩn bị đề thi cuối kỳ cho ba lớp.\n",
	})
	if err != nil || request.ID != "q-1" {
		t.Fatalf("request %+v, %v", request, err)
	}
	if got := f.repo.filed[0]; got.Reason != "Chuẩn bị đề thi cuối kỳ cho ba lớp." || got.Month.Format(time.DateOnly) != "2090-09-01" || got.UserID != teacherID {
		t.Errorf("recorded %+v", got)
	}
	if len(f.notifier.sent) != 2 {
		t.Fatalf("%d notices, want one for each manager", len(f.notifier.sent))
	}
	for i, id := range []string{"m-1", "m-2"} {
		n := f.notifier.sent[i]
		params, _ := n.Params.(notificationsdomain.CreditRequest)
		if n.UserID != id || n.Kind != notificationsdomain.AICreditRequest || n.DedupeKey != "ai_request:q-1" || params.TeacherName != "Nguyễn Thị Lan" || params.Credits != 20_000 {
			t.Errorf("notice %d: %+v", i, n)
		}
	}
}

func TestARequestOutsideTheBoundsIsRefusedWithEveryFieldAtOnce(t *testing.T) {
	f := newFixture(t)
	long := strings.Repeat("ệ", domain.MaxReasonLength+1)
	for name, c := range map[string]struct {
		amount int64
		reason string
		fields []string
	}{
		"too little and no reason": {domain.MinRequest - 1, " \t", []string{"amount", "reason"}},
		"too much":                 {domain.MaxRequest + 1, "đề thi", []string{"amount"}},
		"a reason over the limit":  {domain.MinRequest, long, []string{"reason"}},
	} {
		_, err := f.app.Commands.RequestCredits.Handle(context.Background(), command.RequestCredits{Actor: actor.Actor{ID: teacherID}, Amount: c.amount, Reason: c.reason})
		if got := refusedFields(t, err); strings.Join(got, ",") != strings.Join(c.fields, ",") {
			t.Errorf("%s: refused %v, want %v", name, got, c.fields)
		}
	}
	if _, err := f.app.Commands.RequestCredits.Handle(context.Background(), command.RequestCredits{Actor: actor.Actor{ID: teacherID}, Amount: domain.MaxRequest, Reason: strings.Repeat("ệ", domain.MaxReasonLength)}); err != nil {
		t.Errorf("a request at both limits: %v", err)
	}
	if len(f.repo.filed) != 1 {
		t.Errorf("%d requests reached the store, want only the one at the limits", len(f.repo.filed))
	}
}

func TestARefusedRequestTellsNobody(t *testing.T) {
	f := newFixture(t)
	f.repo.filedErr = domain.ErrRequestPending
	if _, err := f.app.Commands.RequestCredits.Handle(context.Background(), command.RequestCredits{Actor: actor.Actor{ID: teacherID}, Amount: 5_000, Reason: "x"}); !errors.Is(err, domain.ErrRequestPending) {
		t.Fatalf("err = %v", err)
	}
	if len(f.notifier.sent) != 0 {
		t.Errorf("%d notices for a request that was not recorded", len(f.notifier.sent))
	}
}

func TestADecisionTellsTheTeacherHowItWentAndOnlyOnceItIsWritten(t *testing.T) {
	for name, approve := range map[string]bool{"approved": true, "declined": false} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			status := domain.RequestDeclined
			if approve {
				status = domain.RequestApproved
			}
			f.repo.decidedOut = domain.Decided{Request: domain.Request{ID: "q-1", UserID: teacherID, Amount: 20_000, Status: status}, TeacherName: "Lan"}
			_, err := f.app.Commands.DecideRequest.Handle(context.Background(), command.DecideRequest{
				By: domain.Author{UserID: "a-1"}, ID: "0198f5f0-0000-7000-8000-0000000000aa", Approve: approve,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := f.repo.decisions[0]; got.Approve != approve || got.Month.Format(time.DateOnly) != "2090-09-01" {
				t.Errorf("decision %+v", got)
			}
			n := f.notifier.sent[0]
			params, _ := n.Params.(notificationsdomain.CreditRequestDecided)
			if n.UserID != teacherID || n.Kind != notificationsdomain.AICreditRequestDecided || n.DedupeKey != "ai_decision:q-1" || params.Decision != name || params.Credits != 20_000 {
				t.Errorf("notice %+v", n)
			}
		})
	}

	f := newFixture(t)
	f.repo.decidedErr = domain.ErrRequestDecided
	if _, err := f.app.Commands.DecideRequest.Handle(context.Background(), command.DecideRequest{By: domain.Author{UserID: "a-1"}, ID: "0198f5f0-0000-7000-8000-0000000000aa", Approve: true}); !errors.Is(err, domain.ErrRequestDecided) {
		t.Errorf("err = %v", err)
	}
	if len(f.notifier.sent) != 0 {
		t.Error("a decision that was refused told the teacher")
	}
}

func TestADecisionNeedsARequestIdAndAConsoleNamesItsOperator(t *testing.T) {
	f := newFixture(t)
	if _, err := f.app.Commands.DecideRequest.Handle(context.Background(), command.DecideRequest{By: domain.Author{UserID: "a-1"}, ID: "not-a-uuid"}); !errors.Is(err, domain.ErrRequestNotFound) {
		t.Errorf("a malformed id: %v", err)
	}
	if _, err := f.app.Commands.DecideRequest.Handle(context.Background(), command.DecideRequest{By: domain.Author{}, ID: "0198f5f0-0000-7000-8000-0000000000aa"}); err == nil {
		t.Error("the console decided without naming its operator")
	}
	if len(f.repo.decisions) != 0 {
		t.Errorf("the store was asked %d times", len(f.repo.decisions))
	}
}

func TestSetAllowanceTakesOneChangeAtATimeWithTheFiguresItNeeds(t *testing.T) {
	f := newFixture(t)
	yes, no := true, false
	by := domain.Author{UserID: "a-1"}
	for name, c := range map[string]struct {
		cmd    command.SetAllowance
		fields []string
	}{
		"nothing to change":         {command.SetAllowance{By: by, UserID: teacherID}, []string{"custom"}},
		"custom with no figure":     {command.SetAllowance{By: by, UserID: teacherID, Custom: &yes}, []string{"credits"}},
		"custom below the least":    {command.SetAllowance{By: by, UserID: teacherID, Custom: &yes, Credits: domain.MinAllowance - 1}, []string{"credits"}},
		"custom above the most":     {command.SetAllowance{By: by, UserID: teacherID, Custom: &yes, Credits: domain.MaxAllowance + 1}, []string{"credits"}},
		"the default with a figure": {command.SetAllowance{By: by, UserID: teacherID, Custom: &no, Credits: 5_000}, []string{"credits"}},
		"a pause with a figure":     {command.SetAllowance{By: by, UserID: teacherID, Paused: &yes, Credits: 5_000}, []string{"credits"}},
	} {
		_, err := f.app.Commands.SetAllowance.Handle(context.Background(), c.cmd)
		if got := refusedFields(t, err); strings.Join(got, ",") != strings.Join(c.fields, ",") {
			t.Errorf("%s: refused %v, want %v", name, got, c.fields)
		}
	}
	if _, err := f.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: by, UserID: "nobody"}); !errors.Is(err, domain.ErrNotATeacher) {
		t.Errorf("a malformed id: %v", err)
	}
	if len(f.repo.allowances) != 0 {
		t.Fatalf("the store was asked %d times", len(f.repo.allowances))
	}

	f.repo.currentRule = domain.Rules{WarnPercent: 80}
	month, err := f.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: by, UserID: teacherID, Custom: &yes, Credits: 30_000, Paused: &no})
	if err != nil {
		t.Fatal(err)
	}
	got := f.repo.allowances[0]
	if *got.Custom != true || got.Credits != 30_000 || *got.Paused != false || got.Month.Format(time.DateOnly) != "2090-09-01" {
		t.Errorf("stored %+v", got)
	}
	if month.Status != domain.StatusNearLimit {
		t.Errorf("status %s, want the standing against the warning share (16,000 of 20,000 at 80%%)", month.Status)
	}
}

func TestACommandFromTheConsoleNamesItsOperator(t *testing.T) {
	f := newFixture(t)
	yes := true
	if _, err := f.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{UserID: teacherID, Paused: &yes}); err == nil {
		t.Error("set-allowance ran without an operator")
	}
	if _, err := f.app.Commands.AddCredits.Handle(context.Background(), command.AddCredits{Credits: 1}); err == nil {
		t.Error("add-credits ran without an operator")
	}
	limit := domain.LimitPause
	if _, err := f.app.Commands.SetRules.Handle(context.Background(), command.SetRules{OnTeacherLimit: &limit}); err == nil {
		t.Error("set-rules ran without an operator")
	}
}

func TestAddCreditsIsBoundedAndNamesTheApprover(t *testing.T) {
	f := newFixture(t)
	by := domain.Author{UserID: "a-1", IP: "203.0.113.4"}
	for _, bad := range []int64{0, -1, domain.MaxAddition + 1} {
		_, err := f.app.Commands.AddCredits.Handle(context.Background(), command.AddCredits{By: by, Credits: bad})
		if got := refusedFields(t, err); len(got) != 1 || got[0] != "credits" {
			t.Errorf("%d: refused %v", bad, got)
		}
	}
	pool, err := f.app.Commands.AddCredits.Handle(context.Background(), command.AddCredits{By: by, Credits: 50_000})
	if err != nil || pool.Added != 50_000 {
		t.Fatalf("pool %+v, %v", pool, err)
	}
	if got := f.repo.additions[0]; got.By.UserID != by.UserID || got.By.IP != by.IP || got.Credits != 50_000 || got.Month.Format(time.DateOnly) != "2090-09-01" {
		t.Errorf("stored %+v", got)
	}
}

func TestSetRulesChangesAtLeastOneRuleWithinTheBounds(t *testing.T) {
	f := newFixture(t)
	by := domain.Author{UserID: "a-1"}
	low, high := domain.MinAllowance-1, domain.MaxAllowance+1
	warnLow, warnHigh := domain.MinWarnPercent-1, domain.MaxWarnPercent+1
	bad := domain.Limit("ignore")
	for name, c := range map[string]struct {
		cmd    command.SetRules
		fields []string
	}{
		"nothing":                {command.SetRules{By: by}, []string{"defaultAllowance"}},
		"an allowance too small": {command.SetRules{By: by, DefaultAllowance: &low}, []string{"defaultAllowance"}},
		"an allowance too large": {command.SetRules{By: by, DefaultAllowance: &high}, []string{"defaultAllowance"}},
		"a warning too early":    {command.SetRules{By: by, WarnPercent: &warnLow}, []string{"warnPercent"}},
		"a warning too late":     {command.SetRules{By: by, WarnPercent: &warnHigh}, []string{"warnPercent"}},
		"a rule nobody wrote":    {command.SetRules{By: by, OnTeacherLimit: &bad}, []string{"onTeacherLimit"}},
		"three things wrong":     {command.SetRules{By: by, DefaultAllowance: &low, WarnPercent: &warnHigh, OnTeacherLimit: &bad}, []string{"defaultAllowance", "warnPercent", "onTeacherLimit"}},
	} {
		_, err := f.app.Commands.SetRules.Handle(context.Background(), c.cmd)
		if got := refusedFields(t, err); strings.Join(got, ",") != strings.Join(c.fields, ",") {
			t.Errorf("%s: refused %v, want %v", name, got, c.fields)
		}
	}
	allowance, warn, limit := int64(50_000), 90, domain.LimitPause
	if _, err := f.app.Commands.SetRules.Handle(context.Background(), command.SetRules{By: by, DefaultAllowance: &allowance, WarnPercent: &warn, OnTeacherLimit: &limit}); err != nil {
		t.Fatal(err)
	}
	if got := f.repo.rules[0]; *got.DefaultAllowance != 50_000 || *got.WarnPercent != 90 || *got.OnTeacherLimit != domain.LimitPause || got.Month.Format(time.DateOnly) != "2090-09-01" {
		t.Errorf("stored %+v", got)
	}
}

func TestTheAdminViewProjectsTheMonthFromTheClock(t *testing.T) {
	f := newFixture(t)
	f.repo.adminOut = domain.AdminCredits{Pool: domain.Pool{Plan: 200_000, Used: 120_000}}
	view, err := f.app.Queries.AdminCredits.Handle(context.Background(), query.AdminCredits{})
	if err != nil {
		t.Fatal(err)
	}
	if view.DaysToGo != 6 || view.OnPace != 150_000 || !view.WithinPool || view.Month.Format(time.DateOnly) != "2090-09-01" {
		t.Errorf("view %+v", view)
	}
	f.repo.adminOut = domain.AdminCredits{Pool: domain.Pool{Plan: 200_000, Added: 10_000, Used: 180_000}}
	view, _ = f.app.Queries.AdminCredits.Handle(context.Background(), query.AdminCredits{})
	if view.OnPace != 225_000 || view.WithinPool {
		t.Errorf("a month on course for 225,000 of 210,000 reads as within the pool: %+v", view)
	}
	f.repo.adminOut = domain.AdminCredits{Pool: domain.Pool{Plan: 200_000, Added: 25_000, Used: 180_000}}
	view, _ = f.app.Queries.AdminCredits.Handle(context.Background(), query.AdminCredits{})
	if !view.WithinPool {
		t.Errorf("a month on course for exactly the total reads as over: %+v", view)
	}
}

func TestTheUsageQueryNeedsAUser(t *testing.T) {
	f := newFixture(t)
	if _, err := f.app.Queries.Usage.Handle(context.Background(), query.Usage{}); !errors.Is(err, domain.ErrNotATeacher) {
		t.Errorf("err = %v", err)
	}
	usage, err := f.app.Queries.Usage.Handle(context.Background(), query.Usage{UserID: teacherID})
	if err != nil || usage.Month.Format(time.DateOnly) != "2090-09-01" {
		t.Errorf("usage %+v, %v", usage, err)
	}
}

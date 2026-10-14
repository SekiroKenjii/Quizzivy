package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	aiquery "quizzivy/internal/modules/ai/application/query"
	aidomain "quizzivy/internal/modules/ai/domain"
	auditquery "quizzivy/internal/modules/audit/application/query"
	auditdomain "quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/modules/dashboard/application"
	"quizzivy/internal/modules/dashboard/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	organizationquery "quizzivy/internal/modules/organization/application/query"
	"quizzivy/internal/shared/cqrs"
)

type adminStore struct {
	store
	accounts    domain.Accounts
	mustChange  int
	activeSince int
	days        []domain.ActiveDay
	classes     domain.ClassCounts
	orphaned    domain.DisabledTeacherClasses
	held        domain.Held

	asked      []string
	accountsAt time.Time
	sinceDay   time.Time
	from, to   time.Time
}

func (s *adminStore) Accounts(_ context.Context, since time.Time) (domain.Accounts, error) {
	s.asked = append(s.asked, "Accounts")
	s.accountsAt = since
	return s.accounts, nil
}

func (s *adminStore) MustChangePassword(context.Context) (int, error) {
	s.asked = append(s.asked, "MustChangePassword")
	return s.mustChange, nil
}

func (s *adminStore) ActiveSince(_ context.Context, day time.Time) (int, error) {
	s.asked = append(s.asked, "ActiveSince")
	s.sinceDay = day
	return s.activeSince, nil
}

func (s *adminStore) ActiveByDay(_ context.Context, from, to time.Time) ([]domain.ActiveDay, error) {
	s.asked = append(s.asked, "ActiveByDay")
	s.from, s.to = from, to
	return s.days, nil
}

func (s *adminStore) ClassCounts(context.Context) (domain.ClassCounts, error) {
	s.asked = append(s.asked, "ClassCounts")
	return s.classes, nil
}

func (s *adminStore) DisabledTeacherClasses(context.Context) (domain.DisabledTeacherClasses, error) {
	s.asked = append(s.asked, "DisabledTeacherClasses")
	return s.orphaned, nil
}

func (s *adminStore) Held(context.Context) (domain.Held, error) {
	s.asked = append(s.asked, "Held")
	return s.held, nil
}

type ports struct {
	recentLimit int
	recent      []auditdomain.Entry
	credits     aidomain.AdminCredits
	creditsRead int
}

func (p *ports) branding() cqrs.HandlerFunc[organizationquery.Branding, organizationmodel.Branding] {
	return func(context.Context, organizationquery.Branding) (organizationmodel.Branding, error) {
		return organizationmodel.Branding{Name: "Trung tâm Ánh Dương"}, nil
	}
}

func (p *ports) log() cqrs.HandlerFunc[auditquery.Recent, []auditdomain.Entry] {
	return func(_ context.Context, q auditquery.Recent) ([]auditdomain.Entry, error) {
		p.recentLimit = q.Limit
		return p.recent, nil
	}
}

func (p *ports) month() cqrs.HandlerFunc[aiquery.AdminCredits, aidomain.AdminCredits] {
	return func(context.Context, aiquery.AdminCredits) (aidomain.AdminCredits, error) {
		p.creditsRead++
		return p.credits, nil
	}
}

func entry(id int64) auditdomain.Entry {
	return auditdomain.Entry{Record: auditdomain.Record{ID: id, Action: "test.publish", EntityType: "test"}}
}

var (
	marchFourth = time.Date(2026, time.March, 4, 5, 0, 0, 0, time.UTC)
	everyKey    = domain.AdminViewer{UserID: "admin", Users: true, Audit: true, Settings: true, Credits: true, All: true}
)

func centre() (*adminStore, *ports) {
	oldest := time.Date(2026, time.January, 9, 3, 0, 0, 0, time.UTC)
	repo := &adminStore{
		accounts: domain.Accounts{
			Total: 100, Enabled: 95, Students: 83, StudentsNew: 5, Teachers: 15, TeachersDisabled: 2,
			MustChangePassword: 3, NeverSignedIn: 4, OldestNeverSignedIn: &oldest,
		},
		mustChange:  3,
		activeSince: 42,
		days: []domain.ActiveDay{
			{Date: time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC), Students: 20, Staff: 4},
			{Date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), Students: 30, Staff: 6},
		},
		classes:  domain.ClassCounts{Active: 6, Archived: 2},
		orphaned: domain.DisabledTeacherClasses{Count: 2, ClassName: "Lớp 9A", TeacherName: "Cô Lan"},
		held:     domain.Held{MediaBytes: 900, ImportBytes: 100, ImportCount: 3, Attempts: 12},
	}
	return repo, &ports{
		recent: []auditdomain.Entry{entry(2), entry(1)},
		credits: aidomain.AdminCredits{
			Pool:     aidomain.Pool{Plan: 1000, Added: 500, Used: 300},
			Requests: []aidomain.PendingRequest{{TeacherName: "Thầy Nam", Used: 90, Allowance: 100}},
		},
	}
}

func admin(repo *adminStore, p *ports, plan domain.Plan) *application.Application {
	app := application.New(repo).WithAdmin(p.branding(), p.log(), plan, nil).WithCredits(p.month())
	app.SetClock(func() time.Time { return marchFourth })
	return app
}

var wholePlan = domain.Plan{Name: "Trung tâm", SeatLimit: 500, TeacherLimit: 20, MediaBytes: 1000, ImportBytes: 10_000}

func kinds(rows []domain.Attention) []domain.AttentionKind {
	out := make([]domain.AttentionKind, len(rows))
	for i, r := range rows {
		out[i] = r.Kind
	}
	return out
}

func TestTheOverviewOfAnAdminHoldingEveryKeyFillsEverySection(t *testing.T) {
	repo, p := centre()
	got, err := admin(repo, p, wholePlan).Queries.AdminOverview.Handle(context.Background(), query.AdminOverview{Viewer: everyKey})
	if err != nil {
		t.Fatal(err)
	}
	if got.OrganizationName != "Trung tâm Ánh Dương" {
		t.Errorf("organization = %q", got.OrganizationName)
	}
	wantPeople := domain.People{Total: 100, ActiveThisWeek: 42, Teachers: 15, TeachersDisabled: 2, Students: 83, StudentsNewThisMonth: 5}
	if got.People == nil || *got.People != wantPeople {
		t.Errorf("people = %+v, want %+v", got.People, wantPeople)
	}
	if got.Classes == nil || *got.Classes != (domain.ClassCounts{Active: 6, Archived: 2}) {
		t.Errorf("classes = %+v", got.Classes)
	}
	if got.DailyActive == nil || len(got.DailyActive.Days) != domain.ActiveDays {
		t.Fatalf("daily active = %+v", got.DailyActive)
	}
	if last := got.DailyActive.Days[len(got.DailyActive.Days)-1]; !last.Date.Equal(time.Date(2026, time.March, 4, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("the chart ends on %v, want today", last.Date)
	}
	if len(got.RecentChanges) != 2 || p.recentLimit != domain.RecentChanges {
		t.Errorf("recent changes = %d entries asked for %d, want 2 entries asked for %d", len(got.RecentChanges), p.recentLimit, domain.RecentChanges)
	}
	if got.Storage == nil || got.Storage.Seats.Used != 95 || got.Storage.Teachers.Used != 15 || got.Storage.AttemptsKept != 12 {
		t.Errorf("storage = %+v", got.Storage)
	}
	if got.Storage.AICredits == nil || *got.Storage.AICredits != (domain.Credits{Used: 300, Pool: 1500}) {
		t.Errorf("AI credits = %+v, want 300 of 1500", got.Storage.AICredits)
	}
	wantKinds := []domain.AttentionKind{
		domain.AttentionCreditRequests, domain.AttentionMustChange, domain.AttentionNeverSignedIn,
		domain.AttentionDisabledTeachers, domain.AttentionStorage,
	}
	if !slices.Equal(kinds(got.Attention), wantKinds) {
		t.Errorf("attention = %v, want %v", kinds(got.Attention), wantKinds)
	}
	counts := map[domain.AttentionKind]int{}
	for _, r := range got.Attention {
		counts[r.Kind] = r.Count
	}
	if counts[domain.AttentionCreditRequests] != 1 || counts[domain.AttentionMustChange] != 3 || counts[domain.AttentionNeverSignedIn] != 4 ||
		counts[domain.AttentionDisabledTeachers] != 2 || counts[domain.AttentionStorage] != 1 {
		t.Errorf("attention counts = %v", counts)
	}
	never := got.Attention[2].Detail
	if never == nil || never.OldestCreatedAt == nil || !never.OldestCreatedAt.Equal(*repo.accounts.OldestNeverSignedIn) {
		t.Errorf("never signed in detail = %+v, want the oldest account's date", never)
	}
	orphaned := got.Attention[3].Detail
	if orphaned == nil || orphaned.ClassName == nil || *orphaned.ClassName != "Lớp 9A" || orphaned.TeacherName == nil || *orphaned.TeacherName != "Cô Lan" {
		t.Errorf("disabled teacher detail = %+v", orphaned)
	}
}

func TestTheFourteenDayWindowAndTheWeekCrossAMonthBoundary(t *testing.T) {
	repo, p := centre()
	if _, err := admin(repo, p, wholePlan).Queries.AdminOverview.Handle(context.Background(), query.AdminOverview{Viewer: everyKey}); err != nil {
		t.Fatal(err)
	}
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }
	if !repo.from.Equal(day(time.February, 19)) || !repo.to.Equal(day(time.March, 4)) {
		t.Errorf("the chart read %v..%v, want 19 February..4 March: today and the 13 days before it", repo.from, repo.to)
	}
	if !repo.sinceDay.Equal(day(time.February, 26)) {
		t.Errorf("this week began %v, want 26 February: today and the 6 days before it", repo.sinceDay)
	}
	if want := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.FixedZone(domain.DefaultZone, 7*60*60)); !repo.accountsAt.Equal(want) {
		t.Errorf("new students counted from %v, want the first of the month at the centre's midnight, %v", repo.accountsAt, want)
	}
}

func TestADayIsTheCentresDayNotUTCs(t *testing.T) {
	repo, p := centre()
	app := admin(repo, p, wholePlan)
	app.SetClock(func() time.Time { return time.Date(2026, time.February, 28, 18, 30, 0, 0, time.UTC) })
	if _, err := app.Queries.AdminOverview.Handle(context.Background(), query.AdminOverview{Viewer: everyKey}); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC); !repo.to.Equal(want) {
		t.Errorf("today = %v, want 1 March: 01:30 there while it is still 28 February in UTC", repo.to)
	}
	if want := time.Date(2026, time.February, 16, 0, 0, 0, 0, time.UTC); !repo.from.Equal(want) {
		t.Errorf("the chart begins %v, want 16 February", repo.from)
	}
	if want := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.FixedZone(domain.DefaultZone, 7*60*60)); !repo.accountsAt.Equal(want) {
		t.Errorf("the month began %v, want %v", repo.accountsAt, want)
	}
}

func TestAConfiguredZoneMovesTheDay(t *testing.T) {
	zone, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	repo, p := centre()
	app := application.New(repo).WithAdmin(p.branding(), p.log(), wholePlan, zone)
	app.SetClock(func() time.Time { return time.Date(2026, time.March, 1, 3, 0, 0, 0, time.UTC) })
	if _, err := app.Queries.AdminOverview.Handle(context.Background(), query.AdminOverview{Viewer: everyKey}); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC); !repo.to.Equal(want) {
		t.Errorf("today = %v, want 28 February: the evening before in Los Angeles", repo.to)
	}
}

func TestACustomRoleHoldingOnlyTheAuditKeySeesOnlyTheRecentChanges(t *testing.T) {
	repo, p := centre()
	got, err := admin(repo, p, wholePlan).Queries.AdminOverview.Handle(context.Background(), query.AdminOverview{Viewer: domain.AdminViewer{UserID: "auditor", Audit: true}})
	if err != nil {
		t.Fatal(err)
	}
	if got.People != nil || got.Classes != nil || got.DailyActive != nil || got.Storage != nil {
		t.Errorf("sections the keys do not open were filled: %+v", got)
	}
	if got.Attention == nil || len(got.Attention) != 0 {
		t.Errorf("attention = %#v, want an empty list", got.Attention)
	}
	if len(got.RecentChanges) != 2 {
		t.Errorf("recent changes = %d, want 2", len(got.RecentChanges))
	}
	if len(repo.asked) != 0 || p.creditsRead != 0 {
		t.Errorf("a withheld section was still read: repository %v, credits %d times", repo.asked, p.creditsRead)
	}
}

func TestEachKeyOpensItsOwnSectionAndNothingElse(t *testing.T) {
	type opens struct{ people, daily, classes, recent, storage bool }
	cases := []struct {
		name   string
		viewer domain.AdminViewer
		want   opens
		rows   []domain.AttentionKind
	}{
		{"people.users.manage", domain.AdminViewer{Users: true}, opens{people: true, daily: true},
			[]domain.AttentionKind{domain.AttentionMustChange, domain.AttentionNeverSignedIn}},
		{"scope.all", domain.AdminViewer{All: true}, opens{classes: true}, []domain.AttentionKind{domain.AttentionDisabledTeachers}},
		{"system.audit.read", domain.AdminViewer{Audit: true}, opens{recent: true}, nil},
		{"system.settings.write", domain.AdminViewer{Settings: true}, opens{storage: true}, []domain.AttentionKind{domain.AttentionStorage}},
		{"ai.credits.manage", domain.AdminViewer{Credits: true}, opens{}, []domain.AttentionKind{domain.AttentionCreditRequests}},
		{"no key", domain.AdminViewer{}, opens{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo, p := centre()
			got, err := admin(repo, p, wholePlan).Queries.AdminOverview.Handle(context.Background(), query.AdminOverview{Viewer: c.viewer})
			if err != nil {
				t.Fatal(err)
			}
			have := opens{got.People != nil, got.DailyActive != nil, got.Classes != nil, got.RecentChanges != nil, got.Storage != nil}
			if have != c.want {
				t.Errorf("sections filled = %+v, want %+v", have, c.want)
			}
			if !slices.Equal(kinds(got.Attention), c.rows) {
				t.Errorf("attention = %v, want %v", kinds(got.Attention), c.rows)
			}
		})
	}
}

func TestAnAttentionRowWithNothingToCountIsLeftOut(t *testing.T) {
	repo, p := centre()
	repo.accounts.MustChangePassword, repo.accounts.NeverSignedIn, repo.accounts.OldestNeverSignedIn = 0, 0, nil
	repo.orphaned = domain.DisabledTeacherClasses{}
	repo.held = domain.Held{MediaBytes: 799, ImportBytes: 1}
	p.credits.Requests = nil
	got, err := admin(repo, p, wholePlan).Queries.AdminOverview.Handle(context.Background(), query.AdminOverview{Viewer: everyKey})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Attention) != 0 {
		t.Errorf("attention = %v, want none: every count is zero and storage is under 80%%", kinds(got.Attention))
	}
	if got.Storage == nil {
		t.Error("the storage section is the usage, not an alert: it stays")
	}
}

func TestAnEmptyAuditLogIsAnEmptyListNotAWithheldSection(t *testing.T) {
	repo, p := centre()
	p.recent = nil
	got, err := admin(repo, p, wholePlan).Queries.AdminOverview.Handle(context.Background(), query.AdminOverview{Viewer: domain.AdminViewer{Audit: true}})
	if err != nil {
		t.Fatal(err)
	}
	if got.RecentChanges == nil || len(got.RecentChanges) != 0 {
		t.Errorf("recent changes = %#v, want an empty, non-nil list", got.RecentChanges)
	}
}

func TestWithoutTheAIModuleNeitherAIFigureAppears(t *testing.T) {
	repo, p := centre()
	app := application.New(repo).WithAdmin(p.branding(), p.log(), wholePlan, nil)
	app.SetClock(func() time.Time { return marchFourth })
	got, err := app.Queries.AdminOverview.Handle(context.Background(), query.AdminOverview{Viewer: everyKey})
	if err != nil {
		t.Fatal(err)
	}
	if got.Storage == nil || got.Storage.AICredits != nil {
		t.Errorf("AI credits = %+v, want none while the assistant is off", got.Storage)
	}
	for _, kind := range kinds(got.Attention) {
		if kind == domain.AttentionCreditRequests {
			t.Error("a credit-request row appeared with no AI module")
		}
	}
	usage, err := app.Queries.Usage.Handle(context.Background(), query.Usage{Viewer: everyKey})
	if err != nil {
		t.Fatal(err)
	}
	if usage.AICredits != nil {
		t.Errorf("usage AI credits = %+v, want none", usage.AICredits)
	}
}

func TestTheCreditsAreNotReadForAViewerWithoutTheKey(t *testing.T) {
	repo, p := centre()
	viewer := everyKey
	viewer.Credits = false
	app := admin(repo, p, wholePlan)
	overview, err := app.Queries.AdminOverview.Handle(context.Background(), query.AdminOverview{Viewer: viewer})
	if err != nil {
		t.Fatal(err)
	}
	usage, err := app.Queries.Usage.Handle(context.Background(), query.Usage{Viewer: viewer})
	if err != nil {
		t.Fatal(err)
	}
	if p.creditsRead != 0 {
		t.Errorf("the AI module was asked %d times for a viewer without ai.credits.manage", p.creditsRead)
	}
	if overview.Storage.AICredits != nil || usage.AICredits != nil {
		t.Errorf("AI credits shown without the key: %+v %+v", overview.Storage.AICredits, usage.AICredits)
	}
}

func TestTheOverviewNeedsTheCentresName(t *testing.T) {
	repo, _ := centre()
	app := application.New(repo)
	if _, err := app.Queries.AdminOverview.Handle(context.Background(), query.AdminOverview{Viewer: everyKey}); !errors.Is(err, domain.ErrAdminUnavailable) {
		t.Errorf("error = %v, want ErrAdminUnavailable", err)
	}
}

func TestTheSummaryCountsTemporaryPasswordsOnlyForWhoManagesPeople(t *testing.T) {
	repo, p := centre()
	app := admin(repo, p, wholePlan)
	with, err := app.Queries.AdminSummary.Handle(context.Background(), query.AdminSummary{Viewer: domain.AdminViewer{Users: true}})
	if err != nil {
		t.Fatal(err)
	}
	if with.MustChangePassword == nil || *with.MustChangePassword != 3 {
		t.Errorf("must change password = %v, want 3", with.MustChangePassword)
	}
	repo.asked = nil
	without, err := app.Queries.AdminSummary.Handle(context.Background(), query.AdminSummary{Viewer: domain.AdminViewer{Audit: true, Settings: true, All: true, Credits: true}})
	if err != nil {
		t.Fatal(err)
	}
	if without.MustChangePassword != nil || len(repo.asked) != 0 {
		t.Errorf("without people.users.manage the count is %v after asking %v, want null and no query", without.MustChangePassword, repo.asked)
	}
}

func TestUsageWithEveryPlanValueUnsetStatesNoPlanAndNoLimit(t *testing.T) {
	repo, p := centre()
	got, err := admin(repo, p, domain.Plan{}).Queries.Usage.Handle(context.Background(), query.Usage{Viewer: domain.AdminViewer{Settings: true}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Plan != nil || got.Seats.Limit != nil || got.Teachers.Limit != nil || got.Media.Limit != nil || got.Imports.Limit != nil {
		t.Errorf("usage = %+v, want no plan and no limit", got)
	}
	if got.Seats.Used != 95 || got.Teachers.Used != 15 || got.Media.Bytes != 900 || got.Imports.Bytes != 100 || got.Imports.Count != 3 || got.AttemptsKept != 12 {
		t.Errorf("figures = %+v, want what is held even with no plan", got)
	}
}

func TestUsageSetsTheFiguresAgainstThePlan(t *testing.T) {
	repo, p := centre()
	plan := wholePlan
	plan.RenewsOn = time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	got, err := admin(repo, p, plan).Queries.Usage.Handle(context.Background(), query.Usage{Viewer: everyKey})
	if err != nil {
		t.Fatal(err)
	}
	if got.Plan == nil || got.Plan.Name != "Trung tâm" || got.Plan.RenewsOn == nil || !got.Plan.RenewsOn.Equal(plan.RenewsOn) {
		t.Errorf("plan = %+v", got.Plan)
	}
	if got.Seats.Limit == nil || *got.Seats.Limit != 500 || got.Teachers.Limit == nil || *got.Teachers.Limit != 20 ||
		got.Media.Limit == nil || *got.Media.Limit != 1000 || got.Imports.Limit == nil || *got.Imports.Limit != 10_000 {
		t.Errorf("limits = %+v", got)
	}
	if got.AICredits == nil || *got.AICredits != (domain.Credits{Used: 300, Pool: 1500}) {
		t.Errorf("AI credits = %+v, want 300 of 1500", got.AICredits)
	}
}

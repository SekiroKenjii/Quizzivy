package domain_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/shared/actor"
)

var listPrices = domain.Prices{
	Fast:    domain.Price{Input: 100_000, Output: 500_000, CacheRead: 10_000},
	Capable: domain.Price{Input: 2_000_000, Output: 10_000_000, CacheRead: 200_000},
}

func TestACreditIsATenthOfACentOfListPriceRoundedUp(t *testing.T) {
	capable := listPrices.Capable
	maximum := int64(1 << 31)
	for name, c := range map[string]struct {
		tokens domain.Tokens
		want   int64
	}{
		"nothing costs nothing":                {domain.Tokens{}, 0},
		"one input token costs a whole credit": {domain.Tokens{Input: 1}, 1},
		"500 input tokens are exactly one":     {domain.Tokens{Input: 500}, 1},
		"501 input tokens spill into a second": {domain.Tokens{Input: 501}, 2},
		"output costs five times input":        {domain.Tokens{Output: 100}, 1},
		"cache reads cost a tenth of input":    {domain.Tokens{CacheRead: 5_000}, 1},
		"cache writes are charged as input":    {domain.Tokens{CacheWrite: 501}, 2},
		"the four add before they round":       {domain.Tokens{Input: 250, Output: 50, CacheRead: 2_500, CacheWrite: 250}, 2},
		"a generation, 3k in and 3.5k out":     {domain.Tokens{Input: 3_000, Output: 3_500}, 41},
		"a check, 12k in and 2.5k out":         {domain.Tokens{Input: 12_000, Output: 2_500}, 49},
		"a variant, 10k in and 4k out":         {domain.Tokens{Input: 10_000, Output: 4_000}, 60},
		"a page of a scan, 2k in and 1.2k out": {domain.Tokens{Input: 2_000, Output: 1_200}, 16},
		"a score suggestion, 0.8k in 0.3k out": {domain.Tokens{Input: 800, Output: 300}, 5},
		"explaining results, 3k in and 1k out": {domain.Tokens{Input: 3_000, Output: 1_000}, 16},
		"an explanation, 0.6k in and 0.2k out": {domain.Tokens{Input: 600, Output: 200}, 4},
		"the most the provider could report":   {domain.Tokens{Input: maximum, Output: maximum, CacheRead: maximum, CacheWrite: maximum}, (maximum*14_200_000 + 999_999_999) / 1_000_000_000},
	} {
		got, err := capable.Credits(c.tokens)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: %d credits, want %d", name, got, c.want)
		}
	}
}

func TestTheFastTierIsPricedByItsOwnList(t *testing.T) {
	price, err := listPrices.For(domain.TierFast)
	if err != nil {
		t.Fatal(err)
	}
	got, err := price.Credits(domain.Tokens{Input: 90_000, Output: 8_000})
	if err != nil || got != 13 {
		t.Errorf("a clean-up chunk costs %d credits (%v), want 13", got, err)
	}
	if _, err := listPrices.For("slow"); err == nil {
		t.Error("a tier nobody priced was priced")
	}
}

func TestACountTheProviderCannotHaveReportedIsRefused(t *testing.T) {
	price := listPrices.Capable
	for name, tokens := range map[string]domain.Tokens{
		"negative input":       {Input: -1},
		"negative output":      {Output: -1},
		"negative cache read":  {CacheRead: -1},
		"negative cache write": {CacheWrite: -1},
		"more than 2^31":       {Input: 1<<31 + 1},
	} {
		if _, err := price.Credits(tokens); err == nil {
			t.Errorf("%s was priced", name)
		}
	}
}

func TestAMonthIsCountedInTheCentresZone(t *testing.T) {
	saigon := time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	for name, c := range map[string]struct {
		at   time.Time
		want string
	}{
		"the last second of March in Saigon":  {time.Date(2090, 3, 31, 16, 59, 59, 0, time.UTC), "2090-03-01"},
		"the first second of April in Saigon": {time.Date(2090, 3, 31, 17, 0, 0, 0, time.UTC), "2090-04-01"},
		"a New Year in Saigon":                {time.Date(2090, 12, 31, 17, 0, 0, 0, time.UTC), "2091-01-01"},
		"mid month":                           {time.Date(2090, 6, 15, 3, 0, 0, 0, time.UTC), "2090-06-01"},
	} {
		if got := domain.MonthOf(c.at, saigon).Format(time.DateOnly); got != c.want {
			t.Errorf("%s: month %s, want %s", name, got, c.want)
		}
		if got := domain.MonthOf(c.at, saigon); got.Location() != time.UTC || got.Hour() != 0 {
			t.Errorf("%s: the key %v is not a date at UTC midnight", name, got)
		}
	}
}

func TestDaysToGoAndTheProjectionCountTodayAsElapsed(t *testing.T) {
	saigon := time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	day24 := time.Date(2026, 9, 24, 5, 0, 0, 0, saigon)
	if got := domain.DaysToGo(day24, saigon); got != 6 {
		t.Errorf("days to go on 24 September = %d, want 6", got)
	}
	if got := domain.OnPace(120_000, day24, saigon); got != 150_000 {
		t.Errorf("on pace = %d, want 120,000 over 24 days of 30 = 150,000", got)
	}
	first := time.Date(2026, 9, 1, 0, 0, 1, 0, saigon)
	if got := domain.OnPace(1_000, first, saigon); got != 30_000 {
		t.Errorf("on the first day, on pace = %d, want 30,000", got)
	}
	if got := domain.OnPace(7, time.Date(2026, 2, 10, 12, 0, 0, 0, saigon), saigon); got != 20 {
		t.Errorf("a projection of 7 over 10 of 28 days = %d, want it rounded up to 20", got)
	}
	lastDay := time.Date(2026, 9, 30, 23, 0, 0, 0, saigon)
	if got := domain.DaysToGo(lastDay, saigon); got != 0 {
		t.Errorf("days to go on the last day = %d", got)
	}
}

func TestAPoolSpendsWhatWasPlannedAndAddedAndNeverShowsLessThanNothing(t *testing.T) {
	pool := domain.Pool{Plan: 200_000, Added: 50_000, Used: 100_000, Reserved: 1_000}
	if pool.Total() != 250_000 || pool.Left() != 149_000 {
		t.Errorf("total %d, left %d", pool.Total(), pool.Left())
	}
	if (domain.Pool{Plan: 10, Used: 9, Reserved: 5}).Left() != 0 {
		t.Error("a pool over-held shows credits left")
	}
}

func TestAPausedTeacherIsPausedBeforeAnythingElseAndAtTheLimitBeforeNearIt(t *testing.T) {
	for name, c := range map[string]struct {
		used, allowance int64
		warn            int
		paused          bool
		want            domain.TeacherStatus
	}{
		"paused beats at the limit": {20_000, 20_000, 80, true, domain.StatusPaused},
		"unused":                    {0, 20_000, 80, false, domain.StatusOnTrack},
		"just below the warning":    {15_999, 20_000, 80, false, domain.StatusOnTrack},
		"at the warning":            {16_000, 20_000, 80, false, domain.StatusNearLimit},
		"a lower warning share":     {16_000, 20_000, 90, false, domain.StatusOnTrack},
		"one below the allowance":   {19_999, 20_000, 80, false, domain.StatusNearLimit},
		"at the allowance":          {20_000, 20_000, 80, false, domain.StatusAtLimit},
	} {
		if got := domain.StatusOf(c.used, c.allowance, c.warn, c.paused); got != c.want {
			t.Errorf("%s: %s, want %s", name, got, c.want)
		}
	}
}

func TestEveryFeatureHasACategoryAndACeilingFromThePlan(t *testing.T) {
	want := map[domain.Feature]struct {
		category domain.Category
		ceiling  int64
	}{
		domain.FeatureGenerate:    {domain.CategoryGeneration, 200},
		domain.FeatureVariant:     {domain.CategoryGeneration, 300},
		domain.FeatureScore:       {domain.CategoryGrading, 30},
		domain.FeatureExplanation: {domain.CategoryExplanations, 20},
		domain.FeatureDistractors: {domain.CategoryExplanations, 20},
		domain.FeatureImport:      {domain.CategoryImports, 60},
		domain.FeatureCheck:       {domain.CategoryChecks, 300},
		domain.FeatureInsights:    {domain.CategoryChecks, 60},
		domain.FeatureCleanup:     {domain.CategoryChecks, 200},
	}
	features := domain.Features()
	if len(features) != len(want) {
		t.Fatalf("%d features, want %d: %v", len(features), len(want), features)
	}
	for _, f := range features {
		c, ok := want[f]
		if !ok {
			t.Errorf("%s is not in the plan's table", f)
			continue
		}
		if !f.Known() || f.Category() != c.category || f.Ceiling() != c.ceiling {
			t.Errorf("%s: known %v, category %s, ceiling %d; want %s and %d", f, f.Known(), f.Category(), f.Ceiling(), c.category, c.ceiling)
		}
		if f.Ceiling() > domain.MaxCeiling {
			t.Errorf("%s reserves more than a reservation may hold", f)
		}
	}
	if domain.Feature("words").Known() || domain.Feature("words").Ceiling() != 0 || domain.Feature("words").Category() != "" {
		t.Error("an unknown feature is known")
	}
	if got := domain.Categories(); !slices.Equal(got, []domain.Category{
		domain.CategoryGeneration, domain.CategoryGrading, domain.CategoryExplanations, domain.CategoryImports, domain.CategoryChecks,
	}) {
		t.Errorf("the usage card's rows are %v", got)
	}
}

func TestACategoryListIsACopy(t *testing.T) {
	first := domain.Categories()
	first[0] = "changed"
	if domain.Categories()[0] == "changed" {
		t.Error("changing a result changed the next one")
	}
}

func TestTheConsoleActsAsSystemAndASignedInUserAsThemselves(t *testing.T) {
	user := domain.AuthorOf(actor.Actor{ID: "u-1", IP: "203.0.113.9", UserAgent: "Firefox"})
	if user.System() || user.UserID != "u-1" || user.IP != "203.0.113.9" || user.UserAgent != "Firefox" {
		t.Errorf("a signed-in author is %+v", user)
	}
	if !(domain.Author{Operator: "thuong"}).System() {
		t.Error("an author with no user is not System")
	}
}

func TestExhaustedSaysWhichBudgetAndIsTheSentinel(t *testing.T) {
	var err error = &domain.Exhausted{Scope: domain.ScopePool}
	if !errors.Is(err, domain.ErrExhausted) {
		t.Error("errors.Is(err, ErrExhausted) is false")
	}
	if err.Error() == "" {
		t.Error("an empty message")
	}
}

func TestChangedKeepsWhatTheInputDoesNotName(t *testing.T) {
	rules := domain.Rules{PoolCredits: 200_000, DefaultAllowance: 20_000, WarnPercent: 80, OnTeacherLimit: domain.LimitRequest}
	allowance, warn, limit := int64(50_000), 90, domain.LimitPause
	for name, c := range map[string]struct {
		in   domain.SetRulesInput
		want domain.Rules
	}{
		"nothing":   {domain.SetRulesInput{}, rules},
		"allowance": {domain.SetRulesInput{DefaultAllowance: &allowance}, domain.Rules{PoolCredits: 200_000, DefaultAllowance: 50_000, WarnPercent: 80, OnTeacherLimit: domain.LimitRequest}},
		"warning":   {domain.SetRulesInput{WarnPercent: &warn}, domain.Rules{PoolCredits: 200_000, DefaultAllowance: 20_000, WarnPercent: 90, OnTeacherLimit: domain.LimitRequest}},
		"the limit": {domain.SetRulesInput{OnTeacherLimit: &limit}, domain.Rules{PoolCredits: 200_000, DefaultAllowance: 20_000, WarnPercent: 80, OnTeacherLimit: domain.LimitPause}},
		"all three": {domain.SetRulesInput{DefaultAllowance: &allowance, WarnPercent: &warn, OnTeacherLimit: &limit}, domain.Rules{PoolCredits: 200_000, DefaultAllowance: 50_000, WarnPercent: 90, OnTeacherLimit: domain.LimitPause}},
	} {
		if got := rules.Changed(c.in); got != c.want {
			t.Errorf("%s: %+v, want %+v", name, got, c.want)
		}
	}
	if rules.DefaultAllowance != 20_000 {
		t.Error("Changed modified its receiver")
	}
}

package domain_test

import (
	"testing"
	"time"

	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/limits"
)

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func TestLocalDayIsTheCalendarDateInTheCentresZone(t *testing.T) {
	zone, err := time.LoadLocation(domain.DefaultZone)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		at   time.Time
		want time.Time
	}{
		{"late evening UTC is already tomorrow in Ho Chi Minh City", time.Date(2026, time.October, 31, 18, 30, 0, 0, time.UTC), date(2026, time.November, 1)},
		{"early morning UTC is the same day", time.Date(2026, time.November, 1, 2, 0, 0, 0, time.UTC), date(2026, time.November, 1)},
		{"the year turns with the zone, not with UTC", time.Date(2026, time.December, 31, 17, 0, 0, 0, time.UTC), date(2027, time.January, 1)},
	}
	for _, c := range cases {
		if got := domain.LocalDay(c.at, zone); !got.Equal(c.want) {
			t.Errorf("%s: LocalDay(%v) = %v, want %v", c.name, c.at, got, c.want)
		}
	}
}

func TestDailyActiveCoversFourteenDaysOldestFirstAcrossAMonthBoundary(t *testing.T) {
	today := date(2026, time.March, 4)
	got := domain.NewDailyActive(today, []domain.ActiveDay{
		{Date: date(2026, time.February, 20), Students: 3, Staff: 1},
		{Date: date(2026, time.February, 28), Students: 5, Staff: 2},
		{Date: date(2026, time.March, 1), Students: 4, Staff: 2},
		{Date: today, Students: 1},
		{Date: date(2026, time.February, 18), Students: 99, Staff: 99},
	})
	if len(got.Days) != domain.ActiveDays {
		t.Fatalf("days = %d, want %d", len(got.Days), domain.ActiveDays)
	}
	if first := got.Days[0].Date; !first.Equal(date(2026, time.February, 19)) {
		t.Errorf("first day = %v, want 19 February: today and the 13 days before it", first)
	}
	if last := got.Days[len(got.Days)-1].Date; !last.Equal(today) {
		t.Errorf("last day = %v, want today", last)
	}
	for i := 1; i < len(got.Days); i++ {
		if !got.Days[i].Date.Equal(got.Days[i-1].Date.AddDate(0, 0, 1)) {
			t.Fatalf("day %d = %v does not follow %v: a month's end must not skip or repeat a date", i, got.Days[i].Date, got.Days[i-1].Date)
		}
	}
	for _, d := range got.Days {
		if d.Date.Equal(date(2026, time.March, 1)) && (d.Students != 4 || d.Staff != 2) {
			t.Errorf("1 March = %+v, want 4 students and 2 staff", d)
		}
		if d.Date.Equal(date(2026, time.February, 28)) && (d.Students != 5 || d.Staff != 2) {
			t.Errorf("28 February = %+v, want 5 students and 2 staff", d)
		}
		if d.Date.Equal(date(2026, time.March, 2)) && (d.Students != 0 || d.Staff != 0) {
			t.Errorf("2 March has no row and must count zero, got %+v", d)
		}
	}
	for _, d := range got.Days {
		if d.Students == 99 {
			t.Errorf("%v is outside the 14 days and leaked into the chart", d.Date)
		}
	}
}

func TestDailyActiveAveragesStudentsAndStaffRoundedHalfUp(t *testing.T) {
	today := date(2026, time.March, 4)
	cases := []struct {
		name  string
		total int
		want  int
	}{
		{"nobody is zero", 0, 0},
		{"6 over 14 days rounds down", 6, 0},
		{"7 over 14 days is exactly half and rounds up", 7, 1},
		{"20 over 14 days is 1.43", 20, 1},
		{"21 over 14 days is 1.5 and rounds up", 21, 2},
	}
	for _, c := range cases {
		got := domain.NewDailyActive(today, []domain.ActiveDay{{Date: today, Students: c.total - c.total/2, Staff: c.total / 2}})
		if got.Average != c.want {
			t.Errorf("%s: average = %d, want %d", c.name, got.Average, c.want)
		}
	}
	if empty := domain.NewDailyActive(today, nil); len(empty.Days) != domain.ActiveDays || empty.Average != 0 {
		t.Errorf("no rows = %+v, want 14 empty days and an average of 0", empty)
	}
}

func TestPercentRoundsHalfUp(t *testing.T) {
	cases := []struct {
		used, limit int64
		want        int
	}{
		{0, 100, 0}, {1, 3, 33}, {2, 3, 67}, {1, 200, 1}, {79, 100, 79}, {100, 100, 100}, {150, 100, 150},
	}
	for _, c := range cases {
		if got := domain.Percent(c.used, c.limit); got != c.want {
			t.Errorf("Percent(%d, %d) = %d, want %d", c.used, c.limit, got, c.want)
		}
	}
}

func limit(n int64) *int64 { return &n }

func TestUsageWithoutLimitsStatesNoLimit(t *testing.T) {
	held := domain.Held{Seats: 37, Teachers: 6, MediaBytes: 10, ImportBytes: 20, ExportBytes: 5, ImportCount: 2, Attempts: 7}
	got := domain.NewUsage(limits.Limits{}, held, nil)
	if got.Seats.Limit != nil || got.Teachers.Limit != nil || got.Storage.Limit != nil || got.Media.PerTeacherLimit != nil ||
		got.Imports.Limit != nil || got.Email.DailyLimit != nil || got.Database.Limit != nil {
		t.Errorf("an unset limit must be absent, got %+v", got)
	}
	if got.Seats.Used != 37 || got.Teachers.Used != 6 || got.Media.Bytes != 10 || got.Imports.Bytes != 20 || got.Imports.Count != 2 || got.AttemptsKept != 7 {
		t.Errorf("figures = %+v", got)
	}
	if got.Storage.Bytes != 35 {
		t.Errorf("storage = %d, want media, import documents and exports together (35)", got.Storage.Bytes)
	}
	if got.WarnAtPercent != 80 {
		t.Errorf("warn at = %d, want the default 80", got.WarnAtPercent)
	}
	if got.AICredits != nil {
		t.Errorf("AI credits = %+v, want none when none were read", got.AICredits)
	}
	if row := domain.StorageAttention(got); row != nil {
		t.Errorf("storage row = %+v, want none without a limit to measure against", row)
	}
}

func TestUsageStatesTheResolvedLimits(t *testing.T) {
	resolved := limits.Resolve(limits.Stored{limits.Seats: 500, limits.Teachers: 20, limits.WarnAtPercent: 90}, limits.Env{})
	credits := &domain.Credits{Used: 120, Pool: 500}
	got := domain.NewUsage(resolved, domain.Held{Seats: 11, Teachers: 3}, credits)

	if got.Seats.Limit == nil || *got.Seats.Limit != 500 || got.Teachers.Limit == nil || *got.Teachers.Limit != 20 {
		t.Errorf("seat and teacher limits = %+v %+v", got.Seats, got.Teachers)
	}
	if got.Storage.Limit == nil || *got.Storage.Limit != limits.R2FreeStorageBytes {
		t.Errorf("storage limit = %v, want the R2 free tier", got.Storage.Limit)
	}
	if got.Email.DailyLimit == nil || *got.Email.DailyLimit != limits.ResendFreeEmailsPerDay ||
		got.Email.MonthlyLimit == nil || *got.Email.MonthlyLimit != limits.ResendFreeEmailsPerMonth {
		t.Errorf("email limits = %+v, want the Resend free tier", got.Email)
	}
	if got.WarnAtPercent != 90 {
		t.Errorf("warn at = %d, want the stored 90", got.WarnAtPercent)
	}
	if got.AICredits == nil || *got.AICredits != *credits {
		t.Errorf("AI credits = %+v, want %+v", got.AICredits, credits)
	}
}

func TestStorageNeedsAttentionFromTheMeterReadingTheWarningLine(t *testing.T) {
	usage := func(library, open int64) domain.Usage {
		return domain.Usage{
			Media:         domain.Media{LargestLibrary: library, PerTeacherLimit: limit(1000)},
			Imports:       domain.Imports{OpenBytes: open, Limit: limit(1000)},
			WarnAtPercent: 80,
		}
	}
	if row := domain.StorageAttention(usage(794, 0)); row != nil {
		t.Errorf("79.4%% used reads 79 on the meter = %+v, want no row", row)
	}
	shown := domain.StorageAttention(usage(795, 0))
	if shown == nil || shown.Detail == nil || shown.Detail.MediaPercent == nil || *shown.Detail.MediaPercent != 80 {
		t.Errorf("79.5%% used reads 80 on the meter, so the row must come with it, got %+v", shown)
	}
	row := domain.StorageAttention(usage(800, 0))
	if row == nil || row.Kind != domain.AttentionStorage || row.Count != 1 {
		t.Fatalf("80%% used = %+v, want one storage row", row)
	}
	if row.Detail.ImportsPercent == nil || *row.Detail.ImportsPercent != 0 {
		t.Errorf("imports percent = %+v, want 0: a meter with a limit is quoted even below the line", row.Detail.ImportsPercent)
	}
	if both := domain.StorageAttention(usage(1000, 900)); both == nil || both.Count != 2 {
		t.Errorf("both meters over the line = %+v, want a count of 2", both)
	}

	raised := usage(850, 0)
	raised.WarnAtPercent = 90
	if row := domain.StorageAttention(raised); row != nil {
		t.Errorf("85%% under a 90%% line = %+v, want no row", row)
	}

	whole := domain.StorageAttention(domain.Usage{
		Storage:       domain.Bytes{Bytes: 950, Limit: limit(1000)},
		Database:      domain.Bytes{Bytes: 900, Limit: limit(1000)},
		Media:         domain.Media{LargestLibrary: 5000},
		WarnAtPercent: 80,
	})
	if whole == nil || whole.Count != 2 || whole.Detail.StoragePercent == nil || *whole.Detail.StoragePercent != 95 ||
		whole.Detail.DatabasePercent == nil || *whole.Detail.DatabasePercent != 90 || whole.Detail.MediaPercent != nil {
		t.Errorf("file storage and database over the line, a library without a limit = %+v", whole)
	}
}

func TestHeldMailIsOneRowNamingWhenItGoes(t *testing.T) {
	if row := domain.EmailAttention(domain.Usage{}); row != nil {
		t.Errorf("nothing held = %+v, want no row", row)
	}
	until := date(2026, time.November, 1)
	row := domain.EmailAttention(domain.Usage{Email: domain.Email{Held: 4, HeldUntil: &until}})
	if row == nil || row.Kind != domain.AttentionEmailHeld || row.Count != 4 || row.Detail.HeldUntil == nil || !row.Detail.HeldUntil.Equal(until) {
		t.Errorf("four held = %+v", row)
	}
}

func TestCreditRequestsAreOneRowNamingTheOldest(t *testing.T) {
	if row := (domain.CreditsView{}).Attention(); row != nil {
		t.Errorf("no request waiting = %+v, want no row", row)
	}
	view := domain.CreditsView{
		Credits: domain.Credits{Used: 10, Pool: 100},
		Waiting: []domain.WaitingRequest{
			{TeacherName: "Cô Lan", Used: 480, Allowance: 500},
			{TeacherName: "Thầy Nam", Used: 50, Allowance: 50},
		},
	}
	row := view.Attention()
	if row == nil || row.Kind != domain.AttentionCreditRequests || row.Count != 2 {
		t.Fatalf("row = %+v, want ai_credit_requests with a count of 2", row)
	}
	d := row.Detail
	if d == nil || d.TeacherName == nil || *d.TeacherName != "Cô Lan" || d.Used == nil || *d.Used != 480 || d.Allowance == nil || *d.Allowance != 500 {
		t.Errorf("detail = %+v, want the oldest request's teacher and standing", d)
	}
}

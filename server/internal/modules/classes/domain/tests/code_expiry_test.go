package domain_test

import (
	"testing"
	"time"

	"quizzivy/internal/modules/classes/domain"
)

func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("zone %s: %v", name, err)
	}
	return loc
}

func TestACodeWorksThroughTheLastSecondOfItsLastDayInTheIssuersZone(t *testing.T) {
	ict, jst, berlin := zone(t, "Asia/Ho_Chi_Minh"), zone(t, "Asia/Tokyo"), zone(t, "Europe/Berlin")
	for name, c := range map[string]struct {
		now  time.Time
		days int
		zone *time.Location
		want time.Time
	}{
		"a zone without daylight saving, in the morning": {
			time.Date(2026, 10, 10, 9, 0, 0, 0, ict), 30, ict, time.Date(2026, 11, 9, 23, 59, 59, 0, ict),
		},
		"a zone without daylight saving, one second before midnight": {
			time.Date(2026, 10, 10, 23, 59, 58, 0, ict), 1, ict, time.Date(2026, 10, 11, 23, 59, 59, 0, ict),
		},
		"a zone without daylight saving, at midnight": {
			time.Date(2026, 10, 11, 0, 0, 0, 0, ict), 1, ict, time.Date(2026, 10, 12, 23, 59, 59, 0, ict),
		},
		"UTC+9 at 23:30 local, the same date in UTC": {
			time.Date(2026, 10, 10, 23, 30, 0, 0, jst), 7, jst, time.Date(2026, 10, 17, 23, 59, 59, 0, jst),
		},
		"UTC+9 at 00:30 local, still the day before in UTC": {
			time.Date(2026, 10, 11, 0, 30, 0, 0, jst), 7, jst, time.Date(2026, 10, 18, 23, 59, 59, 0, jst),
		},
		"Berlin across the March change": {
			time.Date(2027, 3, 20, 12, 0, 0, 0, berlin), 14, berlin, time.Date(2027, 4, 3, 23, 59, 59, 0, berlin),
		},
		"Berlin ending on the day the clocks change": {
			time.Date(2027, 3, 21, 8, 0, 0, 0, berlin), 7, berlin, time.Date(2027, 3, 28, 23, 59, 59, 0, berlin),
		},
		"Berlin across the October change": {
			time.Date(2026, 10, 20, 22, 0, 0, 0, berlin), 90, berlin, time.Date(2027, 1, 18, 23, 59, 59, 0, berlin),
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := domain.CodeExpiry(c.now, c.days, c.zone)
			if !got.Equal(c.want) {
				t.Errorf("expires %s, want %s", got.In(c.zone), c.want)
			}
			if got.In(c.zone).Hour() != 23 || got.In(c.zone).Minute() != 59 || got.In(c.zone).Second() != 59 {
				t.Errorf("expires at %s, want 23:59:59 local", got.In(c.zone))
			}
		})
	}
}

func TestTheSameInstantExpiresOnTheIssuersDateNotUTCs(t *testing.T) {
	now := time.Date(2026, 10, 10, 15, 30, 0, 0, time.UTC)
	jst, hcm := zone(t, "Asia/Tokyo"), zone(t, "Asia/Ho_Chi_Minh")
	if got, want := domain.CodeExpiry(now, 7, jst), time.Date(2026, 10, 18, 23, 59, 59, 0, jst); !got.Equal(want) {
		t.Errorf("at 00:30 on the 11th in Tokyo the code expires %s, want %s", got.In(jst), want.In(jst))
	}
	if got, want := domain.CodeExpiry(now, 7, hcm), time.Date(2026, 10, 17, 23, 59, 59, 0, hcm); !got.Equal(want) {
		t.Errorf("at 22:30 on the 10th in Ho Chi Minh City the code expires %s, want %s", got.In(hcm), want.In(hcm))
	}
}

func TestAZoneThatCannotBeReadFallsBackToHoChiMinhCity(t *testing.T) {
	hcm := zone(t, domain.DefaultZone)
	now := time.Date(2026, 10, 10, 20, 0, 0, 0, time.UTC)
	for _, name := range []string{"", "Local", "Mars/Olympus_Mons", "not a zone"} {
		got := domain.CodeExpiry(now, 3, domain.ZoneOrDefault(name))
		if want := time.Date(2026, 10, 14, 23, 59, 59, 0, hcm); !got.Equal(want) {
			t.Errorf("zone %q: expires %s, want %s", name, got.In(hcm), want.In(hcm))
		}
	}
	if got := domain.ZoneOrDefault("Asia/Tokyo").String(); got != "Asia/Tokyo" {
		t.Errorf("a known zone resolved to %s", got)
	}
}

func TestAScheduleLabelAndARoomAreStoredTrimmedOrNotAtAll(t *testing.T) {
	text := func(s string) *string { return &s }
	for name, c := range map[string]struct {
		in   *string
		want *string
	}{
		"not sent":              {nil, nil},
		"empty":                 {text(""), nil},
		"spaces":                {text("   "), nil},
		"tabs and a no-break":   {text("\t \n"), nil},
		"padded":                {text("  Thứ 3, 5 · 18:00 \n"), text("Thứ 3, 5 · 18:00")},
		"inner spaces are kept": {text("Phòng  A1"), text("Phòng  A1")},
	} {
		got := domain.LabelOf(c.in)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%s: stored %v, want %v", name, got, c.want)
		}
	}
}

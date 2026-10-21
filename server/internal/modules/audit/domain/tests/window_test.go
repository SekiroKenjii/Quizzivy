package domain_test

import (
	"errors"
	"testing"
	"time"

	"quizzivy/internal/modules/audit/domain"
)

var vietnam = domain.LocationOf(domain.DefaultZone)

func at(year int, month time.Month, day, hour, minute int, loc *time.Location) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, loc)
}

func TestEachRangeStartsAtTheMidnightOfItsFirstDayInTheViewersZone(t *testing.T) {
	now := at(2001, time.March, 15, 12, 30, vietnam)
	cases := []struct {
		kind domain.Range
		want time.Time
	}{
		{domain.Today, at(2001, time.March, 15, 0, 0, vietnam)},
		{domain.Week, at(2001, time.March, 9, 0, 0, vietnam)},
		{domain.Month, at(2001, time.February, 14, 0, 0, vietnam)},
		{domain.Year, at(2001, time.January, 1, 0, 0, vietnam)},
		{"", at(2001, time.March, 9, 0, 0, vietnam)},
	}
	for _, c := range cases {
		got, err := domain.ResolveWindow(c.kind, "", "", vietnam, now)
		if err != nil {
			t.Fatalf("%q: %v", c.kind, err)
		}
		if got.From == nil || !got.From.Equal(c.want) || got.To != nil {
			t.Errorf("%q reads from %v to %v, want from %v and no end", c.kind, got.From, got.To, c.want)
		}
	}
}

func TestADayIsTheViewersAndNotUTCs(t *testing.T) {
	utc := time.FixedZone("UTC", 0)
	losAngeles := domain.LocationOf("America/Los_Angeles")
	instant := at(2001, time.March, 15, 20, 0, utc)

	inUTC, _ := domain.ResolveWindow(domain.Today, "", "", utc, instant)
	inLA, _ := domain.ResolveWindow(domain.Today, "", "", losAngeles, instant)

	if !inUTC.From.Equal(at(2001, time.March, 15, 0, 0, utc)) {
		t.Errorf("UTC today starts %v", inUTC.From)
	}
	if !inLA.From.Equal(at(2001, time.March, 15, 0, 0, losAngeles)) || inLA.From.Equal(*inUTC.From) {
		t.Errorf("a viewer in Los Angeles has today from %v, which is not their own midnight", inLA.From)
	}
}

func TestAllHasNoBounds(t *testing.T) {
	got, err := domain.ResolveWindow(domain.Forever, "", "", vietnam, time.Now())
	if err != nil || got.From != nil || got.To != nil {
		t.Errorf("all reads %+v, %v, want no bounds", got, err)
	}
}

func TestACustomRangeIncludesBothDaysWhole(t *testing.T) {
	got, err := domain.ResolveWindow(domain.Custom, "2001-03-09", "2001-03-14", vietnam, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !got.From.Equal(at(2001, time.March, 9, 0, 0, vietnam)) {
		t.Errorf("from %v", got.From)
	}
	if !got.To.Equal(at(2001, time.March, 15, 0, 0, vietnam)) {
		t.Errorf("to %v, want the midnight after the 14th so that the whole day is in", got.To)
	}

	one, err := domain.ResolveWindow(domain.Custom, "2001-03-15", "2001-03-15", vietnam, time.Now())
	if err != nil || one.To.Sub(*one.From) != 24*time.Hour {
		t.Errorf("a single day reads %+v, %v, want exactly one day", one, err)
	}
}

func TestACustomRangeAcrossTheEndOfAMonthAndAYear(t *testing.T) {
	got, err := domain.ResolveWindow(domain.Custom, "2000-12-31", "2000-12-31", vietnam, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !got.To.Equal(at(2001, time.January, 1, 0, 0, vietnam)) {
		t.Errorf("the 31st of December ends at %v", got.To)
	}
}

func TestTheRefusalsOfADateRangeNameTheField(t *testing.T) {
	cases := []struct {
		name     string
		kind     domain.Range
		from, to string
		field    string
		problem  domain.RangeProblem
	}{
		{"from without custom", domain.Week, "2001-03-01", "", "from", domain.RangeNotCustom},
		{"to without custom", domain.Today, "", "2001-03-01", "to", domain.RangeNotCustom},
		{"both without custom", domain.Forever, "2001-03-01", "2001-03-02", "from", domain.RangeNotCustom},
		{"the default without custom", "", "", "2001-03-02", "to", domain.RangeNotCustom},
		{"custom without from", domain.Custom, "", "2001-03-02", "from", domain.RangeIncomplete},
		{"custom without to", domain.Custom, "2001-03-02", "", "to", domain.RangeIncomplete},
		{"custom without either", domain.Custom, "", "", "from", domain.RangeIncomplete},
		{"to before from", domain.Custom, "2001-03-02", "2001-03-01", "to", domain.RangeReversed},
		{"from that is not a date", domain.Custom, "2001-02-30", "2001-03-01", "from", domain.RangeMalformed},
		{"to that is not a date", domain.Custom, "2001-03-01", "03/04/2001", "to", domain.RangeMalformed},
	}
	for _, c := range cases {
		_, err := domain.ResolveWindow(c.kind, c.from, c.to, vietnam, time.Now())
		var refused *domain.RangeError
		if !errors.As(err, &refused) || refused.Field != c.field || refused.Problem != c.problem {
			t.Errorf("%s: %v, want a refusal of %s as %s", c.name, err, c.field, c.problem)
		}
	}
}

func TestARangeTheLogDoesNotOfferIsRefused(t *testing.T) {
	if _, err := domain.ResolveWindow("decade", "", "", vietnam, time.Now()); !errors.Is(err, domain.ErrUnknownRange) {
		t.Errorf("got %v, want ErrUnknownRange", err)
	}
}

func TestAZoneTheServerDoesNotKnowFallsBackToTheCentres(t *testing.T) {
	for _, zone := range []string{"", "Local", "Not/AZone"} {
		if got := domain.LocationOf(zone); got.String() != domain.DefaultZone {
			t.Errorf("zone %q gave %s, want %s", zone, got, domain.DefaultZone)
		}
	}
	if got := domain.LocationOf("Europe/Paris"); got.String() != "Europe/Paris" {
		t.Errorf("a known zone gave %s", got)
	}
}

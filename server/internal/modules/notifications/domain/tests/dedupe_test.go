package domain_test

import (
	"testing"
	"time"
	"unicode/utf8"

	"quizzivy/internal/modules/notifications/domain"
)

const (
	someClass   = "01935000-0000-7000-8000-00000000c1a5"
	someStudent = "01935000-0000-7000-8000-0000000000c3"
)

func TestPapersHandedInWithinFifteenMinutesShareAKeyAndTheNextQuarterDoesNot(t *testing.T) {
	quarter := time.Date(2026, 10, 10, 9, 15, 0, 0, time.UTC)
	same := domain.SubmittedKey(someAssignment, quarter)
	if got := domain.SubmittedKey(someAssignment, quarter.Add(14*time.Minute+59*time.Second)); got != same {
		t.Errorf("the last second of the quarter keys %q, want %q", got, same)
	}
	if got := domain.SubmittedKey(someAssignment, quarter.Add(15*time.Minute)); got == same {
		t.Errorf("the next quarter shares the key %q", got)
	}
	if got := domain.SubmittedKey(someAssignment, quarter.Add(-time.Second)); got == same {
		t.Errorf("the second before the quarter shares the key %q", got)
	}
	if want := "submitted:" + someAssignment + ":1990693"; same != want {
		t.Errorf("the key is %q, want the plan's submitted:{assignmentId}:{unix seconds / 900} = %q", same, want)
	}
	if other := domain.SubmittedKey(someAttempt, quarter); other == same {
		t.Error("two assignments share a key")
	}
}

func TestEveryKeyHasTheShapeItsProducerDocuments(t *testing.T) {
	closes := time.Date(2026, 10, 12, 8, 30, 0, 0, time.UTC)
	cases := map[string]string{
		domain.FlaggedKey(someAttempt):                             "flagged:" + someAttempt,
		domain.JoinedKey(someClass, someStudent):                   "joined:" + someClass + ":" + someStudent,
		domain.ExtendedKey(someAssignment):                         "extended:" + someAssignment,
		domain.OpenedKey(someAssignment):                           "opened:" + someAssignment,
		domain.ReadyKey(someAttempt):                               "ready:" + someAttempt,
		domain.ClosingKey(someAssignment, closes):                  "closing:" + someAssignment + ":1791793800",
		domain.DueSoonKey(someAssignment, closes, domain.LeadDay):  "due_soon:" + someAssignment + ":1791793800:24h",
		domain.DueSoonKey(someAssignment, closes, domain.LeadHour): "due_soon:" + someAssignment + ":1791793800:1h",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("key %q, want %q", got, want)
		}
		if n := utf8.RuneCountInString(got); n < 1 || n > domain.MaxDedupeKey {
			t.Errorf("key %q is %d characters, outside 1 to %d", got, n, domain.MaxDedupeKey)
		}
	}
}

func TestAMovedCloseIsAnotherThingToRemindAbout(t *testing.T) {
	closes := time.Date(2026, 10, 12, 8, 30, 0, 0, time.UTC)
	if domain.DueSoonKey(someAssignment, closes, domain.LeadHour) == domain.DueSoonKey(someAssignment, closes.Add(time.Hour), domain.LeadHour) {
		t.Error("a reminder for a moved close shares the old close's key")
	}
	if domain.ClosingKey(someAssignment, closes) == domain.ClosingKey(someAssignment, closes.Add(time.Hour)) {
		t.Error("a closing notice for a moved close shares the old close's key")
	}
	if domain.DueSoonKey(someAssignment, closes, domain.LeadDay) == domain.DueSoonKey(someAssignment, closes, domain.LeadHour) {
		t.Error("the day and the hour reminders share a key")
	}
	if domain.DueSoonKey(someAssignment, closes, domain.LeadHour) != domain.DueSoonKey(someAssignment, closes.Add(400*time.Millisecond), domain.LeadHour) {
		t.Error("the same second of the same close keys differently")
	}
}

func TestTheMaterialisingWindowsAreTheOnesThePlanStates(t *testing.T) {
	if domain.DueLookback != 7*24*time.Hour {
		t.Errorf("a due item is made up for %v, want seven days", domain.DueLookback)
	}
	if domain.DueEvery != 5*time.Minute {
		t.Errorf("a user's items materialise every %v, want five minutes", domain.DueEvery)
	}
	if domain.SubmittedBucket != 15*time.Minute {
		t.Errorf("papers merge in buckets of %v, want fifteen minutes", domain.SubmittedBucket)
	}
}

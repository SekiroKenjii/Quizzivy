package domain_test

import (
	"testing"

	"quizzivy/internal/modules/attempts/domain"
)

func TestSparseSequencesOrderOneSessionsEvents(t *testing.T) {
	events := []domain.IntegrityEvent{
		event(1, "paste", 900, "s1", seq(900000)),
		event(30, "window_blur", 10, "s1", seq(5000)),
		event(20, "window_focus", 82, "s1", seq(77000)),
	}
	got := domain.Timelines.Build(start, 3000, 0, events, at(1000))
	want := []string{"window_blur", "window_focus", "paste"}
	if len(got.Events) != len(want) {
		t.Fatalf("events = %d, want %d", len(got.Events), len(want))
	}
	for i, kind := range want {
		if got.Events[i].Kind != kind {
			t.Errorf("event %d = %s, want %s", i, got.Events[i].Kind, kind)
		}
	}
	if got.Events[0].DurationMs == nil || *got.Events[0].DurationMs != 72000 {
		t.Errorf("the blur's duration = %v, want 72000 ms", got.Events[0].DurationMs)
	}
	if got.Summary.AwayEpisodes != 1 || got.Summary.TotalAwayMs != 72000 || got.Summary.PasteCount != 1 {
		t.Errorf("summary = %+v, want one 72000 ms episode and one paste", got.Summary)
	}
}

func TestTwoTabsOfOneSessionAreOrderedByWhenTheyHappened(t *testing.T) {
	events := []domain.IntegrityEvent{
		event(1, "paste", 600, "s1", seq(600000)),
		event(4, "paste", 5, "s1", seq(5000)),
		event(2, "paste", 301, "s1", seq(301000)),
		event(3, "paste", 300, "s1", seq(300000)),
	}
	got := domain.Timelines.Build(start, 3000, 0, events, at(700))
	want := []int{5000, 300000, 301000, 600000}
	if len(got.Events) != len(want) {
		t.Fatalf("events = %d, want %d", len(got.Events), len(want))
	}
	for i, number := range want {
		if got.Events[i].ClientSeq == nil || *got.Events[i].ClientSeq != number {
			t.Errorf("event %d sequence = %v, want %d", i, got.Events[i].ClientSeq, number)
		}
		if got.Events[i].OffsetMs != number {
			t.Errorf("event %d offset = %d ms, want %d", i, got.Events[i].OffsetMs, number)
		}
	}
}

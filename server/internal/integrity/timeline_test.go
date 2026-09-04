package integrity_test

import (
	"testing"
	"time"

	"quizzivy/internal/integrity"
)

var start = time.Date(2026, 9, 4, 9, 48, 2, 0, time.UTC)

func at(seconds float64) time.Time {
	return start.Add(time.Duration(seconds * float64(time.Second)))
}

func seq(n int) *int { return &n }

func event(id int64, kind string, seconds float64, session string, clientSeq *int) integrity.Event {
	return integrity.Event{
		ID: id, Kind: kind, OccurredAt: at(seconds), ReceivedAt: at(seconds),
		ClientSeq: clientSeq, SessionID: session,
	}
}

func TestAnAwayEpisodeIsOneRowWithADurationHoweverManySignalsFired(t *testing.T) {
	events := []integrity.Event{
		event(1, "window_blur", 10, "s1", seq(0)),
		event(2, "tab_hidden", 10.2, "s1", seq(1)),
		event(3, "tab_visible", 82, "s1", seq(2)),
		event(4, "window_focus", 82.1, "s1", seq(3)),
	}
	got := integrity.Build(start, 3000, 0, events, at(200))

	if got.Events[0].DurationMs == nil || *got.Events[0].DurationMs != 72000 {
		t.Fatalf("the first leave should carry the episode's 72s, got %v", got.Events[0].DurationMs)
	}
	for _, e := range got.Events[1:] {
		if e.DurationMs != nil {
			t.Errorf("%s carries a duration; only the opener should", e.Kind)
		}
	}
	if got.Summary.AwayEpisodes != 1 || got.Summary.TotalAwayMs != 72000 {
		t.Errorf("summary %+v, want one 72s episode", got.Summary)
	}
}

func TestAnEpisodeUnderTheThresholdIsTimedButNotCounted(t *testing.T) {
	events := []integrity.Event{
		event(1, "window_blur", 10, "s1", seq(0)),
		event(2, "window_focus", 12, "s1", seq(1)),
	}
	got := integrity.Build(start, 3000, 0, events, at(200))
	if got.Summary.AwayEpisodes != 0 {
		t.Errorf("a 2s notification counted as an episode")
	}
	if got.Summary.TotalAwayMs != 2000 {
		t.Errorf("total %d, want 2000: the time is real even when it is not a strike", got.Summary.TotalAwayMs)
	}
}

func TestATrailingLeaveStaysOpenEndedAndIsCountedNotSummed(t *testing.T) {
	events := []integrity.Event{
		event(1, "window_blur", 10, "s1", seq(0)),
		event(2, "window_focus", 20, "s1", seq(1)),
		event(3, "tab_hidden", 100, "s1", seq(2)),
	}
	got := integrity.Build(start, 3000, 0, events, at(160))

	last := got.Events[len(got.Events)-1]
	if last.Kind != "tab_hidden" || last.DurationMs != nil {
		t.Fatalf("the trailing leave should be present with no duration, got %+v", last)
	}
	if got.Summary.AwayEpisodes != 2 {
		t.Errorf("episodes %d, want 2: the open one counts", got.Summary.AwayEpisodes)
	}
	if got.Summary.TotalAwayMs != 10000 {
		t.Errorf("total %d, want 10000: the open one is not summed", got.Summary.TotalAwayMs)
	}
}

// A resume restarts client_seq at 0 in a new session. The second session's
// events sort after the first's however its clock reads, and a leave in one
// session is closed by the return in the next.
func TestPairingCrossesAResumeBoundaryAndSessionsKeepTheirOwnOrder(t *testing.T) {
	events := []integrity.Event{
		event(1, "window_blur", 10, "s1", seq(0)),
		// Skewed clock: an earlier occurred_at than the event before it.
		event(2, "paste", 9, "s1", seq(1)),
		event(3, "session_takeover", 30, "s1", nil),
		event(4, "resume", 30, "s2", nil),
		event(5, "window_focus", 31, "s2", seq(0)),
	}
	got := integrity.Build(start, 3000, 0, events, at(200))

	kinds := make([]string, len(got.Events))
	for i, e := range got.Events {
		kinds[i] = e.Kind
	}
	want := []string{"window_blur", "paste", "session_takeover", "resume", "window_focus"}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("order %v, want %v", kinds, want)
		}
	}
	if got.Events[0].DurationMs == nil || *got.Events[0].DurationMs != 21000 {
		t.Errorf("the leave before the resume should close on the return after it, got %v", got.Events[0].DurationMs)
	}
	if got.Summary.ResumeCount != 1 || got.Summary.PasteCount != 1 {
		t.Errorf("summary %+v", got.Summary)
	}
}

func TestOfflineAndAudioPairByTheirOwnKindsAndOffsetsRunFromTheStart(t *testing.T) {
	q1, q2 := "q1", "q2"
	events := []integrity.Event{
		{ID: 1, Kind: "audio_play", OccurredAt: at(5), ReceivedAt: at(5), ClientSeq: seq(0), SessionID: "s1", QuestionID: &q1},
		{ID: 2, Kind: "audio_play", OccurredAt: at(6), ReceivedAt: at(6), ClientSeq: seq(1), SessionID: "s1", QuestionID: &q2},
		{ID: 3, Kind: "audio_ended", OccurredAt: at(16), ReceivedAt: at(16), ClientSeq: seq(2), SessionID: "s1", QuestionID: &q2},
		event(4, "network_offline", 20, "s1", seq(3)),
		event(5, "network_online", 68, "s1", seq(4)),
	}
	got := integrity.Build(start, 3000, 1, events, at(200))

	if got.Events[0].DurationMs != nil {
		t.Errorf("q1's play never ended and should stay open")
	}
	if got.Events[1].DurationMs == nil || *got.Events[1].DurationMs != 10000 {
		t.Errorf("q2's play lasted 10s, got %v", got.Events[1].DurationMs)
	}
	if got.Events[3].DurationMs == nil || *got.Events[3].DurationMs != 48000 {
		t.Errorf("the outage lasted 48s, got %v", got.Events[3].DurationMs)
	}
	if got.Events[3].OffsetMs != 20000 {
		t.Errorf("offset %d, want 20000", got.Events[3].OffsetMs)
	}
	if got.Summary.OfflineEpisodes != 1 || got.Summary.AudioReplays != 1 || got.Summary.AwayEpisodes != 0 {
		t.Errorf("summary %+v", got.Summary)
	}
}

func TestAnEmptyLogIsAnEmptyTimelineNotAPanic(t *testing.T) {
	got := integrity.Build(start, 3000, 0, nil, at(1))
	if len(got.Events) != 0 || got.Summary != (integrity.Summary{}) {
		t.Errorf("got %+v", got)
	}
}

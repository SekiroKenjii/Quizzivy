// Package integrity turns an attempt's event log into §10.4's timeline: paired
// durations, offsets from the start, and the summary strip. Counts, never
// verdicts -- the teacher judges; the app reports.
package integrity

import (
	"sort"
	"time"
)

// Event is one row of the log as the teacher sees it.
type Event struct {
	ID         int64
	Kind       string
	OccurredAt time.Time
	ReceivedAt time.Time
	// ClientSeq is nil for a server-written event (`resume`, `session_takeover`).
	ClientSeq  *int
	SessionID  string
	QuestionID *string
	Meta       []byte

	// OffsetMs is milliseconds since the attempt started, never negative.
	OffsetMs int
	// DurationMs is set on the event that opened a paired episode; nil on one
	// that never closed (§10.4).
	DurationMs *int
}

// Summary is the strip above the list.
type Summary struct {
	TotalAwayMs     int
	AwayEpisodes    int
	PasteCount      int
	ResumeCount     int
	AudioReplays    int
	OfflineEpisodes int
}

type Timeline struct {
	StartedAt time.Time
	Events    []Event
	Summary   Summary
}

// Away episodes follow the engine's own rule (useIntegrityMonitor): the first
// leave opens one, the first return closes it, whichever of the two signals
// fires first. Counting blur and hidden separately would report two absences
// for one alt-tab.
var (
	leaves  = map[string]bool{"tab_hidden": true, "window_blur": true}
	returns = map[string]bool{"tab_visible": true, "window_focus": true}
	// pairs are the other paired kinds, each closed by exactly one other kind.
	pairs = map[string]string{
		"fullscreen_exit": "fullscreen_enter",
		"network_offline": "network_online",
		"audio_play":      "audio_ended",
	}
)

// Build orders the log, pairs what pairs, and totals the strip.
//
// minAwayMs is the assignment's threshold: an episode shorter than it is not
// counted, exactly as the engine did not count it against the student.
// audioReplays comes from the plays table rather than the log, because the
// count that mattered was the server's (§11.4).
func Build(startedAt time.Time, minAwayMs, audioReplays int, events []Event, now time.Time) Timeline {
	order(events)
	for i := range events {
		events[i].OffsetMs = max(0, int(events[i].OccurredAt.Sub(startedAt)/time.Millisecond))
		events[i].DurationMs = nil
	}
	summary := tally(events, minAwayMs, now)
	summary.AudioReplays = audioReplays
	pairOthers(events)
	return Timeline{StartedAt: startedAt, Events: events, Summary: summary}
}

// tally pairs each leave with the next return, writing the duration onto the
// leave, and counts the rest of the strip as it goes.
func tally(events []Event, minAwayMs int, now time.Time) Summary {
	var summary Summary
	open := -1
	for i := range events {
		e := &events[i]
		switch {
		case leaves[e.Kind]:
			if open < 0 {
				open = i
			}
		case returns[e.Kind]:
			if open < 0 {
				continue
			}
			d := span(events[open].OccurredAt, e.OccurredAt)
			events[open].DurationMs = &d
			summary.TotalAwayMs += d
			if d >= minAwayMs {
				summary.AwayEpisodes++
			}
			open = -1
		case e.Kind == "paste":
			summary.PasteCount++
		case e.Kind == "resume":
			summary.ResumeCount++
		case e.Kind == "network_offline":
			summary.OfflineEpisodes++
		}
	}
	// Still away: counted, not summed (G-05b).
	if open >= 0 && span(events[open].OccurredAt, now) >= minAwayMs {
		summary.AwayEpisodes++
	}
	return summary
}

// pairOthers closes each opener with the next closer of its kind -- per
// question for audio, since two players can be open at once.
func pairOthers(events []Event) {
	open := map[string]int{}
	for i := range events {
		e := &events[i]
		if closer, ok := pairs[e.Kind]; ok {
			open[key(closer, e.QuestionID)] = i
			continue
		}
		if j, ok := open[key(e.Kind, e.QuestionID)]; ok {
			d := span(events[j].OccurredAt, e.OccurredAt)
			events[j].DurationMs = &d
			delete(open, key(e.Kind, e.QuestionID))
		}
	}
}

func key(kind string, questionID *string) string {
	if questionID == nil {
		return kind
	}
	return kind + ":" + *questionID
}

func span(from, to time.Time) int {
	return max(0, int(to.Sub(from)/time.Millisecond))
}

// order sorts sessions by when they began and, within a session, by
// `client_seq` -- so a skewed client clock cannot scramble a session's own
// order (§10.6). The server's `resume` opens a session and `session_takeover`
// closes one, so they sit at the ends.
func order(events []Event) {
	began := map[string]time.Time{}
	for _, e := range events {
		if first, ok := began[e.SessionID]; !ok || e.ReceivedAt.Before(first) {
			began[e.SessionID] = e.ReceivedAt
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return before(events[i], events[j], began) })
}

// before is order()'s comparator: session start, then the server's place for
// resume/takeover, then the client's own sequence, then the wall clock.
func before(a, b Event, began map[string]time.Time) bool {
	if a.SessionID != b.SessionID {
		if !began[a.SessionID].Equal(began[b.SessionID]) {
			return began[a.SessionID].Before(began[b.SessionID])
		}
		return a.SessionID < b.SessionID
	}
	if ra, rb := place(a), place(b); ra != rb {
		return ra < rb
	}
	if a.ClientSeq != nil && b.ClientSeq != nil && *a.ClientSeq != *b.ClientSeq {
		return *a.ClientSeq < *b.ClientSeq
	}
	if !a.OccurredAt.Equal(b.OccurredAt) {
		return a.OccurredAt.Before(b.OccurredAt)
	}
	return a.ID < b.ID
}

func place(e Event) int {
	switch e.Kind {
	case "resume":
		return 0
	case "session_takeover":
		return 2
	default:
		return 1
	}
}

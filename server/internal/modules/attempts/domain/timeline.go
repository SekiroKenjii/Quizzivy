package domain

import (
	"sort"
	"time"
)

// BuildTimeline orders the log, pairs what pairs, and totals the strip.
func (TimelineManager) Build(startedAt time.Time, minAwayMs, audioReplays int, events []IntegrityEvent, now time.Time) Timeline {
	timelineOrder(events)
	for i := range events {
		events[i].OffsetMs = max(0, int(events[i].OccurredAt.Sub(startedAt)/time.Millisecond))
		events[i].DurationMs = nil
	}
	summary := timelineTally(events, minAwayMs, now)
	summary.AudioReplays = audioReplays
	pairOthers(events)
	return Timeline{StartedAt: startedAt, Events: events, Summary: summary}
}

// TimelineManager turns the raw integrity events into the paired, ordered
// timeline the teacher reads, and the tally beside it.
type TimelineManager struct{}

type Timeline struct {
	StartedAt time.Time
	Events    []IntegrityEvent
	Summary   IntegritySummary
}

// IntegrityEvent is one row of the log as the teacher sees it.
type IntegrityEvent struct {
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

// IntegritySummary is the strip above the list.
type IntegritySummary struct {
	TotalAwayMs     int
	AwayEpisodes    int
	PasteCount      int
	ResumeCount     int
	AudioReplays    int
	OfflineEpisodes int
}

var (
	leaves  = map[string]bool{"tab_hidden": true, "window_blur": true}
	returns = map[string]bool{"tab_visible": true, "window_focus": true}

	pairs = map[string]string{
		"fullscreen_exit": "fullscreen_enter",
		"network_offline": "network_online",
		"audio_play":      "audio_ended",
	}
)

func timelineTally(events []IntegrityEvent, minAwayMs int, now time.Time) IntegritySummary {
	var summary IntegritySummary
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
			d := timelineSpan(events[open].OccurredAt, e.OccurredAt)
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

	if open >= 0 && timelineSpan(events[open].OccurredAt, now) >= minAwayMs {
		summary.AwayEpisodes++
	}
	return summary
}

func pairOthers(events []IntegrityEvent) {
	open := map[string]int{}
	for i := range events {
		e := &events[i]
		if closer, ok := pairs[e.Kind]; ok {
			open[timelineKey(closer, e.QuestionID)] = i
			continue
		}
		if j, ok := open[timelineKey(e.Kind, e.QuestionID)]; ok {
			d := timelineSpan(events[j].OccurredAt, e.OccurredAt)
			events[j].DurationMs = &d
			delete(open, timelineKey(e.Kind, e.QuestionID))
		}
	}
}

func timelineKey(kind string, questionID *string) string {
	if questionID == nil {
		return kind
	}
	return kind + ":" + *questionID
}

func timelineSpan(from, to time.Time) int {
	return max(0, int(to.Sub(from)/time.Millisecond))
}

func timelineOrder(events []IntegrityEvent) {
	began := map[string]time.Time{}
	for _, e := range events {
		if first, ok := began[e.SessionID]; !ok || e.ReceivedAt.Before(first) {
			began[e.SessionID] = e.ReceivedAt
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return timelineBefore(events[i], events[j], began) })
}

func timelineBefore(a, b IntegrityEvent, began map[string]time.Time) bool {
	if a.SessionID != b.SessionID {
		if !began[a.SessionID].Equal(began[b.SessionID]) {
			return began[a.SessionID].Before(began[b.SessionID])
		}
		return a.SessionID < b.SessionID
	}
	if ra, rb := timelinePlace(a), timelinePlace(b); ra != rb {
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

func timelinePlace(e IntegrityEvent) int {
	switch e.Kind {
	case "resume":
		return 0
	case "session_takeover":
		return 2
	default:
		return 1
	}
}

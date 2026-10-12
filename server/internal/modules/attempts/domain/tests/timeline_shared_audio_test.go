package domain_test

import (
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"testing"
)

func sharedPlay(id int64, seconds float64, recording string) domain.IntegrityEvent {
	e := event(id, "audio_play", seconds, "s1", nil)
	e.Meta = fmt.Appendf(nil, `{"scope":"group","recordingId":%q,"playId":"play-%d","plays":1}`, recording, id)
	return e
}

func sharedEnd(id int64, seconds float64, recording, duration string) domain.IntegrityEvent {
	e := event(id, "audio_ended", seconds, "s1", seq(int(id)))
	if duration == "" {
		e.Meta = fmt.Appendf(nil, `{"scope":"group","recordingId":%q}`, recording)
	} else {
		e.Meta = fmt.Appendf(nil, `{"scope":"group","recordingId":%q,"durationMs":%s}`, recording, duration)
	}
	return e
}

func sharedTimeline(events ...domain.IntegrityEvent) domain.Timeline {
	return domain.Timelines.Build(start, 3000, 0, events, at(100))
}

func sharedDuration(t *testing.T, timeline domain.Timeline, id int64, want int) {
	t.Helper()
	for _, e := range timeline.Events {
		if e.ID != id {
			continue
		}
		if want < 0 {
			if e.DurationMs != nil {
				t.Errorf("event %d duration = %d, want nil", id, *e.DurationMs)
			}
		} else if e.DurationMs == nil || *e.DurationMs != want {
			t.Errorf("event %d duration = %v, want %d", id, e.DurationMs, want)
		}
		return
	}
	t.Fatalf("event %d missing", id)
}

func TestASharedPlayCarriesTheDurationItsEndReports(t *testing.T) {
	got := sharedTimeline(sharedPlay(1, 10, "A"), sharedEnd(2, 75, "A", "64000"))
	sharedDuration(t, got, 1, 64000)
}

func TestAnEndClosesThePlayOfItsOwnRecording(t *testing.T) {
	got := sharedTimeline(sharedPlay(1, 10, "A"), sharedPlay(2, 20, "B"),
		sharedEnd(3, 30, "B", "9000"), sharedEnd(4, 75, "A", "64000"))
	sharedDuration(t, got, 1, 64000)
	sharedDuration(t, got, 2, 9000)
}

func TestAnEndWithoutADurationFallsBackToTheClocks(t *testing.T) {
	got := sharedTimeline(sharedPlay(1, 10, "A"), sharedEnd(2, 75, "A", ""))
	sharedDuration(t, got, 1, 65000)
}

func TestAnAbsurdReportedDurationIsNotBelieved(t *testing.T) {
	for _, duration := range []string{"-5", "90000000", `"long"`, "null", "true", "{}", "[]", "1e999", "86400000.1"} {
		t.Run(duration, func(t *testing.T) {
			got := sharedTimeline(sharedPlay(1, 10, "A"), sharedEnd(2, 75, "A", duration))
			sharedDuration(t, got, 1, 65000)
		})
	}
}

func TestReportedSharedDurationBoundariesAndFractionalMilliseconds(t *testing.T) {
	for _, test := range []struct {
		duration string
		want     int
	}{
		{"0", 0}, {"86400000", 86400000}, {"12.75", 12},
	} {
		t.Run(test.duration, func(t *testing.T) {
			got := sharedTimeline(sharedPlay(1, 10, "A"), sharedEnd(2, 75, "A", test.duration))
			sharedDuration(t, got, 1, test.want)
		})
	}
}

func TestAnEndWithNoOpenPlayOfItsRecordingChangesNothing(t *testing.T) {
	got := sharedTimeline(sharedPlay(1, 10, "A"), sharedEnd(2, 75, "B", "64000"))
	sharedDuration(t, got, 1, -1)
}

func TestAQuestionsOwnPlayIsStillPairedByTheQuestion(t *testing.T) {
	question := "q1"
	play := event(1, "audio_play", 10, "s1", seq(0))
	play.QuestionID = &question
	play.Meta = []byte(`{"recordingId":"A"}`)
	end := sharedEnd(3, 75, "B", "64000")
	end.QuestionID = &question
	got := sharedTimeline(play, sharedPlay(2, 20, "q1"), end)
	sharedDuration(t, got, 1, 65000)
	sharedDuration(t, got, 2, -1)
}

func TestAMetaThatIsNotJSONDoesNotBreakTheTimeline(t *testing.T) {
	for _, meta := range []string{"{", "null", "[]", `"not an object"`, "{}", `{"recordingId":null}`, `{"recordingId":123}`, `{"recordingId":{}}`} {
		t.Run(meta, func(t *testing.T) {
			end := sharedEnd(2, 75, "A", "64000")
			end.Meta = []byte(meta)
			got := sharedTimeline(sharedPlay(1, 10, "A"), end)
			sharedDuration(t, got, 1, -1)
		})
	}
}

func TestMalformedSharedPlayMetadataCannotNameARecording(t *testing.T) {
	play := sharedPlay(1, 10, "A")
	play.Meta = []byte(`{"recordingId":false}`)
	got := sharedTimeline(play, sharedEnd(2, 75, "A", "64000"))
	sharedDuration(t, got, 1, -1)
}

func TestASecondPlayOfARecordingIsTheOneItsEndCloses(t *testing.T) {
	got := sharedTimeline(sharedPlay(1, 10, "A"), sharedPlay(2, 20, "A"), sharedEnd(3, 30, "A", "7000"))
	sharedDuration(t, got, 1, -1)
	sharedDuration(t, got, 2, 7000)
}

func TestASharedEndConsumesItsOpenPlayOnce(t *testing.T) {
	got := sharedTimeline(sharedPlay(1, 10, "A"), sharedEnd(2, 20, "A", "7000"), sharedEnd(3, 30, "A", "19000"))
	sharedDuration(t, got, 1, 7000)
}

func TestNonAudioPairingIgnoresRecordingAndDurationMetadata(t *testing.T) {
	for _, test := range []struct{ open, end string }{
		{"network_offline", "network_online"}, {"fullscreen_exit", "fullscreen_enter"},
	} {
		t.Run(test.open, func(t *testing.T) {
			open := event(1, test.open, 10, "s1", seq(0))
			open.Meta = []byte(`{"recordingId":"A"}`)
			end := event(2, test.end, 75, "s1", seq(1))
			end.Meta = []byte(`{"recordingId":"B","durationMs":64000}`)
			got := sharedTimeline(open, end)
			sharedDuration(t, got, 1, 65000)
		})
	}
}

func TestLegacyAudioWithoutARecordingKeepsItsClockSpan(t *testing.T) {
	play := event(1, "audio_play", 10, "s1", seq(0))
	end := event(2, "audio_ended", 75, "s1", seq(1))
	end.Meta = []byte(`{"durationMs":64000}`)
	sharedDuration(t, sharedTimeline(play, end), 1, 65000)
}

func TestFallbackClockSpanStillClampsBackwardsTime(t *testing.T) {
	play := event(1, "audio_play", 75, "s1", seq(0))
	play.Meta = []byte(`{"recordingId":"A"}`)
	end := sharedEnd(2, 10, "A", "")
	sharedDuration(t, sharedTimeline(play, end), 1, 0)
}

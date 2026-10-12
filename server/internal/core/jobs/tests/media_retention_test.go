package jobs_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/core/jobs"
	mediacommand "quizzivy/internal/modules/media/application/command"
	mediadomain "quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/cqrs"
)

type mediaSweeps struct {
	mu      sync.Mutex
	calls   int
	results []mediaResult
}

type mediaResult struct {
	swept mediacommand.Swept
	err   error
}

func (s *mediaSweeps) handler() cqrs.CommandHandler[mediacommand.Sweep, mediacommand.Swept] {
	return cqrs.HandlerFunc[mediacommand.Sweep, mediacommand.Swept](func(ctx context.Context, _ mediacommand.Sweep) (mediacommand.Swept, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.calls++
		if _, ok := ctx.Deadline(); !ok {
			return mediacommand.Swept{}, errors.New("a sweep ran without a time budget")
		}
		if s.calls <= len(s.results) {
			r := s.results[s.calls-1]
			return r.swept, r.err
		}
		return mediacommand.Swept{Removed: 1}, nil
	})
}

func (s *mediaSweeps) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func runMediaSweeps(t *testing.T, s *mediaSweeps, schedule jobs.MediaRetention) (stop func(), logs func() string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	var mu sync.Mutex
	logger := slog.New(slog.NewJSONHandler(writerFunc(func(p []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return out.Write(p)
	}), nil))
	done := make(chan struct{})
	go func() {
		jobs.SweepRetiredMedia(ctx, logger, s.handler(), schedule)
		close(done)
	}()
	stop = func() {
		cancel()
		<-done
	}
	logs = func() string {
		mu.Lock()
		defer mu.Unlock()
		return out.String()
	}
	return stop, logs
}

func waitForSweeps(s *mediaSweeps, n int) {
	deadline := time.Now().Add(2 * time.Second)
	for s.count() < n && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
}

func TestTheMediaSweepRunsAtStartAndNotAgainWithinItsPeriod(t *testing.T) {
	s := &mediaSweeps{}
	stop, logs := runMediaSweeps(t, s, jobs.MediaRetention{Every: time.Hour, Retry: 30 * time.Millisecond, Budget: time.Second})
	waitForSweeps(s, 1)
	time.Sleep(100 * time.Millisecond)
	stop()
	if s.count() != 1 {
		t.Fatalf("a successful sweep ran %d times in its period, want once at start-up", s.count())
	}
	if !strings.Contains(logs(), `"msg":"media retention swept"`) || !strings.Contains(logs(), `"removed":1`) {
		t.Errorf("logs: %s", logs())
	}
}

func TestTheMediaSweepRetriesSoonAfterAFailureAndAfterAFileItCouldNotRemove(t *testing.T) {
	cause := errors.New("storage unreachable")
	s := &mediaSweeps{results: []mediaResult{
		{err: cause},
		{swept: mediacommand.Swept{Removed: 2, Failed: 1, Cause: cause}},
	}}
	stop, logs := runMediaSweeps(t, s, jobs.MediaRetention{Every: time.Hour, Retry: 30 * time.Millisecond, Budget: time.Second})
	waitForSweeps(s, 3)
	time.Sleep(100 * time.Millisecond)
	stop()
	if s.count() != 3 {
		t.Fatalf("the sweep ran %d times, want the start, a retry after the failure and a retry after the partial one, then the period", s.count())
	}
	for _, code := range []string{"MEDIA_RETENTION_FAILED", "MEDIA_RETENTION_INCOMPLETE", "storage unreachable"} {
		if !strings.Contains(logs(), code) {
			t.Errorf("logs do not carry %q: %s", code, logs())
		}
	}
}

func TestTheMediaSweepStopsWhenTheProcessDoes(t *testing.T) {
	s := &mediaSweeps{}
	stop, _ := runMediaSweeps(t, s, jobs.MediaRetention{Every: time.Millisecond, Retry: time.Millisecond, Budget: time.Second})
	waitForSweeps(s, 3)
	stop()
	after := s.count()
	time.Sleep(20 * time.Millisecond)
	if s.count() != after {
		t.Error("the sweep kept running after its context ended")
	}
}

func TestTheMediaSweepReportsAtErrorEveryFileItDeletedButCouldNotMark(t *testing.T) {
	lost := &mediadomain.PurgeMarkError{ID: "019a0000-0000-7000-8000-00000000f68a", Cause: errors.New("connection reset")}
	s := &mediaSweeps{results: []mediaResult{{swept: mediacommand.Swept{Failed: 1, Cause: lost, Unmarked: []*mediadomain.PurgeMarkError{lost}}}}}
	stop, logs := runMediaSweeps(t, s, jobs.MediaRetention{Every: time.Hour, Retry: time.Hour, Budget: time.Second})
	waitForSweeps(s, 1)
	time.Sleep(50 * time.Millisecond)
	stop()
	for _, want := range []string{`"level":"ERROR"`, `"code":"MEDIA_RETENTION_UNMARKED"`, `"asset_id":"019a0000-0000-7000-8000-00000000f68a"`, "connection reset"} {
		if !strings.Contains(logs(), want) {
			t.Errorf("logs do not carry %s: %s", want, logs())
		}
	}
}

package application_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
)

type activityLog struct {
	mu      sync.Mutex
	batches [][]domain.ActivityEntry
	err     error
}

func (l *activityLog) RecordActivity(_ context.Context, entries []domain.ActivityEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return l.err
	}
	l.batches = append(l.batches, entries)
	return nil
}

func (l *activityLog) entries() []domain.ActivityEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	var all []domain.ActivityEntry
	for _, b := range l.batches {
		all = append(all, b...)
	}
	return all
}

type activityClock struct{ now time.Time }

func (c *activityClock) Now() time.Time { return c.now }

func activityApp(t *testing.T, store domain.Activity, start time.Time) (*application.Application, *activityClock) {
	t.Helper()
	zone, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		zone = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	app := application.New(nil, nil, 0, nil, nil)
	clock := &activityClock{now: start}
	app.SetActivity(store, zone, nil)
	app.SetClock(clock.Now)
	return app, clock
}

func stamp(t *testing.T, app *application.Application, user string) {
	t.Helper()
	if _, err := app.Commands.StampActivity.Handle(context.Background(), command.StampActivity{UserID: user}); err != nil {
		t.Fatal(err)
	}
}

func TestAHundredRequestsInFiveMinutesAreOneUpdateAndOneDay(t *testing.T) {
	store := &activityLog{}
	start := time.Date(2026, 10, 14, 3, 0, 0, 0, time.UTC)
	app, clock := activityApp(t, store, start)

	for i := range 100 {
		clock.now = start.Add(time.Duration(i) * 2 * time.Second)
		stamp(t, app, "user-1")
	}
	app.FlushActivity(context.Background())

	got := store.entries()
	if len(got) != 1 {
		t.Fatalf("%d entries written for 100 requests inside five minutes, want 1", len(got))
	}
	if !got[0].At.Equal(start) {
		t.Errorf("the entry is stamped %v, want the first request's %v", got[0].At, start)
	}
	if want := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC); !got[0].Day.Equal(want) {
		t.Errorf("day = %v, want %v", got[0].Day, want)
	}
}

func TestAUserIsLetThroughAgainAfterFiveMinutes(t *testing.T) {
	store := &activityLog{}
	start := time.Date(2026, 10, 14, 3, 0, 0, 0, time.UTC)
	app, clock := activityApp(t, store, start)

	stamp(t, app, "user-1")
	clock.now = start.Add(domainThrottle - time.Second)
	stamp(t, app, "user-1")
	clock.now = start.Add(domainThrottle)
	stamp(t, app, "user-1")
	stamp(t, app, "user-2")
	app.FlushActivity(context.Background())

	if got := len(store.entries()); got != 3 {
		t.Errorf("%d entries, want 3: user-1 twice and user-2 once", got)
	}
}

const domainThrottle = 5 * time.Minute

func TestTheDayTurnsAtMidnightInTheAppZone(t *testing.T) {
	store := &activityLog{}
	// 16:59:30 UTC is 23:59:30 in Ho Chi Minh City; 17:00:10 is 00:00:10 the next day.
	before := time.Date(2026, 10, 14, 16, 59, 30, 0, time.UTC)
	app, clock := activityApp(t, store, before)

	stamp(t, app, "user-1")
	clock.now = before.Add(40 * time.Second)
	stamp(t, app, "user-1")
	app.FlushActivity(context.Background())

	got := store.entries()
	if len(got) != 2 {
		t.Fatalf("%d entries, want 2: the first request of the new local day passes the throttle", len(got))
	}
	days := map[time.Time]bool{}
	for _, e := range got {
		days[e.Day] = true
	}
	for _, want := range []time.Time{time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)} {
		if !days[want] {
			t.Errorf("no entry for the day %v: %v", want.Format(time.DateOnly), days)
		}
	}
}

func TestAnIdleApplicationWritesNothing(t *testing.T) {
	store := &activityLog{}
	app, _ := activityApp(t, store, time.Now())
	app.FlushActivity(context.Background())
	app.FlushActivity(context.Background())
	if len(store.batches) != 0 {
		t.Errorf("%d writes with nothing waiting", len(store.batches))
	}
}

func TestEntriesAreWrittenOnceAndAFailedWriteDoesNotQueueThemAgain(t *testing.T) {
	store := &activityLog{err: errors.New("database is down")}
	app, _ := activityApp(t, store, time.Now())
	stamp(t, app, "user-1")
	app.FlushActivity(context.Background())

	store.err = nil
	app.FlushActivity(context.Background())
	if len(store.batches) != 0 {
		t.Errorf("a failed batch was retried: %d writes", len(store.batches))
	}
}

func TestTwoHundredWaitingEntriesAreWrittenWithoutWaitingForTheTimer(t *testing.T) {
	store := &activityLog{}
	app, _ := activityApp(t, store, time.Now())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { app.RunActivity(ctx); close(done) }()

	for i := range 200 {
		stamp(t, app, "user-"+strconv.Itoa(i))
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(store.entries()) < 200 {
		if time.Now().After(deadline) {
			t.Fatalf("%d of 200 entries written", len(store.entries()))
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
}

func TestStoppingFlushesWhatIsWaiting(t *testing.T) {
	store := &activityLog{}
	app, _ := activityApp(t, store, time.Now())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { app.RunActivity(ctx); close(done) }()

	stamp(t, app, "user-1")
	cancel()
	<-done
	if got := len(store.entries()); got != 1 {
		t.Errorf("%d entries written on shutdown, want 1", got)
	}
}

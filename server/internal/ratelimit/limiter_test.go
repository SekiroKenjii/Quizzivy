package ratelimit

import (
	"testing"
	"time"
)

func TestEleventhRequestInAMinuteIsRejected(t *testing.T) {
	// §6.5's per-IP limit on the join endpoints: 10/min.
	l := New(100, PerMinute(10))
	now := time.Now()
	l.SetClock(func() time.Time { return now })

	for i := 1; i <= 10; i++ {
		if ok, _ := l.Allow("1.2.3.4"); !ok {
			t.Fatalf("request %d was rejected; the first 10 must pass", i)
		}
	}

	ok, retry := l.Allow("1.2.3.4")
	if ok {
		t.Fatal("the 11th request in a minute must be rejected")
	}
	if retry <= 0 {
		t.Error("a rejection must carry a positive Retry-After")
	}
	if retry > time.Minute {
		t.Errorf("Retry-After = %v, longer than the window itself", retry)
	}
}

func TestBucketRefillsOverTheWindow(t *testing.T) {
	l := New(100, PerMinute(10))
	now := time.Now()
	l.SetClock(func() time.Time { return now })

	for i := 0; i < 10; i++ {
		l.Allow("ip")
	}
	if ok, _ := l.Allow("ip"); ok {
		t.Fatal("expected exhaustion")
	}

	// Six seconds is a tenth of the window, so one token comes back.
	now = now.Add(6 * time.Second)
	if ok, _ := l.Allow("ip"); !ok {
		t.Error("one token should have refilled after a tenth of the window")
	}
	if ok, _ := l.Allow("ip"); ok {
		t.Error("only one token should have refilled")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l := New(100, PerMinute(2))
	l.Allow("a")
	l.Allow("a")
	if ok, _ := l.Allow("a"); ok {
		t.Fatal("key a should be exhausted")
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Error("key b must not be affected by key a")
	}
}

func TestBothWindowsApply(t *testing.T) {
	// §6.5 stacks 10/min and 60/hour. The hourly cap must bite even when the
	// per-minute one keeps refilling.
	l := New(100, PerMinute(10), PerHour(60))
	now := time.Now()
	l.SetClock(func() time.Time { return now })

	allowed := 0
	for minute := 0; minute < 12; minute++ {
		for i := 0; i < 10; i++ {
			if ok, _ := l.Allow("ip"); ok {
				allowed++
			}
		}
		now = now.Add(time.Minute)
	}

	if allowed > 75 {
		t.Errorf("allowed %d requests in 12 minutes; the hourly cap of 60 is not binding", allowed)
	}
	if allowed < 60 {
		t.Errorf("allowed only %d; the hourly cap should permit at least 60", allowed)
	}
}

func TestRejectionDoesNotDrainTheOtherBucket(t *testing.T) {
	// A request blocked by the minute rule must not also spend an hourly token,
	// or a burst would eat the hour's budget without ever being served.
	l := New(100, PerMinute(2), PerHour(100))
	now := time.Now()
	l.SetClock(func() time.Time { return now })

	l.Allow("ip")
	l.Allow("ip")
	for i := 0; i < 50; i++ {
		l.Allow("ip") // all rejected by the minute rule
	}

	now = now.Add(time.Minute)
	allowed := 0
	for i := 0; i < 2; i++ {
		if ok, _ := l.Allow("ip"); ok {
			allowed++
		}
	}
	if allowed != 2 {
		t.Errorf("after the minute window reset, allowed %d/2 -- rejected requests drained the hourly bucket", allowed)
	}
}

func TestMemoryIsBounded(t *testing.T) {
	// An unbounded map keyed by client IP is a memory-exhaustion vector on
	// exactly the endpoints §6.5 protects.
	l := New(50, PerMinute(10))
	for i := 0; i < 500; i++ {
		l.Allow(string(rune('a'+i%26)) + string(rune('0'+i/26)))
	}
	if l.Len() > 50 {
		t.Errorf("tracking %d keys, capacity is 50", l.Len())
	}
}

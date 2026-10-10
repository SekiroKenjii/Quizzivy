package support

import (
	"sync"
	"time"
)

const sweepAbove = 4096

// Throttle lets one key run once in every interval, in this process. It is
// the whole of the "at most once per five minutes per user" rule for
// materialising due items: another process has its own, and the store's
// dedupe keys make a second run a no-op.
type Throttle struct {
	interval time.Duration
	mu       sync.Mutex
	last     map[string]time.Time
}

func NewThrottle(interval time.Duration) *Throttle {
	return &Throttle{interval: interval, last: make(map[string]time.Time)}
}

// Allow reports whether key may run at now, and when it may, records the
// run. A key that ran less than the interval before now may not; a now
// before the recorded run counts as less than the interval after it.
func (t *Throttle) Allow(key string, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if last, ran := t.last[key]; ran && now.Sub(last) < t.interval {
		return false
	}
	if len(t.last) >= sweepAbove {
		t.sweep(now)
	}
	t.last[key] = now
	return true
}

// Forget erases the record of key's run, so that its next call may run. The
// run that failed is the one to forget.
func (t *Throttle) Forget(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.last, key)
}

func (t *Throttle) sweep(now time.Time) {
	for key, last := range t.last {
		if now.Sub(last) >= t.interval {
			delete(t.last, key)
		}
	}
}

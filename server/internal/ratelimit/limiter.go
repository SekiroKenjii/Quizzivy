package ratelimit

import (
	"math"
	"sync"
	"time"
)

// Rule is a token bucket: Burst tokens, refilled to full over Window.
type Rule struct {
	Burst  int
	Window time.Duration
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// Limiter is a fixed-capacity keyed token-bucket store.
//
// Capacity is bounded on purpose. An unbounded map keyed by client IP is a
// memory-exhaustion vector on precisely the endpoints §6.5 exists to protect:
// an attacker spraying distinct source addresses would grow it without limit.
// When full, the least-recently-seen entry is evicted.
type Limiter struct {
	mu       sync.Mutex
	rules    []Rule
	buckets  map[string][]bucket
	capacity int
	now      func() time.Time
}

func New(capacity int, rules ...Rule) *Limiter {
	if capacity <= 0 {
		capacity = 10_000
	}
	return &Limiter{
		rules:    rules,
		buckets:  make(map[string][]bucket, capacity),
		capacity: capacity,
		now:      time.Now,
	}
}

// SetClock replaces the time source. Tests only.
func (l *Limiter) SetClock(now func() time.Time) { l.now = now }

// Allow consumes one token from every rule for key. It returns false with the
// time to wait if any rule is exhausted -- and consumes nothing in that case,
// so a rejected request does not also drain the other buckets.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	state, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= l.capacity {
			l.evictOldestLocked()
		}
		state = make([]bucket, len(l.rules))
		for i, rule := range l.rules {
			state[i] = bucket{tokens: float64(rule.Burst), lastSeen: now}
		}
	}

	// Refill first, then check every rule before consuming from any.
	var retry time.Duration
	blocked := false
	for i, rule := range l.rules {
		elapsed := now.Sub(state[i].lastSeen)
		perSecond := float64(rule.Burst) / rule.Window.Seconds()
		state[i].tokens = math.Min(float64(rule.Burst), state[i].tokens+elapsed.Seconds()*perSecond)
		state[i].lastSeen = now

		if state[i].tokens < 1 {
			blocked = true
			need := time.Duration((1 - state[i].tokens) / perSecond * float64(time.Second))
			if need > retry {
				retry = need
			}
		}
	}

	if blocked {
		l.buckets[key] = state
		if retry < time.Second {
			retry = time.Second // Retry-After is whole seconds; never advertise 0
		}
		return false, retry
	}

	for i := range state {
		state[i].tokens--
	}
	l.buckets[key] = state
	return true, 0
}

func (l *Limiter) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	first := true
	for k, v := range l.buckets {
		if len(v) == 0 {
			delete(l.buckets, k)
			return
		}
		if first || v[0].lastSeen.Before(oldest) {
			oldestKey, oldest, first = k, v[0].lastSeen, false
		}
	}
	delete(l.buckets, oldestKey)
}

// Len reports how many keys are tracked. Tests and metrics.
func (l *Limiter) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

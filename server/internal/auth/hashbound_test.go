package auth

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Argon2id is memory-HARD: `defaultMemory` is an arena allocated for the whole
// duration of every call, not a budget. On the 512 MB production machine eight
// simultaneous logins exceed the entire instance, and a class of thirty
// students signing in together is an ordinary Tuesday. The failure mode is not
// slowness -- it is the OOM killer.
//
// Rate limiting does not bound this: §6.5's buckets are per-IP and per-email,
// and thirty students on thirty phones are thirty IPs.

func TestConcurrentHashesAreBoundedByTheLimit(t *testing.T) {
	const limit = 2
	SetMaxConcurrentHashes(limit)
	t.Cleanup(func() { SetMaxConcurrentHashes(DefaultMaxConcurrentHashes) })

	var inFlight, peak atomic.Int32
	var wg sync.WaitGroup
	release := make(chan struct{})

	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-release
			_ = withHashSlot(context.Background(), func() {
				now := inFlight.Add(1)
				for {
					was := peak.Load()
					if now <= was || peak.CompareAndSwap(was, now) {
						break
					}
				}
				// Long enough that overlap is unmissable if the bound is gone.
				time.Sleep(20 * time.Millisecond)
				inFlight.Add(-1)
			})
		}()
	}
	close(release)
	wg.Wait()

	if got := peak.Load(); got > limit {
		t.Fatalf("%d hashes ran at once, want at most %d -- the bound is not holding", got, limit)
	}
	if peak.Load() < limit {
		t.Errorf("peak concurrency was %d with a limit of %d; the slots are not being used",
			peak.Load(), limit)
	}
}

func TestAWaiterGivesUpItsPlaceWhenItsCallerIsGone(t *testing.T) {
	// A queue is only safe if it drains. Without the context, a client that
	// hung up would still be holding a place in line for a 64 MiB allocation
	// nobody is waiting for.
	SetMaxConcurrentHashes(1)
	t.Cleanup(func() { SetMaxConcurrentHashes(DefaultMaxConcurrentHashes) })

	occupied := make(chan struct{})
	done := make(chan struct{})
	go func() {
		_ = withHashSlot(context.Background(), func() {
			close(occupied)
			<-done
		})
	}()
	<-occupied

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	ran := false
	err := withHashSlot(ctx, func() { ran = true })

	if err == nil {
		t.Fatal("a waiter acquired a slot that was held")
	}
	if ran {
		t.Error("the work ran despite the context being done")
	}
	close(done)
}

func TestTheSlotIsReturnedAfterEachHash(t *testing.T) {
	// A leaked token is indistinguishable from a hang: the next login waits
	// forever on a slot nobody holds.
	SetMaxConcurrentHashes(1)
	t.Cleanup(func() { SetMaxConcurrentHashes(DefaultMaxConcurrentHashes) })

	for i := range 5 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if _, err := HashPassword(ctx, "mật-khẩu"); err != nil {
			t.Fatalf("hash %d: %v", i+1, err)
		}
		cancel()
	}
}

// currentRSSMiB reads VmRSS, which rises and falls -- unlike VmHWM, which is a
// high-water mark and would carry over whatever an earlier test allocated.
func currentRSSMiB(t *testing.T) float64 {
	t.Helper()
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Skip("no /proc/self/status; this measurement is Linux-only")
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			kb, err := strconv.ParseFloat(strings.Fields(line)[1], 64)
			if err != nil {
				t.Fatalf("parse VmRSS: %v", err)
			}
			return kb / 1024
		}
	}
	t.Skip("VmRSS not reported")
	return 0
}

func TestTheBoundActuallyCapsMemory(t *testing.T) {
	// The point of the whole exercise, measured rather than argued. Sixteen
	// unbounded hashes peak at ~1 GiB; bounded at two they must stay near the
	// arena size times two.
	if runtime.GOOS != "linux" {
		t.Skip("RSS measurement is Linux-specific")
	}
	const limit = 2
	SetMaxConcurrentHashes(limit)
	t.Cleanup(func() { SetMaxConcurrentHashes(DefaultMaxConcurrentHashes) })

	runtime.GC()
	baseline := currentRSSMiB(t)

	stop := make(chan struct{})
	var observed atomic.Uint64
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				now := uint64(currentRSSMiB(t))
				for {
					was := observed.Load()
					if now <= was || observed.CompareAndSwap(was, now) {
						break
					}
				}
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()

	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = HashPassword(context.Background(), "mật-khẩu-của-học-viên")
		}()
	}
	wg.Wait()
	close(stop)

	growth := float64(observed.Load()) - baseline
	// Two arenas is 128 MiB. Allow generous slack for the Go heap and for the
	// allocator not returning pages instantly -- the unbounded number is
	// ~1024 MiB, so anything near the limit is unambiguous.
	const ceiling = 400.0
	if growth > ceiling {
		t.Fatalf("RSS grew %.0f MiB for 16 hashes bounded at %d; expected roughly %d arenas, "+
			"so the bound is not holding", growth, limit, limit)
	}
	t.Logf("16 hashes bounded at %d grew RSS by %.0f MiB (unbounded would be ~%d MiB)",
		limit, growth, 16*64)
}

package domain_test

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/domain"
)

func TestAWaiterGivesUpItsPlaceWhenItsCallerIsGone(t *testing.T) {
	domain.SetMaxConcurrentHashes(1)
	t.Cleanup(func() { domain.SetMaxConcurrentHashes(domain.DefaultMaxConcurrentHashes) })

	var holders sync.WaitGroup
	for range 3 {
		holders.Add(1)
		go func() {
			defer holders.Done()
			_, _ = domain.HashPassword(context.Background(), "giữ-chỗ-trong-lúc-đợi")
		}()
	}
	time.Sleep(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, err := domain.HashPassword(ctx, "người-đợi")
	holders.Wait()

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a waiter whose context ended got %v, want the context's error", err)
	}
}

func TestTheSlotIsReturnedAfterEachHash(t *testing.T) {
	domain.SetMaxConcurrentHashes(1)
	t.Cleanup(func() { domain.SetMaxConcurrentHashes(domain.DefaultMaxConcurrentHashes) })

	for i := range 5 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if _, err := domain.HashPassword(ctx, "mật-khẩu"); err != nil {
			t.Fatalf("hash %d: %v", i+1, err)
		}
		cancel()
	}
}

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
	if runtime.GOOS != "linux" {
		t.Skip("RSS measurement is Linux-specific")
	}
	const limit = 2
	domain.SetMaxConcurrentHashes(limit)
	t.Cleanup(func() { domain.SetMaxConcurrentHashes(domain.DefaultMaxConcurrentHashes) })

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
			_, _ = domain.HashPassword(context.Background(), "mật-khẩu-của-học-viên")
		}()
	}
	wg.Wait()
	close(stop)

	growth := float64(observed.Load()) - baseline
	const ceiling = 400.0
	if growth > ceiling {
		t.Fatalf("RSS grew %.0f MiB for 16 hashes bounded at %d; expected roughly %d arenas, "+
			"so the bound is not holding", growth, limit, limit)
	}
}

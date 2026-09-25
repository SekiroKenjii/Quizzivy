package adapters_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"quizzivy/internal/core/adapters"
)

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestAWakeReachesTheWorkerAndABurstCoalesces(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/wake" {
			t.Errorf("wake sent %s %s", r.Method, r.URL.Path)
		}
		calls.Add(1)
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))
	defer worker.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wake := adapters.NewWorkerWake(ctx, worker.URL+"/wake", slog.Default())

	wake.Wake()
	eventually(t, "the first wake", func() bool { return calls.Load() == 1 })
	for range 10 {
		wake.Wake()
	}
	close(release)
	eventually(t, "the coalesced wake", func() bool { return calls.Load() == 2 })
	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 2 {
		t.Fatalf("a burst of wakes sent %d requests, want 2", calls.Load())
	}
}

func TestAWakeIsRetriedWhileTheWorkerIsNotReady(t *testing.T) {
	var calls atomic.Int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer worker.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapters.NewWorkerWake(ctx, worker.URL+"/wake", slog.Default()).Wake()
	eventually(t, "the retried wake", func() bool { return calls.Load() == 2 })
}

func TestWakeNeverBlocksTheCaller(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wake := adapters.NewWorkerWake(ctx, "http://127.0.0.1:1/wake", slog.Default())
	start := time.Now()
	for range 100 {
		wake.Wake()
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("100 wakes to an unreachable worker took %v", elapsed)
	}
}

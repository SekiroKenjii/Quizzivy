package core

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"quizzivy/internal/config"
)

// TestShutdownDrainsInFlightRequests pins the shutdown context.
func TestShutdownDrainsInFlightRequests(t *testing.T) {
	const requestDuration = 300 * time.Millisecond

	port := freePort(t)
	cfg := config.Config{Port: port, Env: "test"}

	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(requestDuration)
		fmt.Fprint(w, "drained")
	})

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() {
		served <- serve(ctx, slog.New(slog.DiscardHandler), cfg, handler)
	}()
	waitForListener(t, port)

	body := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://localhost:" + port + "/")
		if err != nil {
			body <- "request failed: " + err.Error()
			return
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		body <- string(b)
	}()

	<-started
	cancel()

	select {
	case got := <-body:
		if got != "drained" {
			t.Errorf("in-flight request got %q, want %q: shutdown cut it off instead of draining",
				got, "drained")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the in-flight request never completed")
	}

	select {
	case err := <-served:
		if err != nil {
			t.Errorf("serve returned %v, want nil -- a cancelled shutdown context would "+
				"surface here as context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after the context was cancelled")
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func waitForListener(t *testing.T, port string) {
	t.Helper()
	for range 100 {
		conn, err := net.DialTimeout("tcp", "localhost:"+port, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("server never started listening")
}

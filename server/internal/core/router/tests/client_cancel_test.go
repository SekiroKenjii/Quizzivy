package router_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/core/router"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/shared/cqrs"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.buf.Bytes())
}

type logRecord struct {
	Level   string `json:"level"`
	Msg     string `json:"msg"`
	Status  int    `json:"status"`
	Request string `json:"request_id"`
}

func logRecords(t *testing.T, logs *syncBuffer) []logRecord {
	t.Helper()
	var out []logRecord
	scanner := bufio.NewScanner(bytes.NewReader(logs.bytes()))
	for scanner.Scan() {
		var r logRecord
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			t.Fatalf("log line %q: %v", scanner.Text(), err)
		}
		out = append(out, r)
	}
	return out
}

func recordsAt(records []logRecord, level string) []logRecord {
	var out []logRecord
	for _, r := range records {
		if r.Level == level {
			out = append(out, r)
		}
	}
	return out
}

func recordsNamed(records []logRecord, msg string) []logRecord {
	var out []logRecord
	for _, r := range records {
		if r.Msg == msg {
			out = append(out, r)
		}
	}
	return out
}

func accessRecord(t *testing.T, records []logRecord) logRecord {
	t.Helper()
	found := recordsNamed(records, "http")
	if len(found) != 1 {
		t.Fatalf("%d access lines, want one per request: %v", len(found), records)
	}
	return found[0]
}

func debugLoggedRouter(t *testing.T, deps router.Deps) (http.Handler, *syncBuffer) {
	t.Helper()
	logs := &syncBuffer{}
	logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	deps.Principals = newFakePrincipals()
	h, err := router.New(deps, logger, []string{allowedOrigin}, "")
	if err != nil {
		t.Fatalf("router.New: %v", err)
	}
	return h, logs
}

func logoutFailing(t *testing.T, fail func(ctx context.Context) error) (http.Handler, *syncBuffer) {
	t.Helper()
	logout := func(ctx context.Context, _ command.Logout) (cqrs.Nothing, error) {
		return cqrs.Nothing{}, fail(ctx)
	}
	app := &application.Application{Commands: application.Commands{
		Logout: cqrs.HandlerFunc[command.Logout, cqrs.Nothing](logout),
	}}
	return debugLoggedRouter(t, router.Deps{
		DB:      fakeDB{},
		Tokens:  testIssuer(t),
		Modules: router.Modules{Identity: identityhttp.NewIdentity(app, time.Hour, true, nil)},
	})
}

func logoutOn(ctx context.Context, h http.Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/auth/logout", nil)
	req.RemoteAddr = "203.0.113.40:5555"
	req.Header.Set("Cookie", "quizzivy_refresh=anything")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestARequestTheClientCancelledMidHandlerIsNotAServerError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h, logs := logoutFailing(t, func(request context.Context) error {
		cancel()
		return fmt.Errorf("logout: revoke family: %w", request.Err())
	})

	rec := logoutOn(ctx, h)

	if rec.Code != 499 {
		t.Errorf("status = %d, want 499", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want none", rec.Body.String())
	}
	records := logRecords(t, logs)
	if errs := recordsAt(records, "ERROR"); len(errs) != 0 {
		t.Errorf("a client's cancellation logged %d lines at ERROR: %v", len(errs), errs)
	}
	if access := accessRecord(t, records); access.Status != 499 || access.Level != "INFO" {
		t.Errorf("access line = %+v, want status 499 at INFO: no 5xx count may include it", access)
	}
	cancelledLines := recordsNamed(records, "client_cancelled")
	if len(cancelledLines) != 1 || cancelledLines[0].Level != "DEBUG" || cancelledLines[0].Request == "" {
		t.Errorf("client_cancelled lines = %v, want one at DEBUG naming the request id", cancelledLines)
	}
}

func TestAClientThatDisconnectsMidRequestIsLoggedAs499OnARealConnection(t *testing.T) {
	started := make(chan struct{})
	h, logs := logoutFailing(t, func(request context.Context) error {
		close(started)
		<-request.Done()
		return fmt.Errorf("logout: revoke family: %w", request.Err())
	})
	server := httptest.NewServer(h)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/auth/logout", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Cookie", "quizzivy_refresh=anything")
	finished := make(chan error, 1)
	go func() {
		resp, err := server.Client().Do(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
		finished <- err
	}()

	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("the request never reached the handler")
	}
	cancel()
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Fatalf("client error = %v, want context.Canceled", err)
	}

	var records []logRecord
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		records = logRecords(t, logs)
		if len(recordsNamed(records, "http")) > 0 {
			break
		}
	}
	if access := accessRecord(t, records); access.Status != 499 {
		t.Errorf("access status = %d, want 499: the server saw the connection close as the request context ending", access.Status)
	}
	if errs := recordsAt(records, "ERROR"); len(errs) != 0 {
		t.Errorf("a disconnect logged %d lines at ERROR: %v", len(errs), errs)
	}
}

func TestAFailureThatIsNotTheClientsCancellationIsStillAServerError(t *testing.T) {
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	gone, goneCancel := context.WithCancel(context.Background())
	goneCancel()
	cases := map[string]struct {
		request context.Context
		fail    error
	}{
		"a deadline the server imposed passed": {
			context.Background(), fmt.Errorf("logout: revoke family: %w", context.DeadlineExceeded),
		},
		"a context the server derived was cancelled while the client was still there": {
			context.Background(), fmt.Errorf("logout: revoke family: %w", context.Canceled),
		},
		"a database fault met a request the client also cancelled": {
			gone, errors.New("logout: revoke family: serialization failure"),
		},
		"a server deadline passed on a request the client also cancelled": {
			gone, fmt.Errorf("logout: revoke family: %w", context.DeadlineExceeded),
		},
		"the request's own deadline passed": {
			expired, fmt.Errorf("logout: revoke family: %w", context.DeadlineExceeded),
		},
	}
	for name, c := range cases {
		h, logs := logoutFailing(t, func(context.Context) error { return c.fail })

		rec := logoutOn(c.request, h)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s: status = %d, want 500", name, rec.Code)
		}
		if code := errorCode(t, rec); code != "INTERNAL" {
			t.Errorf("%s: code = %q, want INTERNAL", name, code)
		}
		records := logRecords(t, logs)
		if errs := recordsAt(records, "ERROR"); len(errs) != 1 || errs[0].Msg != "handler" {
			t.Errorf("%s: ERROR lines = %v, want the one handler line", name, errs)
		}
		if access := accessRecord(t, records); access.Status != http.StatusInternalServerError {
			t.Errorf("%s: access status = %d, want 500", name, access.Status)
		}
	}
}

func healthzOn(ctx context.Context, h http.Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "203.0.113.40:5555"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAHealthProbeTheCallerAbandonedIsNotAServiceUnavailable(t *testing.T) {
	gone, cancel := context.WithCancel(context.Background())
	cancel()
	h, logs := debugLoggedRouter(t, router.Deps{DB: fakeDB{err: fmt.Errorf("ping: %w", context.Canceled)}})

	rec := healthzOn(gone, h)

	if rec.Code != 499 {
		t.Errorf("status = %d, want 499", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want none", rec.Body.String())
	}
	if access := accessRecord(t, logRecords(t, logs)); access.Status != 499 {
		t.Errorf("access status = %d, want 499", access.Status)
	}
}

func TestAHealthProbeWhosePingTimedOutIsStillServiceUnavailable(t *testing.T) {
	for name, err := range map[string]error{
		"the pool's own two-second budget ran out":              fmt.Errorf("ping: %w", context.DeadlineExceeded),
		"a derived context was cancelled with the caller still": fmt.Errorf("ping: %w", context.Canceled),
		"the database refused":                                  errors.New("connection refused"),
	} {
		h, _ := debugLoggedRouter(t, router.Deps{DB: fakeDB{err: err}})

		if rec := healthzOn(context.Background(), h); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: status = %d, want 503", name, rec.Code)
		}
	}
}

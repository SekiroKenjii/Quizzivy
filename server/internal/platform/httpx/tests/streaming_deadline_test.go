package httpx_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/platform/httpx"
)

var quiet = slog.New(slog.DiscardHandler)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) lines() []map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(b.buf.String()), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err == nil {
			out = append(out, record)
		}
	}
	return out
}

func TestAResponseControllerReachesTheConnectionThroughTheAccessLog(t *testing.T) {
	answered := make(chan error, 1)
	h := httpx.Logging(quiet)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		answered <- http.NewResponseController(w).SetReadDeadline(time.Now().Add(time.Minute))
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	response, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()

	if err := <-answered; err != nil {
		t.Errorf("SetReadDeadline through httpx.Logging = %v, want nil: every writer around the server's must implement Unwrap", err)
	}
}

func streamingChain(logs io.Writer, next http.Handler) http.Handler {
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	streaming := map[string]struct{}{"POST /teacher/media": {}}
	return httpx.RequestID(httpx.Logging(logger)(httpx.LimitRequestBody(logger, streaming, 100, nil)(next)))
}

func TestAStreamedUploadWhoseDeadlineCannotBeSetIsLoggedAsAWarning(t *testing.T) {
	logs := &lockedBuffer{}
	reached := false
	h := httpx.LimitRequestBody(slog.New(slog.NewJSONHandler(logs, nil)), map[string]struct{}{"POST /teacher/media": {}}, 100, nil)(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	request := httptest.NewRequest(http.MethodPost, "/teacher/media", strings.NewReader("file"))
	request.Pattern = "POST /teacher/media"

	h.ServeHTTP(httptest.NewRecorder(), request)

	warnings := logs.lines()
	if len(warnings) != 1 || warnings[0]["level"] != "WARN" || warnings[0]["msg"] != "streaming_read_deadline" ||
		warnings[0]["path"] != "/teacher/media" || !strings.Contains(warnings[0]["err"].(string), "not supported") {
		t.Errorf("log = %v, want one WARN streaming_read_deadline naming the path and the error", warnings)
	}
	if !reached {
		t.Error("the upload was refused because its longer deadline could not be set; it must be held to the server's instead")
	}
}

func TestAStreamedUploadOnARealConnectionLogsNoWarning(t *testing.T) {
	logs := &lockedBuffer{}
	done := make(chan struct{})
	mux := http.NewServeMux()
	mux.Handle("POST /teacher/media", streamingChain(logs, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(done)
	})))
	server := httptest.NewServer(mux)
	defer server.Close()

	response, err := http.Post(server.URL+"/teacher/media", "application/octet-stream", strings.NewReader("file"))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	<-done

	for _, record := range logs.lines() {
		if record["level"] == "WARN" {
			t.Errorf("logged %v, want no warning: the deadline can be set on a real connection", record)
		}
	}
}

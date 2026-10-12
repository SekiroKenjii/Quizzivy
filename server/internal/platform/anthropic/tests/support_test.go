package anthropic_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/platform/anthropic"
)

const testKey = "fixture-provider-key-4c8e2b10"

type exchange struct {
	status int
	header map[string]string
	body   string
	delay  time.Duration
}

type recorded struct {
	header http.Header
	body   map[string]any
}

type fakeAPI struct {
	server   *httptest.Server
	mu       sync.Mutex
	script   []exchange
	requests []recorded
}

func newAPI(t *testing.T, script ...exchange) *fakeAPI {
	t.Helper()
	api := &fakeAPI{script: script}
	api.server = httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(api.server.Close)
	return api
}

func (a *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	a.mu.Lock()
	a.requests = append(a.requests, recorded{header: r.Header.Clone(), body: body})
	next := a.script[min(len(a.requests), len(a.script))-1]
	a.mu.Unlock()
	if next.delay > 0 {
		select {
		case <-time.After(next.delay):
		case <-r.Context().Done():
			return
		}
	}
	for name, value := range next.header {
		w.Header().Set(name, value)
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(next.status)
	_, _ = io.WriteString(w, next.body)
}

func (a *fakeAPI) calls() []recorded {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]recorded(nil), a.requests...)
}

func (a *fakeAPI) client(t *testing.T, tweak func(*anthropic.Config)) *anthropic.Client {
	t.Helper()
	cfg := anthropic.Config{APIKey: testKey, BaseURL: a.server.URL, HTTPClient: a.server.Client(), RetryBackoff: time.Millisecond}
	if tweak != nil {
		tweak(&cfg)
	}
	client, err := anthropic.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

func request() anthropic.Request {
	return anthropic.Request{
		Model:     "model-fixture",
		System:    "You write quiz questions.",
		MaxTokens: 1000,
		Blocks:    []anthropic.Block{anthropic.TextBlock("A passage about water.")},
		Schema:    json.RawMessage(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"string"}}},"required":["items"],"additionalProperties":false}`),
	}
}

func message(text, stopReason string) string {
	encoded, _ := json.Marshal(text)
	return `{"id":"msg_fixture","type":"message","role":"assistant","model":"model-fixture","content":[{"type":"text","text":` + string(encoded) + `}],` +
		`"stop_reason":"` + stopReason + `","stop_sequence":null,` +
		`"usage":{"input_tokens":120,"output_tokens":34,"cache_read_input_tokens":5,"cache_creation_input_tokens":7}}`
}

func ok(text string) exchange {
	return exchange{status: http.StatusOK, header: map[string]string{"request-id": "req_fixture_ok"}, body: message(text, "end_turn")}
}

func failure(status int, errorType, text string) exchange {
	return exchange{status: status, header: map[string]string{"request-id": "req_fixture_failed"}, body: `{"type":"error","error":{"type":"` + errorType + `","message":"` + text + `"}}`}
}

func sse(events ...string) exchange {
	var body strings.Builder
	for _, event := range events {
		name := event[:strings.Index(event, "|")]
		body.WriteString("event: " + name + "\ndata: " + event[strings.Index(event, "|")+1:] + "\n\n")
	}
	return exchange{status: http.StatusOK, header: map[string]string{"Content-Type": "text/event-stream", "request-id": "req_fixture_stream"}, body: body.String()}
}

func streamOf(chunks ...string) exchange {
	events := []string{
		`message_start|{"type":"message_start","message":{"id":"msg_s","type":"message","role":"assistant","model":"model-fixture","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":120,"output_tokens":1,"cache_read_input_tokens":5,"cache_creation_input_tokens":7}}}`,
		`content_block_start|{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
	}
	for _, chunk := range chunks {
		encoded, _ := json.Marshal(chunk)
		events = append(events, `content_block_delta|{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":`+string(encoded)+`}}`)
	}
	events = append(events,
		`content_block_stop|{"type":"content_block_stop","index":0}`,
		`message_delta|{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":40}}`,
		`message_stop|{"type":"message_stop"}`,
	)
	return sse(events...)
}

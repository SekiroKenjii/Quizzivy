package adapters_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/platform/anthropic"
	"quizzivy/internal/platform/config"
)

const aiFixtureKey = "fixture-provider-key-77d1e0a9"

type aiProvider struct {
	server *httptest.Server
	mu     sync.Mutex
	bodies []map[string]any
	status int
	reply  string
}

func newAIProvider(t *testing.T, status int, reply string) *aiProvider {
	t.Helper()
	p := &aiProvider{status: status, reply: reply}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		p.mu.Lock()
		p.bodies = append(p.bodies, body)
		p.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("request-id", "req_adapter_fixture")
		w.WriteHeader(p.status)
		_, _ = io.WriteString(w, p.reply)
	}))
	t.Cleanup(p.server.Close)
	return p
}

func (p *aiProvider) adapter(t *testing.T) adapters.AI {
	t.Helper()
	client, err := anthropic.New(anthropic.Config{APIKey: aiFixtureKey, BaseURL: p.server.URL, HTTPClient: p.server.Client(), RetryBackoff: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return adapters.NewAIOver(client, "fast-model-fixture", "capable-model-fixture")
}

func aiMessage(text, stopReason string) string {
	encoded, _ := json.Marshal(text)
	return `{"id":"m","type":"message","role":"assistant","model":"x","content":[{"type":"text","text":` + string(encoded) + `}],"stop_reason":"` + stopReason + `","stop_sequence":null,` +
		`"usage":{"input_tokens":11,"output_tokens":22,"cache_read_input_tokens":33,"cache_creation_input_tokens":44}}`
}

func aiRequest(tier ports.Tier) ports.Request {
	return ports.Request{
		Tier:      tier,
		System:    "system prompt",
		Blocks:    []ports.Block{ports.PDFBlock([]byte("%PDF-fixture")), ports.ImageBlock("image/webp", []byte("webp")), ports.TextBlock("read")},
		Schema:    json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false}`),
		MaxTokens: 500,
	}
}

func TestWithTheKeyUnsetThereIsNoCompleter(t *testing.T) {
	completer, err := adapters.NewAI(config.AI{}, nil)
	if err != nil || completer != nil {
		t.Fatalf("NewAI with no provider = %v, %v; want a nil Completer", completer, err)
	}
}

func TestWithTheProviderConfiguredThereIsACompleterThatNamesNoKey(t *testing.T) {
	completer, err := adapters.NewAI(config.AI{APIKey: config.Secret(aiFixtureKey), Fast: config.AITier{Model: "fast-model-fixture"}, Capable: config.AITier{Model: "capable-model-fixture"}}, nil)
	if err != nil || completer == nil {
		t.Fatalf("NewAI = %v, %v", completer, err)
	}
}

func TestEachTierIsServedByItsOwnModelAndTheBlocksCrossUnchanged(t *testing.T) {
	provider := newAIProvider(t, http.StatusOK, aiMessage(`{"ok":true}`, "end_turn"))
	adapter := provider.adapter(t)
	for tier, model := range map[ports.Tier]string{ports.TierFast: "fast-model-fixture", ports.TierCapable: "capable-model-fixture"} {
		response, err := adapter.Complete(context.Background(), aiRequest(tier))
		if err != nil {
			t.Fatalf("%s: %v", tier, err)
		}
		if string(response.Object) != `{"ok":true}` || response.RequestID != "req_adapter_fixture" {
			t.Errorf("%s: response = %s (%q)", tier, response.Object, response.RequestID)
		}
		if want := (ports.Usage{InputTokens: 11, OutputTokens: 22, CacheReadTokens: 33, CacheWriteTokens: 44}); response.Usage != want {
			t.Errorf("%s: usage = %+v, want %+v", tier, response.Usage, want)
		}
		sent := provider.bodies[len(provider.bodies)-1]
		if sent["model"] != model {
			t.Errorf("%s: model on the wire = %v, want %s", tier, sent["model"], model)
		}
		content := sent["messages"].([]any)[0].(map[string]any)["content"].([]any)
		var types []string
		for _, block := range content {
			types = append(types, block.(map[string]any)["type"].(string))
		}
		if strings.Join(types, ",") != "document,image,text" {
			t.Errorf("%s: blocks on the wire = %v", tier, types)
		}
	}
}

func TestAnUnknownTierIsRejectedBeforeAnyRequest(t *testing.T) {
	provider := newAIProvider(t, http.StatusOK, aiMessage(`{}`, "end_turn"))
	_, err := provider.adapter(t).Complete(context.Background(), aiRequest("economy"))
	if !errors.Is(err, ports.ErrRejected) || len(provider.bodies) != 0 {
		t.Fatalf("error = %v after %d requests", err, len(provider.bodies))
	}
}

func TestProviderFailuresBecomeThePortsErrorsAndKeepWhatWasBilled(t *testing.T) {
	cases := []struct {
		name   string
		status int
		reply  string
		want   error
		billed bool
	}{
		{"rate limited", http.StatusTooManyRequests, `{"type":"error","error":{"type":"rate_limit_error","message":"slow"}}`, ports.ErrUnavailable, false},
		{"server error", http.StatusInternalServerError, `{"type":"error","error":{"type":"api_error","message":"boom"}}`, ports.ErrUnavailable, false},
		{"bad key", http.StatusUnauthorized, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key ` + aiFixtureKey + `"}}`, ports.ErrRejected, false},
		{"bad request", http.StatusBadRequest, `{"type":"error","error":{"type":"invalid_request_error","message":"schema"}}`, ports.ErrRejected, false},
		{"refusal", http.StatusOK, aiMessage("", "refusal"), ports.ErrRefused, true},
		{"not json", http.StatusOK, aiMessage("sorry", "end_turn"), ports.ErrInvalidResponse, true},
	}
	for _, c := range cases {
		provider := newAIProvider(t, c.status, c.reply)
		response, err := provider.adapter(t).Complete(context.Background(), aiRequest(ports.TierCapable))
		if !errors.Is(err, c.want) {
			t.Errorf("%s: error = %v, want %v", c.name, err, c.want)
			continue
		}
		if strings.Contains(err.Error(), aiFixtureKey) {
			t.Errorf("%s: the key reached the error: %v", c.name, err)
		}
		if billed := response.Usage.InputTokens == 11 && response.RequestID != ""; billed != c.billed {
			t.Errorf("%s: usage kept = %v, want %v (%+v)", c.name, billed, c.billed, response)
		}
	}
}

func TestACancelledCallerIsNotReportedAsAProviderFailure(t *testing.T) {
	provider := newAIProvider(t, http.StatusOK, aiMessage(`{}`, "end_turn"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := provider.adapter(t).Complete(ctx, aiRequest(ports.TierFast))
	if !errors.Is(err, context.Canceled) || errors.Is(err, ports.ErrUnavailable) {
		t.Fatalf("error = %v", err)
	}
}

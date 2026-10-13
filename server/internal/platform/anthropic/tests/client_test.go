package anthropic_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/platform/anthropic"
)

func complete(t *testing.T, client *anthropic.Client, req anthropic.Request) (anthropic.Response, error) {
	t.Helper()
	return client.Complete(context.Background(), req)
}

func errorOfKind(t *testing.T, err error, kind error) *anthropic.Error {
	t.Helper()
	var typed *anthropic.Error
	if !errors.Is(err, kind) || !errors.As(err, &typed) {
		t.Fatalf("error = %v, want an *anthropic.Error of kind %v", err, kind)
	}
	return typed
}

func requireKind(t *testing.T, err error, kind error) {
	t.Helper()
	_ = errorOfKind(t, err, kind)
}

func TestAHappyCompletionReturnsTheObjectTheUsageAndTheRequestID(t *testing.T) {
	api := newAPI(t, ok(`{"items":["a","b"]}`))
	response, err := complete(t, api.client(t, nil), request())
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if string(response.Object) != `{"items":["a","b"]}` {
		t.Errorf("Object = %s", response.Object)
	}
	if want := (anthropic.Usage{InputTokens: 120, OutputTokens: 34, CacheReadTokens: 5, CacheWriteTokens: 7}); response.Usage != want {
		t.Errorf("Usage = %+v, want %+v", response.Usage, want)
	}
	if response.RequestID != "req_fixture_ok" {
		t.Errorf("RequestID = %q", response.RequestID)
	}
}

func TestTheRequestCarriesTheKeyTheModelTheSchemaAndEveryBlockInOrder(t *testing.T) {
	api := newAPI(t, ok(`{"items":[]}`))
	req := request()
	req.Blocks = []anthropic.Block{
		anthropic.PDFBlock([]byte("%PDF-fixture")),
		anthropic.ImageBlock("image/png", []byte("png-fixture")),
		anthropic.TextBlock("Read these."),
	}
	if _, err := complete(t, api.client(t, nil), req); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	sent := api.calls()
	if len(sent) != 1 {
		t.Fatalf("requests = %d, want 1", len(sent))
	}
	if got := sent[0].header.Get("X-Api-Key"); got != testKey {
		t.Errorf("x-api-key = %q, want the configured key", got)
	}
	if got := sent[0].header.Get("Authorization"); got != "" {
		t.Errorf("an Authorization header was sent: %q", got)
	}
	body := sent[0].body
	if body["model"] != "model-fixture" || body["max_tokens"] != float64(1000) {
		t.Errorf("model and max_tokens = %v and %v", body["model"], body["max_tokens"])
	}
	if _, streamed := body["stream"]; streamed {
		t.Errorf("a 1000-token call was streamed")
	}
	system, _ := body["system"].([]any)
	if len(system) != 1 || system[0].(map[string]any)["text"] != "You write quiz questions." {
		t.Errorf("system = %v", body["system"])
	}
	format := body["output_config"].(map[string]any)["format"].(map[string]any)
	schema := format["schema"].(map[string]any)
	if format["type"] != "json_schema" || schema["additionalProperties"] != false || schema["type"] != "object" {
		t.Errorf("output_config.format = %v", format)
	}
	turns := body["messages"].([]any)
	content := turns[0].(map[string]any)["content"].([]any)
	var types []string
	for _, block := range content {
		types = append(types, block.(map[string]any)["type"].(string))
	}
	if turns[0].(map[string]any)["role"] != "user" || strings.Join(types, ",") != "document,image,text" {
		t.Errorf("turn = %v, block types = %v", turns[0], types)
	}
	if source := content[0].(map[string]any)["source"].(map[string]any); source["media_type"] != "application/pdf" || source["type"] != "base64" {
		t.Errorf("document source = %v", source)
	}
	if source := content[1].(map[string]any)["source"].(map[string]any); source["media_type"] != "image/png" || source["data"] != "cG5nLWZpeHR1cmU=" {
		t.Errorf("image source = %v", source)
	}
}

func TestACallStreamsOnlyAboveFourThousandNinetySixTokens(t *testing.T) {
	for tokens, streamed := range map[int]bool{anthropic.StreamingThreshold: false, anthropic.StreamingThreshold + 1: true} {
		api := newAPI(t, ok(`{"items":[]}`), streamOf(`{"items":`, `["a"]}`))
		if streamed {
			api = newAPI(t, streamOf(`{"items":`, `["a"]}`))
		}
		req := request()
		req.MaxTokens = tokens
		response, err := complete(t, api.client(t, nil), req)
		if err != nil {
			t.Fatalf("Complete with %d tokens: %v", tokens, err)
		}
		_, sentStream := api.calls()[0].body["stream"]
		if sentStream != streamed {
			t.Errorf("max_tokens %d: stream = %v, want %v", tokens, sentStream, streamed)
		}
		if streamed {
			if string(response.Object) != `{"items":["a"]}` || response.RequestID != "req_fixture_stream" {
				t.Errorf("streamed answer = %s (request %q)", response.Object, response.RequestID)
			}
			if want := (anthropic.Usage{InputTokens: 120, OutputTokens: 40, CacheReadTokens: 5, CacheWriteTokens: 7}); response.Usage != want {
				t.Errorf("streamed usage = %+v, want %+v", response.Usage, want)
			}
		}
	}
}

func TestA429ThenA200SendsTheRequestAgainAndSucceeds(t *testing.T) {
	limited := failure(http.StatusTooManyRequests, "rate_limit_error", "slow down")
	limited.header["Retry-After"] = "0"
	api := newAPI(t, limited, ok(`{"items":[]}`))
	response, err := complete(t, api.client(t, nil), request())
	if err != nil || string(response.Object) != `{"items":[]}` {
		t.Fatalf("Complete = %s, %v", response.Object, err)
	}
	if n := len(api.calls()); n != 2 {
		t.Errorf("requests = %d, want 2", n)
	}
}

func TestA5xxIsRetriedThreeTimesThenTheTypedError(t *testing.T) {
	api := newAPI(t, failure(http.StatusBadGateway, "api_error", "upstream failed"))
	_, err := complete(t, api.client(t, nil), request())
	typed := errorOfKind(t, err, anthropic.ErrUnavailable)
	if typed.Status != http.StatusBadGateway || typed.RequestID != "req_fixture_failed" || typed.Message != "upstream failed" {
		t.Errorf("error = %+v", typed)
	}
	if n := len(api.calls()); n != 1+anthropic.MaxRetries {
		t.Errorf("requests = %d, want %d", n, 1+anthropic.MaxRetries)
	}
}

func TestA5xxTwiceThenA200Succeeds(t *testing.T) {
	broken := failure(http.StatusServiceUnavailable, "overloaded_error", "overloaded")
	api := newAPI(t, broken, broken, ok(`{"items":[]}`))
	if _, err := complete(t, api.client(t, nil), request()); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if n := len(api.calls()); n != 3 {
		t.Errorf("requests = %d, want 3", n)
	}
}

func TestA429ThatNeverClearsEndsAsRateLimited(t *testing.T) {
	api := newAPI(t, failure(http.StatusTooManyRequests, "rate_limit_error", "slow down"))
	_, err := complete(t, api.client(t, nil), request())
	requireKind(t, err, anthropic.ErrRateLimited)
	if n := len(api.calls()); n != 1+anthropic.MaxRetries {
		t.Errorf("requests = %d, want %d", n, 1+anthropic.MaxRetries)
	}
}

func TestA4xxIsNeverSentAgain(t *testing.T) {
	cases := []struct {
		status    int
		errorType string
		kind      error
	}{
		{http.StatusBadRequest, "invalid_request_error", anthropic.ErrInvalidRequest},
		{http.StatusNotFound, "not_found_error", anthropic.ErrInvalidRequest},
		{http.StatusRequestEntityTooLarge, "request_too_large", anthropic.ErrInvalidRequest},
		{http.StatusConflict, "api_error", anthropic.ErrInvalidRequest},
		{http.StatusRequestTimeout, "api_error", anthropic.ErrInvalidRequest},
		{http.StatusUnauthorized, "authentication_error", anthropic.ErrAuth},
		{http.StatusPaymentRequired, "billing_error", anthropic.ErrAuth},
		{http.StatusForbidden, "permission_error", anthropic.ErrAuth},
	}
	for _, c := range cases {
		api := newAPI(t, failure(c.status, c.errorType, "refused"))
		_, err := complete(t, api.client(t, nil), request())
		requireKind(t, err, c.kind)
		if n := len(api.calls()); n != 1 {
			t.Errorf("status %d: requests = %d, want 1", c.status, n)
		}
	}
}

func TestAnErrorNeverCarriesTheKeyEvenWhenTheProviderEchoesIt(t *testing.T) {
	echo := failure(http.StatusUnauthorized, "authentication_error", "invalid x-api-key: "+testKey)
	echo.header["request-id"] = "req_" + testKey
	api := newAPI(t, echo)
	client := api.client(t, nil)
	_, err := complete(t, client, request())
	typed := errorOfKind(t, err, anthropic.ErrAuth)
	renderings := []string{
		err.Error(), fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err), typed.Message, typed.RequestID,
		fmt.Sprintf("%+v", typed), fmt.Sprintf("%v", client), fmt.Sprintf("%+v", client), fmt.Sprintf("%#v", client),
		fmt.Sprintf("%+v", anthropic.Config{APIKey: testKey}), fmt.Sprintf("%#v", anthropic.Config{APIKey: testKey}),
	}
	for _, text := range renderings {
		if strings.Contains(text, testKey) || strings.Contains(text, "4c8e2b10") {
			t.Errorf("the key reached a rendering: %s", text)
		}
	}
	if !strings.Contains(typed.Message, "[redacted]") {
		t.Errorf("the redaction left no mark: %q", typed.Message)
	}
}

func TestAFailedConnectionIsUnavailableAndNamesNoKey(t *testing.T) {
	api := newAPI(t, ok(`{}`))
	client := api.client(t, nil)
	api.server.Close()
	_, err := complete(t, client, request())
	typed := errorOfKind(t, err, anthropic.ErrUnavailable)
	if strings.Contains(typed.Error(), testKey) {
		t.Errorf("the key reached a transport error: %v", typed)
	}
}

func TestARefusalIsTypedNotRetriedAndKeepsTheUsage(t *testing.T) {
	refusal := ok("")
	refusal.body = strings.Replace(message("", "refusal"), `"stop_sequence":null`, `"stop_sequence":null,"stop_details":{"type":"refusal","category":"cyber","explanation":null}`, 1)
	api := newAPI(t, refusal)
	response, err := complete(t, api.client(t, nil), request())
	typed := errorOfKind(t, err, anthropic.ErrRefused)
	if !strings.Contains(typed.Message, "cyber") {
		t.Errorf("the refusal category was lost: %q", typed.Message)
	}
	if response.Usage.InputTokens != 120 || response.Usage.OutputTokens != 34 || response.RequestID != "req_fixture_ok" {
		t.Errorf("a refusal dropped what the provider billed: %+v", response)
	}
	if n := len(api.calls()); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
}

func TestAnAnswerThatIsNotOneCompleteJSONObjectIsInvalidAndKeepsTheUsage(t *testing.T) {
	truncated := ok("")
	truncated.body = message(`{"items":["a",`, "max_tokens")
	cases := map[string]exchange{
		"not JSON":           ok("here are your questions"),
		"an array":           ok(`[1,2]`),
		"two objects":        ok(`{"a":1}{"b":2}`),
		"empty":              ok("   "),
		"hit max_tokens":     truncated,
		"stopped for a tool": {status: http.StatusOK, body: message(`{"items":[]}`, "tool_use")},
	}
	for name, answer := range cases {
		api := newAPI(t, answer)
		response, err := complete(t, api.client(t, nil), request())
		requireKind(t, err, anthropic.ErrInvalidResponse)
		if response.Usage.InputTokens != 120 || response.Object != nil {
			t.Errorf("%s: response = %+v", name, response)
		}
		if n := len(api.calls()); n != 1 {
			t.Errorf("%s: requests = %d, want 1", name, n)
		}
	}
}

func TestThinkingBlocksAreSkippedAndTheTextBlocksAreJoined(t *testing.T) {
	mixed := ok("")
	mixed.body = `{"id":"m","type":"message","role":"assistant","model":"model-fixture","content":[` +
		`{"type":"thinking","thinking":"","signature":"sig"},` +
		`{"type":"text","text":"{\"items\":"},{"type":"text","text":"[]}"}],` +
		`"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":2}}`
	response, err := complete(t, newAPI(t, mixed).client(t, nil), request())
	if err != nil || string(response.Object) != `{"items":[]}` {
		t.Fatalf("Complete = %s, %v", response.Object, err)
	}
}

func TestACallThatOutlivesItsBudgetTimesOutOnceWithoutARetry(t *testing.T) {
	api := newAPI(t, exchange{status: http.StatusOK, body: message(`{}`, "end_turn"), delay: 5 * time.Second})
	client := api.client(t, func(c *anthropic.Config) { c.Timeout = 60 * time.Millisecond })
	started := time.Now()
	_, err := complete(t, client, request())
	requireKind(t, err, anthropic.ErrTimeout)
	if time.Since(started) > 3*time.Second {
		t.Errorf("the call took %v against a 60ms budget", time.Since(started))
	}
	if n := len(api.calls()); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
}

func TestADocumentGetsTheLongerBudget(t *testing.T) {
	slow := exchange{status: http.StatusOK, header: map[string]string{"request-id": "req_slow"}, body: message(`{"items":[]}`, "end_turn"), delay: 250 * time.Millisecond}
	tweak := func(c *anthropic.Config) { c.Timeout = 60 * time.Millisecond; c.DocumentTimeout = 5 * time.Second }

	_, err := complete(t, newAPI(t, slow).client(t, tweak), request())
	requireKind(t, err, anthropic.ErrTimeout)

	withPDF := request()
	withPDF.Blocks = append(withPDF.Blocks, anthropic.PDFBlock([]byte("%PDF-fixture")))
	if _, err := complete(t, newAPI(t, slow).client(t, tweak), withPDF); err != nil {
		t.Fatalf("a PDF call inside its own budget failed: %v", err)
	}
}

func TestTheDefaultBudgetsAreSixtyAndOneHundredTwentySeconds(t *testing.T) {
	if anthropic.DefaultTimeout != 60*time.Second || anthropic.DefaultDocumentTimeout != 120*time.Second {
		t.Fatalf("budgets = %v and %v", anthropic.DefaultTimeout, anthropic.DefaultDocumentTimeout)
	}
}

func TestARetryAfterLongerThanTheBudgetEndsTheCallAtOnce(t *testing.T) {
	limited := failure(http.StatusTooManyRequests, "rate_limit_error", "slow down")
	limited.header["Retry-After"] = "30"
	api := newAPI(t, limited)
	client := api.client(t, func(c *anthropic.Config) { c.Timeout = 2 * time.Second })
	started := time.Now()
	_, err := complete(t, client, request())
	requireKind(t, err, anthropic.ErrRateLimited)
	if time.Since(started) > time.Second || len(api.calls()) != 1 {
		t.Errorf("waited %v over %d requests for a Retry-After the budget cannot cover", time.Since(started), len(api.calls()))
	}
}

func TestARetryAfterIsHonoured(t *testing.T) {
	limited := failure(http.StatusTooManyRequests, "rate_limit_error", "slow down")
	limited.header["Retry-After"] = "1"
	api := newAPI(t, limited, ok(`{"items":[]}`))
	started := time.Now()
	if _, err := complete(t, api.client(t, nil), request()); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if time.Since(started) < 900*time.Millisecond {
		t.Errorf("retried after %v, before the Retry-After of 1s", time.Since(started))
	}
}

func TestACancelledCallerGetsContextCanceled(t *testing.T) {
	api := newAPI(t, exchange{status: http.StatusOK, body: message(`{}`, "end_turn"), delay: 5 * time.Second})
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	_, err := api.client(t, nil).Complete(ctx, request())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	var typed *anthropic.Error
	if errors.As(err, &typed) {
		t.Errorf("a cancelled caller was reported as a provider failure: %v", typed)
	}
}

func TestAStreamThatFailsMidwayWithOverloadIsRetried(t *testing.T) {
	cut := sse(
		`message_start|{"type":"message_start","message":{"id":"m","type":"message","role":"assistant","model":"model-fixture","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}}`,
		`error|{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
	)
	api := newAPI(t, cut, streamOf(`{"items":[]}`))
	req := request()
	req.MaxTokens = 8000
	response, err := complete(t, api.client(t, nil), req)
	if err != nil || string(response.Object) != `{"items":[]}` {
		t.Fatalf("Complete = %s, %v", response.Object, err)
	}
	if n := len(api.calls()); n != 2 {
		t.Errorf("requests = %d, want 2", n)
	}
}

func TestAStreamThatEndsBeforeItsMessageIsCompleteIsUnavailable(t *testing.T) {
	early := sse(
		`message_start|{"type":"message_start","message":{"id":"m","type":"message","role":"assistant","model":"model-fixture","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1}}}`,
	)
	req := request()
	req.MaxTokens = 8000
	api := newAPI(t, early)
	_, err := complete(t, api.client(t, nil), req)
	requireKind(t, err, anthropic.ErrUnavailable)
}

func TestARequestTheClientCannotSendIsRefusedBeforeAnyRequest(t *testing.T) {
	broken := map[string]func(*anthropic.Request){
		"no model":      func(r *anthropic.Request) { r.Model = " " },
		"no max tokens": func(r *anthropic.Request) { r.MaxTokens = 0 },
		"no blocks":     func(r *anthropic.Request) { r.Blocks = nil },
		"blank text":    func(r *anthropic.Request) { r.Blocks = []anthropic.Block{anthropic.TextBlock("  ")} },
		"empty image":   func(r *anthropic.Request) { r.Blocks = []anthropic.Block{anthropic.ImageBlock("image/png", nil)} },
		"bmp image": func(r *anthropic.Request) {
			r.Blocks = []anthropic.Block{anthropic.ImageBlock("image/bmp", []byte("x"))}
		},
		"empty pdf":      func(r *anthropic.Request) { r.Blocks = []anthropic.Block{anthropic.PDFBlock(nil)} },
		"unknown kind":   func(r *anthropic.Request) { r.Blocks = []anthropic.Block{{Kind: "video"}} },
		"no schema":      func(r *anthropic.Request) { r.Schema = nil },
		"schema array":   func(r *anthropic.Request) { r.Schema = []byte(`[]`) },
		"schema trailer": func(r *anthropic.Request) { r.Schema = []byte(`{"type":"object"} {}`) },
		"schema empty":   func(r *anthropic.Request) { r.Schema = []byte(`{}`) },
	}
	for name, mutate := range broken {
		api := newAPI(t, ok(`{}`))
		req := request()
		mutate(&req)
		_, err := complete(t, api.client(t, nil), req)
		requireKind(t, err, anthropic.ErrInvalidRequest)
		if n := len(api.calls()); n != 0 {
			t.Errorf("%s: %d requests were sent", name, n)
		}
	}
}

func TestTheClientReadsNothingFromTheEnvironment(t *testing.T) {
	decoy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("a request reached the host named by the environment")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(decoy.Close)
	t.Setenv("ANTHROPIC_BASE_URL", decoy.URL)
	t.Setenv("ANTHROPIC_API_KEY", "environment-key-fixture")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "environment-token-fixture")
	t.Setenv("ANTHROPIC_CUSTOM_HEADERS", "X-Decoy: 1")
	api := newAPI(t, ok(`{"items":[]}`))
	if _, err := complete(t, api.client(t, nil), request()); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	header := api.calls()[0].header
	if header.Get("X-Api-Key") != testKey || header.Get("Authorization") != "" || header.Get("X-Decoy") != "" {
		t.Errorf("the environment reached the request: %v", header)
	}
}

func TestNewRefusesWhatCannotWork(t *testing.T) {
	for name, cfg := range map[string]anthropic.Config{
		"no key":           {},
		"key with a space": {APIKey: "two words"},
		"key with newline": {APIKey: "key\n"},
		"relative base":    {APIKey: testKey, BaseURL: "/v1"},
		"ftp base":         {APIKey: testKey, BaseURL: "ftp://example.test"},
	} {
		_, err := anthropic.New(cfg)
		if err == nil {
			t.Errorf("%s: New succeeded", name)
		} else if strings.Contains(err.Error(), testKey) {
			t.Errorf("%s: the refusal echoes the key: %v", name, err)
		}
	}
}

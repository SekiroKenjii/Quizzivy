// Package anthropic is the platform client for Anthropic's Messages API: one structured completion per call, with retries on 429 and 5xx, streaming for long outputs, the usage the provider reports and errors that never carry the API key. It is the only package that imports the provider's SDK.
package anthropic

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
)

const (
	// DefaultTimeout is the time budget of one Complete call, retries included.
	DefaultTimeout = 60 * time.Second
	// DefaultDocumentTimeout is the budget of a call that carries a PDF.
	DefaultDocumentTimeout = 120 * time.Second
	// DefaultRetryBackoff is the wait before the first retry; each retry waits twice as long as the one before.
	DefaultRetryBackoff = time.Second
	// StreamingThreshold is the largest MaxTokens a call makes without streaming.
	StreamingThreshold = 4096
	// MaxRetries is how many times a 429 or a 5xx is sent again.
	MaxRetries = 3

	maxRetryAfter = 30 * time.Second
)

// BlockKind says what a Block holds.
type BlockKind string

// The kinds of content a request can carry.
const (
	BlockText  BlockKind = "text"
	BlockImage BlockKind = "image"
	BlockPDF   BlockKind = "pdf"
)

// Block is one piece of the user's turn: text, an image with its media type (JPEG, PNG, GIF or WebP) or a PDF document.
type Block struct {
	Kind      BlockKind
	Text      string
	MediaType string
	Data      []byte
}

// TextBlock is a block of text.
func TextBlock(text string) Block { return Block{Kind: BlockText, Text: text} }

// ImageBlock is an image of the given media type.
func ImageBlock(mediaType string, data []byte) Block {
	return Block{Kind: BlockImage, MediaType: mediaType, Data: data}
}

// PDFBlock is a PDF document.
func PDFBlock(data []byte) Block {
	return Block{Kind: BlockPDF, MediaType: "application/pdf", Data: data}
}

// Request is one completion: the model, a system prompt, the user's turn and the JSON Schema the answer must follow. Schema must stay inside the provider's structured-output subset of JSON Schema; the provider refuses anything else with a 4xx, which is ErrInvalidRequest. Its properties are sent in the order written, and the model writes them in that order.
type Request struct {
	Model     string
	System    string
	Blocks    []Block
	Schema    json.RawMessage
	MaxTokens int
}

// Response is a completed answer: Object is one valid JSON object, Usage what the provider reported, RequestID the provider's id for the call.
type Response struct {
	Object    json.RawMessage
	Usage     Usage
	RequestID string
}

// Config configures a Client. Only APIKey is required. BaseURL, HTTPClient and the durations exist for tests; production leaves them zero. Logger receives one line per retry and is silent when nil.
type Config struct {
	APIKey          string
	BaseURL         string
	HTTPClient      *http.Client
	Timeout         time.Duration
	DocumentTimeout time.Duration
	RetryBackoff    time.Duration
	Logger          *slog.Logger
}

func (Config) String() string { return "anthropic.Config" }

func (Config) GoString() string { return "anthropic.Config" }

// Client sends completions to the Messages API. It is safe for concurrent use.
type Client struct {
	sdk             sdk.Client
	scrub           scrubber
	timeout         time.Duration
	documentTimeout time.Duration
	backoff         time.Duration
	logger          *slog.Logger
}

func (*Client) String() string { return "anthropic.Client" }

func (*Client) GoString() string { return "anthropic.Client" }

// New builds a client. It reads nothing from the environment, so a stray ANTHROPIC_* variable cannot redirect the key or add a credential source.
func New(cfg Config) (*Client, error) {
	if cfg.APIKey == "" || strings.IndexFunc(cfg.APIKey, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return nil, errors.New("anthropic: an API key without whitespace is required")
	}
	options := []option.RequestOption{option.WithoutEnvironmentDefaults(), option.WithAPIKey(cfg.APIKey), option.WithMaxRetries(0)}
	if cfg.BaseURL != "" {
		base, err := url.Parse(cfg.BaseURL)
		if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
			return nil, errors.New("anthropic: BaseURL must be an http(s) URL")
		}
		options = append(options, option.WithBaseURL(cfg.BaseURL))
	}
	if cfg.HTTPClient != nil {
		options = append(options, option.WithHTTPClient(cfg.HTTPClient))
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Client{
		sdk:             sdk.NewClient(options...),
		scrub:           scrubber{key: cfg.APIKey},
		timeout:         orDefault(cfg.Timeout, DefaultTimeout),
		documentTimeout: orDefault(cfg.DocumentTimeout, DefaultDocumentTimeout),
		backoff:         orDefault(cfg.RetryBackoff, DefaultRetryBackoff),
		logger:          logger,
	}, nil
}

func orDefault(d, fallback time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return fallback
}

// Complete sends the request and returns the provider's answer as one JSON object. It streams when MaxTokens exceeds StreamingThreshold, sends a 429 or a 5xx again up to MaxRetries times with a doubling back-off that honours Retry-After, never sends a 4xx again, and gives every attempt together the client's Timeout (DocumentTimeout when a block is a PDF). A cancelled ctx returns context.Canceled; every other failure is an *Error whose Kind is one of the package's sentinels. With ErrRefused and ErrInvalidResponse the Response still carries Usage and RequestID, because the provider billed those tokens and a caller that meters usage settles what was spent; with any other error it is the zero value.
func (c *Client) Complete(ctx context.Context, req Request) (Response, error) {
	params, err := buildParams(req)
	if err != nil {
		return Response{}, c.scrub.newError(ErrInvalidRequest, 0, "", err.Error())
	}
	budget := c.timeout
	if hasPDF(req.Blocks) {
		budget = c.documentTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	streamed := req.MaxTokens > StreamingThreshold
	for attempt := 0; ; attempt++ {
		response, failed := c.attempt(callCtx, params, streamed)
		if failed == nil {
			return response, nil
		}
		if !failed.retryable || attempt == MaxRetries {
			return response, failed.err
		}
		if !c.wait(callCtx, attempt, failed) {
			if interrupted := c.interrupted(callCtx); interrupted != nil {
				return Response{}, interrupted
			}
			return response, failed.err
		}
	}
}

func (c *Client) interrupted(ctx context.Context) error {
	switch err := ctx.Err(); {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return context.Canceled
	default:
		return c.scrub.newError(ErrTimeout, 0, "", "")
	}
}

func (c *Client) attempt(ctx context.Context, params sdk.MessageNewParams, streamed bool) (Response, *failure) {
	message, requestID, err := c.send(ctx, params, streamed)
	if err != nil {
		if interrupted := c.interrupted(ctx); interrupted != nil {
			return Response{}, &failure{err: interrupted}
		}
		failed := c.scrub.fromSDK(err, requestID)
		return Response{}, &failed
	}
	return c.read(message, requestID)
}

func (c *Client) send(ctx context.Context, params sdk.MessageNewParams, streamed bool) (sdk.Message, string, error) {
	var requestID string
	capture := option.WithMiddleware(func(r *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		response, err := next(r)
		if response != nil {
			requestID = response.Header.Get("request-id")
		}
		return response, err
	})
	if !streamed {
		message, err := c.sdk.Messages.New(ctx, params, capture)
		if err != nil {
			return sdk.Message{}, requestID, err
		}
		return *message, requestID, nil
	}
	stream := c.sdk.Messages.NewStreaming(ctx, params, capture)
	defer func() { _ = stream.Close() }()
	var message sdk.Message
	for stream.Next() {
		if err := message.Accumulate(stream.Current()); err != nil {
			return sdk.Message{}, requestID, fmt.Errorf("%w: %v", errMalformedStream, err)
		}
	}
	return message, requestID, stream.Err()
}

func (c *Client) read(message sdk.Message, requestID string) (Response, *failure) {
	response := Response{Usage: usageOf(message.Usage), RequestID: requestID}
	switch message.StopReason {
	case sdk.StopReasonEndTurn, sdk.StopReasonStopSequence:
	case sdk.StopReasonRefusal:
		return response, c.rejected(ErrRefused, requestID, "category "+string(message.StopDetails.Category), false)
	case sdk.StopReasonMaxTokens:
		return response, c.rejected(ErrInvalidResponse, requestID, "the answer reached max_tokens before the JSON was complete", false)
	case "":
		return response, c.rejected(ErrUnavailable, requestID, "the stream ended before the message was complete", true)
	default:
		return response, c.rejected(ErrInvalidResponse, requestID, "unexpected stop reason "+string(message.StopReason), false)
	}
	object, ok := jsonObject(answerText(message))
	if !ok {
		return response, c.rejected(ErrInvalidResponse, requestID, "the answer is not one JSON object", false)
	}
	response.Object = object
	return response, nil
}

func (c *Client) rejected(kind error, requestID, message string, retryable bool) *failure {
	return &failure{err: c.scrub.newError(kind, 0, requestID, message), retryable: retryable}
}

func answerText(message sdk.Message) string {
	var text strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	return text.String()
}

func jsonObject(text string) (json.RawMessage, bool) {
	trimmed := bytes.TrimSpace([]byte(text))
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return nil, false
	}
	return json.RawMessage(trimmed), true
}

func (c *Client) wait(ctx context.Context, attempt int, failed *failure) bool {
	delay := c.backoff << attempt
	delay += time.Duration(rand.Int64N(int64(delay)/5 + 1))
	if hint, ok := parseRetryAfter(failed.retryAfter, time.Now()); ok {
		delay = max(delay, min(hint, maxRetryAfter))
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= delay {
		return false
	}
	var apiErr *Error
	if errors.As(failed.err, &apiErr) {
		c.logger.Warn("anthropic_retry", "attempt", attempt+1, "kind", apiErr.Kind.Error(), "status", apiErr.Status, "request_id", apiErr.RequestID, "delay", delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second, true
	}
	if at, err := http.ParseTime(value); err == nil {
		return max(at.Sub(now), 0), true
	}
	return 0, false
}

func hasPDF(blocks []Block) bool {
	for _, block := range blocks {
		if block.Kind == BlockPDF {
			return true
		}
	}
	return false
}

var imageMediaTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/gif": true, "image/webp": true}

func buildParams(req Request) (sdk.MessageNewParams, error) {
	if strings.TrimSpace(req.Model) == "" {
		return sdk.MessageNewParams{}, errors.New("a model is required")
	}
	if req.MaxTokens < 1 {
		return sdk.MessageNewParams{}, errors.New("the request needs a MaxTokens of at least 1")
	}
	schema, err := compactSchema(req.Schema)
	if err != nil {
		return sdk.MessageNewParams{}, err
	}
	blocks, err := contentBlocks(req.Blocks)
	if err != nil {
		return sdk.MessageNewParams{}, err
	}
	params := sdk.MessageNewParams{
		Model:        sdk.Model(req.Model),
		MaxTokens:    int64(req.MaxTokens),
		Messages:     []sdk.MessageParam{sdk.NewUserMessage(blocks...)},
		OutputConfig: sdk.OutputConfigParam{Format: param.Override[sdk.JSONOutputFormatParam](json.RawMessage(`{"type":"json_schema","schema":` + string(schema) + `}`))},
	}
	if req.System != "" {
		params.System = []sdk.TextBlockParam{{Text: req.System}}
	}
	return params, nil
}

func compactSchema(raw json.RawMessage) (json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var schema map[string]any
	if err := decoder.Decode(&schema); err != nil || len(schema) == 0 || decoder.More() {
		return nil, errors.New("the request needs a Schema that is one non-empty JSON object")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, bytes.TrimSpace(raw)); err != nil {
		return nil, errors.New("the request needs a Schema that is one non-empty JSON object")
	}
	return compact.Bytes(), nil
}

func contentBlocks(blocks []Block) ([]sdk.ContentBlockParamUnion, error) {
	if len(blocks) == 0 {
		return nil, errors.New("at least one content block is required")
	}
	out := make([]sdk.ContentBlockParamUnion, 0, len(blocks))
	for i, block := range blocks {
		converted, err := contentBlock(block)
		if err != nil {
			return nil, fmt.Errorf("block %d: %w", i, err)
		}
		out = append(out, converted)
	}
	return out, nil
}

func contentBlock(block Block) (sdk.ContentBlockParamUnion, error) {
	switch block.Kind {
	case BlockText:
		if strings.TrimSpace(block.Text) == "" {
			return sdk.ContentBlockParamUnion{}, errors.New("a text block needs text")
		}
		return sdk.NewTextBlock(block.Text), nil
	case BlockImage:
		if !imageMediaTypes[block.MediaType] || len(block.Data) == 0 {
			return sdk.ContentBlockParamUnion{}, errors.New("an image block needs data and a JPEG, PNG, GIF or WebP media type")
		}
		return sdk.NewImageBlockBase64(block.MediaType, base64.StdEncoding.EncodeToString(block.Data)), nil
	case BlockPDF:
		if len(block.Data) == 0 {
			return sdk.ContentBlockParamUnion{}, errors.New("a PDF block needs data")
		}
		return sdk.NewDocumentBlock(sdk.Base64PDFSourceParam{Data: base64.StdEncoding.EncodeToString(block.Data)}), nil
	}
	return sdk.ContentBlockParamUnion{}, fmt.Errorf("unknown block kind %q", block.Kind)
}

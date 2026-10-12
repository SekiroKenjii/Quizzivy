// Package ports declares what the AI assistant needs from outside its own module. The assistant suggests and meters: it reads through ports typed as other modules' handlers, writes only its own ledger, and reaches the language-model provider only through Completer.
package ports

import (
	"context"
	"encoding/json"
	"errors"
)

// Tier names a class of model. The deployment, not the code, decides which model serves each and what it costs.
type Tier string

// The two tiers: Fast for cheap bulk work, Capable for work that needs judgement or sight.
const (
	TierFast    Tier = "fast"
	TierCapable Tier = "capable"
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
func PDFBlock(data []byte) Block { return Block{Kind: BlockPDF, Data: data} }

// Request is one structured completion: the tier, a system prompt, the user's turn, the JSON Schema the answer must follow and the most tokens the answer may use. Schema must stay inside the provider's structured-output subset of JSON Schema.
type Request struct {
	Tier      Tier
	System    string
	Blocks    []Block
	Schema    json.RawMessage
	MaxTokens int
}

// Usage is what the provider reported for a completion. InputTokens counts only the tokens that were neither read from nor written to the prompt cache, so the four counts never overlap.
type Usage struct {
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
}

// Response is a completed answer: Object is one valid JSON object that the caller decodes strictly, Usage is what to meter and RequestID is the provider's id for the call, for the ledger.
type Response struct {
	Object    json.RawMessage
	Usage     Usage
	RequestID string
}

// Completer asks the provider for one structured completion; a nil Completer means the assistant is off, so every AI operation answers 501 and nothing is metered. It returns the errors below, each joined with the provider's own detail, which never holds the API key, and context.Canceled for a cancelled ctx. With ErrRefused and ErrInvalidResponse the Response still carries Usage and RequestID, because the provider billed those tokens; with any other error it is the zero value.
type Completer interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

var (
	// ErrUnavailable is a failure a later attempt may cure: the provider was down, overloaded or rate limiting after every retry, or the call outlived its time budget.
	ErrUnavailable = errors.New("ai: the provider is unavailable")
	// ErrRefused is a completion the provider ended with a refusal.
	ErrRefused = errors.New("ai: the model refused the request")
	// ErrInvalidResponse is an answer that is not one complete JSON object.
	ErrInvalidResponse = errors.New("ai: the answer is not a complete JSON object")
	// ErrRejected is a request or a key the provider refused outright: a defect in the prompt, the schema or the configuration that retrying cannot cure and an operator must read.
	ErrRejected = errors.New("ai: the provider rejected the request")
)

package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/platform/anthropic"
	"quizzivy/internal/platform/config"
)

// AI adapts the platform Anthropic client to the ai module's Completer port: it names the model that serves each tier, translates the request and the usage, and turns the client's failures into the port's errors.
type AI struct {
	client *anthropic.Client
	models map[ports.Tier]string
}

// NewAI returns the Completer for the configured provider, and a nil Completer when none is configured: the assistant is then off, as media is without storage, and every AI operation answers 501.
func NewAI(cfg config.AI, logger *slog.Logger) (ports.Completer, error) {
	if !cfg.Enabled() {
		return nil, nil
	}
	client, err := anthropic.New(anthropic.Config{APIKey: cfg.APIKey.Reveal(), Logger: logger})
	if err != nil {
		return nil, err
	}
	return NewAIOver(client, cfg.Fast.Model, cfg.Capable.Model), nil
}

// NewAIOver adapts a client that is already built, serving the fast tier with fastModel and the capable tier with capableModel.
func NewAIOver(client *anthropic.Client, fastModel, capableModel string) AI {
	return AI{client: client, models: map[ports.Tier]string{ports.TierFast: fastModel, ports.TierCapable: capableModel}}
}

func (a AI) Complete(ctx context.Context, req ports.Request) (ports.Response, error) {
	model, ok := a.models[req.Tier]
	if !ok {
		return ports.Response{}, fmt.Errorf("%w: unknown tier %q", ports.ErrRejected, req.Tier)
	}
	response, err := a.client.Complete(ctx, anthropic.Request{
		Model:     model,
		System:    req.System,
		Blocks:    providerBlocks(req.Blocks),
		Schema:    req.Schema,
		MaxTokens: req.MaxTokens,
	})
	return ports.Response{
		Object:    response.Object,
		RequestID: response.RequestID,
		Usage: ports.Usage{
			InputTokens:      response.Usage.InputTokens,
			OutputTokens:     response.Usage.OutputTokens,
			CacheReadTokens:  response.Usage.CacheReadTokens,
			CacheWriteTokens: response.Usage.CacheWriteTokens,
		},
	}, aiError(err)
}

func providerBlocks(blocks []ports.Block) []anthropic.Block {
	out := make([]anthropic.Block, len(blocks))
	for i, block := range blocks {
		out[i] = anthropic.Block{Kind: anthropic.BlockKind(block.Kind), Text: block.Text, MediaType: block.MediaType, Data: block.Data}
		if block.Kind == ports.BlockPDF {
			out[i].MediaType = "application/pdf"
		}
	}
	return out
}

var aiErrorKinds = []struct{ provider, port error }{
	{anthropic.ErrRefused, ports.ErrRefused},
	{anthropic.ErrInvalidResponse, ports.ErrInvalidResponse},
	{anthropic.ErrRateLimited, ports.ErrUnavailable},
	{anthropic.ErrUnavailable, ports.ErrUnavailable},
	{anthropic.ErrTimeout, ports.ErrUnavailable},
	{anthropic.ErrAuth, ports.ErrRejected},
	{anthropic.ErrInvalidRequest, ports.ErrRejected},
}

func aiError(err error) error {
	for _, kind := range aiErrorKinds {
		if errors.Is(err, kind.provider) {
			return fmt.Errorf("%w: %w", kind.port, err)
		}
	}
	return err
}

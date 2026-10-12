package wiring

import (
	"log/slog"

	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/platform/config"
)

// ai returns the provider behind the AI assistant's port, or a nil Completer when no ANTHROPIC_API_KEY is set: the assistant is then off, as media is without storage, and every AI operation answers 501. The log lines name the models, never the key.
func ai(cfg config.Config, logger *slog.Logger) (ports.Completer, error) {
	completer, err := adapters.NewAI(cfg.AI, logger)
	if err != nil {
		return nil, err
	}
	if completer == nil {
		logger.Info("ai assistant disabled (no ANTHROPIC_API_KEY)")
		return nil, nil
	}
	logger.Info("ai assistant enabled", "fast_model", cfg.AI.Fast.Model, "capable_model", cfg.AI.Capable.Model)
	return completer, nil
}

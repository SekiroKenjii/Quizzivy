package wiring

import (
	"log/slog"

	"quizzivy/internal/core/adapters"
	aiapp "quizzivy/internal/modules/ai/application"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/domain"
	airepo "quizzivy/internal/modules/ai/repositories"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
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

func aiCredits(cfg config.Config, dbx db.Context, notifier ports.Notifier, logger *slog.Logger) (*aiapp.Application, error) {
	zone, err := config.AppLocation()
	if err != nil {
		return nil, err
	}
	return aiapp.New(airepo.NewPostgres(dbx), aiapp.Options{
		Prices:   aiPrices(cfg.AI),
		Zone:     zone,
		Notifier: notifier,
		Logger:   logger,
	}), nil
}

func aiPrices(cfg config.AI) domain.Prices {
	return domain.Prices{Fast: priceOf(cfg.Fast.Prices), Capable: priceOf(cfg.Capable.Prices)}
}

func priceOf(p config.TierPrices) domain.Price {
	return domain.Price{Input: int64(p.Input), Output: int64(p.Output), CacheRead: int64(p.CacheRead)}
}

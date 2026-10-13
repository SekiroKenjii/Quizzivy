package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"quizzivy/internal/modules/ai/application/command"
	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/shared/actor"
)

const finishBudget = 10 * time.Second

type asker struct{ *support.Service }

type answer struct {
	Object json.RawMessage
	Usage  domain.Usage
}

func (a asker) ask(ctx context.Context, who actor.Actor, p prompts.Prompt) (answer, error) {
	if a.Completer == nil {
		return answer{}, domain.ErrOff
	}
	reservation, err := command.ReserveHandler{Service: a.Service}.Handle(ctx, command.Reserve{Actor: who, Feature: p.Feature, Ceiling: p.Feature.Ceiling()})
	if err != nil {
		return answer{}, err
	}
	tier := ports.Tier(p.Tier)
	response, callErr := a.Completer.Complete(ctx, ports.Request{
		Tier: tier, System: p.System, Blocks: []ports.Block{ports.TextBlock(p.User)}, Schema: p.Schema, MaxTokens: p.MaxTokens,
	})

	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishBudget)
	defer cancel()
	settled, finishErr := command.FinishHandler{Service: a.Service}.Handle(finishCtx, command.Finish{Reservation: reservation, Tier: tier, Response: response, Err: callErr})
	if finishErr != nil {
		a.Logger.ErrorContext(ctx, "ai reservation not ended", "feature", p.Feature, "reservation", reservation.ID, "error", finishErr)
	}
	if callErr != nil {
		return answer{}, a.failure(ctx, p.Feature, response.RequestID, callErr)
	}
	left := int64(0)
	if usage, err := a.Repo.Usage(ctx, a.Month(), who.ID); err == nil {
		left = usage.Left
	}
	return answer{Object: response.Object, Usage: domain.Usage{Credits: settled.Charged, Left: left}}, nil
}

func (a asker) failure(ctx context.Context, feature domain.Feature, requestID string, err error) error {
	switch {
	case errors.Is(err, ports.ErrUnavailable), errors.Is(err, ports.ErrRefused), errors.Is(err, ports.ErrInvalidResponse):
		a.Logger.WarnContext(ctx, "ai call failed", "feature", feature, "request_id", requestID, "error", err)
		return fmt.Errorf("%w: %w", domain.ErrUnavailable, err)
	case ctx.Err() != nil:
		return ctx.Err()
	}
	a.Logger.ErrorContext(ctx, "ai call rejected", "feature", feature, "request_id", requestID, "error", err)
	return fmt.Errorf("ai: %s: %w", feature, err)
}

func unusable(err error) error {
	if errors.Is(err, prompts.ErrUnusable) {
		return fmt.Errorf("%w: %w", domain.ErrUnavailable, err)
	}
	return err
}

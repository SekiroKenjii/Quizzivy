package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
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

type session struct {
	asker
	feature     domain.Feature
	tier        ports.Tier
	reservation domain.Reservation
	tally       command.Tally
}

type outcome struct {
	Response ports.Response
	Err      error
}

func (a asker) ask(ctx context.Context, who actor.Actor, p prompts.Prompt) (answer, error) {
	if a.Completer == nil {
		return answer{}, domain.ErrOff
	}
	var response ports.Response
	usage, err := a.session(ctx, who, p.Feature, ports.Tier(p.Tier), func(s *session) error {
		var callErr error
		response, callErr = s.call(ctx, p)
		if callErr != nil {
			return a.failure(ctx, p.Feature, response.RequestID, callErr)
		}
		return nil
	})
	if err != nil {
		return answer{}, err
	}
	return answer{Object: response.Object, Usage: usage}, nil
}

func (a asker) session(ctx context.Context, who actor.Actor, feature domain.Feature, tier ports.Tier, body func(*session) error) (usage domain.Usage, err error) {
	reservation, err := command.ReserveHandler{Service: a.Service}.Handle(ctx, command.Reserve{Actor: who, Feature: feature, Ceiling: feature.Ceiling()})
	if err != nil {
		return domain.Usage{}, err
	}
	s := &session{asker: a, feature: feature, tier: tier, reservation: reservation}
	returned := false
	defer func() {
		endCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finishBudget)
		defer cancel()
		settled := s.end(endCtx)
		if returned && err == nil {
			usage = domain.Usage{Credits: settled.Charged, Left: a.left(endCtx, who.ID)}
		}
	}()
	err = body(s)
	returned = true
	return usage, err
}

func (a asker) left(ctx context.Context, userID string) int64 {
	usage, err := a.Repo.Usage(ctx, a.Month(), userID)
	if err != nil {
		return 0
	}
	return usage.Left
}

func (s *session) end(ctx context.Context) domain.Settled {
	settled, err := command.FinishHandler{Service: s.Service}.Handle(ctx, command.Finish{
		Reservation: s.reservation, Tier: s.tier, Response: s.tally.Response(),
	})
	if err != nil {
		s.Logger.ErrorContext(ctx, "ai reservation not ended", "feature", s.feature, "reservation", s.reservation.ID, "error", err)
	}
	return settled
}

func (s *session) call(ctx context.Context, p prompts.Prompt) (ports.Response, error) {
	response, err := s.Completer.Complete(ctx, ports.Request{
		Tier: s.tier, System: p.System, Blocks: []ports.Block{ports.TextBlock(p.User)}, Schema: p.Schema, MaxTokens: p.MaxTokens,
	})
	s.tally.Add(response, err)
	return response, err
}

type crash struct {
	value any
	stack []byte
}

func (c *crash) Error() string {
	return fmt.Sprintf("ai: a provider call panicked: %v\n%s", c.value, c.stack)
}

func (s *session) callAll(ctx context.Context, calls []prompts.Prompt) []outcome {
	outcomes := make([]outcome, len(calls))
	crashes := make([]*crash, len(calls))
	var wg sync.WaitGroup
	for i := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					crashes[i] = &crash{value: recovered, stack: debug.Stack()}
				}
			}()
			outcomes[i].Response, outcomes[i].Err = s.call(ctx, calls[i])
		}()
	}
	wg.Wait()
	for _, c := range crashes {
		if c != nil {
			panic(c)
		}
	}
	return outcomes
}

func (a asker) failure(ctx context.Context, feature domain.Feature, requestID string, err error) error {
	switch {
	case errors.Is(err, ports.ErrRejected):
		a.Logger.ErrorContext(ctx, "ai call rejected", "feature", feature, "request_id", requestID, "error", err)
		return fmt.Errorf("%w: %w", domain.ErrUnavailable, err)
	case errors.Is(err, ports.ErrUnavailable), errors.Is(err, ports.ErrRefused), errors.Is(err, ports.ErrInvalidResponse), errors.Is(err, context.DeadlineExceeded):
		a.Logger.WarnContext(ctx, "ai call failed", "feature", feature, "request_id", requestID, "error", err)
		return fmt.Errorf("%w: %w", domain.ErrUnavailable, err)
	case ctx.Err() != nil:
		return ctx.Err()
	}
	a.Logger.ErrorContext(ctx, "ai call failed unexpectedly", "feature", feature, "request_id", requestID, "error", err)
	return fmt.Errorf("ai: %s: %w", feature, err)
}

func unusable(err error) error {
	if errors.Is(err, prompts.ErrUnusable) {
		return fmt.Errorf("%w: %w", domain.ErrUnavailable, err)
	}
	return err
}

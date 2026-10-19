package query

import (
	"context"
	"slices"
	"strings"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/shared/actor"
)

// VariantCallsInFlight is the most requests one variant has with the provider at the same time.
const VariantCallsInFlight = 5

// Variant proposes rewordings, reorderings and new wrong options for the standalone questions of the draft of TestID, which the caller copies with the existing duplicate write and applies to the copy. The members of a group are not varied: a copy of the test carries them as they are, and a draft whose questions are all members answers domain.ErrNothingToRead. It changes no answer, point or passage and writes nothing. It answers domain.ErrNotFound for a test the caller does not reach. One variant is one bounded pass, however long the draft: it asks about prompts.VariantChunkSize questions in each request, at most VariantCallsInFlight at the same time, within Service.CleanupBudget, under one reservation of the feature's ceiling that ends with one Finish of what all of them were billed. The changes of every request that answered usably are returned in paper order, so a request that failed or ran out of time leaves its questions as they are. The pass answers domain.ErrUnavailable only when no request gave a usable change.
type Variant struct {
	Actor      actor.Actor
	TestID     string
	Changes    []string
	Difficulty string
}

// Varied is the proposed changes in paper order and what the call cost.
type Varied struct {
	Changes []prompts.Change
	Usage   domain.Usage
}

type VariantHandler struct{ *support.Service }

func (h VariantHandler) Handle(ctx context.Context, q Variant) (Varied, error) {
	if h.Completer == nil {
		return Varied{}, domain.ErrOff
	}
	paper, err := h.Tests.Draft(ctx, q.Actor.Scope, q.TestID)
	if err != nil {
		return Varied{}, err
	}
	calls, parts, err := prompts.VariantCalls(prompts.VariantInput{Questions: standalone(paper.Questions), Changes: q.Changes, Difficulty: q.Difficulty})
	if err != nil {
		return Varied{}, err
	}
	var changes []prompts.Change
	usage, err := asker(h).session(ctx, q.Actor, domain.FeatureVariant, ports.Tier(calls[0].Tier), func(s *session) error {
		var runErr error
		changes, runErr = h.run(ctx, s, calls, parts)
		return runErr
	})
	if err != nil {
		return Varied{}, err
	}
	return Varied{Changes: changes, Usage: usage}, nil
}

func (h VariantHandler) run(ctx context.Context, s *session, calls []prompts.Prompt, parts []prompts.VariantInput) ([]prompts.Change, error) {
	passCtx, cancel := context.WithTimeout(ctx, h.CleanupBudget)
	defer cancel()
	outcomes := s.callEach(passCtx, calls, VariantCallsInFlight)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var changes []prompts.Change
	usable, requestIDs := 0, make([]string, 0, len(outcomes))
	var firstFailure error
	for i, o := range outcomes {
		requestIDs = append(requestIDs, o.Response.RequestID)
		var failure error
		if o.Err != nil {
			failure = asker(h).failure(ctx, domain.FeatureVariant, o.Response.RequestID, o.Err)
		} else if found, readErr := prompts.DecodeVariant(o.Response.Object, parts[i]); readErr != nil {
			failure = asker(h).unusable(ctx, domain.FeatureVariant, o.Response.RequestID, readErr)
		} else {
			changes = append(changes, found...)
		}
		if failure == nil {
			usable++
		} else if firstFailure == nil {
			firstFailure = failure
		}
	}
	h.Logger.InfoContext(ctx, "ai variant", "calls", len(outcomes), "usable", usable, "spent_the_budget", passCtx.Err() != nil, "request_ids", strings.Join(requestIDs, ","))
	if usable == 0 {
		return nil, firstFailure
	}
	slices.SortStableFunc(changes, func(a, b prompts.Change) int { return a.Number - b.Number })
	if len(changes) > prompts.MaxChanges {
		changes = changes[:prompts.MaxChanges]
	}
	return changes, nil
}

func standalone(questions []domain.PaperQuestion) []domain.PaperQuestion {
	var out []domain.PaperQuestion
	for _, q := range questions {
		if q.GroupID == "" {
			out = append(out, q)
		}
	}
	return out
}

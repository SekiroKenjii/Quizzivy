package query

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/shared/actor"
)

// MaxCleanupQuestions is the most of a bank one clean-up reads.
const MaxCleanupQuestions = 2000

// Cleanup finds near-duplicates and untagged questions among the caller's own questions, newest first, at most MaxCleanupQuestions of them, even for a caller whose scope reaches everyone's; questions shared with the caller are skipped. It changes nothing, and it answers domain.ErrNothingToRead for a caller who owns none. One request is one bounded pass, however large the bank: it reserves the feature's ceiling once, makes at most five calls to the provider at the same time, within Service.CleanupBudget, and ends the reservation with one Finish of what all of them were billed. Tags are asked of the first prompts.MaxTagsAsked questions without any, prompts.TagChunkSize to a call. Duplicates are not searched for chunk by chunk: every pair of the bank is compared by its words (domain.SimilarQuestions), so two questions never in the same chunk are compared all the same, and one call judges the closest pairs. The result is Truncated when the pass left something undone: the bank held more than it read, more questions without tags than it asked about, the reservation could not cover every call at the tier's prices, the time ran out, or a call failed. The pass answers domain.ErrUnavailable only when no call gave a usable answer, and then settles only what the provider billed.
type Cleanup struct {
	Actor    actor.Actor
	Checks   []string
	Language domain.Language
}

// Cleaned is what the clean-up found, how many questions it read, whether the pass left something undone, and what the pass cost.
type Cleaned struct {
	Scanned   int
	Truncated bool
	Findings  prompts.CleanupFindings
	Usage     domain.Usage
}

type CleanupHandler struct{ *support.Service }

type cleanupStep struct {
	prompt prompts.Prompt
	read   func(json.RawMessage, *prompts.CleanupFindings) error
}

type cleanupPlan struct {
	steps []cleanupStep
	cut   bool
}

func (h CleanupHandler) Handle(ctx context.Context, q Cleanup) (Cleaned, error) {
	if h.Completer == nil {
		return Cleaned{}, domain.ErrOff
	}
	bank, err := h.Questions.Own(ctx, q.Actor.Scope, MaxCleanupQuestions)
	if err != nil {
		return Cleaned{}, err
	}
	if len(bank.Questions) == 0 {
		return Cleaned{}, domain.ErrNothingToRead
	}
	plan, err := h.plan(ctx, q, bank.Questions)
	if err != nil {
		return Cleaned{}, err
	}
	var found prompts.CleanupFindings
	var complete bool
	usage, err := asker(h).session(ctx, q.Actor, domain.FeatureCleanup, ports.TierFast, func(s *session) error {
		var runErr error
		found, complete, runErr = h.run(ctx, s, plan)
		return runErr
	})
	if err != nil {
		return Cleaned{}, err
	}
	return Cleaned{Scanned: len(bank.Questions), Truncated: bank.Truncated || plan.cut || !complete, Findings: found, Usage: usage}, nil
}

func (h CleanupHandler) plan(ctx context.Context, q Cleanup, bank []domain.BankQuestion) (cleanupPlan, error) {
	var steps []cleanupStep
	more := false
	if slices.Contains(q.Checks, prompts.CleanupDuplicates) {
		step, err := h.duplicatesStep(ctx, q, bank)
		if err != nil {
			return cleanupPlan{}, err
		}
		steps = append(steps, step...)
	}
	if slices.Contains(q.Checks, prompts.CleanupTags) {
		tagging, left, err := taggingSteps(bank)
		if err != nil {
			return cleanupPlan{}, err
		}
		steps, more = append(steps, tagging...), left
	}
	kept, trimmed, err := h.fitReservation(steps)
	if err != nil {
		return cleanupPlan{}, err
	}
	if len(steps) > 0 && len(kept) == 0 {
		h.Logger.ErrorContext(ctx, "ai clean-up: the smallest call can cost more than the reservation", "ceiling", domain.FeatureCleanup.Ceiling())
		return cleanupPlan{}, fmt.Errorf("%w: no call of the clean-up fits its reservation at the configured prices", domain.ErrUnavailable)
	}
	return cleanupPlan{steps: kept, cut: more || trimmed}, nil
}

func (h CleanupHandler) duplicatesStep(ctx context.Context, q Cleanup, bank []domain.BankQuestion) ([]cleanupStep, error) {
	pairs := domain.SimilarQuestions(bank, prompts.MaxCandidatePairs)
	if len(pairs) == 0 {
		return nil, nil
	}
	in := prompts.DuplicatesInput{Bank: bank, Pairs: pairs, Language: h.LanguageFor(ctx, q.Actor.ID, q.Language)}
	prompt, err := prompts.Duplicates(in)
	if err != nil {
		return nil, err
	}
	return []cleanupStep{{prompt: prompt, read: func(raw json.RawMessage, found *prompts.CleanupFindings) error {
		duplicates, err := prompts.DecodeDuplicates(raw, in)
		found.Duplicates = append(found.Duplicates, duplicates...)
		return err
	}}}, nil
}

func taggingSteps(bank []domain.BankQuestion) (steps []cleanupStep, more bool, err error) {
	chunks, more := prompts.TagChunks(bank)
	for _, asked := range chunks {
		in := prompts.TaggingInput{Bank: bank, Asked: asked}
		prompt, err := prompts.Tagging(in)
		if err != nil {
			return nil, false, err
		}
		steps = append(steps, cleanupStep{prompt: prompt, read: func(raw json.RawMessage, found *prompts.CleanupFindings) error {
			tags, err := prompts.DecodeTagging(raw, in)
			found.Tags = append(found.Tags, tags...)
			return err
		}})
	}
	return steps, more, nil
}

func (h CleanupHandler) fitReservation(steps []cleanupStep) (kept []cleanupStep, trimmed bool, err error) {
	price, err := h.Prices.For(domain.TierFast)
	if err != nil {
		return nil, false, err
	}
	var spent int64
	for i, step := range steps {
		p := step.prompt
		worst, err := price.Bound(len(p.System)+len(p.User)+len(p.Schema), p.MaxTokens)
		if err != nil {
			return nil, false, err
		}
		if spent+worst > domain.FeatureCleanup.Ceiling() {
			return steps[:i], true, nil
		}
		spent += worst
	}
	return steps, false, nil
}

func (h CleanupHandler) run(ctx context.Context, s *session, plan cleanupPlan) (found prompts.CleanupFindings, complete bool, err error) {
	found = prompts.CleanupFindings{Duplicates: []prompts.Duplicate{}, Tags: []prompts.TagSuggestion{}}
	if len(plan.steps) == 0 {
		return found, true, nil
	}
	passCtx, cancel := context.WithTimeout(ctx, h.CleanupBudget)
	defer cancel()
	calls := make([]prompts.Prompt, len(plan.steps))
	for i, step := range plan.steps {
		calls[i] = step.prompt
	}
	outcomes := s.callAll(passCtx, calls)
	if ctx.Err() != nil {
		return found, false, ctx.Err()
	}
	usable, requestIDs := 0, make([]string, 0, len(outcomes))
	var firstFailure error
	for i, o := range outcomes {
		requestIDs = append(requestIDs, o.Response.RequestID)
		var failure error
		if o.Err != nil {
			failure = asker(h).failure(ctx, domain.FeatureCleanup, o.Response.RequestID, o.Err)
		} else if readErr := plan.steps[i].read(o.Response.Object, &found); readErr != nil {
			h.Logger.WarnContext(ctx, "ai clean-up: an answer had nothing usable", "request_id", o.Response.RequestID, "error", readErr)
			failure = unusable(readErr)
		}
		if failure == nil {
			usable++
		} else if firstFailure == nil {
			firstFailure = failure
		}
	}
	h.Logger.InfoContext(ctx, "ai clean-up", "calls", len(outcomes), "usable", usable, "spent_the_budget", passCtx.Err() != nil, "request_ids", strings.Join(requestIDs, ","))
	if usable == 0 {
		return found, false, firstFailure
	}
	return found, firstFailure == nil, nil
}

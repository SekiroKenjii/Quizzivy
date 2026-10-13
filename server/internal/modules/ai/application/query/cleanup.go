package query

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/shared/actor"
)

// MaxCleanupQuestions is the most of a bank one clean-up reads.
const MaxCleanupQuestions = 2000

// Cleanup finds near-duplicates and untagged questions among the caller's own questions, newest first, at most MaxCleanupQuestions and at most prompts.MaxCleanupChars of them, even for a caller whose scope reaches everyone's. It asks tags for the first prompts.MaxTagsAsked questions without any. Questions shared with the caller are skipped. It changes nothing. It answers domain.ErrNothingToRead for a caller who owns none.
type Cleanup struct {
	Actor    actor.Actor
	Checks   []string
	Language domain.Language
}

// Cleaned is what the clean-up found, how many questions it read, whether the bank held more than it read or more untagged questions than it asked tags for, and what the call cost.
type Cleaned struct {
	Scanned   int
	Truncated bool
	Findings  prompts.CleanupFindings
	Usage     domain.Usage
}

type CleanupHandler struct{ *support.Service }

func (h CleanupHandler) Handle(ctx context.Context, q Cleanup) (Cleaned, error) {
	if h.Completer == nil {
		return Cleaned{}, domain.ErrOff
	}
	bank, err := h.Questions.Own(ctx, q.Actor.Scope, MaxCleanupQuestions)
	if err != nil {
		return Cleaned{}, err
	}
	fit, cut := prompts.FitCleanup(bank.Questions)
	in := prompts.CleanupInput{Questions: fit, Checks: q.Checks, Language: h.LanguageFor(ctx, q.Actor.ID, q.Language)}
	prompt, err := prompts.Cleanup(in)
	if err != nil {
		return Cleaned{}, err
	}
	got, err := asker(h).ask(ctx, q.Actor, prompt)
	if err != nil {
		return Cleaned{}, err
	}
	findings, err := prompts.DecodeCleanup(got.Object, in)
	if err != nil {
		return Cleaned{}, unusable(err)
	}
	return Cleaned{Scanned: len(fit), Truncated: bank.Truncated || cut || in.Capped(), Findings: findings, Usage: got.Usage}, nil
}

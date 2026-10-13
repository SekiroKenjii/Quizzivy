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

// Cleanup finds near-duplicates and untagged questions among the caller's own questions, newest first and at most MaxCleanupQuestions, even for a caller whose scope reaches everyone's. Questions shared with the caller are skipped. It changes nothing. It answers domain.ErrNothingToRead for a caller who owns none and domain.ErrTooLarge for a bank too long to read at once.
type Cleanup struct {
	Actor    actor.Actor
	Checks   []string
	Language domain.Language
}

// Cleaned is what the clean-up found, how many questions it read, whether the bank held more, and what the call cost.
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
	in := prompts.CleanupInput{Questions: bank.Questions, Checks: q.Checks, Language: h.LanguageFor(ctx, q.Actor.ID, q.Language)}
	prompt, err := prompts.Cleanup(in)
	if err != nil {
		return Cleaned{}, err
	}
	got, err := asker{h.Service}.ask(ctx, q.Actor, prompt)
	if err != nil {
		return Cleaned{}, err
	}
	findings, err := prompts.DecodeCleanup(got.Object, in)
	if err != nil {
		return Cleaned{}, unusable(err)
	}
	return Cleaned{Scanned: len(bank.Questions), Truncated: bank.Truncated, Findings: findings, Usage: got.Usage}, nil
}

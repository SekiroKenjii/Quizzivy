package query

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/shared/actor"
)

// Check looks for mistakes in the draft of TestID: wrong or doubtful keys, unclear wording, answers given away and spelling. It reads standalone questions and the members of the test's groups alike, in the order the builder numbers them. Nothing is changed; a finding's fix is written by the caller through the existing question write. It answers domain.ErrNotFound for a test the caller does not reach.
type Check struct {
	Actor    actor.Actor
	TestID   string
	Checks   []string
	Language domain.Language
}

// Checked is the findings, worst first, and what the call cost.
type Checked struct {
	Findings []prompts.Finding
	Usage    domain.Usage
}

type CheckHandler struct{ *support.Service }

func (h CheckHandler) Handle(ctx context.Context, q Check) (Checked, error) {
	if h.Completer == nil {
		return Checked{}, domain.ErrOff
	}
	paper, err := h.Tests.Draft(ctx, q.Actor.Scope, q.TestID)
	if err != nil {
		return Checked{}, err
	}
	in := prompts.CheckInput{Questions: paper.Questions, Checks: q.Checks, Language: h.LanguageFor(ctx, q.Actor.ID, q.Language)}
	prompt, err := prompts.Check(in)
	if err != nil {
		return Checked{}, err
	}
	got, err := asker(h).ask(ctx, q.Actor, prompt)
	if err != nil {
		return Checked{}, err
	}
	findings, err := prompts.DecodeCheck(got.Object, in)
	if err != nil {
		return Checked{}, asker(h).unusable(ctx, domain.FeatureCheck, got.RequestID, err)
	}
	return Checked{Findings: findings, Usage: got.Usage}, nil
}

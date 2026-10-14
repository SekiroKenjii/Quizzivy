package query

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/shared/actor"
)

// Variant proposes rewordings, reorderings and new wrong options for the standalone questions of the draft of TestID, which the caller copies with the existing duplicate write and applies to the copy. The members of a group are not varied: a copy of the test carries them as they are, and a draft whose questions are all members answers domain.ErrNothingToRead. It changes no answer, point or passage and writes nothing. It answers domain.ErrNotFound for a test the caller does not reach.
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
	in := prompts.VariantInput{Questions: standalone(paper.Questions), Changes: q.Changes, Difficulty: q.Difficulty}
	prompt, err := prompts.Variant(in)
	if err != nil {
		return Varied{}, err
	}
	got, err := asker(h).ask(ctx, q.Actor, prompt)
	if err != nil {
		return Varied{}, err
	}
	changes, err := prompts.DecodeVariant(got.Object, in)
	if err != nil {
		return Varied{}, asker(h).unusable(ctx, domain.FeatureVariant, got.RequestID, err)
	}
	return Varied{Changes: changes, Usage: got.Usage}, nil
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

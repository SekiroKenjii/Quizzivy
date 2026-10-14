package query

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

// Insights explains how an assignment's handed-in papers came out. The caller must hold teaching.grading (Grants) and have left on the preference that lets the assistant read student answers, else domain.ErrMissingPermission and domain.ErrReadOff. It sends per question its number, type, prompt excerpt, key, correct rate and the most frequent wrong answer texts with their counts, and the number of papers, and nothing that says who wrote what. It writes nothing. It answers domain.ErrNotFound for an assignment the caller does not reach and domain.ErrNothingToRead when no paper was handed in.
type Insights struct {
	Actor        actor.Actor
	Grants       access.Set
	AssignmentID string
}

// Interpreted is the insights, how many papers they rest on and what the call cost.
type Interpreted struct {
	Basis int
	Items []prompts.Insight
	Usage domain.Usage
}

type InsightsHandler struct{ *support.Service }

func (h InsightsHandler) Handle(ctx context.Context, q Insights) (Interpreted, error) {
	if h.Completer == nil {
		return Interpreted{}, domain.ErrOff
	}
	if !q.Grants.Has(access.TeachingGrading) {
		return Interpreted{}, domain.ErrMissingPermission
	}
	if err := h.MayReadStudentAnswers(ctx, q.Actor.ID); err != nil {
		return Interpreted{}, err
	}
	results, err := h.Analysis.Results(ctx, q.Actor.Scope, q.AssignmentID)
	if err != nil {
		return Interpreted{}, err
	}
	in := prompts.InsightsInput{Results: results, Language: h.LanguageFor(ctx, q.Actor.ID, "")}
	prompt, err := prompts.Insights(in)
	if err != nil {
		return Interpreted{}, err
	}
	got, err := asker(h).ask(ctx, q.Actor, prompt)
	if err != nil {
		return Interpreted{}, err
	}
	items, err := prompts.DecodeInsights(got.Object, in)
	if err != nil {
		return Interpreted{}, asker(h).unusable(ctx, domain.FeatureInsights, got.RequestID, err)
	}
	return Interpreted{Basis: results.HandedIn, Items: items, Usage: got.Usage}, nil
}

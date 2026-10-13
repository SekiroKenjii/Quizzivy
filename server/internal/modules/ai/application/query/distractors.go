package query

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/shared/actor"
)

// Distractors suggests wrong options for a stored single or multiple choice question, none of them one it already has. It writes nothing. It answers domain.ErrNotFound for a question the caller does not reach, domain.ErrNotChoice for another type and domain.ErrNoKey for a question with no correct option.
type Distractors struct {
	Actor      actor.Actor
	QuestionID string
}

// Suggested is the wrong options and what the call cost.
type Suggested struct {
	Items []prompts.Distractor
	Usage domain.Usage
}

type DistractorsHandler struct{ *support.Service }

func (h DistractorsHandler) Handle(ctx context.Context, q Distractors) (Suggested, error) {
	if h.Completer == nil {
		return Suggested{}, domain.ErrOff
	}
	question, err := h.Questions.Get(ctx, q.Actor.Scope, q.QuestionID)
	if err != nil {
		return Suggested{}, err
	}
	in := prompts.DistractorsInput{Question: question, Language: h.LanguageFor(ctx, q.Actor.ID, "")}
	prompt, err := prompts.Distractors(in)
	if err != nil {
		return Suggested{}, err
	}
	got, err := asker(h).ask(ctx, q.Actor, prompt)
	if err != nil {
		return Suggested{}, err
	}
	items, err := prompts.DecodeDistractors(got.Object, in)
	if err != nil {
		return Suggested{}, unusable(err)
	}
	return Suggested{Items: items, Usage: got.Usage}, nil
}

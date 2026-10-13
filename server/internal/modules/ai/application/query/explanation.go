package query

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/shared/actor"
)

// Explanation writes an explanation of the answer to a stored question in the language asked, in one sentence when Short. It writes nothing. It answers domain.ErrNotFound for a question the caller does not reach and domain.ErrNoKey for one with no answer key.
type Explanation struct {
	Actor      actor.Actor
	QuestionID string
	Language   domain.Language
	Short      bool
}

// Explained is the explanation and what the call cost.
type Explained struct {
	Text  string
	Usage domain.Usage
}

type ExplanationHandler struct{ *support.Service }

func (h ExplanationHandler) Handle(ctx context.Context, q Explanation) (Explained, error) {
	if h.Completer == nil {
		return Explained{}, domain.ErrOff
	}
	question, err := h.Questions.Get(ctx, q.Actor.Scope, q.QuestionID)
	if err != nil {
		return Explained{}, err
	}
	prompt, err := prompts.Explanation(prompts.ExplanationInput{Question: question, Language: q.Language, Short: q.Short})
	if err != nil {
		return Explained{}, err
	}
	got, err := asker(h).ask(ctx, q.Actor, prompt)
	if err != nil {
		return Explained{}, err
	}
	text, err := prompts.DecodeExplanation(got.Object)
	if err != nil {
		return Explained{}, unusable(err)
	}
	return Explained{Text: text, Usage: got.Usage}, nil
}

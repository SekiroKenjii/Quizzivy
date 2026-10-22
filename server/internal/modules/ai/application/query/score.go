package query

import (
	"context"
	"strings"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

// Score suggests a mark for a short answer, or a band for an essay, with a band for each of its rubric criteria when it has them. The caller must hold teaching.grading (Grants) and must have left on the preference that lets the assistant read student answers, else domain.ErrMissingPermission and domain.ErrReadOff. It sends the question, its accepted or sample answers, an essay's criterion labels and the student's answer text, and nothing that says who the student is. It writes no mark. It answers domain.ErrNotFound for an attempt the caller does not reach or a question not on its paper, domain.ErrAttemptInProgress and domain.ErrAttemptVoided for a paper that cannot be marked, domain.ErrNotManual for a question that is not short_answer or essay or whose answer the grader marked itself, and domain.ErrUnanswered for a blank answer.
type Score struct {
	Actor      actor.Actor
	Grants     access.Set
	AttemptID  string
	QuestionID string
}

// Scored is the suggestion and what the call cost.
type Scored struct {
	Suggestion prompts.Suggestion
	Usage      domain.Usage
}

type ScoreHandler struct{ *support.Service }

func (h ScoreHandler) Handle(ctx context.Context, q Score) (Scored, error) {
	if h.Completer == nil {
		return Scored{}, domain.ErrOff
	}
	if !q.Grants.Has(access.TeachingGrading) {
		return Scored{}, domain.ErrMissingPermission
	}
	if err := h.MayReadStudentAnswers(ctx, q.Actor.ID); err != nil {
		return Scored{}, err
	}
	graded, err := h.Grading.Answer(ctx, q.Actor.Scope, q.AttemptID, q.QuestionID)
	if err != nil {
		return Scored{}, err
	}
	if graded.Type != prompts.ShortAnswer && graded.Type != prompts.Essay {
		return Scored{}, domain.ErrNotManual
	}
	if strings.TrimSpace(graded.Answer) == "" {
		return Scored{}, domain.ErrUnanswered
	}
	if !graded.Manual {
		return Scored{}, domain.ErrNotManual
	}
	in := prompts.ScoreInput{Answer: graded, Language: h.LanguageFor(ctx, q.Actor.ID, "")}
	got, err := asker(h).ask(ctx, q.Actor, prompts.Score(in))
	if err != nil {
		return Scored{}, err
	}
	suggestion, err := prompts.DecodeScore(got.Object, in)
	if err != nil {
		return Scored{}, asker(h).unusable(ctx, domain.FeatureScore, got.RequestID, err)
	}
	return Scored{Suggestion: suggestion, Usage: got.Usage}, nil
}

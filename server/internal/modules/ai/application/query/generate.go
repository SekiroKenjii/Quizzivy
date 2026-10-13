package query

import (
	"context"
	"unicode/utf8"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/validation"
)

// MinSource is the fewest characters a source may hold: a few sentences.
const MinSource = 40

// Generate drafts questions from a source: text the teacher pasted, or a shared-content passage of the draft of TestID named by StimulusID. Exactly one of the two is named. It writes nothing; the teacher adds the questions they keep through the existing writes.
type Generate struct {
	Actor        actor.Actor
	Source       string
	TestID       string
	StimulusID   string
	Types        []string
	Count        int
	Level        string
	Explanations bool
	Language     domain.Language
}

// Generated is the draft questions and what the call cost.
type Generated struct {
	Items []prompts.Question
	Usage domain.Usage
}

type GenerateHandler struct{ *support.Service }

func (h GenerateHandler) Handle(ctx context.Context, q Generate) (Generated, error) {
	if h.Completer == nil {
		return Generated{}, domain.ErrOff
	}
	source, err := h.source(ctx, q)
	if err != nil {
		return Generated{}, err
	}
	in := prompts.GenerateInput{Source: source, Types: q.Types, Count: q.Count, Level: q.Level, Explanations: q.Explanations, Language: h.LanguageFor(ctx, q.Actor.ID, q.Language)}
	got, err := asker(h).ask(ctx, q.Actor, prompts.Generate(in))
	if err != nil {
		return Generated{}, err
	}
	items, err := prompts.DecodeGenerate(got.Object, in)
	if err != nil {
		return Generated{}, unusable(err)
	}
	return Generated{Items: items, Usage: got.Usage}, nil
}

func (h GenerateHandler) source(ctx context.Context, q Generate) (string, error) {
	var refused validation.Error
	named := q.Source != ""
	passage := q.TestID != "" || q.StimulusID != ""
	switch {
	case named == passage:
		refused.Add("source", "name a source, or a passage of a test")
	case passage && (q.TestID == "" || q.StimulusID == ""):
		refused.Add("stimulusId", "name the test and the passage")
	}
	if err := refused.OrNil(); err != nil {
		return "", err
	}
	text := q.Source
	if passage {
		found, err := h.Tests.Passage(ctx, q.Actor.Scope, q.TestID, q.StimulusID)
		if err != nil {
			return "", err
		}
		text = found.Text
	}
	if utf8.RuneCountInString(text) < MinSource {
		refused.Add("source", "add a source of at least a few sentences")
	}
	if err := refused.OrNil(); err != nil {
		return "", err
	}
	return text, nil
}

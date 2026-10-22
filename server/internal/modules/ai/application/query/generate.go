package query

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/validation"
)

// MinSource is the fewest characters a source may hold: a few sentences.
const MinSource = 40

// MinTopic is the fewest characters a standalone run's topic may hold.
const MinTopic = 3

// Generate drafts questions. True / False / Not given is written only from a passage: a standalone request that names it is refused on types. From a passage, it reads text the teacher pasted or a shared-content passage of the draft of TestID named by StimulusID, exactly one of the two. Standalone, it writes questions that each stand alone, about Source as a topic, for the section of the draft of TestID named by SectionID, or both; a section's existing questions are read so they are not repeated. It writes nothing; the teacher adds the questions they keep through the existing writes.
type Generate struct {
	Actor        actor.Actor
	Source       string
	TestID       string
	StimulusID   string
	SectionID    string
	Standalone   bool
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
	in := prompts.GenerateInput{Types: q.Types, Count: q.Count, Level: q.Level, Explanations: q.Explanations, Standalone: q.Standalone}
	var err error
	if q.Standalone {
		in.Source, in.Section, err = h.topic(ctx, q)
	} else {
		in.Source, err = h.source(ctx, q)
	}
	if err != nil {
		return Generated{}, err
	}
	in.Language = h.LanguageFor(ctx, q.Actor.ID, q.Language)
	got, err := asker(h).ask(ctx, q.Actor, prompts.Generate(in))
	if err != nil {
		return Generated{}, err
	}
	items, err := prompts.DecodeGenerate(got.Object, in)
	if err != nil {
		return Generated{}, asker(h).unusable(ctx, domain.FeatureGenerate, got.RequestID, err)
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
	case q.SectionID != "":
		refused.Add("sectionId", "a section is read only for standalone questions")
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

func (h GenerateHandler) topic(ctx context.Context, q Generate) (string, *domain.Section, error) {
	var refused validation.Error
	switch {
	case slices.Contains(q.Types, prompts.Tfng):
		refused.Add("types", "True / False / Not given is written from a passage")
	case q.StimulusID != "":
		refused.Add("stimulusId", "standalone questions are not written from a passage")
	case q.SectionID != "" && q.TestID == "":
		refused.Add("testId", "name the test that holds the section")
	case q.SectionID == "" && q.TestID != "":
		refused.Add("sectionId", "name the section of the test")
	case q.SectionID == "" && utf8.RuneCountInString(strings.TrimSpace(q.Source)) < MinTopic:
		refused.Add("source", "name a topic, or a section of a test")
	case q.Source != "" && utf8.RuneCountInString(strings.TrimSpace(q.Source)) < MinTopic:
		refused.Add("source", "name a topic of at least a few letters")
	}
	if err := refused.OrNil(); err != nil {
		return "", nil, err
	}
	if q.SectionID == "" {
		return q.Source, nil, nil
	}
	section, err := h.Tests.Section(ctx, q.Actor.Scope, q.TestID, q.SectionID)
	if err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(q.Source) == "" && len(section.Questions) == 0 && strings.TrimSpace(section.Instructions) == "" && strings.TrimSpace(section.Title) == "" {
		refused.Add("source", "name a topic: the section has no title, instructions or questions yet")
		return "", nil, refused.OrNil()
	}
	return q.Source, &section, nil
}

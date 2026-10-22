package support

import (
	"quizzivy/internal/modules/questions/domain"
)

// InputOf is q as a write that creates a copy of it: every part of it, with no
// ids, so a matching item is keyed to its copied option by position.
func InputOf(q domain.Question) domain.Input {
	in := domain.Input{
		Type:               q.Type,
		Level:              q.Level,
		Skill:              q.Skill,
		Prompt:             q.Prompt,
		PromptContent:      q.PromptContent,
		MediaAssetID:       q.MediaAssetID,
		MediaAlt:           q.MediaAlt,
		Audio:              q.Audio,
		Transcript:         q.Transcript,
		Points:             q.Points,
		Explanation:        q.Explanation,
		ExplanationContent: q.ExplanationContent,
		SampleAnswer:       q.SampleAnswer,
		SelectCount:        q.SelectCount,
		MaxWords:           q.MaxWords,
		MatchAllowReuse:    q.MatchAllowReuse,
		Tags:               append([]string{}, q.Tags...),
	}
	for _, o := range q.Options {
		in.Options = append(in.Options, domain.OptionInput{Text: o.Text, IsCorrect: o.IsCorrect, Content: o.Content})
	}
	for _, b := range q.Blanks {
		in.Blanks = append(in.Blanks, domain.BlankInput{GapID: b.GapID,
			Ordinal: b.Ordinal, AcceptedAnswers: append([]string{}, b.AcceptedAnswers...), CaseSensitive: b.CaseSensitive,
		})
	}
	for _, item := range q.MatchItems {
		in.MatchItems = append(in.MatchItems, domain.MatchItemInput{Text: item.Text, CorrectOption: item.CorrectOption})
	}
	for _, criterion := range q.Rubric {
		in.Rubric = append(in.Rubric, domain.RubricCriterionInput{Label: criterion.Label})
	}
	return in
}

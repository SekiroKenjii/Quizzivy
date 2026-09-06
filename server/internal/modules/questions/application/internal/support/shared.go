package support

import (
	"quizzivy/internal/modules/questions/domain"
)

func InputOf(q domain.Question) domain.Input {
	in := domain.Input{
		Type:         q.Type,
		Prompt:       q.Prompt,
		MediaAssetID: q.MediaAssetID,
		Audio:        q.Audio,
		Transcript:   q.Transcript,
		Points:       q.Points,
		Explanation:  q.Explanation,
		SampleAnswer: q.SampleAnswer,
		Tags:         append([]string{}, q.Tags...),
	}
	for _, o := range q.Options {
		in.Options = append(in.Options, domain.OptionInput{Text: o.Text, IsCorrect: o.IsCorrect})
	}
	for _, b := range q.Blanks {
		in.Blanks = append(in.Blanks, domain.BlankInput{
			Ordinal: b.Ordinal, AcceptedAnswers: append([]string{}, b.AcceptedAnswers...), CaseSensitive: b.CaseSensitive,
		})
	}
	return in
}

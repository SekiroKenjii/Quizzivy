package domain

import (
	"errors"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	"strings"
)

func validateQuestion(section DraftSection, q DraftQuestion, add func(Violation)) {
	var invalid *questionsdomain.ValidationError
	if !errors.As(questionInput(q).Validate(q.MediaAssetKind), &invalid) {
		return
	}
	seen := make(map[Rule]bool)
	for _, field := range invalid.Fields {
		rule := questionRule(field.Field, q.Type)
		if !seen[rule] {
			add(Violation{Rule: rule, Message: field.Message, SectionID: section.ID, QuestionID: q.SourceID})
			seen[rule] = true
		}
	}
}

func questionInput(q DraftQuestion) questionsdomain.Input {
	in := questionsdomain.Input{
		Type: questionsdomain.Type(q.Type), Prompt: q.Prompt, Points: q.Points,
		PromptContent: q.PromptContent, ExplanationContent: q.ExplanationContent,
		MediaAssetID: q.MediaAssetID, Transcript: q.Transcript,
		Explanation: q.Explanation, SampleAnswer: q.SampleAnswer,
	}
	if q.AllowSeek != nil || q.ShowTranscript != nil || q.MaxPlays != nil {
		in.Audio = &questionsdomain.AudioPolicy{MaxPlays: q.MaxPlays}
		if q.AllowSeek != nil {
			in.Audio.AllowSeek = *q.AllowSeek
		}
		if q.ShowTranscript != nil {
			in.Audio.ShowTranscriptAfterSubmit = *q.ShowTranscript
		}
	}
	for _, option := range q.Options {
		in.Options = append(in.Options, questionsdomain.OptionInput{Text: option.Text, Content: option.Content, IsCorrect: option.IsCorrect})
	}
	for _, blank := range q.Blanks {
		in.Blanks = append(in.Blanks, questionsdomain.BlankInput{GapID: blank.GapID, Ordinal: blank.Ordinal, AcceptedAnswers: blank.AcceptedAnswers, CaseSensitive: blank.CaseSensitive})
	}
	return in
}

func questionRule(field, questionType string) Rule {
	switch {
	case field == "points":
		return PointsPositive
	case field == "promptContent" || field == "explanationContent":
		return QuestionContentValid
	case strings.HasPrefix(field, "options[") && strings.HasSuffix(field, ".content"):
		return OptionContentValid
	case strings.HasPrefix(field, "options"):
		return ChoiceHasCorrectOption
	case strings.Contains(field, "acceptedAnswers"):
		return BlankHasAcceptedAnswer
	case questionType == "fill_blank" && (strings.HasPrefix(field, "blanks") || field == "prompt"):
		return BlankPlaceholdersMatch
	case strings.HasPrefix(field, "audio") || field == "transcript" || field == "mediaAssetId":
		return AudioQuestionHasAsset
	default:
		return QuestionValid
	}
}

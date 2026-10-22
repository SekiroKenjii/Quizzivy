package http

import (
	"encoding/json"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/platform/httpapi"
	"strconv"
)

// ToAPIInput returns an editable question shape for a complete-context graph, retaining keys and stable gap bindings.
func ToAPIInput(in domain.Input) (openapi.StoredQuestionInput, error) {
	points, err := strconv.ParseFloat(in.Points, 64)
	if err != nil {
		return openapi.StoredQuestionInput{}, err
	}
	tags := append([]string{}, in.Tags...)
	out := openapi.StoredQuestionInput{Level: (*openapi.QuestionLevel)(in.Level), Skill: (*openapi.QuestionSkill)(in.Skill), Type: openapi.QuestionType(in.Type), Prompt: in.Prompt, PromptContent: in.PromptContent, Points: points,
		Explanation: in.Explanation, ExplanationContent: in.ExplanationContent, SampleAnswer: in.SampleAnswer, Transcript: in.Transcript, MediaAlt: in.MediaAlt, Tags: &tags,
		SelectCount: in.SelectCount, MaxWords: in.MaxWords, MatchAllowReuse: in.MatchAllowReuse}
	if in.Type == domain.Matching {
		items := make([]openapi.MatchItemInput, len(in.MatchItems))
		for i, item := range in.MatchItems {
			items[i] = openapi.MatchItemInput{Text: item.Text, CorrectOption: item.CorrectOption}
			if item.ID != nil {
				items[i].Id = httpapi.Ptr(httpapi.ParseUUID(*item.ID))
			}
		}
		out.MatchItems = &items
	}
	if in.Type == domain.Essay {
		rubric := make([]openapi.RubricCriterionInput, len(in.Rubric))
		for i, criterion := range in.Rubric {
			rubric[i] = openapi.RubricCriterionInput{Label: criterion.Label}
			if criterion.ID != nil {
				rubric[i].Id = httpapi.Ptr(httpapi.ParseUUID(*criterion.ID))
			}
		}
		out.Rubric = &rubric
	}
	if in.MediaAssetID != nil {
		out.MediaAssetId = httpapi.Ptr(httpapi.ParseUUID(*in.MediaAssetID))
	}
	if in.Audio != nil {
		out.Audio = &openapi.AudioPolicy{MaxPlays: in.Audio.MaxPlays, AllowSeek: in.Audio.AllowSeek, ShowTranscriptAfterSubmit: in.Audio.ShowTranscriptAfterSubmit, AllowPause: httpapi.Ptr(in.Audio.PauseAllowed())}
	}
	options := make([]struct {
		Content   json.RawMessage `json:"content,omitempty"`
		Id        *openapi.Uuid   `json:"id,omitempty"`
		IsCorrect bool            `json:"isCorrect"`
		Text      string          `json:"text"`
	}, len(in.Options))
	for i, option := range in.Options {
		options[i].Content = option.Content
		options[i].IsCorrect = option.IsCorrect
		options[i].Text = option.Text
		if option.ID != nil {
			options[i].Id = httpapi.Ptr(httpapi.ParseUUID(*option.ID))
		}
	}
	out.Options = &options
	blanks := make([]struct {
		AcceptedAnswers []string      `json:"acceptedAnswers"`
		CaseSensitive   *bool         `json:"caseSensitive,omitempty"`
		GapId           *string       `json:"gapId,omitempty"`
		Id              *openapi.Uuid `json:"id,omitempty"`
		Ordinal         int           `json:"ordinal"`
	}, len(in.Blanks))
	for i, blank := range in.Blanks {
		blanks[i].AcceptedAnswers = blank.AcceptedAnswers
		blanks[i].CaseSensitive = httpapi.Ptr(blank.CaseSensitive)
		blanks[i].GapId = blank.GapID
		blanks[i].Ordinal = blank.Ordinal
		if blank.ID != nil {
			blanks[i].Id = httpapi.Ptr(httpapi.ParseUUID(*blank.ID))
		}
	}
	out.Blanks = &blanks
	return out, nil
}

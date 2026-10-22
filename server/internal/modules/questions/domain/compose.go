package domain

import (
	"encoding/json"
	"fmt"

	"quizzivy/internal/shared/content"
	"quizzivy/internal/shared/validation"
)

// Composed returns the input with every text it carries composed to NFC: the prose documents and the plain text beside
// them, the transcript, the sample answer, the alt text, the tags, the options, the accepted answers, the matching items
// and the rubric's labels. A prose document and
// its plain companion are composed together, so the companion still equals the document's projection when it did before.
// A text that composing leaves over its limit comes back as a *ValidationError naming the field.
func (in Input) Composed() (Input, error) {
	var c content.Composer
	out := in

	prompt := in.Prompt
	var composedPrompt *string
	out.PromptContent, composedPrompt = composeProse(&c, in.PromptContent, &prompt, validation.Pair("promptContent",
		"Nội dung có định dạng không hợp lệ hoặc không khớp văn bản.", "The formatted text is not valid or does not match the plain text."))
	out.Prompt = *composedPrompt
	out.ExplanationContent, out.Explanation = composeProse(&c, in.ExplanationContent, in.Explanation, validation.Pair("explanationContent",
		"Lời giải có định dạng không hợp lệ hoặc không khớp văn bản.", "The formatted explanation is not valid or does not match its plain text."))
	out.SampleAnswer = c.Optional("sampleAnswer", in.SampleAnswer, 0)
	out.Transcript = c.Optional("transcript", in.Transcript, 0)
	out.MediaAlt = c.Optional("mediaAlt", in.MediaAlt, MaxMediaAltLength)
	out.Tags = c.Each("tags", in.Tags, 0)
	out.Options = composeOptions(&c, in.Options)
	out.Blanks = composeBlanks(&c, in.Blanks)
	out.MatchItems = composeMatchItems(&c, in.MatchItems)
	out.Rubric = composeRubric(&c, in.Rubric)

	if err := c.Err(); err != nil {
		return in, err
	}
	return out, nil
}

func composeOptions(c *content.Composer, options []OptionInput) []OptionInput {
	if options == nil {
		return nil
	}
	out := make([]OptionInput, len(options))
	for i, option := range options {
		text := option.Text
		raw, composed := composeProse(c, option.Content, &text, validation.Pair(fmt.Sprintf("options[%d].content", i),
			"Định dạng phương án không hợp lệ hoặc không khớp nội dung văn bản.",
			"The option's formatting is not valid or does not match its text."))
		option.Content, option.Text = raw, *composed
		out[i] = option
	}
	return out
}

func composeBlanks(c *content.Composer, blanks []BlankInput) []BlankInput {
	if blanks == nil {
		return nil
	}
	out := make([]BlankInput, len(blanks))
	for i, blank := range blanks {
		blank.AcceptedAnswers = c.Each(fmt.Sprintf("blanks[%d].acceptedAnswers", i), blank.AcceptedAnswers, 0)
		out[i] = blank
	}
	return out
}

// composeProse composes one prose document and its plain companion, refusing
// refusal's field with refusal's message when the document cannot be composed.
func composeMatchItems(c *content.Composer, items []MatchItemInput) []MatchItemInput {
	if items == nil {
		return nil
	}
	out := make([]MatchItemInput, len(items))
	for i, item := range items {
		item.Text = c.Text(fmt.Sprintf("matchItems[%d].text", i), item.Text, MaxMatchItemLength)
		out[i] = item
	}
	return out
}

func composeRubric(c *content.Composer, rubric []RubricCriterionInput) []RubricCriterionInput {
	if rubric == nil {
		return nil
	}
	out := make([]RubricCriterionInput, len(rubric))
	for i, criterion := range rubric {
		criterion.Label = c.Text(fmt.Sprintf("rubric[%d].label", i), criterion.Label, MaxRubricLabel)
		out[i] = criterion
	}
	return out
}

func composeProse(c *content.Composer, raw json.RawMessage, companion *string, refusal validation.Field) (json.RawMessage, *string) {
	field := refusal.Field
	if !hasProse(raw) {
		return raw, c.Optional(field, companion, 0)
	}
	document, err := content.Parse(raw)
	if err != nil {
		return raw, c.Optional(field, companion, 0)
	}
	composed, err := content.Normalize(document)
	if err != nil {
		c.Refuse(field, refusal.Message, refusal.English)
		return raw, companion
	}
	encoded, err := composed.MarshalJSON()
	if err != nil {
		c.Refuse(field, refusal.Message, refusal.English)
		return raw, companion
	}
	if companion == nil {
		return encoded, nil
	}
	if *companion == document.PlainText() {
		projection := composed.PlainText()
		return encoded, &projection
	}
	return encoded, c.Optional(field, companion, 0)
}

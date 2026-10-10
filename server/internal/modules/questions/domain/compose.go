package domain

import (
	"encoding/json"
	"fmt"

	"quizzivy/internal/shared/content"
)

// Composed returns the input with every text it carries composed to NFC: the prose documents and the plain text beside
// them, the transcript, the sample answer, the alt text, the tags, the options and the accepted answers. A prose document and
// its plain companion are composed together, so the companion still equals the document's projection when it did before.
// A text that composing leaves over its limit comes back as a *ValidationError naming the field.
func (in Input) Composed() (Input, error) {
	var c content.Composer
	out := in

	prompt := in.Prompt
	var composedPrompt *string
	out.PromptContent, composedPrompt = composeProse(&c, "promptContent", in.PromptContent, &prompt,
		"Nội dung có định dạng không hợp lệ hoặc không khớp văn bản.")
	out.Prompt = *composedPrompt
	out.ExplanationContent, out.Explanation = composeProse(&c, "explanationContent", in.ExplanationContent, in.Explanation,
		"Lời giải có định dạng không hợp lệ hoặc không khớp văn bản.")
	out.SampleAnswer = c.Optional("sampleAnswer", in.SampleAnswer, 0)
	out.Transcript = c.Optional("transcript", in.Transcript, 0)
	out.MediaAlt = c.Optional("mediaAlt", in.MediaAlt, MaxMediaAltLength)
	out.Tags = c.Each("tags", in.Tags, 0)
	out.Options = composeOptions(&c, in.Options)
	out.Blanks = composeBlanks(&c, in.Blanks)

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
		raw, composed := composeProse(c, fmt.Sprintf("options[%d].content", i), option.Content, &text,
			"Định dạng phương án không hợp lệ hoặc không khớp nội dung văn bản.")
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

func composeProse(c *content.Composer, field string, raw json.RawMessage, companion *string, refusal string) (json.RawMessage, *string) {
	if !hasProse(raw) {
		return raw, c.Optional(field, companion, 0)
	}
	document, err := content.Parse(raw)
	if err != nil {
		return raw, c.Optional(field, companion, 0)
	}
	composed, err := content.Normalize(document)
	if err != nil {
		c.Refuse(field, refusal)
		return raw, companion
	}
	encoded, err := composed.MarshalJSON()
	if err != nil {
		c.Refuse(field, refusal)
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

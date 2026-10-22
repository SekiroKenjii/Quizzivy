package content

import "bytes"

// ParseQuestion validates semantic prose without asset or gap bindings.
func ParseQuestion(raw []byte) (Document, error) {
	d, err := ParseQuestionPrompt(raw)
	if err != nil || len(d.GapIDs()) != 0 {
		return Document{}, ErrInvalidDocument
	}
	return d, nil
}

// ParseQuestionPrompt validates semantic prose with stable gaps and no asset bindings.
func ParseQuestionPrompt(raw []byte) (Document, error) {
	d, err := Parse(raw)
	if err != nil || d.Format() != semanticFormat || len(d.Assets()) != 0 {
		return Document{}, ErrInvalidDocument
	}
	return d, nil
}

// ParsePromptWithMedia validates a question prompt under DG-109's media rules: ParseQuestionPrompt's prose and gaps,
// plus images and at most one audio marker among the document's own blocks. Assets lists the images; the marker names
// none, and PlacesQuestionAudio reports it. A prompt ParseQuestionPrompt accepts parses here to the same projection.
func ParsePromptWithMedia(raw []byte) (Document, error) {
	d, err := parse(raw, promptProfile)
	if err != nil || d.Format() != semanticFormat {
		return Document{}, ErrInvalidDocument
	}
	return d, nil
}

// ParseExplanationWithMedia validates an explanation under DG-109's media rules: ParseQuestion's prose, plus images and
// audio, each naming its asset, among the document's own blocks. An explanation ParseQuestion accepts parses here to
// the same projection.
func ParseExplanationWithMedia(raw []byte) (Document, error) {
	d, err := parse(raw, explanationProfile)
	if err != nil || d.Format() != semanticFormat || len(d.GapIDs()) != 0 {
		return Document{}, ErrInvalidDocument
	}
	return d, nil
}

// ParseMaterial validates a shared group material, which is always semantic content because frozen versions store only that format.
func ParseMaterial(raw []byte) (Document, error) {
	d, err := Parse(raw)
	if err != nil || d.Format() != semanticFormat {
		return Document{}, ErrInvalidDocument
	}
	return d, nil
}

// The fields of a question whose prose may show media (DG-109), as the reference tables of 00154 name them.
const (
	PromptField      = "prompt"
	ExplanationField = "explanation"
)

// FieldAsset is one media asset a question's prose names, with the field that names it.
type FieldAsset struct {
	Field string
	AssetReference
}

// QuestionMedia lists the assets a question's prompt and explanation name, prompt first, each in document order and
// once per field, under DG-109's rules. An absent or null document names none; the prompt's audio marker names none
// either, because it places the question's own audio. A document that fails the rules is an error, never read as
// naming nothing.
func QuestionMedia(prompt, explanation []byte) ([]FieldAsset, error) {
	var out []FieldAsset
	for _, prose := range []struct {
		field string
		raw   []byte
		parse func([]byte) (Document, error)
	}{
		{PromptField, prompt, ParsePromptWithMedia},
		{ExplanationField, explanation, ParseExplanationWithMedia},
	} {
		raw := bytes.TrimSpace(prose.raw)
		if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
			continue
		}
		document, err := prose.parse(raw)
		if err != nil {
			return nil, err
		}
		for _, asset := range document.Assets() {
			out = append(out, FieldAsset{Field: prose.field, AssetReference: asset})
		}
	}
	return out, nil
}

package content

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

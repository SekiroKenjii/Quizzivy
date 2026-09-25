package content

// ParseOption validates the inline learner-safe subset: one paragraph containing marked text and breaks.
func ParseOption(raw []byte) (Document, error) {
	d, err := Parse(raw)
	if err != nil || d.Format() != semanticFormat {
		return Document{}, ErrInvalidDocument
	}
	value, _ := decode(raw)
	root, ok := value.(map[string]any)
	if !ok {
		return Document{}, ErrInvalidDocument
	}
	blocks, ok := root["blocks"].([]any)
	if !ok || len(blocks) != 1 || kind(blocks[0], "type") != "paragraph" {
		return Document{}, ErrInvalidDocument
	}
	paragraph := blocks[0].(map[string]any)
	for _, node := range paragraph["content"].([]any) {
		if typ := kind(node, "type"); typ != "text" && typ != "break" {
			return Document{}, ErrInvalidDocument
		}
	}
	return d, nil
}

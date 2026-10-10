package content

import (
	"bytes"
	"encoding/json"

	"golang.org/x/text/unicode/norm"
)

// NFC returns s in Unicode Normalization Form C, the form the brand font draws.
func NFC(s string) string { return norm.NFC.String(s) }

// NFCPtr returns a pointer to the composed text of *s, or nil for nil.
func NFCPtr(s *string) *string {
	if s == nil {
		return nil
	}
	composed := NFC(*s)
	return &composed
}

// NFCAll returns a new slice holding the composed text of each element, or nil for nil.
func NFCAll(values []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = NFC(value)
	}
	return out
}

// Normalize returns d with the text it shows composed to NFC: the text of a text node, the label of a gap or an audio, the
// alt of an image and the Markdown of a legacy document. Links, asset and gap identities, marks and numbers are left as they
// are. A document that is already composed comes back unchanged, byte for byte; one that is not is encoded again and
// validated again, so it answers ErrInvalidDocument if composing made it exceed a limit.
func Normalize(d Document) (Document, error) {
	if len(d.data) == 0 {
		return Document{}, ErrInvalidDocument
	}
	var value any
	if err := json.Unmarshal(d.data, &value); err != nil {
		return Document{}, ErrInvalidDocument
	}
	if !compose(value) {
		return d, nil
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return Document{}, ErrInvalidDocument
	}
	return Parse(bytes.TrimSpace(out.Bytes()))
}

func compose(value any) bool {
	switch node := value.(type) {
	case map[string]any:
		changed := composeNode(node)
		for _, child := range node {
			if compose(child) {
				changed = true
			}
		}
		return changed
	case []any:
		changed := false
		for _, child := range node {
			if compose(child) {
				changed = true
			}
		}
		return changed
	}
	return false
}

func composeNode(node map[string]any) bool {
	switch kind(node, typeKey) {
	case text:
		return composeField(node, text)
	case gap, audio:
		return composeField(node, labelKey)
	case image:
		return composeField(node, "alt")
	}
	if kind(node, "format") == "legacy_markdown_v1" {
		return composeField(node, "markdown")
	}
	return false
}

func composeField(node map[string]any, key string) bool {
	current, ok := node[key].(string)
	if !ok {
		return false
	}
	composed := NFC(current)
	if composed == current {
		return false
	}
	node[key] = composed
	return true
}

package content_test

import (
	"quizzivy/internal/shared/content"
	"strings"
	"testing"
)

func TestOptionContentPreservesMarkedUnicodeAndLineBreaks(t *testing.T) {
	raw := `{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"ngh","marks":["underline"]},{"type":"text","text":"ề","marks":[]},{"type":"break"},{"type":"text","text":"H₂O","marks":["bold"]}]}]}`
	document, err := content.ParseOption([]byte(raw))
	if err != nil || document.PlainText() != "nghề\nH₂O" {
		t.Fatalf("option = %q, %v", document.PlainText(), err)
	}
	for _, invalid := range []string{
		strings.Replace(raw, `"format":"semantic_v1"`, `"format":"semantic_v1","format":"semantic_v1"`, 1),
		strings.Replace(raw, `"type":"paragraph"`, `"type":"heading","level":2`, 1),
		strings.Replace(raw, `"marks":["underline"]`, `"marks":["underline"],"isCorrect":true`, 1),
		`{"format":"legacy_markdown_v1","markdown":"**word**"}`,
		`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"gap1","label":"1"}]}]}`,
		`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"link","href":"https://example.com","content":[{"type":"text","text":"word","marks":[]}]}]}]}`,
		`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[]},{"type":"paragraph","content":[]}]}`,
	} {
		if _, err := content.ParseOption([]byte(invalid)); err == nil {
			t.Fatalf("accepted unsupported option %s", invalid)
		}
	}
}

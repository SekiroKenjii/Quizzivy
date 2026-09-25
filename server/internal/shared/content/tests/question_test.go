package content_test

import (
	"quizzivy/internal/shared/content"
	"testing"
)

func TestQuestionProseRejectsUnboundReferencesAtAnyDepth(t *testing.T) {
	for _, raw := range []string{
		`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"gap1","label":"1"}]}]}`,
		`{"format":"semantic_v1","blocks":[{"type":"list","ordered":false,"start":1,"items":[[{"type":"paragraph","content":[]},{"type":"image","assetId":"01935000-0000-7000-8000-000000000001","alt":"Example"}]]}]}`,
		`{"format":"legacy_markdown_v1","markdown":"legacy"}`,
		`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[],"sampleAnswer":"hidden"}]}`,
		`{"format":"semantic_v1","format":"semantic_v1","blocks":[{"type":"paragraph","content":[]}]}`,
	} {
		if _, err := content.ParseQuestion([]byte(raw)); err == nil {
			t.Fatalf("accepted unbound/unsafe prose: %s", raw)
		}
	}
}

func TestQuestionProsePreservesTableAndUnicodeProjection(t *testing.T) {
	raw := []byte(`{"format":"semantic_v1","blocks":[{"type":"heading","level":2,"content":[{"type":"text","text":"Câu hỏi","marks":["underline"]}]},{"type":"table","rows":[[{"header":true,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Điều kiện","marks":[]}]}]},{"header":true,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Giá trị","marks":[]}]}]}]]}]}`)
	d, err := content.ParseQuestion(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.PlainText() != "Câu hỏi\n\nĐiều kiện\tGiá trị" {
		t.Fatalf("projection: %q", d.PlainText())
	}
}

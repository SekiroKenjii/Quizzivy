package adapters_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/platform/word"
	"slices"
	"testing"
)

func evidenceWord(t *testing.T, body string) []byte {
	t.Helper()
	base := wordPackage(t, false)
	source, err := zip.NewReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, entry := range source.File {
		file, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name == "word/document.xml" {
			data = []byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body + `</w:body></w:document>`)
		}
		dest, err := writer.Create(entry.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := dest.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestRecognitionProjectionRetainsUnicodeMarksButKeepsHiddenAndChangedTextPrivate(t *testing.T) {
	data := evidenceWord(t, `<w:p><w:r><w:t>Câu 1. </w:t></w:r><w:r><w:rPr><w:u w:val="single"/><w:color w:val="FF0000"/></w:rPr><w:t>nghé</w:t></w:r></w:p><w:p><w:r><w:rPr><w:vanish/></w:rPr><w:t>hidden answer</w:t></w:r></w:p><w:del><w:p><w:r><w:delText>deleted answer</w:delText></w:r></w:p></w:del>`)
	raw, err := word.Extract(context.Background(), bytes.NewReader(data), int64(len(data)), "synthetic", word.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	projected := adapters.ImportEvidence(raw, "exam")
	texts := 0
	for _, b := range projected.Blocks {
		switch b.Text {
		case "Câu 1. nghé":
			texts++
			if !b.Safe || len(b.Spans) != 2 || b.Spans[1].Start != 7 || b.Spans[1].End != 11 || !slices.Contains(b.Spans[1].Marks, "underline") || !slices.Contains(b.Reasons, "SEMANTIC_FORMATTING_LOSS") {
				t.Fatalf("source semantics lost: %+v", b)
			}
		case "hidden answer", "deleted answer":
			texts++
			if b.Safe {
				t.Fatal("private branch eligible for learner prose")
			}
		}
	}
	if texts != 3 {
		t.Fatal("projection discarded evidence")
	}
}

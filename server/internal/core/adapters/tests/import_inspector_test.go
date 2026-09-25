package adapters_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/imports/domain"
	"testing"
)

func wordPackage(t *testing.T, active bool) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	parts := map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Synthetic exam</w:t></w:r></w:p></w:body></w:document>`,
	}
	if active {
		parts["word/vbaProject.bin"] = "unsafe"
	}
	for name, data := range parts {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestImportInspectionDetectsActualWordStructureAndRejectsActivePackages(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
		want error
	}{
		{"native", wordPackage(t, false), nil},
		{"macro", wordPackage(t, true), domain.ErrUnsupported},
		{"mislabeled", []byte("<html>not Word</html>"), domain.ErrInvalid},
		{"legacy-or-encrypted", []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, domain.ErrUnsupported},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := (adapters.ImportInspector{}).Inspect(context.Background(), bytes.NewReader(test.data), int64(len(test.data)))
			if !errors.Is(err, test.want) {
				t.Fatalf("want %v, got %v", test.want, err)
			}
		})
	}
}

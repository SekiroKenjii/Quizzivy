package content_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"quizzivy/internal/shared/content"
	"quizzivy/internal/shared/validation"

	"golang.org/x/text/unicode/norm"
)

func decomposed(s string) string { return norm.NFD.String(s) }

func rich(t testing.TB, raw string) content.Document {
	t.Helper()
	d, err := content.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("fixture rejected: %v\n%s", err, raw)
	}
	return d
}

func marshalled(t testing.TB, d content.Document) string {
	t.Helper()
	raw, err := d.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

var linkWithMark = "https://example.com/" + norm.NFD.String("nghé") + "?q=1"

const (
	audioAsset = "01935000-0000-7000-8000-0000000000a1"
	imageAsset = "01935000-0000-7000-8000-0000000000a2"
)

func sameJSON(t testing.TB, a, b string) bool {
	t.Helper()
	var left, right any
	if err := json.Unmarshal([]byte(a), &left); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(b), &right); err != nil {
		t.Fatal(err)
	}
	return reflect.DeepEqual(left, right)
}

func semantic(text, label, alt, audio, cell, item string) string {
	return fmt.Sprintf(`{"format":"semantic_v1","blocks":[`+
		`{"type":"heading","level":2,"content":[{"type":"text","text":%q,"marks":["bold"]}]},`+
		`{"type":"paragraph","content":[{"type":"text","text":%q,"marks":[]},{"type":"gap","id":"Gap-1","label":%q},`+
		`{"type":"link","href":%q,"content":[{"type":"text","text":%q,"marks":["italic"]}]}]},`+
		`{"type":"image","assetId":%q,"alt":%q},`+
		`{"type":"audio","assetId":%q,"label":%q},`+
		`{"type":"list","ordered":true,"start":3,"items":[[{"type":"paragraph","content":[{"type":"text","text":%q,"marks":[]}]}]]},`+
		`{"type":"table","rows":[[{"header":true,"rowSpan":1,"colSpan":1,"content":[{"type":"paragraph","content":[{"type":"text","text":%q,"marks":[]}]}]}]]}]}`,
		text, text, label, linkWithMark, text, imageAsset, alt, audioAsset, audio, item, cell)
}

func TestNormalizeComposesEveryTextBearingField(t *testing.T) {
	composed := semantic("Nghe và điền vào chỗ trống", "Điền", "Hình ảnh cái nón", "Bài nghe số một", "Ô đầu tiên", "Mục thứ nhất")
	typed := semantic(decomposed("Nghe và điền vào chỗ trống"), decomposed("Điền"), decomposed("Hình ảnh cái nón"),
		decomposed("Bài nghe số một"), decomposed("Ô đầu tiên"), decomposed("Mục thứ nhất"))
	if typed == composed {
		t.Fatal("the fixture is not decomposed")
	}
	got, err := content.Normalize(rich(t, typed))
	if err != nil {
		t.Fatal(err)
	}
	if want := rich(t, composed); !sameJSON(t, marshalled(t, got), marshalled(t, want)) {
		t.Fatalf("composed document differs:\n got %s\nwant %s", marshalled(t, got), marshalled(t, want))
	}
	if got.PlainText() != norm.NFC.String(got.PlainText()) {
		t.Fatalf("plain text is not composed: %q", got.PlainText())
	}
	if text := strings.ReplaceAll(marshalled(t, got), linkWithMark, ""); text != norm.NFC.String(text) {
		t.Fatalf("a field is still decomposed: %s", text)
	}
}

func TestNormalizeComposesTheMarkdownOfALegacyDocument(t *testing.T) {
	typed := decomposed("Điền vào chỗ trống: {{1}}")
	got, err := content.Normalize(rich(t, string(legacy(t, typed))))
	if err != nil {
		t.Fatal(err)
	}
	if got.PlainText() != norm.NFC.String(typed) {
		t.Fatalf("markdown=%q", got.PlainText())
	}
}

func TestNormalizeReturnsAComposedDocumentUnchangedByteForByte(t *testing.T) {
	raw := `{ "format" : "semantic_v1", "blocks" : [ {"type":"paragraph","content":[{"marks":[],"type":"text","text":"a & b < c à"}]} ] }`
	d := rich(t, raw)
	got, err := content.Normalize(d)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte(marshalled(t, got)), []byte(raw)) {
		t.Fatalf("a composed document was encoded again: %s", marshalled(t, got))
	}
}

func TestNormalizeLeavesIdentitiesLinksMarksAndNumbersAlone(t *testing.T) {
	typed := semantic(decomposed("Điền"), "Điền", "Ảnh", "Âm thanh", "Ô", "Mục")
	got, err := content.Normalize(rich(t, typed))
	if err != nil {
		t.Fatal(err)
	}
	raw := marshalled(t, got)
	for _, kept := range []string{
		`"href":"` + linkWithMark + `"`, `"id":"Gap-1"`, `"assetId":"` + imageAsset + `"`, `"assetId":"` + audioAsset + `"`,
		`"marks":["bold"]`, `"marks":["italic"]`, `"level":2`, `"start":3`, `"ordered":true`, `"rowSpan":1`, `"colSpan":1`, `"header":true`,
	} {
		if !strings.Contains(raw, kept) {
			t.Fatalf("%s was changed: %s", kept, raw)
		}
	}
	if gaps := got.GapIDs(); len(gaps) != 1 || gaps[0] != "Gap-1" {
		t.Fatalf("gaps=%v", gaps)
	}
	if assets := got.Assets(); len(assets) != 2 {
		t.Fatalf("assets=%v", assets)
	}
}

func TestNormalizedDocumentParsesAgain(t *testing.T) {
	got, err := content.Normalize(rich(t, semantic(decomposed("Điền"), "Điền", "Ảnh", "Âm thanh", "Ô", "Mục")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := content.Parse([]byte(marshalled(t, got))); err != nil {
		t.Fatalf("the composed document no longer validates: %v", err)
	}
}

func TestNormalizeDoesNotChangeItsArgument(t *testing.T) {
	typed := semantic(decomposed("Điền"), "Điền", "Ảnh", "Âm thanh", "Ô", "Mục")
	d := rich(t, typed)
	if _, err := content.Normalize(d); err != nil {
		t.Fatal(err)
	}
	if marshalled(t, d) != typed {
		t.Fatal("the source document was mutated")
	}
}

func TestNormalizeRefusesTheZeroDocument(t *testing.T) {
	if _, err := content.Normalize(content.Document{}); !errors.Is(err, content.ErrInvalidDocument) {
		t.Fatalf("err=%v", err)
	}
}

func TestNormalizeRefusesWhatComposingMakesTooLong(t *testing.T) {
	const lengthens = "क़"
	label := strings.Repeat(lengthens, 32)
	raw := fmt.Sprintf(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"g1","label":%q}]}]}`, label)
	d := rich(t, raw)
	if _, err := content.Normalize(d); !errors.Is(err, content.ErrInvalidDocument) {
		t.Fatalf("a label of 32 characters that composes to 64 was accepted: %v", err)
	}
}

func TestComposerComposesAndLeavesUnlimitedFieldsAlone(t *testing.T) {
	var c content.Composer
	typed := decomposed("Đề kiểm tra")
	if got := c.Text("title", typed, 0); got != "Đề kiểm tra" {
		t.Fatalf("title=%q", got)
	}
	if got := c.Optional("note", nil, 10); got != nil {
		t.Fatalf("a missing field became %v", *got)
	}
	empty := ""
	if got := c.Optional("note", &empty, 10); got == nil || *got != "" {
		t.Fatalf("an empty field changed: %v", got)
	}
	if got := c.Each("tags", []string{decomposed("nghé"), "shared"}, 0); len(got) != 2 || got[0] != "nghé" || got[1] != "shared" {
		t.Fatalf("tags=%q", got)
	}
	if got := c.Each("tags", nil, 0); got != nil {
		t.Fatalf("a missing list became %v", got)
	}
	if err := c.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestComposerRefusesAValueComposingLeavesOverItsLimit(t *testing.T) {
	var c content.Composer
	const lengthens = "क़"
	typed := strings.Repeat(lengthens, 100)
	if got := c.Text("title", typed, 200); got != strings.Repeat(norm.NFC.String(lengthens), 100) {
		t.Fatalf("a value within its limit after composing was refused")
	}
	if err := c.Err(); err != nil {
		t.Fatal(err)
	}
	atLimit := strings.Repeat(lengthens, 101)
	if got := c.Text("title", atLimit, 200); got != atLimit {
		t.Fatal("a refused value must come back as typed")
	}
	_ = c.Optional("note", &atLimit, 200)
	_ = c.Each("tags", []string{"ok", atLimit}, 200)
	var invalid *validation.Error
	if err := c.Err(); !errors.As(err, &invalid) {
		t.Fatalf("err=%v", err)
	}
	var names []string
	for _, field := range invalid.Fields {
		names = append(names, field.Field)
	}
	if strings.Join(names, ",") != "title,note,tags[1]" {
		t.Fatalf("fields=%v", names)
	}
}

func TestNFCWithinComposesUnlessComposingWouldTakeTheTextOverItsLimit(t *testing.T) {
	if got := content.NFCWithin(decomposed("Nguyễn Văn Á"), 200); got != "Nguyễn Văn Á" {
		t.Fatalf("got %q", got)
	}
	if got := content.NFCWithin(decomposed("Nguyễn Văn Á"), 0); got != "Nguyễn Văn Á" {
		t.Fatalf("a limit of zero means none: %q", got)
	}
	atLimit := strings.Repeat("क़", 101)
	if got := content.NFCWithin(atLimit, 200); got != atLimit {
		t.Fatal("a text composing would take over its limit must come back as it was")
	}
	within := strings.Repeat("क़", 100)
	if got := content.NFCWithin(within, 200); got != strings.Repeat(norm.NFC.String("क़"), 100) {
		t.Fatal("a text that composes within its limit was left alone")
	}
}

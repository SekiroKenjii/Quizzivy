package domain_test

import (
	"testing"

	"quizzivy/internal/modules/tests/domain"
)

func TestChangeNoteIsTrimmedAndABlankOneIsNone(t *testing.T) {
	text := func(s string) *string { return &s }
	cases := []struct {
		name string
		in   *string
		want *string
	}{
		{"absent", nil, nil},
		{"empty", text(""), nil},
		{"spaces", text("   "), nil},
		{"tabs and newlines", text("\t\n \r\n"), nil},
		{"no-break space", text("  "), nil},
		{"plain", text("Sửa câu 2"), text("Sửa câu 2")},
		{"trimmed", text("  Sửa câu 2 \n"), text("Sửa câu 2")},
		{"inner spacing is kept", text("Sửa  câu\n2"), text("Sửa  câu\n2")},
		{"no-break space at the edge", text(" Thêm phần Nghe "), text("Thêm phần Nghe")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := domain.Publishing.ChangeNote(c.in)
			switch {
			case c.want == nil && got != nil:
				t.Fatalf("ChangeNote(%q) = %q, want none", *c.in, *got)
			case c.want != nil && got == nil:
				t.Fatalf("ChangeNote(%q) = none, want %q", *c.in, *c.want)
			case c.want != nil && *got != *c.want:
				t.Fatalf("ChangeNote(%q) = %q, want %q", *c.in, *got, *c.want)
			}
		})
	}
}

func TestChangeNoteDoesNotChangeItsInput(t *testing.T) {
	raw := "  giữ nguyên  "
	_ = domain.Publishing.ChangeNote(&raw)
	if raw != "  giữ nguyên  " {
		t.Fatalf("the caller's note became %q", raw)
	}
}

package validation_test

import (
	"strings"
	"testing"

	"quizzivy/internal/shared/validation"
)

func glyph(code rune) string { return string(code) }

func TestTidyComposesAndNormalisesLineEndings(t *testing.T) {
	acute, circumflex := glyph(0x301), glyph(0x302)
	for _, c := range []struct{ name, in, want string }{
		{"a decomposed word is composed", "Gia" + acute + "o vie" + circumflex + "n", "Giáo viên"},
		{"a composed word is kept", "Giáo viên", "Giáo viên"},
		{"space around the text goes", " \t Tổ chấm \n ", "Tổ chấm"},
		{"CR LF becomes LF", "một\r\nhai", "một\nhai"},
		{"a lone CR becomes LF", "một\rhai", "một\nhai"},
		{"LF is kept", "một\nhai", "một\nhai"},
		{"a blank text is empty", " \r\n ", ""},
	} {
		if got := validation.Tidy(c.in); got != c.want {
			t.Errorf("%s: Tidy(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestFirstHiddenFindsWhatAReaderCannotSee(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		want rune
	}{
		{"a zero-width space", "Tổ" + glyph(0x200b) + "chấm", 0x200b},
		{"a zero-width joiner", "a" + glyph(0x200d) + "b", 0x200d},
		{"a soft hyphen", "a" + glyph(0xad) + "b", 0xad},
		{"a byte order mark", glyph(0xfeff) + "a", 0xfeff},
		{"a right-to-left override", "a" + glyph(0x202e) + "b", 0x202e},
		{"a tab", "a\tb", '\t'},
		{"a line feed", "a\nb", '\n'},
		{"a carriage return", "a\rb", '\r'},
		{"a line separator", "a" + glyph(0x2028) + "b", 0x2028},
		{"a paragraph separator", "a" + glyph(0x2029) + "b", 0x2029},
		{"a delete", "a\x7fb", 0x7f},
		{"the Hangul filler", "a" + glyph(0x3164), 0x3164},
		{"the halfwidth Hangul filler", glyph(0xffa0), 0xffa0},
		{"a Hangul choseong filler", glyph(0x115f), 0x115f},
		{"a Hangul jungseong filler", glyph(0x1160), 0x1160},
		{"the Braille blank", glyph(0x2800), 0x2800},
		{"a private-use character", "a" + glyph(0xe000), 0xe000},
		{"a non-character", "a" + glyph(0xfffe), 0xfffe},
		{"an unassigned code point", glyph(0x378), 0x378},
		{"the first of two", "a" + glyph(0x200b) + glyph(0x3164), 0x200b},
	} {
		got, found := validation.FirstHidden(c.in)
		if !found || got != c.want {
			t.Errorf("%s: FirstHidden(%q) = %U, %v; want %U, true", c.name, c.in, got, found, c.want)
		}
	}
}

func TestFirstHiddenLeavesWhatAReaderSees(t *testing.T) {
	for _, in := range []string{
		"", "Giáo viên", "Đặng", "Tổ 1 & 2", "a b", "x" + glyph(0x323) + glyph(0x302), "漢字", "Привет", "مرحبا",
		glyph(0x2764) + glyph(0xfe0f), "5" + glyph(0xfe0f) + glyph(0x20e3), glyph(0x1f600), glyph(0xa0) + "a", glyph(0x3000) + "a",
	} {
		if got, found := validation.FirstHidden(in); found {
			t.Errorf("FirstHidden(%q) = %U, true; want none", in, got)
		}
	}
}

func TestFirstHiddenSkipsTheRunesTheCallerAllows(t *testing.T) {
	if got, found := validation.FirstHidden("a\nb", '\n'); found {
		t.Errorf("FirstHidden with LF allowed = %U, true; want none", got)
	}
	if got, found := validation.FirstHidden("a\n\tb", '\n'); !found || got != '\t' {
		t.Errorf("FirstHidden with LF allowed = %U, %v; want the tab", got, found)
	}
}

func TestHasGlyphNeedsALetterADigitAMarkOrASymbol(t *testing.T) {
	for in, want := range map[string]bool{
		"Tổ chấm":                     true,
		"2026":                        true,
		glyph(0x2014):                 true,
		glyph(0x2764) + glyph(0xfe0f): true,
		glyph(0x1f600):                true,
		glyph(0x301) + "x":            true,
		glyph(0x301):                  false,
		glyph(0xfe0f):                 false,
		glyph(0x301) + glyph(0x323):   false,
		"":                            false,
		" ":                           false,
		glyph(0x3164):                 false,
	} {
		if got := validation.HasGlyph(in); got != want {
			t.Errorf("HasGlyph(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestHiddenMessageNamesTheCodePoint(t *testing.T) {
	for r, want := range map[rune]string{0x200b: "U+200B", '\n': "U+000A", 0x1f600: "U+1F600"} {
		if got := validation.HiddenMessage(r); !strings.Contains(got, want) {
			t.Errorf("HiddenMessage(%U) = %q, want it to name %s", r, got, want)
		}
	}
}

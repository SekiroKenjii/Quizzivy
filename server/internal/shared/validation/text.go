package validation

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// NoGlyphMessage is the field message for a text that holds nothing a reader
// can see: marks and spaces alone.
const NoGlyphMessage = "must contain a letter, a digit, a punctuation mark or a symbol"

// Tidy returns s in Unicode NFC, with CR LF and a lone CR as LF and the space
// around it removed, so the composed and decomposed forms of one text, and the
// line endings of any client, are one value. Count and compare a text after
// Tidy, never before.
func Tidy(s string) string {
	s = strings.ReplaceAll(norm.NFC.String(s), "\r\n", "\n")
	return strings.TrimSpace(strings.ReplaceAll(s, "\r", "\n"))
}

// FirstHidden returns the first rune of s that a reader cannot see, and false
// when there is none. A rune is hidden when it is not graphic (a control, a
// format character such as a zero-width space, a line or paragraph separator,
// a private-use or unassigned code point) or one of the blank fillers: the
// Hangul fillers, the halfwidth Hangul filler and the Braille blank. Runes
// listed in allowed are never reported. Which code points are unassigned
// follows the Unicode tables of the Go toolchain that builds the server.
func FirstHidden(s string, allowed ...rune) (rune, bool) {
	for _, r := range s {
		if (!unicode.IsGraphic(r) || blank(r)) && !slices.Contains(allowed, r) {
			return r, true
		}
	}
	return 0, false
}

// HasGlyph reports whether s holds a letter, a digit, a punctuation mark or a
// symbol, which a lone combining accent, a variation selector or a blank filler
// is not.
func HasGlyph(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		return unicode.In(r, unicode.L, unicode.N, unicode.P, unicode.S) && !blank(r)
	})
}

// HiddenMessage is the field message for a text that holds the hidden rune r,
// naming its code point.
func HiddenMessage(r rune) string {
	return fmt.Sprintf("must not contain control or invisible characters (found U+%04X)", r)
}

func blank(r rune) bool {
	switch r {
	case 0x115f, 0x1160, 0x2800, 0x3164, 0xffa0:
		return true
	}
	return false
}

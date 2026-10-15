package domain

import (
	"unicode"

	"quizzivy/internal/shared/validation"
)

// HiddenInEmail returns the first character of address that a reader cannot
// see or that is a space, and false when there is none. Such a character does
// not make an address malformed to the contract, but it lets a look-alike of an
// existing address pass the check for one that is taken. A combining mark that
// draws nothing of its own (a variation selector, the combining grapheme joiner)
// is hidden; an accent is not, because a Vietnamese address may be typed
// decomposed. The address is judged as given: it is neither composed nor
// trimmed, because the mailbox it names is the bytes it was typed with. One
// rule serves a form and a file of accounts.
func HiddenInEmail(address string) (hidden rune, found bool) {
	for _, r := range address {
		if _, unseen := validation.FirstHidden(string(r)); unseen || glyphless(r) || unicode.IsSpace(r) || unicode.In(r, unicode.Z) {
			return r, true
		}
	}
	return 0, false
}

func glyphless(r rune) bool {
	switch {
	case r == 0x034F:
		return true
	case r >= 0x180B && r <= 0x180F:
		return true
	case r >= 0xFE00 && r <= 0xFE0F:
		return true
	default:
		return r >= 0xE0100 && r <= 0xE01EF
	}
}

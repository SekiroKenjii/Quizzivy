package domain

import (
	"unicode"

	"quizzivy/internal/shared/validation"
)

// HiddenInEmail returns the first character of address that a reader cannot
// see or that is a space, and false when there is none. Such a character does
// not make an address malformed to the contract, but it lets a look-alike of an
// existing address pass the check for one that is taken. The address is judged
// as given: it is neither composed nor trimmed, because the mailbox it names is
// the bytes it was typed with. One rule serves a form and a file of accounts.
func HiddenInEmail(address string) (hidden rune, found bool) {
	for _, r := range address {
		if _, unseen := validation.FirstHidden(string(r)); unseen || unicode.IsSpace(r) || unicode.In(r, unicode.Z) {
			return r, true
		}
	}
	return 0, false
}

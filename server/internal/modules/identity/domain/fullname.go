package domain

import (
	"strings"
	"unicode/utf8"

	"quizzivy/internal/shared/validation"
)

// FullNameFault says why CleanFullName refused a name.
type FullNameFault int

// The faults, in the order they are looked for.
const (
	FullNameOK FullNameFault = iota
	FullNameEmpty
	FullNameTooLong
	FullNameHidden
	FullNameNoGlyph
)

// CleanFullName returns raw as the full name of an account: in Unicode NFC,
// with each run of white space one space and none at either end. It also says
// why the name cannot be kept: it is empty, longer than MaxFullNameLength
// characters, holds a character a reader cannot see (the first is returned as
// hidden), or holds no letter, digit, punctuation mark or symbol. The one rule
// serves a form and a file of accounts alike, so the two admit the same names.
func CleanFullName(raw string) (name string, fault FullNameFault, hidden rune) {
	name = validation.Tidy(strings.Join(strings.Fields(raw), " "))
	switch {
	case name == "":
		return name, FullNameEmpty, 0
	case utf8.RuneCountInString(name) > MaxFullNameLength:
		return name, FullNameTooLong, 0
	}
	if r, found := validation.FirstHidden(name); found {
		return name, FullNameHidden, r
	}
	if !validation.HasGlyph(name) {
		return name, FullNameNoGlyph, 0
	}
	return name, FullNameOK, 0
}

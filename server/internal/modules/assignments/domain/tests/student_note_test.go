package domain_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/shared/answered"
)

func answeredWhitespace(t *testing.T) []rune {
	t.Helper()
	literal := regexp.MustCompile(`E'((?:\\u[0-9A-Fa-f]{4})+)'`).FindStringSubmatch(answered.SaysSomething("a"))
	if literal == nil {
		t.Fatal("the answered rule no longer carries its whitespace literal")
	}
	var set []rune
	for _, escape := range strings.Split(literal[1], `\u`)[1:] {
		code, err := strconv.ParseUint(escape, 16, 32)
		if err != nil {
			t.Fatal(err)
		}
		set = append(set, rune(code))
	}
	return set
}

func TestTheNoteIsTrimmedOfTheWhitespaceTheAnsweredRuleCounts(t *testing.T) {
	set := answeredWhitespace(t)
	if len(set) != 25 {
		t.Fatalf("the answered rule's set holds %d code points, want 25", len(set))
	}
	for _, space := range set {
		blank := string(space)
		if got := domain.StudentNoteOf(&blank); got != nil {
			t.Errorf("U+%04X alone is stored as %q, want none", space, *got)
		}
		padded := string(space) + "Mang bút" + string(space) + string(space)
		got := domain.StudentNoteOf(&padded)
		if got == nil || *got != "Mang bút" {
			t.Errorf("U+%04X around a note: stored %v, want the note trimmed", space, got)
		}
	}
}

func TestTheNoteKeepsWhatThatSetDoesNotTrim(t *testing.T) {
	inner := "Mang\u00A0theo\nmáy tính"
	for _, note := range []string{inner, "\u0085", "a\u0085", "\u200B", "\u200Bz"} {
		got := domain.StudentNoteOf(&note)
		if got == nil || *got != note {
			t.Errorf("%q: stored %v, want it as written", note, got)
		}
	}
	var none *string
	if got := domain.StudentNoteOf(none); got != nil {
		t.Errorf("no note: stored %q", *got)
	}
}

package domain_test

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"quizzivy/internal/modules/identity/domain"
)

func TestTheWordsStayTypeable(t *testing.T) {
	for range 300 {
		got, err := domain.Passwords.Temporary()
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range strings.Split(got, "-")[:2] {
			for _, r := range w {
				if r > unicode.MaxASCII || !unicode.IsLower(r) {
					t.Fatalf("%q carries %q: a word read aloud must be plain lowercase ASCII", got, w)
				}
			}
		}
	}
}

func TestATemporaryPasswordCanActuallyBeUsed(t *testing.T) {
	for range 200 {
		got, err := domain.Passwords.Temporary()
		if err != nil {
			t.Fatalf("TemporaryPassword: %v", err)
		}

		if len(got) < domain.MinPasswordLength {
			t.Fatalf("%q is %d bytes, under the %d the login schema requires",
				got, len(got), domain.MinPasswordLength)
		}
		if n := utf8.RuneCountInString(got); n < domain.MinPasswordLength || n > domain.MaxPasswordLength {
			t.Fatalf("%q is %d runes, outside [%d,%d]", got, n, domain.MinPasswordLength, domain.MaxPasswordLength)
		}

		parts := strings.Split(got, "-")
		if len(parts) != 3 {
			t.Fatalf("%q is not word-word-digits", got)
		}
		if len(parts[2]) != 2 {
			t.Errorf("%q: a leading zero is read aloud as one digit and typed as two", got)
		}
	}
}

func TestTemporaryPasswordsDiffer(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		got, err := domain.Passwords.Temporary()
		if err != nil {
			t.Fatal(err)
		}
		seen[got] = true
	}
	if len(seen) < 50 {
		t.Errorf("100 draws produced %d distinct passwords", len(seen))
	}
}

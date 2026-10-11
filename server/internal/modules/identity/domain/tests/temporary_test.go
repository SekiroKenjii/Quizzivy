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
		for _, w := range strings.Split(got, "-")[:3] {
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
		if len(parts) != 4 {
			t.Fatalf("%q is not word-word-word-digits", got)
		}
		if len(parts[3]) != 2 || parts[3][0] == '0' {
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

func TestATemporaryPasswordCarriesAboutTwentyFourBits(t *testing.T) {
	if bits := domain.Passwords.TemporaryBits(); bits < 24 || bits > 25 {
		t.Errorf("a temporary password carries %.2f bits, want about 24.4 (F-53: three words and two digits)", bits)
	}
}

func TestEveryWordOfATemporaryPasswordIsDrawnIndependently(t *testing.T) {
	positions := [3]map[string]bool{{}, {}, {}}
	for range 600 {
		got, err := domain.Passwords.Temporary()
		if err != nil {
			t.Fatal(err)
		}
		for i, w := range strings.Split(got, "-")[:3] {
			positions[i][w] = true
		}
	}
	for i, seen := range positions {
		if len(seen) < 40 {
			t.Errorf("word %d took %d distinct values in 600 draws, want most of the list", i+1, len(seen))
		}
	}
}

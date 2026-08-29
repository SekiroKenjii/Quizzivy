package auth

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// The word list is hand-curated, so these guard the properties curation can
// silently break: an accented word cannot be typed by the student who needed
// the reset, and a duplicate quietly costs entropy nobody recounts.
func TestTheWordListStaysTypeableAndDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, w := range temporaryWords {
		if seen[w] {
			t.Errorf("%q appears twice; a duplicate costs entropy without saying so", w)
		}
		seen[w] = true

		for _, r := range w {
			if r > unicode.MaxASCII {
				t.Errorf("%q is not ASCII: Argon2id hashes bytes, so an accented "+
					"word is a different password from the one read aloud", w)
				break
			}
			if !unicode.IsLower(r) {
				t.Errorf("%q is not lowercase; the hyphens are the only separator", w)
				break
			}
		}
	}

	// Below this the two-word space stops being defensible even single-use.
	if len(seen) < 60 {
		t.Errorf("word list is %d entries, want at least 60", len(seen))
	}
}

func TestATemporaryPasswordCanActuallyBeUsed(t *testing.T) {
	for range 200 {
		got, err := TemporaryPassword()
		if err != nil {
			t.Fatalf("TemporaryPassword: %v", err)
		}

		// The login endpoint enforces this before any handler runs, so a short
		// one locks the student out of the account it was meant to rescue.
		if len(got) < MinPasswordLength {
			t.Fatalf("%q is %d bytes, under the %d the login schema requires",
				got, len(got), MinPasswordLength)
		}
		// ChangePassword counts runes, not bytes; the list is ASCII so these
		// agree, and this fails loudly if an accented word ever slips in.
		if n := utf8.RuneCountInString(got); n < MinPasswordLength || n > MaxPasswordLength {
			t.Fatalf("%q is %d runes, outside [%d,%d]", got, n, MinPasswordLength, MaxPasswordLength)
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
		got, err := TemporaryPassword()
		if err != nil {
			t.Fatal(err)
		}
		seen[got] = true
	}
	// Not a randomness test -- a stuck generator returning one value is the
	// failure this catches.
	if len(seen) < 50 {
		t.Errorf("100 draws produced %d distinct passwords", len(seen))
	}
}

package joincode_test

import (
	"strings"
	"testing"

	"quizzivy/internal/modules/classes/domain/joincode"
)

// A join code is a bearer secret (§6.1): whoever holds it can enrol. These
// tests are about the two properties that follow -- it must be unguessable, and
// it must survive being read aloud, written down, and typed back in.

func TestNormalizeAcceptsHoweverItWasTypedBack(t *testing.T) {
	canonical := joincode.Normalize("K7M3P9QR")
	if canonical != "K7M3P9QR" {
		t.Fatalf("canonical form changed: %q", canonical)
	}

	for _, typed := range []string{
		"K7M3P9QR",
		"K7M3-P9QR",
		"k7m3-p9qr",
		"k7m3 p9qr",
		"  K7M3 - P9QR  ",
		"K7M3–P9QR", // en dash, courtesy of a phone
		"K7M3_P9QR",
	} {
		if got := joincode.Normalize(typed); got != canonical {
			t.Errorf("Normalize(%q) = %q, want %q", typed, got, canonical)
		}
	}
}

func TestNormalizeIsIdempotentAcrossThePlansThreeSpellings(t *testing.T) {
	first := joincode.Normalize("abcd-1234")
	for _, typed := range []string{"abcd-1234", "ABCD1234", "abcd 1234"} {
		if got := joincode.Normalize(typed); got != first {
			t.Errorf("Normalize(%q) = %q, want %q", typed, got, first)
		}
	}
	if strings.ContainsAny(first, "1") {
		t.Errorf("Normalize kept a character outside the alphabet: %q", first)
	}

	// Idempotent in the strict sense too: normalizing twice changes nothing.
	if again := joincode.Normalize(first); again != first {
		t.Errorf("Normalize is not idempotent: %q -> %q", first, again)
	}
}

func TestGeneratedCodesAvoidTheAmbiguousCharacters(t *testing.T) {
	const runs = 100_000
	seen := map[rune]int{}

	for i := range runs {
		code, err := joincode.Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if len(code) != joincode.Length {
			t.Fatalf("run %d: length = %d, want %d", i, len(code), joincode.Length)
		}
		for _, r := range code {
			if strings.ContainsRune("0O1I", r) {
				t.Fatalf("run %d: code %q contains the ambiguous character %q", i, code, r)
			}
			if !strings.ContainsRune(joincode.Alphabet, r) {
				t.Fatalf("run %d: code %q contains %q, which is outside the alphabet", i, code, r)
			}
			seen[r]++
		}
	}
	if len(seen) != len(joincode.Alphabet) {
		t.Errorf("only %d of %d alphabet characters were ever produced", len(seen), len(joincode.Alphabet))
	}
	expected := runs * joincode.Length / len(joincode.Alphabet)
	for r, count := range seen {
		if count < expected/2 || count > expected*2 {
			t.Errorf("character %q appeared %d times, expected around %d -- the selection is biased",
				r, count, expected)
		}
	}
}

func TestCodesDoNotRepeat(t *testing.T) {
	seen := make(map[string]struct{}, 10_000)
	for range 10_000 {
		code, err := joincode.Generate()
		if err != nil {
			t.Fatal(err)
		}
		if _, dup := seen[code]; dup {
			t.Fatalf("generated %q twice in ten thousand draws", code)
		}
		seen[code] = struct{}{}
	}
}

func TestFormatGroupsForDisplayAndSurvivesTheRoundTrip(t *testing.T) {
	code, err := joincode.Generate()
	if err != nil {
		t.Fatal(err)
	}
	grouped := joincode.Format(code)
	if len(grouped) != joincode.Length+1 || grouped[4] != '-' {
		t.Fatalf("Format(%q) = %q, want XXXX-XXXX", code, grouped)
	}
	// What the teacher sees must hash to what was stored.
	if joincode.Normalize(grouped) != code {
		t.Errorf("the displayed form does not normalize back: %q -> %q", grouped, joincode.Normalize(grouped))
	}
}

func TestTheHintIsTheLastFourCharacters(t *testing.T) {
	if got := joincode.Hint("K7M3P9QR"); got != "P9QR" {
		t.Errorf("Hint = %q, want P9QR", got)
	}
}

func TestHashingIsStableAndTheCodeIsNotRecoverableFromIt(t *testing.T) {
	code, err := joincode.Generate()
	if err != nil {
		t.Fatal(err)
	}
	h := joincode.Hash(code)
	if len(h) != 32 {
		t.Fatalf("hash is %d bytes, want 32 -- the column CHECKs for it", len(h))
	}
	if !joincode.Equal(h, joincode.Hash(code)) {
		t.Error("hashing the same code twice produced different output")
	}
	if joincode.Equal(h, joincode.Hash("K7M3P9QR")) {
		t.Error("two different codes hashed alike")
	}
	if strings.Contains(string(h), code) {
		t.Error("the plaintext is recoverable from the stored hash")
	}
}

func TestDifferentSpellingsOfOneCodeHashAlike(t *testing.T) {
	want := joincode.Hash(joincode.Normalize("K7M3P9QR"))
	for _, typed := range []string{"k7m3-p9qr", "K7M3 P9QR", " k7m3p9qr "} {
		if !joincode.Equal(want, joincode.Hash(joincode.Normalize(typed))) {
			t.Errorf("%q hashed to a different value", typed)
		}
	}
}

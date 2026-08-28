// Package join owns class join codes: generating them, recognising them, and
// revoking them. A join code is a BEARER SECRET (§6.1) -- whoever holds it can
// enrol -- so it is generated, stored and compared like one.
package join

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"
)

// Alphabet is §6.1's, exactly. Thirty-two characters, and the omissions are the
// point: `I` and `O` are gone because they read as `1` and `0`, and `1` and `0`
// are gone for the same reason.
//
// `L` stays, and that is not an oversight. §6.1's "(no 0/O, 1/I/L)" names the
// two CONFUSION GROUPS, not a blocklist of characters -- from {1, I, L} it
// keeps one member, and with `1` and `I` both absent there is nothing left for
// `L` to be mistaken for. Removing it as well would make the alphabet 31
// characters, which is worse: no longer a power of two, so uniform selection
// needs rejection sampling for no gain in legibility.
const Alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// Length is the number of characters in a code, before grouping.
const Length = 8

// HintLength is how much of a code survives the one-time reveal, for the admin
// to recognise which code is active (§13.3).
const HintLength = 4

// Generate returns a new code in canonical form (ungrouped, upper case).
//
// len(Alphabet) is 32, which divides 256, so masking a random byte with 31
// selects uniformly with no modulo bias and no rejection loop. If the alphabet
// ever changes length this stops being true, hence the assertion.
func Generate() (string, error) {
	if len(Alphabet) != 32 {
		return "", fmt.Errorf("join: alphabet is %d characters; uniform selection assumes 32", len(Alphabet))
	}

	raw := make([]byte, Length)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("join: generate code: %w", err)
	}

	out := make([]byte, Length)
	for i, b := range raw {
		out[i] = Alphabet[b&31]
	}
	return string(out), nil
}

// Format groups a code for display: XXXX-XXXX (§6.1).
func Format(code string) string {
	if len(code) != Length {
		return code
	}
	return code[:4] + "-" + code[4:]
}

// Normalize turns what a person typed into the canonical form used for hashing.
//
// §6.1 accepts a code with or without the dash and in any case. In practice
// people also paste it with spaces, or with a non-ASCII dash their phone
// substituted, so anything that is not a character of the alphabet is dropped
// rather than enumerated. Normalizing does NOT validate: an unrecognisable code
// simply hashes to something no row holds, which is the same answer a wrong
// code gets and reveals nothing extra (§6.5).
func Normalize(input string) string {
	var b strings.Builder
	b.Grow(len(input))
	for _, r := range strings.ToUpper(strings.TrimSpace(input)) {
		if strings.ContainsRune(Alphabet, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Hash is the stored form. SHA-256, not Argon2id: a code is 8 characters from a
// 32-symbol alphabet, so 40 bits of entropy from a CSPRNG. That is far too much
// to guess online at §6.5's rate limits, and unlike a password it is not chosen
// by a human, not reused elsewhere, and short-lived.
func Hash(normalized string) []byte {
	sum := sha256.Sum256([]byte(normalized))
	return sum[:]
}

// Equal compares two code hashes in constant time (§13.5).
//
// The lookup itself is a b-tree probe on code_hash and is not constant-time;
// this guards the place §13.5 actually names, and keeps the property if a
// caller ever compares two hashes it already holds.
func Equal(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// Hint is the last four characters, which is all that remains visible after the
// one-time reveal.
func Hint(normalized string) string {
	if len(normalized) < HintLength {
		return normalized
	}
	return normalized[len(normalized)-HintLength:]
}

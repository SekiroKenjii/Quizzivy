package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"
	"time"
)

// Generate returns a new code in canonical form (ungrouped, upper case).
func (JoinCodeManager) Generate() (string, error) {
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
func (JoinCodeManager) Format(code string) string {
	if len(code) != Length {
		return code
	}
	return code[:4] + "-" + code[4:]
}

// Normalize turns what a person typed into the canonical form used for hashing.
func (JoinCodeManager) Normalize(input string) string {
	var b strings.Builder
	b.Grow(len(input))
	for _, r := range strings.ToUpper(strings.TrimSpace(input)) {
		if strings.ContainsRune(Alphabet, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Hash is the legacy lookup hash, SHA-256, which v0.7.0 stored and its rows
// still carry. New rows store JoinCodeKeys.Hash instead: 40 bits of code space
// is too little to stop a database dump being hashed through offline.
func (JoinCodeManager) Hash(normalized string) []byte {
	sum := sha256.Sum256([]byte(normalized))
	return sum[:]
}

// Equal compares two code hashes in constant time (§13.5).
func (JoinCodeManager) Equal(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// Hint is the last four characters, which is all that remains visible after the
// one-time reveal.
func (JoinCodeManager) Hint(normalized string) string {
	if len(normalized) < HintLength {
		return normalized
	}
	return normalized[len(normalized)-HintLength:]
}

// JoinCodeManager mints, normalises, formats and hints join codes and computes
// their legacy hash; it holds no key.
type JoinCodeManager struct{}

var JoinCodes JoinCodeManager

// Alphabet is §6.1's, exactly. Thirty-two characters, and the omissions are the
// point: `I` and `O` are gone because they read as `1` and `0`, and `1` and `0`
// are gone for the same reason.
const Alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// Length is the number of characters in a code, before grouping.
const Length = 8

// HintLength is how much of a code survives the one-time reveal, for the admin
// to recognise which code is active (§13.3).
const HintLength = 4

// CodeState is what decides whether a join code can still be redeemed.
type CodeState struct {
	SelfJoinEnabled bool
	RevokedAt       *time.Time
	ExpiresAt       time.Time
	MaxUses         *int
	UsesCount       int
}

// CodeRow is everything a preview decision needs, in one round trip.
type CodeRow struct {
	CodeState
	ClassID     string
	ClassName   string
	Lookup      StoredLookup
	TeacherName *string
}

// StoredCode is a class's active code as stored: its metadata, how it is
// found, and its ciphertext when it is sealed.
type StoredCode struct {
	IssuedCode
	Lookup     StoredLookup
	Ciphertext []byte
}

// ActiveJoinCode is a class's active code read back for its teacher. Code is
// the canonical code, or empty when it cannot be read: a legacy code, which
// only a hash holds, or one sealed under a key this server no longer holds.
type ActiveJoinCode struct {
	IssuedCode
	Code   string
	Legacy bool
}

// IssuedCode is the metadata of an active code, never the code itself.
type IssuedCode struct {
	ID        string
	ClassID   string
	Hint      string
	ExpiresAt time.Time
	MaxUses   *int
	UsesCount int
}

// PreviewOutcome is why a code was or was not accepted. Each maps to a distinct
// error code so the student gets an accurate message -- "ask your teacher for a
// new code" is useful where "wrong code" is not -- without any of them naming a
// class (§6.5).
type PreviewOutcome int

const (
	PreviewOK PreviewOutcome = iota
	PreviewInvalid
	PreviewRevoked
	PreviewExpired
	PreviewExhausted
)

// PreviewResult carries exactly the three fields §6.5 permits, and the outcome.
type PreviewResult struct {
	Outcome     PreviewOutcome
	ClassID     string
	ClassName   string
	TeacherName string
}

// Usable reports PreviewOK or the reason the code is refused. A class with
// self-join closed answers exactly as a nonexistent code does, so the endpoints
// cannot be used to discover which classes exist.
func (c CodeState) Usable(now time.Time) PreviewOutcome {
	switch {
	case !c.SelfJoinEnabled:
		return PreviewInvalid
	case c.RevokedAt != nil:
		return PreviewRevoked
	case !c.ExpiresAt.After(now):
		return PreviewExpired
	case c.MaxUses != nil && c.UsesCount >= *c.MaxUses:
		return PreviewExhausted
	}
	return PreviewOK
}

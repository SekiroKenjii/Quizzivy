// Package auth owns credentials, sessions and the login flow (§5).
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// §13.5: "Passwords: Argon2id (bcrypt cost >= 12 if unavailable)."
//
// Hashes are stored in PHC string format:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
//
// The parameters travel with the hash, which matters more than it looks. Tuning
// them later re-hashes new passwords without invalidating old ones, and a hash
// generated elsewhere -- the dev seed, a migration from another system -- still
// verifies. A bare hash plus package-level constants would silently stop
// matching the day the constants changed, and the symptom would be "wrong
// password" for every existing user.
const (
	defaultMemory  = 64 * 1024 // 64 MiB
	defaultTime    = 3
	defaultThreads = 2
	defaultKeyLen  = 32
	saltLen        = 16
)

var (
	ErrInvalidHash        = errors.New("password hash is not in PHC format")
	ErrUnsupportedVariant = errors.New("password hash is not argon2id")
	ErrIncompatibleAlg    = errors.New("password hash uses an unsupported argon2 version")
)

type params struct {
	memory  uint32
	time    uint32
	threads uint8
	keyLen  uint32
}

// HashPassword produces a PHC-format Argon2id hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, defaultTime, defaultMemory, defaultThreads, defaultKeyLen)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, defaultMemory, defaultTime, defaultThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches the encoded hash.
//
// The comparison is constant-time. A byte-wise compare that returns early leaks
// how much of the derived key matched, which over many attempts narrows the
// search — the reason §13.5 asks for constant-time comparison on the join code
// applies here too.
func VerifyPassword(password, encoded string) (bool, error) {
	p, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, p.time, p.memory, p.threads, p.keyLen)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func decodeHash(encoded string) (params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// "", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash
	if len(parts) != 6 || parts[0] != "" {
		return params{}, nil, nil, ErrInvalidHash
	}
	if parts[1] != "argon2id" {
		return params{}, nil, nil, ErrUnsupportedVariant
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return params{}, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return params{}, nil, nil, ErrIncompatibleAlg
	}

	var p params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return params{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return params{}, nil, nil, ErrInvalidHash
	}
	want, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return params{}, nil, nil, ErrInvalidHash
	}
	p.keyLen = uint32(len(want))
	if p.keyLen == 0 || len(salt) == 0 {
		return params{}, nil, nil, ErrInvalidHash
	}
	return p, salt, want, nil
}

// dummyHash is verified against when no user matches, so a login attempt for an
// unknown email costs the same Argon2id work as one for a real account.
//
// Without it, "no such user" returns in microseconds while a real account
// spends ~50ms hashing -- and that difference is measurable over a handful of
// requests, turning the login endpoint into a user-enumeration oracle. §6.5
// asks the join endpoints not to leak which classes exist; the same reasoning
// applies to which accounts do.
var dummyHash string

func init() {
	h, err := HashPassword("quizzivy-timing-equaliser")
	if err != nil {
		panic("auth: cannot initialise dummy hash: " + err.Error())
	}
	dummyHash = h
}

// BurnPasswordTime performs the same work as a real verification and discards
// the result. Called when no user matches.
func BurnPasswordTime(password string) {
	_, _ = VerifyPassword(password, dummyHash)
}

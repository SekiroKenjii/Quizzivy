package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// §13.5: "Passwords: Argon2id (bcrypt cost >= 12 if unavailable)."
const (
	defaultMemory  = 64 * 1024 // 64 MiB
	defaultTime    = 3
	defaultThreads = 2
	defaultKeyLen  = 32
	saltLen        = 16
)

// DefaultMaxConcurrentHashes bounds how many Argon2id operations run at once.
const DefaultMaxConcurrentHashes = 4

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

// hashSlots bounds concurrent Argon2id work. A buffered channel rather than a
// semaphore package: the whole contract is "hold one of N tokens", and adding a
// dependency for that would be more code than this is.
var hashSlots = make(chan struct{}, DefaultMaxConcurrentHashes)

// SetMaxConcurrentHashes resizes the bound.
//
// Must be called BEFORE the server starts serving, which is the only reason
// this is safe without a lock: every handler goroutine is created after, and
// goroutine creation is a happens-before edge. Calling it once main is running
// is a data race.
func SetMaxConcurrentHashes(n int) {
	if n < 1 {
		n = 1
	}
	hashSlots = make(chan struct{}, n)
}

// withHashSlot runs fn holding one of the concurrency tokens.
//
// Callers WAIT rather than being refused. A student queueing behind three
// classmates waits a few hundred milliseconds; being told to try again is a
// worse answer to "the lesson started". The context is what stops that queue
// growing without limit -- a caller whose client has already gone gives its
// place up instead of allocating 64 MiB for nobody.
func withHashSlot(ctx context.Context, fn func()) error {
	// Captured once: re-reading at release would deadlock across a resize.
	slots := hashSlots

	select {
	case slots <- struct{}{}:
	case <-ctx.Done():
		return fmt.Errorf("waiting for a password-hash slot: %w", ctx.Err())
	}
	defer func() { <-slots }()
	fn()
	return nil
}

// HashPassword produces a PHC-format Argon2id hash.
func HashPassword(ctx context.Context, password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	var key []byte
	if err := withHashSlot(ctx, func() {
		key = argon2.IDKey([]byte(password), salt, defaultTime, defaultMemory, defaultThreads, defaultKeyLen)
	}); err != nil {
		return "", err
	}

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
func VerifyPassword(ctx context.Context, password, encoded string) (bool, error) {
	p, salt, want, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	var got []byte
	if err := withHashSlot(ctx, func() {
		got = argon2.IDKey([]byte(password), salt, p.time, p.memory, p.threads, p.keyLen)
	}); err != nil {
		return false, err
	}
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
var dummyHash string

func init() {
	h, err := HashPassword(context.Background(), "quizzivy-timing-equaliser")
	if err != nil {
		panic("auth: cannot initialise dummy hash: " + err.Error())
	}
	dummyHash = h
}

// BurnPasswordTime performs the same work as a real verification and discards
// the result. Called when no user matches.
func BurnPasswordTime(ctx context.Context, password string) {
	_, _ = VerifyPassword(ctx, password, dummyHash)
}

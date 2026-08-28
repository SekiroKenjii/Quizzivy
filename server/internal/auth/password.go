// Package auth owns credentials, sessions and the login flow (§5).
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

// DefaultMaxConcurrentHashes bounds how many Argon2id operations run at once.
//
// Argon2id is memory-HARD by design: `defaultMemory` is not a budget, it is an
// arena allocated for the duration of every single call. Measured on this
// build, peak RSS is 64 MiB per concurrent hash, essentially exactly:
//
//	 1 concurrent ->   67 MiB
//	 4 concurrent ->  260 MiB
//	 8 concurrent ->  517 MiB
//	30 concurrent -> 1927 MiB
//
// The production machine is a 512 MB Fly instance, so EIGHT simultaneous
// logins exceed it before the Go runtime, the pgx pool and HTTP buffers are
// counted. A class of thirty students signing in at the start of a lesson is
// an ordinary Tuesday, and the symptom is not slowness -- it is the OOM killer
// taking the process down mid-request.
//
// Rate limiting does not help: §6.5's limits are per-IP and per-email, and
// thirty students on thirty phones are thirty IPs. Nor is this only an
// accident. Every failed login for an UNKNOWN email also runs a full hash, on
// purpose (see dummyHash), so an attacker spending nothing can make the server
// allocate 64 MiB per request.
//
// Four leaves ~256 MiB for hashing and ~256 MiB for everything else. It also
// costs almost nothing in throughput, because Argon2id is CPU-hard as well as
// memory-hard: thirty simultaneous logins complete in 1.79s at four slots and
// 1.57s at eight, so doubling the memory buys 12%. Raise it when the machine
// grows; the arithmetic is memory / 64 MiB, minus headroom.
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
	// Captured ONCE. Reading the package var again at release time would
	// return whatever SetMaxConcurrentHashes last installed -- so a resize
	// between acquire and release makes the release drain a channel this call
	// never filled, and block forever. Found by the bound test, which resizes
	// in its cleanup; production only resizes at startup, where it would have
	// stayed hidden until someone made the limit reloadable.
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

	// Decoding happens outside the slot: it is microseconds of parsing, and
	// holding a 64 MiB token to do it would shrink the useful concurrency for
	// no reason. The slot covers exactly the memory-hard part.
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
//
// Without it, "no such user" returns in microseconds while a real account
// spends ~50ms hashing -- and that difference is measurable over a handful of
// requests, turning the login endpoint into a user-enumeration oracle. §6.5
// asks the join endpoints not to leak which classes exist; the same reasoning
// applies to which accounts do.
var dummyHash string

func init() {
	// context.Background(): this runs at package initialisation, before any
	// request exists and while every slot is free.
	h, err := HashPassword(context.Background(), "quizzivy-timing-equaliser")
	if err != nil {
		panic("auth: cannot initialise dummy hash: " + err.Error())
	}
	dummyHash = h
}

// BurnPasswordTime performs the same work as a real verification and discards
// the result. Called when no user matches.
//
// THE DISCARDED WORK IS THE POINT. This looks like an expensive no-op and is
// not: without it, "no such user" returns in microseconds while a real account
// spends tens of milliseconds hashing, and that gap is measurable over a
// handful of requests. Anyone optimising this into a cheaper body, or removing
// the call sites in service.go as wasted effort, turns login into a
// user-enumeration oracle.
//
// TestBurnPasswordTimeDoesRealWork holds a floor under it for exactly that
// reason, and TestDummyHashUsesTheCurrentCostParameters checks the other half:
// that the hash it verifies against still costs what a real one costs.
func BurnPasswordTime(ctx context.Context, password string) {
	_, _ = VerifyPassword(ctx, password, dummyHash)
}

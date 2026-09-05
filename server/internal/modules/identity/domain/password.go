package domain

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/argon2"
)

// PasswordManager is the password policy: Argon2id hashing under a memory
// bound, verification in constant shape, and the temporary passwords a teacher
// hands out.
type PasswordManager struct{}

// SetMaxConcurrentHashes resizes the bound.
func (PasswordManager) SetMaxConcurrentHashes(n int) {
	if n < 1 {
		n = 1
	}
	hashSlots = make(chan struct{}, n)
}

// HashPassword produces a PHC-format Argon2id hash.
func (PasswordManager) Hash(ctx context.Context, password string) (string, error) {
	salt := make([]byte, SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	var key []byte
	if err := withHashSlot(ctx, func() {
		key = argon2.IDKey([]byte(password), salt, DefaultTime, DefaultMemory, DefaultThreads, DefaultKeyLen)
	}); err != nil {
		return "", err
	}

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, DefaultMemory, DefaultTime, DefaultThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches the encoded hash.
func (PasswordManager) Verify(ctx context.Context, password, encoded string) (bool, error) {
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

// BurnPasswordTime performs the same work as a real verification and discards
// the result. Called when no user matches.
func (PasswordManager) BurnTime(ctx context.Context, password string) {
	_, _ = Passwords.Verify(ctx, password, dummyHash)
}

// TemporaryPassword returns a password a teacher can read across a room.
func (PasswordManager) Temporary() (string, error) {
	first, err := word()
	if err != nil {
		return "", err
	}
	second, err := word()
	if err != nil {
		return "", err
	}
	n, err := rand.Int(rand.Reader, big.NewInt(90))
	if err != nil {
		return "", fmt.Errorf("temporary password: %w", err)
	}

	out := fmt.Sprintf("%s-%s-%d", first, second, n.Int64()+10)
	if len(out) < MinPasswordLength {
		return "", fmt.Errorf("temporary password %q is shorter than the minimum", out)
	}
	return out, nil
}

var Passwords PasswordManager

// Password bounds from api/openapi.yaml. The maximum exists because Argon2id
// hashes whatever it is given, and a megabyte of "password" is a free way to
// burn CPU on an authenticated endpoint.
const (
	MinPasswordLength = 8
	MaxPasswordLength = 512
)

const (
	DefaultMemory  = 64 * 1024
	DefaultTime    = 3
	DefaultThreads = 2
	DefaultKeyLen  = 32
	SaltLen        = 16
)

// DefaultMaxConcurrentHashes bounds how many Argon2id operations run at once.
const DefaultMaxConcurrentHashes = 4

type params struct {
	memory  uint32
	time    uint32
	threads uint8
	keyLen  uint32
}

var hashSlots = make(chan struct{}, DefaultMaxConcurrentHashes)

func withHashSlot(ctx context.Context, fn func()) error {

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

func decodeHash(encoded string) (params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")

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

var dummyHash string

func init() {
	h, err := Passwords.Hash(context.Background(), "quizzivy-timing-equaliser")
	if err != nil {
		panic("auth: cannot initialise dummy hash: " + err.Error())
	}
	dummyHash = h
}

var temporaryWords = []string{
	"ao", "bao", "bien", "bo", "bong", "buom", "ca", "cam", "canh", "cao",
	"cay", "che", "chim", "cho", "com", "cua", "dao", "den", "deo", "dua",
	"duong", "ga", "gao", "gio", "hat", "hoa", "hong", "keo", "kem", "khoai",
	"la", "lam", "meo", "mua", "mut", "nai", "nam", "nau", "ngo",
	"nho", "nui", "oi", "ong", "pho", "quat", "rung", "sao", "sen", "song",
	"suoi", "tau", "thap", "tho", "thom", "tim", "trang", "tre", "trong",
	"vang", "voi", "xanh", "xoai", "yen",
}

func word() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(temporaryWords))))
	if err != nil {
		return "", fmt.Errorf("temporary password: %w", err)
	}
	return temporaryWords[n.Int64()], nil
}

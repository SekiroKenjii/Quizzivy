package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
)

// temporaryWords is the vocabulary a temporary password is built from.
var temporaryWords = []string{
	"ao", "bao", "bien", "bo", "bong", "buom", "ca", "cam", "canh", "cao",
	"cay", "che", "chim", "cho", "com", "cua", "dao", "den", "deo", "dua",
	"duong", "ga", "gao", "gio", "hat", "hoa", "hong", "keo", "kem", "khoai",
	"la", "lam", "meo", "mua", "mut", "nai", "nam", "nau", "ngo",
	"nho", "nui", "oi", "ong", "pho", "quat", "rung", "sao", "sen", "song",
	"suoi", "tau", "thap", "tho", "thom", "tim", "trang", "tre", "trong",
	"vang", "voi", "xanh", "xoai", "yen",
}

// TemporaryPassword returns a password a teacher can read across a room.
func TemporaryPassword() (string, error) {
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

func word() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(temporaryWords))))
	if err != nil {
		return "", fmt.Errorf("temporary password: %w", err)
	}
	return temporaryWords[n.Int64()], nil
}

// NewTemporaryPassword returns a fresh temporary password and its hash.
//
// The hash is computed here, outside any caller transaction, because Argon2id
// runs behind a four-slot semaphore that BLOCKS: hashing inside a transaction
// would hold row locks open for however long the other three slots take.
func (s *Service) NewTemporaryPassword(ctx context.Context) (password, hash string, err error) {
	password, err = TemporaryPassword()
	if err != nil {
		return "", "", err
	}
	hash, err = HashPassword(ctx, password)
	if err != nil {
		return "", "", err
	}
	return password, hash, nil
}

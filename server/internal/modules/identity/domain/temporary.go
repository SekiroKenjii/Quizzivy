package domain

import (
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

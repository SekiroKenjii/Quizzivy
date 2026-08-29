package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
)

// temporaryWords is the vocabulary a temporary password is built from.
//
// Unaccented Vietnamese, because the deck's own sample is `tho-vang-42` -- "thỏ
// vàng" with the diacritics stripped -- and the password is typed on whatever
// keyboard the student has. Argon2id hashes bytes, so "thỏ" and "tho" are
// different passwords and an accented word would fail for exactly the student
// who could not type it.
//
// Curated rather than generated: the cost here is not list size, it is that two
// words sit next to each other and must not read as an insult or as somebody's
// name when they do.
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
//
// Shape is two words and two digits, `tho-vang-42`, which the design deck fixes
// (§G-07: "The temporary password is words, not entropy soup. It gets read
// aloud across a classroom."). Entropy is bought from the word list rather than
// from a third word, so the thing stays sayable.
//
// This is deliberately weak by cryptographic standards and is only defensible
// because it is single-use, forces a change at first sign-in, and sits behind
// the login rate limit. It must never become a permanent credential.
func TemporaryPassword() (string, error) {
	first, err := word()
	if err != nil {
		return "", err
	}
	second, err := word()
	if err != nil {
		return "", err
	}
	// Two digits, so 10..99 -- a leading zero would be read aloud as one digit
	// and typed as two.
	n, err := rand.Int(rand.Reader, big.NewInt(90))
	if err != nil {
		return "", fmt.Errorf("temporary password: %w", err)
	}

	out := fmt.Sprintf("%s-%s-%d", first, second, n.Int64()+10)
	// The login endpoint enforces minLength 8 before any handler runs, so a
	// short password would not merely fail the later change -- the student
	// could not sign in with it at all, and would see a validation error that
	// reads like a server bug.
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

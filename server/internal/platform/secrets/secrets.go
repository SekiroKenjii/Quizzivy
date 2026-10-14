// Package secrets seals small values the app has to keep for a short while, such as a set-password token waiting in the email outbox, under keys derived from SECRETS_KEY: one HKDF-SHA256 key per purpose and AES-256-GCM over each value.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// KeySize is the length in bytes of SECRETS_KEY and of every key derived from it.
const KeySize = 32

// Purpose names what a derived key seals. A value sealed for one purpose
// cannot be opened under another.
type Purpose string

// The purposes the app seals values for.
const (
	PurposeEmailOutbox     Purpose = "email-outbox"
	PurposeUserImportSheet Purpose = "user-import-sheet"
)

// ErrUnreadable is what Open answers for a value it cannot open: a wrong key,
// another purpose or other associated data, or bytes that were changed.
var ErrUnreadable = errors.New("secrets: the sealed value cannot be opened")

const infoPrefix = "quizzivy/secrets/v1/"

// ParseKey decodes SECRETS_KEY: standard base64 of exactly KeySize bytes. Its
// errors never repeat the value.
func ParseKey(encoded string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, errors.New("SECRETS_KEY must be standard base64; generate one with: openssl rand -base64 32")
	}
	if len(raw) != KeySize {
		return nil, fmt.Errorf("SECRETS_KEY must decode to exactly %d bytes, got %d; generate one with: openssl rand -base64 32", KeySize, len(raw))
	}
	return raw, nil
}

// DeriveKey is the key for purpose: HKDF-SHA256 over master with no salt and
// the info "quizzivy/secrets/v1/" followed by the purpose.
func DeriveKey(master []byte, purpose Purpose) ([]byte, error) {
	if len(master) != KeySize {
		return nil, fmt.Errorf("secrets: the master key must be %d bytes", KeySize)
	}
	if purpose == "" {
		return nil, errors.New("secrets: a purpose is required")
	}
	return hkdf.Key(sha256.New, master, nil, infoPrefix+string(purpose), KeySize)
}

// Sealer seals and opens values for one purpose. It is safe for concurrent use.
type Sealer struct {
	aead cipher.AEAD
}

// New is the Sealer for purpose under the master key.
func New(master []byte, purpose Purpose) (*Sealer, error) {
	key, err := DeriveKey(master, purpose)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

// Seal encrypts plaintext bound to aad. The result is a random 12-byte nonce
// followed by the ciphertext and its tag.
func (s *Sealer) Seal(plaintext, aad []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize(), s.aead.NonceSize()+len(plaintext)+s.aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, plaintext, aad), nil
}

// Open decrypts what Seal produced for the same aad, or answers ErrUnreadable.
func (s *Sealer) Open(sealed, aad []byte) ([]byte, error) {
	size := s.aead.NonceSize()
	if len(sealed) < size+s.aead.Overhead() {
		return nil, ErrUnreadable
	}
	plaintext, err := s.aead.Open(nil, sealed[:size], sealed[size:], aad)
	if err != nil {
		return nil, ErrUnreadable
	}
	return plaintext, nil
}

package domain

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrJoinCodeKeyUnavailable is a sealed code whose key id names neither the
// current key nor the previous one.
var ErrJoinCodeKeyUnavailable = errors.New("join code key unavailable")

// JoinCodeKeySize is the length of a raw join-code key in bytes.
const JoinCodeKeySize = 32

// SealedSize is the stored length of a sealed code: a 12-byte nonce, the
// 8-byte code and a 16-byte tag.
const SealedSize = nonceSize + Length + tagSize

const (
	nonceSize = 12
	tagSize   = 16
)

// LookupScheme is how a stored code row is found.
type LookupScheme int16

const (
	// LookupLegacy is the SHA-256 of the code, as v0.7.0 stored it.
	LookupLegacy LookupScheme = 1
	// LookupKeyed is the HMAC-SHA256 of the code under a join-code key.
	LookupKeyed LookupScheme = 2
)

// StoredLookup is what a code row holds to be found by: its scheme, the id of
// the key a keyed row was hashed under, and the hash.
type StoredLookup struct {
	Scheme LookupScheme
	KeyID  *int16
	Hash   []byte
}

// JoinCodeKeys holds the current join-code key and, while a rotation is under
// way, the previous one. Each raw key yields, through HKDF-SHA256 with no salt,
// an AES-256-GCM key, an HMAC-SHA256 lookup key and a non-zero 16-bit id. The
// zero value holds no key: Seal refuses and a lookup matches only legacy rows.
type JoinCodeKeys struct {
	current  *joinCodeKey
	previous *joinCodeKey
}

type joinCodeKey struct {
	id     int16
	aead   cipher.AEAD
	lookup []byte
}

// NewJoinCodeKeys derives the keys from the current raw key and an optional
// previous one, each exactly JoinCodeKeySize bytes. It refuses a key whose id
// derives to zero, and a previous key whose id equals the current key's, since
// a stored key id could then not tell the two apart.
func NewJoinCodeKeys(current, previous []byte) (JoinCodeKeys, error) {
	cur, err := deriveJoinCodeKey(current)
	if err != nil {
		return JoinCodeKeys{}, fmt.Errorf("current join code key: %w", err)
	}
	keys := JoinCodeKeys{current: cur}
	if previous == nil {
		return keys, nil
	}
	prev, err := deriveJoinCodeKey(previous)
	if err != nil {
		return JoinCodeKeys{}, fmt.Errorf("previous join code key: %w", err)
	}
	if prev.id == cur.id {
		return JoinCodeKeys{}, fmt.Errorf("the current and previous join code keys derive the same key id %d; generate another current key", cur.id)
	}
	keys.previous = prev
	return keys, nil
}

func deriveJoinCodeKey(raw []byte) (*joinCodeKey, error) {
	if len(raw) != JoinCodeKeySize {
		return nil, fmt.Errorf("must be %d bytes, got %d", JoinCodeKeySize, len(raw))
	}
	aeadKey, err := hkdf.Key(sha256.New, raw, nil, "quizzivy join-code aead v1", 32)
	if err != nil {
		return nil, err
	}
	lookup, err := hkdf.Key(sha256.New, raw, nil, "quizzivy join-code lookup v1", 32)
	if err != nil {
		return nil, err
	}
	idBytes, err := hkdf.Key(sha256.New, raw, nil, "quizzivy join-code key-id v1", 2)
	if err != nil {
		return nil, err
	}
	id := int16(binary.BigEndian.Uint16(idBytes))
	if id == 0 {
		return nil, errors.New("derives key id 0, which marks no key; generate another")
	}
	block, err := aes.NewCipher(aeadKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &joinCodeKey{id: id, aead: aead, lookup: lookup}, nil
}

// CurrentID is the id of the key new codes are sealed and hashed under.
func (k JoinCodeKeys) CurrentID() int16 {
	if k.current == nil {
		return 0
	}
	return k.current.id
}

// Seal encrypts a canonical code under the current key with a random nonce,
// bound to its class and code ids as additional data, and returns
// nonce‖ciphertext‖tag, SealedSize bytes.
func (k JoinCodeKeys) Seal(classID, codeID, code string) ([]byte, error) {
	if k.current == nil {
		return nil, errors.New("join code: no key to seal with")
	}
	if len(code) != Length {
		return nil, fmt.Errorf("join code: seal needs a canonical %d-character code", Length)
	}
	aad, err := sealedAAD(classID, codeID)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceSize, SealedSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("join code: nonce: %w", err)
	}
	return k.current.aead.Seal(nonce, nonce, []byte(code), aad), nil
}

// Open decrypts a sealed code with the key keyID names. A key id that is
// neither the current nor the previous key's answers
// ErrJoinCodeKeyUnavailable; a ciphertext that fails authentication, including
// one moved to another class or code id, answers an error.
func (k JoinCodeKeys) Open(classID, codeID string, keyID int16, sealed []byte) (string, error) {
	key := k.byID(keyID)
	if key == nil {
		return "", ErrJoinCodeKeyUnavailable
	}
	if len(sealed) != SealedSize {
		return "", fmt.Errorf("join code: sealed code is %d bytes, want %d", len(sealed), SealedSize)
	}
	aad, err := sealedAAD(classID, codeID)
	if err != nil {
		return "", err
	}
	code, err := key.aead.Open(nil, sealed[:nonceSize], sealed[nonceSize:], aad)
	if err != nil {
		return "", errors.New("join code: sealed code does not authenticate")
	}
	return string(code), nil
}

// Hash is the keyed lookup hash of a canonical code under the current key,
// the value a new code row stores in code_hash.
func (k JoinCodeKeys) Hash(code string) []byte {
	if k.current == nil {
		return nil
	}
	return k.current.hash(code)
}

// LookupHashes is every hash a canonical code may be stored under: the keyed
// hash under the current key, under the previous key when one is set, and the
// legacy SHA-256.
func (k JoinCodeKeys) LookupHashes(code string) JoinCodeLookup {
	var l JoinCodeLookup
	for _, key := range []*joinCodeKey{k.current, k.previous} {
		if key != nil {
			l.candidates = append(l.candidates, lookupCandidate{scheme: LookupKeyed, keyID: key.id, hash: key.hash(code)})
		}
	}
	l.candidates = append(l.candidates, lookupCandidate{scheme: LookupLegacy, hash: JoinCodes.Hash(code)})
	return l
}

func (k JoinCodeKeys) byID(id int16) *joinCodeKey {
	for _, key := range []*joinCodeKey{k.current, k.previous} {
		if key != nil && key.id == id {
			return key
		}
	}
	return nil
}

func (key *joinCodeKey) hash(code string) []byte {
	mac := hmac.New(sha256.New, key.lookup)
	mac.Write([]byte(code))
	return mac.Sum(nil)
}

func sealedAAD(classID, codeID string) ([]byte, error) {
	class, err := uuid.Parse(classID)
	if err != nil {
		return nil, fmt.Errorf("join code: class id: %w", err)
	}
	code, err := uuid.Parse(codeID)
	if err != nil {
		return nil, fmt.Errorf("join code: code id: %w", err)
	}
	return append(class[:], code[:]...), nil
}

// JoinCodeLookup is what one typed code may be stored under, each hash tied to
// its scheme and, for a keyed hash, its key id.
type JoinCodeLookup struct {
	candidates []lookupCandidate
}

type lookupCandidate struct {
	scheme LookupScheme
	keyID  int16
	hash   []byte
}

// Hashes lists the candidate hashes for the database to search.
func (l JoinCodeLookup) Hashes() [][]byte {
	out := make([][]byte, len(l.candidates))
	for i, c := range l.candidates {
		out[i] = c.hash
	}
	return out
}

// Matches reports, in constant time per comparison, whether a stored row is
// this code: a legacy row only against the SHA-256 candidate, a keyed row only
// against the candidate under its own key id.
func (l JoinCodeLookup) Matches(s StoredLookup) bool {
	for _, c := range l.candidates {
		if c.scheme != s.Scheme {
			continue
		}
		if c.scheme == LookupKeyed && (s.KeyID == nil || *s.KeyID != c.keyID) {
			continue
		}
		return JoinCodes.Equal(c.hash, s.Hash)
	}
	return false
}

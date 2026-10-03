package domain_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/google/uuid"

	"quizzivy/internal/modules/classes/domain"
)

var (
	pinnedKey = []byte("0123456789abcdef0123456789abcdef")
	otherKey  = bytes.Repeat([]byte{0xb2}, domain.JoinCodeKeySize)
)

const (
	aClass = "0193a000-0000-7000-8000-000000000001"
	aCode  = "0193a000-0000-7000-8000-000000000002"
	code   = "K7M3P9QR"
)

func keysOf(t *testing.T, current, previous []byte) domain.JoinCodeKeys {
	t.Helper()
	keys, err := domain.NewJoinCodeKeys(current, previous)
	if err != nil {
		t.Fatalf("keys: %v", err)
	}
	return keys
}

func decoded(t *testing.T, s string) []byte {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	raw, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestASealedCodeOpensToItselfAndIsThirtySixBytes(t *testing.T) {
	keys := keysOf(t, pinnedKey, nil)
	sealed, err := keys.Seal(aClass, aCode, code)
	if err != nil {
		t.Fatal(err)
	}
	if len(sealed) != domain.SealedSize || domain.SealedSize != 36 {
		t.Fatalf("sealed is %d bytes, SealedSize %d, want 36", len(sealed), domain.SealedSize)
	}
	if bytes.Contains(sealed, []byte(code)) {
		t.Fatal("the sealed code holds the plaintext")
	}
	opened, err := keys.Open(aClass, aCode, keys.CurrentID(), sealed)
	if err != nil || opened != code {
		t.Fatalf("opened %q (%v), want %q", opened, err, code)
	}
	again, err := keys.Seal(aClass, aCode, code)
	if err != nil || bytes.Equal(again, sealed) {
		t.Fatalf("two seals of one code are identical (%v): the nonce is not fresh", err)
	}
}

func TestASealedCodeRefusesAnotherClassOrCodeId(t *testing.T) {
	keys := keysOf(t, pinnedKey, nil)
	sealed, err := keys.Seal(aClass, aCode, code)
	if err != nil {
		t.Fatal(err)
	}
	another := uuid.NewString()
	for label, ids := range map[string][2]string{
		"another class":      {another, aCode},
		"another code":       {aClass, another},
		"the ids swapped":    {aCode, aClass},
		"another class only": {another, another},
	} {
		if opened, err := keys.Open(ids[0], ids[1], keys.CurrentID(), sealed); err == nil {
			t.Errorf("%s opened the code as %q", label, opened)
		}
	}
}

func TestEveryFlippedByteOfASealedCodeIsRefused(t *testing.T) {
	keys := keysOf(t, pinnedKey, nil)
	sealed, err := keys.Seal(aClass, aCode, code)
	if err != nil {
		t.Fatal(err)
	}
	for i := range sealed {
		flipped := bytes.Clone(sealed)
		flipped[i] ^= 0x01
		if opened, err := keys.Open(aClass, aCode, keys.CurrentID(), flipped); err == nil {
			t.Errorf("byte %d flipped still opened as %q", i, opened)
		}
	}
	if _, err := keys.Open(aClass, aCode, keys.CurrentID(), sealed[:35]); err == nil {
		t.Error("a truncated sealed code opened")
	}
}

func TestThePreviousKeyOpensWhatItSealed(t *testing.T) {
	before := keysOf(t, pinnedKey, nil)
	sealed, err := before.Seal(aClass, aCode, code)
	if err != nil {
		t.Fatal(err)
	}
	rotating := keysOf(t, otherKey, pinnedKey)
	if rotating.CurrentID() == before.CurrentID() {
		t.Fatal("the test keys share an id")
	}
	if opened, err := rotating.Open(aClass, aCode, before.CurrentID(), sealed); err != nil || opened != code {
		t.Fatalf("the previous key opened %q (%v), want %q", opened, err, code)
	}
	after := keysOf(t, otherKey, nil)
	if _, err := after.Open(aClass, aCode, before.CurrentID(), sealed); !errors.Is(err, domain.ErrJoinCodeKeyUnavailable) {
		t.Fatalf("with the old key gone: %v, want ErrJoinCodeKeyUnavailable", err)
	}
	if _, err := rotating.Open(aClass, aCode, rotating.CurrentID(), sealed); err == nil {
		t.Fatal("the current key opened a code the previous key sealed")
	}
}

func TestTheDerivationIsPinned(t *testing.T) {
	keys := keysOf(t, pinnedKey, nil)
	if keys.CurrentID() != -468 {
		t.Errorf("key id %d, want -468", keys.CurrentID())
	}
	if got := hex.EncodeToString(keys.Hash(code)); got != "08ab2a368029866b21e8527cfee2aef14e5c96afb543720c96bf85872a589ec9" {
		t.Errorf("keyed hash %s", got)
	}

	block, err := aes.NewCipher(unhex(t, "6f298e74041c6418a70305e1122fafca43ced4936f5e5a801f91aa82fbbbfbf8"))
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	class, codeUUID := uuid.MustParse(aClass), uuid.MustParse(aCode)
	nonce := bytes.Repeat([]byte{0x5a}, 12)
	byHand := gcm.Seal(bytes.Clone(nonce), nonce, []byte(code), append(class[:], codeUUID[:]...))
	if opened, err := keys.Open(aClass, aCode, keys.CurrentID(), byHand); err != nil || opened != code {
		t.Fatalf("a code sealed by hand with the pinned AES key opened as %q (%v)", opened, err)
	}

	legacy := sha256.Sum256([]byte(code))
	hashes := keysOf(t, otherKey, pinnedKey).LookupHashes(code).Hashes()
	want := [][]byte{keysOf(t, otherKey, nil).Hash(code), keys.Hash(code), legacy[:]}
	if len(hashes) != len(want) {
		t.Fatalf("%d candidate hashes, want %d", len(hashes), len(want))
	}
	for i := range want {
		if !bytes.Equal(hashes[i], want[i]) {
			t.Errorf("candidate %d is %x, want %x", i, hashes[i], want[i])
		}
	}
}

func TestALookupMatchesARowOnlyUnderItsOwnSchemeAndKey(t *testing.T) {
	keys := keysOf(t, otherKey, pinnedKey)
	current, previous := keys.CurrentID(), keysOf(t, pinnedKey, nil).CurrentID()
	unknown := current + previous
	keyed := keys.Hash(code)
	legacy := sha256.Sum256([]byte(code))
	underPrevious := keysOf(t, pinnedKey, nil).Hash(code)
	lookup := keys.LookupHashes(code)
	for label, c := range map[string]struct {
		row  domain.StoredLookup
		want bool
	}{
		"a keyed row under the current key":        {domain.StoredLookup{Scheme: domain.LookupKeyed, KeyID: &current, Hash: keyed}, true},
		"a keyed row under the previous key":       {domain.StoredLookup{Scheme: domain.LookupKeyed, KeyID: &previous, Hash: underPrevious}, true},
		"a legacy row":                             {domain.StoredLookup{Scheme: domain.LookupLegacy, Hash: legacy[:]}, true},
		"a keyed row holding the legacy hash":      {domain.StoredLookup{Scheme: domain.LookupKeyed, KeyID: &current, Hash: legacy[:]}, false},
		"a legacy row holding the keyed hash":      {domain.StoredLookup{Scheme: domain.LookupLegacy, Hash: keyed}, false},
		"a current-key row holding the old hash":   {domain.StoredLookup{Scheme: domain.LookupKeyed, KeyID: &current, Hash: underPrevious}, false},
		"a keyed row under an unknown key":         {domain.StoredLookup{Scheme: domain.LookupKeyed, KeyID: &unknown, Hash: keyed}, false},
		"a keyed row with no key id":               {domain.StoredLookup{Scheme: domain.LookupKeyed, Hash: keyed}, false},
		"a keyed row for another code":             {domain.StoredLookup{Scheme: domain.LookupKeyed, KeyID: &current, Hash: keys.Hash("ABCDEFGH")}, false},
		"a row of a scheme this binary never made": {domain.StoredLookup{Scheme: 3, KeyID: &current, Hash: keyed}, false},
	} {
		if got := lookup.Matches(c.row); got != c.want {
			t.Errorf("%s: matched %v, want %v", label, got, c.want)
		}
	}
}

func TestKeysThatCannotBeToldApartAreRefused(t *testing.T) {
	for label, pair := range map[string][2][]byte{
		"a short current key":             {pinnedKey[:31], nil},
		"a long previous key":             {pinnedKey, append(bytes.Clone(otherKey), 0)},
		"a key whose id derives to zero":  {decoded(t, "hKTQlPb89dUPW90aMQEsebUsqWVq6L6s/601MYM5iso="), nil},
		"a previous key deriving to zero": {pinnedKey, decoded(t, "hKTQlPb89dUPW90aMQEsebUsqWVq6L6s/601MYM5iso=")},
		"two keys sharing an id":          {decoded(t, "ovQ973Cblj2hHmbuyvEFMnfxX4HnylP2y3/5nUxjrMQ="), decoded(t, "rsRptkDMi+CBc4IkhT3v0dkbR2WkWDKwB7EGv8j3+Sk=")},
		"the same key twice":              {pinnedKey, pinnedKey},
	} {
		if _, err := domain.NewJoinCodeKeys(pair[0], pair[1]); err == nil {
			t.Errorf("%s was accepted", label)
		}
	}
	if _, err := domain.NewJoinCodeKeys(decoded(t, "ovQ973Cblj2hHmbuyvEFMnfxX4HnylP2y3/5nUxjrMQ="), nil); err != nil {
		t.Errorf("one key of the colliding pair alone was refused: %v", err)
	}
}

func TestAKeylessValueSealsNothingAndFindsOnlyLegacyRows(t *testing.T) {
	var keys domain.JoinCodeKeys
	if _, err := keys.Seal(aClass, aCode, code); err == nil {
		t.Error("a keyless value sealed a code")
	}
	if keys.CurrentID() != 0 || keys.Hash(code) != nil {
		t.Error("a keyless value has a key")
	}
	legacy := sha256.Sum256([]byte(code))
	if hashes := keys.LookupHashes(code).Hashes(); len(hashes) != 1 || !bytes.Equal(hashes[0], legacy[:]) {
		t.Errorf("a keyless lookup searches %x, want only the legacy hash", hashes)
	}
	if _, err := keysOf(t, pinnedKey, nil).Seal(aClass, aCode, "K7M3-P9QR"); err == nil {
		t.Error("a grouped, non-canonical code was sealed")
	}
}

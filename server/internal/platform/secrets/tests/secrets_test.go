package secrets_test

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"quizzivy/internal/platform/secrets"
)

func master() []byte {
	key := make([]byte, secrets.KeySize)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestDerivedKeysMatchVectorsComputedOutsideGo(t *testing.T) {
	for purpose, want := range map[secrets.Purpose]string{
		secrets.PurposeEmailOutbox:     "b6aadab748c275a198488e18e9393e3689d7bdfc97604e1687908cbc18e77210",
		secrets.PurposeUserImportSheet: "1b48d43e62165ade437418d2b279a5b6f9124a38f873912222e46169a3579545",
	} {
		got, err := secrets.DeriveKey(master(), purpose)
		if err != nil {
			t.Fatal(err)
		}
		if hex.EncodeToString(got) != want {
			t.Errorf("%s derives %x, want %s (openssl kdf HKDF, SHA256, info quizzivy/secrets/v1/%s)", purpose, got, want, purpose)
		}
	}
}

func TestAValueSealedOutsideGoOpens(t *testing.T) {
	sealer, err := secrets.New(master(), secrets.PurposeEmailOutbox)
	if err != nil {
		t.Fatal(err)
	}
	sealed := unhex(t, "000102030405060708090a0bd9803a8a47f5e8e0f349e8fc3b74c7ec4582d8ced271a2e6bcc758e44926c70771f8")
	got, err := sealer.Open(sealed, []byte("0192a7c0-0000-7000-8000-000000000001"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "set-password-token" {
		t.Fatalf("opened %q", got)
	}
}

func TestASealedValueOpensOnlyUnderItsPurposeKeyAndAssociatedData(t *testing.T) {
	outbox, err := secrets.New(master(), secrets.PurposeEmailOutbox)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := outbox.Seal([]byte("token"), []byte("row-1"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("token")) {
		t.Fatal("the sealed value carries the plaintext")
	}
	if got, err := outbox.Open(sealed, []byte("row-1")); err != nil || string(got) != "token" {
		t.Fatalf("round trip: %q %v", got, err)
	}
	again, _ := outbox.Seal([]byte("token"), []byte("row-1"))
	if bytes.Equal(again, sealed) {
		t.Fatal("two seals of one value are equal; the nonce is not random")
	}

	sheet, _ := secrets.New(master(), secrets.PurposeUserImportSheet)
	other := master()
	other[0] ^= 1
	otherKey, _ := secrets.New(other, secrets.PurposeEmailOutbox)
	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 1
	for name, open := range map[string]func() ([]byte, error){
		"another row":     func() ([]byte, error) { return outbox.Open(sealed, []byte("row-2")) },
		"another purpose": func() ([]byte, error) { return sheet.Open(sealed, []byte("row-1")) },
		"another key":     func() ([]byte, error) { return otherKey.Open(sealed, []byte("row-1")) },
		"changed bytes":   func() ([]byte, error) { return outbox.Open(tampered, []byte("row-1")) },
		"too short":       func() ([]byte, error) { return outbox.Open(sealed[:10], []byte("row-1")) },
	} {
		if _, err := open(); !errors.Is(err, secrets.ErrUnreadable) {
			t.Errorf("%s: %v, want ErrUnreadable", name, err)
		}
	}
}

func TestParseKeyTakesStandardBase64OfThirtyTwoBytesAndNeverRepeatsTheValue(t *testing.T) {
	good := base64.StdEncoding.EncodeToString(master())
	if got, err := secrets.ParseKey(" " + good + "\n"); err != nil || !bytes.Equal(got, master()) {
		t.Fatalf("a good key: %v", err)
	}
	for name, value := range map[string]string{
		"not base64":   "not-a-key!!",
		"url encoding": base64.URLEncoding.EncodeToString(bytes.Repeat([]byte{0xfb}, secrets.KeySize)),
		"16 bytes":     base64.StdEncoding.EncodeToString(master()[:16]),
		"48 bytes":     base64.StdEncoding.EncodeToString(append(master(), master()[:16]...)),
		"empty":        "",
	} {
		_, err := secrets.ParseKey(value)
		if err == nil {
			t.Errorf("%s was accepted", name)
			continue
		}
		if value != "" && strings.Contains(err.Error(), value) {
			t.Errorf("%s: the error repeats the value: %v", name, err)
		}
	}
}

func TestDeriveKeyRefusesAShortMasterAndAnEmptyPurpose(t *testing.T) {
	if _, err := secrets.DeriveKey(master()[:16], secrets.PurposeEmailOutbox); err == nil {
		t.Error("a 16-byte master was accepted")
	}
	if _, err := secrets.DeriveKey(master(), ""); err == nil {
		t.Error("an empty purpose was accepted")
	}
}

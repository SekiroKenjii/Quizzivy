package ratelimit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func post(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
}

func TestExtractsTheField(t *testing.T) {
	key := JSONFieldKey("email", 4096)
	if got := key(post(`{"email":"a@b.co","password":"x"}`)); got != "a@b.co" {
		t.Errorf("got %q", got)
	}
}

func TestTheHandlerStillSeesTheWholeBody(t *testing.T) {
	body := `{"email":"a@b.co","password":"hunter22"}`
	r := post(body)
	JSONFieldKey("email", 4096)(r)

	got, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Errorf("handler would read %q, want %q", got, body)
	}
}

func TestCaseAndWhitespaceShareABucket(t *testing.T) {
	key := JSONFieldKey("email", 4096)
	a := key(post(`{"email":"  A@B.CO "}`))
	b := key(post(`{"email":"a@b.co"}`))
	if a != b {
		t.Errorf("%q and %q landed in different buckets", a, b)
	}
}

func TestOversizedBodyYieldsNoKeyButIsStillReadable(t *testing.T) {
	big := `{"email":"a@b.co","pad":"` + strings.Repeat("x", 5000) + `"}`
	r := post(big)
	if got := JSONFieldKey("email", 1024)(r); got != "" {
		t.Errorf("oversized body produced key %q; the per-IP limit should carry it instead", got)
	}
	got, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != big {
		t.Errorf("handler lost part of an oversized body: got %d bytes, want %d", len(got), len(big))
	}
}

func TestMalformedOrMissingYieldsNoKey(t *testing.T) {
	key := JSONFieldKey("email", 4096)
	for name, body := range map[string]string{
		"not json":     `not json at all`,
		"missing":      `{"password":"x"}`,
		"wrong type":   `{"email":123}`,
		"empty body":   ``,
		"null value":   `{"email":null}`,
		"nested array": `[{"email":"a@b.co"}]`,
	} {
		t.Run(name, func(t *testing.T) {
			if got := key(post(body)); got != "" {
				t.Errorf("got %q, want no key", got)
			}
		})
	}
}

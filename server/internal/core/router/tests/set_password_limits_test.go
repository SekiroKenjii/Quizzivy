package router_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const strongEnough = "correct-horse-9"

func checkLink(h http.Handler, address, token string) int {
	return fromAddress(h, "/auth/set-password/check", address, fmt.Sprintf(`{"token":%q}`, token))
}

func setWithLink(h http.Handler, address, token string) int {
	return fromAddress(h, "/auth/set-password", address, fmt.Sprintf(`{"token":%q,"newPassword":%q}`, token, strongEnough))
}

func fromAddress(h http.Handler, path, address, body string) int {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = address + ":40000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestAClassOpeningItsLinksTogetherBehindOneAddressIsNeverLimited(t *testing.T) {
	h := newTestRouter(t, fakeDB{})
	for i := range 40 {
		token := fmt.Sprintf("class-link-%02d", i)
		if got := checkLink(h, schoolAddress, token); got == http.StatusTooManyRequests {
			t.Fatalf("check %d of a class of 40 was limited", i+1)
		}
		if got := setWithLink(h, schoolAddress, token); got == http.StatusTooManyRequests {
			t.Fatalf("set %d of a class of 40 was limited", i+1)
		}
	}
}

func TestTheCheckIsLimitedPerAddressAndPerToken(t *testing.T) {
	h := newTestRouter(t, fakeDB{})
	for i := range 120 {
		if got := checkLink(h, "198.51.100.101", fmt.Sprintf("token-%03d", i)); got == http.StatusTooManyRequests {
			t.Fatalf("check %d from one address was limited within its 120 a minute", i+1)
		}
	}
	if got := checkLink(h, "198.51.100.101", "token-another"); got != http.StatusTooManyRequests {
		t.Errorf("the 121st check in a minute from one address: %d, want 429", got)
	}

	for range 20 {
		if got := checkLink(h, "198.51.100.102", "one-token"); got == http.StatusTooManyRequests {
			t.Fatal("a check of one token was limited within its 20 an hour")
		}
	}
	if got := checkLink(h, "198.51.100.102", "one-token"); got != http.StatusTooManyRequests {
		t.Errorf("the 21st check of one token: %d, want 429", got)
	}
	if got := checkLink(h, "198.51.100.103", "one-token"); got != http.StatusTooManyRequests {
		t.Errorf("the 22nd check of the same token from another address: %d, want 429: the token's bucket is not the address's", got)
	}
	if got := checkLink(h, "198.51.100.102", "a-different-token"); got == http.StatusTooManyRequests {
		t.Error("another token from the same address was limited by the first token's bucket")
	}
}

func TestSettingAPasswordIsLimitedPerAddressAndPerToken(t *testing.T) {
	h := newTestRouter(t, fakeDB{})
	for i := range 60 {
		if got := setWithLink(h, "198.51.100.111", fmt.Sprintf("token-%03d", i)); got == http.StatusTooManyRequests {
			t.Fatalf("set %d from one address was limited within its 60 a minute", i+1)
		}
	}
	if got := setWithLink(h, "198.51.100.111", "token-another"); got != http.StatusTooManyRequests {
		t.Errorf("the 61st set in a minute from one address: %d, want 429", got)
	}

	for range 10 {
		if got := setWithLink(h, "198.51.100.112", "one-token"); got == http.StatusTooManyRequests {
			t.Fatal("a set of one token was limited within its 10 an hour")
		}
	}
	if got := setWithLink(h, "198.51.100.113", "one-token"); got != http.StatusTooManyRequests {
		t.Errorf("the 11th set of one token, from another address: %d, want 429", got)
	}
}

func TestAWeakPasswordStillSpendsTheTokensBudget(t *testing.T) {
	h := newTestRouter(t, fakeDB{})
	weak := `{"token":"guessed-token","newPassword":"short"}`
	for i := range 10 {
		if got := fromAddress(h, "/auth/set-password", "198.51.100.121", weak); got != http.StatusBadRequest {
			t.Fatalf("attempt %d with a weak password: %d, want 400", i+1, got)
		}
	}
	if got := fromAddress(h, "/auth/set-password", "198.51.100.121", weak); got != http.StatusTooManyRequests {
		t.Errorf("the 11th attempt with one token: %d, want 429 before validation", got)
	}
}

func TestAGoogleOnlyAccountMayAskForThreeLinksAMinute(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	for i := range 3 {
		if rec := sendFrom(t, h, issuer, http.MethodPost, "/auth/set-password/request", teacherUser, centreAddress, ""); rec.Code != http.StatusNotImplemented {
			t.Fatalf("request %d: %d, want it to reach the handler within the budget of three", i+1, rec.Code)
		}
	}
	if rec := sendFrom(t, h, issuer, http.MethodPost, "/auth/set-password/request", teacherUser, centreAddress, ""); rec.Code != http.StatusTooManyRequests {
		t.Errorf("the 4th request in a minute: %d, want 429", rec.Code)
	}
	if rec := sendFrom(t, h, issuer, http.MethodPost, "/auth/set-password/request", adminUser, centreAddress, ""); rec.Code != http.StatusNotImplemented {
		t.Errorf("a colleague behind the same address: %d, want the handler: budgets are per user", rec.Code)
	}
	if rec := sendFrom(t, h, issuer, http.MethodPost, "/auth/set-password/request", "", centreAddress, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("an anonymous request: %d, want 401", rec.Code)
	}
}

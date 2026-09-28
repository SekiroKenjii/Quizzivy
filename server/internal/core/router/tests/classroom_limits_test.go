package router_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const schoolAddress = "203.0.113.40"

func fromTheSchool(h http.Handler, path, body string) int {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = schoolAddress + ":40000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestAClassOfFortyBehindOneAddressIsNeverLimited(t *testing.T) {
	h := newTestRouter(t, fakeDB{})
	const code = `{"joinCode":"K7M3-P9QR"}`
	for i := range 40 {
		if got := fromTheSchool(h, "/auth/login", fmt.Sprintf(`{"email":"hocvien%02d@example.com","password":"quizzivy-dev"}`, i)); got == http.StatusTooManyRequests {
			t.Fatalf("login %d of a class of 40 was limited", i+1)
		}
	}
	for i := range 120 {
		if got := fromTheSchool(h, "/join/preview", code); got == http.StatusTooManyRequests {
			t.Fatalf("preview %d of 120, three for each of 40 students, was limited", i+1)
		}
	}
	for i := range 40 {
		if got := fromTheSchool(h, "/app/classes/join", code); got == http.StatusTooManyRequests {
			t.Fatalf("join %d of a class of 40 was limited", i+1)
		}
	}
}

func TestOneDeviceHammeringOneAccountIsStillLimited(t *testing.T) {
	h := newTestRouter(t, fakeDB{})
	body := `{"email":"an@example.com","password":"wrong"}`
	for i := range 10 {
		if got := fromTheSchool(h, "/auth/login", body); got == http.StatusTooManyRequests {
			t.Fatalf("login %d for one account was limited within its ten a minute", i+1)
		}
	}
	if got := fromTheSchool(h, "/auth/login", body); got != http.StatusTooManyRequests {
		t.Errorf("the 11th login for one account from one address: %d, want 429", got)
	}
	if got := fromTheSchool(h, "/auth/login", `{"email":"binh@example.com","password":"wrong"}`); got == http.StatusTooManyRequests {
		t.Error("a classmate's first login from the same address was limited")
	}
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.77:40000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Error("the same account from another address was limited by the first address's bucket")
	}
}

func TestNoBodyShapeSlipsPastTheAccountBucket(t *testing.T) {
	h := newTestRouter(t, fakeDB{})
	trailing := `{"email":"victim@example.com","password":"guess"}x`
	for i := range 10 {
		if got := fromTheSchool(h, "/auth/login", trailing); got == http.StatusTooManyRequests {
			t.Fatalf("login %d with a trailing byte was limited early", i+1)
		}
	}
	if got := fromTheSchool(h, "/auth/login", trailing); got != http.StatusTooManyRequests {
		t.Errorf("the 11th login with a trailing byte for one account: %d, want 429", got)
	}
	padded := `{"email":"other@example.com",` + strings.Repeat(" ", 8200) + `"password":"guess"}`
	if got := fromTheSchool(h, "/auth/login", padded); got != http.StatusRequestEntityTooLarge {
		t.Errorf("a login body padded past 8 KiB: %d, want 413 before any handler", got)
	}
	for _, path := range []string{"/join/preview", "/auth/google"} {
		if got := fromTheSchool(h, path, `{"joinCode":"K7M3-P9QR",`+strings.Repeat(" ", 8200)+`"x":1}`); got != http.StatusRequestEntityTooLarge {
			t.Errorf("%s with a body past 8 KiB: %d, want 413", path, got)
		}
	}
	issuer := testIssuer(t)
	rec := sendAs(t, roleRouter(t, issuer, rolePrincipals()), issuer, http.MethodPost, "/app/classes/join", studentUser, `{"joinCode":"K7M3-P9QR",`+strings.Repeat(" ", 8200)+`"x":1}`)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("a student's join with a body past 8 KiB: %d, want 413", rec.Code)
	}
}

func TestAnOverlongEmailFillsNoBucket(t *testing.T) {
	h := newTestRouter(t, fakeDB{})
	body := `{"email":"` + strings.Repeat("a", 250) + `@b.co","password":"guess"}`
	for i := range 11 {
		if got := fromTheSchool(h, "/auth/login", body); got != http.StatusBadRequest {
			t.Fatalf("login %d with a 255-character email: %d, want the schema's 400 and no bucket charged", i+1, got)
		}
	}
}

package router_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestALoginHoldingANulIsRefusedAndStillSpendsItsBuckets(t *testing.T) {
	for name, body := range map[string]string{
		"in the email":    `{"email":"an\u0000@example.com","password":"wrong-password"}`,
		"in the password": `{"email":"an@example.com","password":"wrong\u0000password"}`,
	} {
		t.Run(name, func(t *testing.T) {
			h := newTestRouter(t, fakeDB{})
			for i := range 10 {
				if got := fromTheSchool(h, "/auth/login", body); got != http.StatusBadRequest {
					t.Fatalf("login %d: status = %d, want 400", i+1, got)
				}
			}
			if got := fromTheSchool(h, "/auth/login", body); got != http.StatusTooManyRequests {
				t.Errorf("the 11th login: status = %d, want 429: a refused body spends the budget as any other", got)
			}
		})
	}
}

func TestABeaconHoldingANulIsRefusedBeforeTheHandler(t *testing.T) {
	const target = "/app/attempts/019535d9-3df7-79fb-b466-fa907fa17f9e/events"
	const body = `{"beaconToken":"token","sessionId":"019535d9-3df7-79fb-b466-fa907fa17f9e","events":[{"kind":"pa@NUL@ste","occurredAt":"2026-10-11T10:00:00Z","clientSeq":1}]}`
	post := func(text string) int {
		req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(text))
		req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
		req.RemoteAddr = "203.0.113.9:40000"
		rec := httptest.NewRecorder()
		newTestRouter(t, fakeDB{}).ServeHTTP(rec, req)
		return rec.Code
	}

	if got := post(strings.ReplaceAll(body, "@NUL@", "x")); got == http.StatusBadRequest {
		t.Fatalf("the beacon without the NUL was refused with %d", got)
	}
	if got := post(strings.ReplaceAll(body, "@NUL@", `\u0000`)); got != http.StatusBadRequest {
		t.Errorf("the beacon holding a NUL: status = %d, want 400", got)
	}
}

func TestASignedInWriteHoldingANulIsRefusedAndAnUnsignedOneIsStillAskedToSignIn(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	const body = `{"name":"Lớp 10\u0000A"}`

	rec := sendAs(t, h, issuer, http.MethodPost, "/teacher/classes", teacherUser, body)
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "VALIDATION_FAILED" {
		t.Errorf("a teacher's class name holding a NUL: status = %d, want 400 VALIDATION_FAILED: %s", rec.Code, rec.Body.String())
	}
	if rec := sendAs(t, h, issuer, http.MethodPost, "/teacher/classes", "", body); rec.Code != http.StatusUnauthorized {
		t.Errorf("the same body without a token: status = %d, want 401 before the body is read", rec.Code)
	}
	if rec := sendAs(t, h, issuer, http.MethodGet, "/teacher/tests?q=a%00b", teacherUser, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("a search holding a NUL: status = %d, want 400", rec.Code)
	}
}

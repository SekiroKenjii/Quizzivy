package router_test

import (
	"net/http"
	"testing"
)

func TestDashboardRangeIsValidatedBeforeTheHandler(t *testing.T) {
	issuer := testIssuer(t)
	h := roleRouter(t, issuer, rolePrincipals())
	for _, value := range []string{"7d", "14d", "30d", "60d"} {
		rec := sendAs(t, h, issuer, http.MethodGet, "/teacher/dashboard?range="+value, teacherUser, "")
		if value == "60d" {
			if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "VALIDATION_FAILED" {
				t.Fatalf("invalid range status=%d body=%s", rec.Code, rec.Body)
			}
		} else if rec.Code != http.StatusNotImplemented {
			t.Fatalf("valid range %s status=%d body=%s", value, rec.Code, rec.Body)
		}
	}
	rec := sendAs(t, h, issuer, http.MethodGet, "/teacher/summary", teacherUser, "")
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("absent summary port status=%d body=%s", rec.Code, rec.Body)
	}
}

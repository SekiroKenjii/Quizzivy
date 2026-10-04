package router_test

import (
	"net/http"
	"testing"
)

const readAttemptPath = "/app/attempts/019535d9-3df7-79fb-b466-fa907fa17f9e"

func TestAMalformedSessionParameterIsNamed(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())
	student, err := issuer.Issue(studentUser, "student", 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name           string
		acceptLanguage string
		want           string
	}{
		{name: "Vietnamese by default", want: `Tham số "session" không hợp lệ.`},
		{name: "English preferred", acceptLanguage: "en", want: `The parameter "session" is not valid.`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantValidationFailure(t, sendIn(t, handler, http.MethodGet, readAttemptPath+"?session=bad", student, tc.acceptLanguage, ""), tc.want)
		})
	}
}

func TestAWellFormedSessionParameterReachesTheOperation(t *testing.T) {
	issuer := testIssuer(t)
	handler := roleRouter(t, issuer, rolePrincipals())

	rec := sendAs(t, handler, issuer, http.MethodGet, readAttemptPath+"?session=019535d9-3df7-79fb-b466-fa907fa17f9f", studentUser, "")

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501 from a router with no attempts module: %s", rec.Code, rec.Body.String())
	}
}

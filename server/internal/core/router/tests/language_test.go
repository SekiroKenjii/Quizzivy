package router_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTheRoutersOwnAnswersSpeakTheCallersLanguage(t *testing.T) {
	issuer := testIssuer(t)
	student, err := issuer.Issue(studentUser, "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	unbuilt := roleRouter(t, issuer, rolePrincipals())
	failing := logoutRouter(t, errors.New("revoke failed"))

	for _, c := range []struct {
		name   string
		send   func(t *testing.T, acceptLanguage string) *httptest.ResponseRecorder
		status int
		vi, en string
	}{
		{
			name: "an operation whose module the router does not have",
			send: func(t *testing.T, acceptLanguage string) *httptest.ResponseRecorder {
				return sendIn(t, unbuilt, http.MethodPatch, saveAnswersPath, student, acceptLanguage,
					saveAnswersKeyedBy(t, "019535d9-3df7-79fb-b466-fa907fa17fa0"))
			},
			status: http.StatusNotImplemented,
			vi:     "Chức năng này chưa được xây dựng.",
			en:     "This feature has not been built yet.",
		},
		{
			name: "a logout whose revoke fails",
			send: func(_ *testing.T, acceptLanguage string) *httptest.ResponseRecorder {
				header := map[string]string{"Cookie": "quizzivy_refresh=anything"}
				if acceptLanguage != "" {
					header["Accept-Language"] = acceptLanguage
				}
				return send(failing, http.MethodPost, "/auth/logout", header)
			},
			status: http.StatusInternalServerError,
			vi:     "Đã xảy ra lỗi. Vui lòng thử lại.",
			en:     "Something went wrong. Try again.",
		},
	} {
		for acceptLanguage, want := range map[string]string{"": c.vi, "en": c.en} {
			t.Run(c.name+"/Accept-Language="+acceptLanguage, func(t *testing.T) {
				rec := c.send(t, acceptLanguage)
				if rec.Code != c.status {
					t.Fatalf("status = %d, want %d: %s", rec.Code, c.status, rec.Body.String())
				}
				code, message := errorCodeAndMessage(t, rec)
				if code != "INTERNAL" || message != want {
					t.Errorf("answer = %s %q, want INTERNAL %q", code, message, want)
				}
			})
		}
	}
}

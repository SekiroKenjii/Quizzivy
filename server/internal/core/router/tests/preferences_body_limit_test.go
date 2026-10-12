package router_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type preferencesUnread struct{ reads int }

func (r *preferencesUnread) Read([]byte) (int, error) { r.reads++; return 0, nil }
func (*preferencesUnread) Close() error               { return nil }

func TestPreferenceRawCapRunsAfterAuthenticationAndBeforeValidation(t *testing.T) {
	issuer := testIssuer(t)
	handler := newAuthTestRouter(t, issuer)
	token, err := issuer.Issue("01935000-0000-7000-8000-000000000001", 0)
	if err != nil {
		t.Fatal(err)
	}
	unread := &preferencesUnread{}
	req := httptest.NewRequest("PATCH", "/me/preferences", unread)
	req.ContentLength = 100000
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 401 || unread.reads != 0 {
		t.Fatalf("auth status%d reads%d", rec.Code, unread.reads)
	}
	for _, size := range []int{8192, 8193} {
		for _, unknown := range []bool{false, true} {
			body := strings.Repeat(" ", size-2) + "{}"
			req := httptest.NewRequest("PATCH", "/me/preferences", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			if unknown {
				req.ContentLength = -1
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			want := http.StatusNotImplemented
			if size > 8192 {
				want = 400
			}
			if rec.Code != want {
				t.Fatalf("size%d unknown%v status%d body%s", size, unknown, rec.Code, rec.Body.String())
			}
		}
	}
	for _, body := range []string{`null`, `{"theme":null}`, `{"unknown":true}`, `{"assignmentDefaults":{"durationMinutes":601}}`} {
		req := httptest.NewRequest("PATCH", "/me/preferences", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 400 {
			t.Fatalf("invalid%s=%d", body, rec.Code)
		}
	}
}

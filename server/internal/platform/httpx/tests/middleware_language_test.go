package httpx_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/ratelimit"
)

const bodyLimit = 3 << 19

type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

func letThrough() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func matched(method, path string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.Pattern = method + " " + path
	return r
}

func spentBudget(t *testing.T) (http.Handler, *http.Request) {
	t.Helper()
	reg := ratelimit.NewRegistry()
	reg.Add("POST /auth/login", 100, ratelimit.PerMinute(1))
	h := httpx.RateLimit(reg, ratelimit.ClientIP(""))(letThrough())
	if got := login(h, "198.51.100.30", "an@example.com"); got != http.StatusNoContent {
		t.Fatalf("the first login: %d, want it through", got)
	}
	r := matched(http.MethodPost, "/auth/login")
	r.RemoteAddr = "198.51.100.30:1234"
	return h, r
}

func noToken(*testing.T) (http.Handler, *http.Request) {
	verify := func(string) (httpx.Principal, error) {
		return httpx.Principal{}, errors.New("no token was sent")
	}
	return httpx.RequireAuth(map[string]struct{}{"POST /open": {}}, verify)(letThrough()), matched(http.MethodGet, "/self")
}

func withToken(pattern, token string) func(*testing.T) (http.Handler, *http.Request) {
	return func(t *testing.T) (http.Handler, *http.Request) {
		t.Helper()
		method, path, _ := strings.Cut(pattern, " ")
		r := matched(method, path)
		r.Header.Set("Authorization", "Bearer "+token)
		return gate(t, nil), r
	}
}

func limitedBody(body func() *http.Request) func(*testing.T) (http.Handler, *http.Request) {
	return func(*testing.T) (http.Handler, *http.Request) {
		r := body()
		r.Pattern = "POST /auth/login"
		return httpx.LimitRequestBody(nil, bodyLimit, nil)(letThrough()), r
	}
}

func TestSharedMiddlewareSpeaksTheCallersLanguage(t *testing.T) {
	for _, c := range []struct {
		name    string
		refused func(*testing.T) (http.Handler, *http.Request)
		status  int
		code    string
		header  string
		vi, en  string
	}{
		{
			name:    "the limiter with a spent budget",
			refused: spentBudget,
			status:  http.StatusTooManyRequests,
			code:    "RATE_LIMITED",
			header:  "Retry-After",
			vi:      "Bạn thao tác quá nhanh. Vui lòng thử lại sau.",
			en:      "You are going too fast. Please try again later.",
		},
		{
			name:    "a gated route with no token",
			refused: noToken,
			status:  http.StatusUnauthorized,
			code:    "UNAUTHORIZED",
			header:  "WWW-Authenticate",
			vi:      "Phiên đăng nhập không hợp lệ. Vui lòng đăng nhập lại.",
			en:      "Your session is not valid. Please sign in again.",
		},
		{
			name:    "a principal without the permission",
			refused: withToken("GET /teacher", "student@0"),
			status:  http.StatusForbidden,
			code:    "FORBIDDEN",
			vi:      "Bạn không có quyền truy cập chức năng này.",
			en:      "You do not have permission to use this feature.",
		},
		{
			name:    "a resolver that fails",
			refused: withToken("GET /self", "broken@0"),
			status:  http.StatusInternalServerError,
			code:    "INTERNAL",
			vi:      "Đã xảy ra lỗi. Vui lòng thử lại.",
			en:      "Something went wrong. Try again.",
		},
		{
			name: "a Content-Length above the limit",
			refused: limitedBody(func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{}`))
				r.ContentLength = bodyLimit + 1
				return r
			}),
			status: http.StatusRequestEntityTooLarge,
			code:   "VALIDATION_FAILED",
			vi:     "Dữ liệu gửi lên vượt quá giới hạn 1.5 MiB.",
			en:     "The submitted data exceeds the 1.5 MiB limit.",
		},
		{
			name: "a body of undeclared length above the limit",
			refused: limitedBody(func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(strings.Repeat(" ", bodyLimit+1)))
				r.ContentLength = -1
				return r
			}),
			status: http.StatusRequestEntityTooLarge,
			code:   "VALIDATION_FAILED",
			vi:     "Dữ liệu gửi lên vượt quá giới hạn 1.5 MiB.",
			en:     "The submitted data exceeds the 1.5 MiB limit.",
		},
		{
			name: "a body that fails to read",
			refused: limitedBody(func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/auth/login", failingBody{})
			}),
			status: http.StatusBadRequest,
			code:   "VALIDATION_FAILED",
			vi:     "Không đọc được dữ liệu gửi lên.",
			en:     "The submitted data could not be read.",
		},
	} {
		for acceptLanguage, want := range map[string]string{"": c.vi, "en": c.en} {
			t.Run(c.name+"/Accept-Language="+acceptLanguage, func(t *testing.T) {
				h, r := c.refused(t)
				if acceptLanguage != "" {
					r.Header.Set("Accept-Language", acceptLanguage)
				}
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, r)

				if rec.Code != c.status {
					t.Fatalf("status = %d, want %d: %s", rec.Code, c.status, rec.Body.String())
				}
				if c.header != "" && rec.Header().Get(c.header) == "" {
					t.Errorf("the %d carries no %s", rec.Code, c.header)
				}
				var body struct {
					Error struct {
						Code    string `json:"code"`
						Message string `json:"message"`
					} `json:"error"`
				}
				if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
					t.Fatalf("response is not the error envelope: %v", err)
				}
				if body.Error.Code != c.code || body.Error.Message != want {
					t.Errorf("answer = %s %q, want %s %q", body.Error.Code, body.Error.Message, c.code, want)
				}
			})
		}
	}
}

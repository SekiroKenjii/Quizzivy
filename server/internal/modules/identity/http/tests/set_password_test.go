package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

const (
	linkExpired     = "Liên kết này đã hết hạn hoặc đã được dùng. Hãy nhờ trung tâm gửi liên kết mới."
	linkExpiredEN   = "This link has expired or has already been used. Ask your centre for a new one."
	secretToken     = "q3Wc0b0kZ9fT1uXH2nP7l8mV5sYd4RjA6eG-hLpUoNw"
	acceptedNewPass = "một-mật-khẩu-9"
)

type linkSpy struct {
	check     query.CheckSetPasswordLink
	set       command.SetPasswordWithLink
	requested command.RequestSetPasswordLink
}

type linkOutcome struct {
	link    domain.SetPasswordLink
	session model.Session
	err     error
}

func linkApp(spy *linkSpy, outcome linkOutcome) *application.Application {
	return &application.Application{
		Commands: application.Commands{
			SetPasswordWithLink: cqrs.HandlerFunc[command.SetPasswordWithLink, model.Session](func(_ context.Context, cmd command.SetPasswordWithLink) (model.Session, error) {
				spy.set = cmd
				return outcome.session, outcome.err
			}),
			RequestSetPasswordLink: cqrs.HandlerFunc[command.RequestSetPasswordLink, cqrs.Nothing](func(_ context.Context, cmd command.RequestSetPasswordLink) (cqrs.Nothing, error) {
				spy.requested = cmd
				return cqrs.Nothing{}, outcome.err
			}),
		},
		Queries: application.Queries{
			CheckSetPasswordLink: cqrs.HandlerFunc[query.CheckSetPasswordLink, domain.SetPasswordLink](func(_ context.Context, q query.CheckSetPasswordLink) (domain.SetPasswordLink, error) {
				spy.check = q
				return outcome.link, outcome.err
			}),
		},
	}
}

type served struct {
	status int
	header http.Header
	body   []byte
	err    error
}

func serve(t *testing.T, accept, userID string, call func(context.Context, http.ResponseWriter) error) served {
	t.Helper()
	var result served
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { result.err = call(r.Context(), w) })
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(inner)
	if userID != "" {
		handler = httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
			httpx.RequireAuth(nil, func(string) (httpx.Principal, error) { return httpx.Principal{UserID: userID}, nil })(inner))
	}
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.Header.Set("User-Agent", "link-http-test")
	if userID != "" {
		request.Header.Set("Authorization", "Bearer fixture")
	}
	if accept != "" {
		request.Header.Set("Accept-Language", accept)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	result.status, result.header, result.body = recorder.Code, recorder.Header(), recorder.Body.Bytes()
	return result
}

func withoutRequestID(t *testing.T, raw []byte) string {
	t.Helper()
	var envelope map[string]map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("the answer is not the error envelope: %v: %s", err, raw)
	}
	delete(envelope["error"], "requestId")
	out, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func checkLink(h identityhttp.Identity, token string) func(context.Context, http.ResponseWriter) error {
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.CheckSetPasswordLink(ctx, openapi.CheckSetPasswordLinkRequestObject{Body: &openapi.CheckSetPasswordLinkJSONRequestBody{Token: token}})
		return render(w, openapi.CheckSetPasswordLinkResponseObject.VisitCheckSetPasswordLinkResponse, response, err)
	}
}

func setWithLink(h identityhttp.Identity, token, password string) func(context.Context, http.ResponseWriter) error {
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.SetPasswordWithLink(ctx, openapi.SetPasswordWithLinkRequestObject{Body: &openapi.SetPasswordWithLinkJSONRequestBody{Token: token, NewPassword: password}})
		return render(w, openapi.SetPasswordWithLinkResponseObject.VisitSetPasswordWithLinkResponse, response, err)
	}
}

func requestLink(h identityhttp.Identity) func(context.Context, http.ResponseWriter) error {
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.RequestSetPasswordLink(ctx, openapi.RequestSetPasswordLinkRequestObject{})
		return render(w, openapi.RequestSetPasswordLinkResponseObject.VisitRequestSetPasswordLinkResponse, response, err)
	}
}

func TestEveryInvalidLinkIsTheSame410OnBothOperationsBodyAndAll(t *testing.T) {
	for _, language := range []struct{ accept, message string }{{"", linkExpired}, {"en", linkExpiredEN}} {
		h := identityhttp.NewIdentity(linkApp(&linkSpy{}, linkOutcome{err: domain.ErrSetPasswordLinkInvalid}), time.Hour, false, nil)
		answers := map[string]served{
			"check, a token":       serve(t, language.accept, "", checkLink(h, secretToken)),
			"check, another token": serve(t, language.accept, "", checkLink(h, "another-token")),
			"set, a token":         serve(t, language.accept, "", setWithLink(h, secretToken, acceptedNewPass)),
			"set, another token":   serve(t, language.accept, "", setWithLink(h, "another-token", acceptedNewPass)),
		}
		var first string
		for name, got := range answers {
			if got.err != nil || got.status != http.StatusGone {
				t.Fatalf("%s [%q]: status %d, err %v, want 410", name, language.accept, got.status, got.err)
			}
			body := withoutRequestID(t, got.body)
			if first == "" {
				first = body
			}
			if body != first {
				t.Errorf("%s [%q] answered %s, other invalid links answered %s: the answers must be identical", name, language.accept, body, first)
			}
			var decoded answered
			if err := json.Unmarshal(got.body, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Error.Code != openapi.SETPASSWORDLINKINVALID || decoded.Error.Message != language.message {
				t.Errorf("%s [%q] answered %s %q, want SET_PASSWORD_LINK_INVALID %q", name, language.accept, decoded.Error.Code, decoded.Error.Message, language.message)
			}
			if bytes.Contains(got.body, []byte("another-token")) || bytes.Contains(got.body, []byte(secretToken)) {
				t.Errorf("%s echoed the token: %s", name, got.body)
			}
		}
	}
}

func TestAMissingBodyAnswersTheSame410(t *testing.T) {
	h := identityhttp.NewIdentity(linkApp(&linkSpy{}, linkOutcome{err: domain.ErrSetPasswordLinkInvalid}), time.Hour, false, nil)
	wantCheck := withoutRequestID(t, serve(t, "", "", checkLink(h, secretToken)).body)
	got := serve(t, "", "", func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.CheckSetPasswordLink(ctx, openapi.CheckSetPasswordLinkRequestObject{})
		return render(w, openapi.CheckSetPasswordLinkResponseObject.VisitCheckSetPasswordLinkResponse, response, err)
	})
	if got.status != http.StatusGone || withoutRequestID(t, got.body) != wantCheck {
		t.Errorf("a check with no body answered %d %s, want the invalid link's 410", got.status, got.body)
	}
	got = serve(t, "", "", func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.SetPasswordWithLink(ctx, openapi.SetPasswordWithLinkRequestObject{})
		return render(w, openapi.SetPasswordWithLinkResponseObject.VisitSetPasswordWithLinkResponse, response, err)
	})
	if got.status != http.StatusGone || withoutRequestID(t, got.body) != wantCheck {
		t.Errorf("a set with no body answered %d %s, want the invalid link's 410", got.status, got.body)
	}
}

func TestACheckNamesTheAccountAndNothingElse(t *testing.T) {
	spy := &linkSpy{}
	expires := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	h := identityhttp.NewIdentity(linkApp(spy, linkOutcome{link: domain.SetPasswordLink{
		UserID: uuid.NewString(), Email: "an@example.test", FullName: "Nguyễn Văn An", Purpose: domain.LinkReset, ExpiresAt: expires,
	}}), time.Hour, false, nil)

	got := serve(t, "", "", checkLink(h, secretToken))

	if got.err != nil || got.status != http.StatusOK {
		t.Fatalf("status %d, err %v", got.status, got.err)
	}
	if spy.check.Token != secretToken {
		t.Errorf("the query got the token %q", spy.check.Token)
	}
	var body map[string]any
	if err := json.Unmarshal(got.body, &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 4 || body["email"] != "an@example.test" || body["fullName"] != "Nguyễn Văn An" || body["purpose"] != "reset" || body["expiresAt"] != "2026-10-15T12:00:00Z" {
		t.Errorf("body = %v, want exactly the email, full name, purpose and expiry", body)
	}
	if bytes.Contains(got.body, []byte(secretToken)) || strings.Contains(string(got.body), "userId") {
		t.Errorf("the check leaked the token or the account's id: %s", got.body)
	}
}

func TestUsingALinkSignsInWithTheRefreshTokenInTheCookieOnly(t *testing.T) {
	spy := &linkSpy{}
	user := domain.User{ID: uuid.NewString(), Email: "an@example.test", FullName: "Nguyễn Văn An"}
	h := identityhttp.NewIdentity(linkApp(spy, linkOutcome{session: model.Session{
		AccessToken: "the-access-token", ExpiresIn: 900, RefreshToken: "the-refresh-token", User: user,
	}}), time.Hour, true, nil)

	got := serve(t, "", "", setWithLink(h, secretToken, acceptedNewPass))

	if got.err != nil || got.status != http.StatusOK {
		t.Fatalf("status %d, err %v: %s", got.status, got.err, got.body)
	}
	if spy.set.Token != secretToken || spy.set.NewPassword != acceptedNewPass || spy.set.IP != "203.0.113.9" || spy.set.UserAgent != "link-http-test" {
		t.Errorf("the command got %+v", spy.set)
	}
	cookies := (&http.Response{Header: got.header}).Cookies()
	if len(cookies) != 1 || cookies[0].Value != "the-refresh-token" || !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Fatalf("cookies = %+v, want one HttpOnly Secure cookie holding the refresh token", cookies)
	}
	var body struct {
		AccessToken string `json:"accessToken"`
		ExpiresIn   int    `json:"expiresIn"`
		User        struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(got.body, &body); err != nil {
		t.Fatal(err)
	}
	if body.AccessToken != "the-access-token" || body.ExpiresIn != 900 || body.User.Email != "an@example.test" {
		t.Errorf("body = %+v", body)
	}
	for _, secret := range []string{"the-refresh-token", secretToken, acceptedNewPass} {
		if bytes.Contains(got.body, []byte(secret)) {
			t.Errorf("the body carries %q", secret)
		}
	}
}

func TestAPasswordThatBreaksTheLengthRuleIsAFieldError(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"short": {domain.ErrPasswordTooShort, "Mật khẩu mới phải có ít nhất 8 ký tự."},
		"long":  {domain.ErrPasswordTooLong, "Mật khẩu mới quá dài."},
	} {
		t.Run(name, func(t *testing.T) {
			h := identityhttp.NewIdentity(linkApp(&linkSpy{}, linkOutcome{err: c.err}), time.Hour, false, nil)

			got := serve(t, "", "", setWithLink(h, secretToken, "x"))

			var body struct {
				Error struct {
					Code    openapi.ErrorCode `json:"code"`
					Details map[string]string `json:"details"`
				} `json:"error"`
			}
			if err := json.Unmarshal(got.body, &body); err != nil {
				t.Fatalf("%v: %s", err, got.body)
			}
			if got.status != http.StatusBadRequest || body.Error.Code != openapi.VALIDATIONFAILED || body.Error.Details["newPassword"] != c.want {
				t.Errorf("answered %d %s %v, want 400 VALIDATION_FAILED naming newPassword with %q", got.status, body.Error.Code, body.Error.Details, c.want)
			}
		})
	}
}

func TestWithPasswordSignInOffUsingAndRequestingALinkAre422(t *testing.T) {
	for _, language := range []struct{ accept, message string }{
		{"", "Trung tâm đã tắt đăng nhập bằng mật khẩu. Hãy đăng nhập bằng Google."},
		{"en", "Your centre has turned off password sign-in. Sign in with Google."},
	} {
		h := identityhttp.NewIdentity(linkApp(&linkSpy{}, linkOutcome{err: domain.ErrPasswordSignInDisabled}), time.Hour, false, nil)
		for name, got := range map[string]served{
			"set":     serve(t, language.accept, "", setWithLink(h, secretToken, acceptedNewPass)),
			"request": serve(t, language.accept, uuid.NewString(), requestLink(h)),
		} {
			var decoded answered
			if err := json.Unmarshal(got.body, &decoded); err != nil {
				t.Fatalf("%s: %v: %s", name, err, got.body)
			}
			if got.status != http.StatusUnprocessableEntity || decoded.Error.Code != openapi.PASSWORDSIGNINDISABLED || decoded.Error.Message != language.message {
				t.Errorf("%s [%q] answered %d %s %q, want 422 PASSWORD_SIGN_IN_DISABLED %q", name, language.accept, got.status, decoded.Error.Code, decoded.Error.Message, language.message)
			}
		}
	}
}

func TestAskingForALinkAnswers204ForTheSignedInAccountAndRefusesTheRest(t *testing.T) {
	caller := uuid.NewString()
	for name, c := range map[string]struct {
		err    error
		status int
		code   openapi.ErrorCode
	}{
		"asked":            {nil, http.StatusNoContent, ""},
		"disabled account": {domain.ErrAccountDisabled, http.StatusUnauthorized, openapi.UNAUTHORIZED},
		"account is gone":  {domain.ErrUserNotFound, http.StatusUnauthorized, openapi.UNAUTHORIZED},
	} {
		t.Run(name, func(t *testing.T) {
			spy := &linkSpy{}
			h := identityhttp.NewIdentity(linkApp(spy, linkOutcome{err: c.err}), time.Hour, false, nil)

			got := serve(t, "", caller, requestLink(h))

			if got.err != nil || got.status != c.status {
				t.Fatalf("status %d, err %v, want %d", got.status, got.err, c.status)
			}
			if spy.requested.UserID != caller || spy.requested.IP != "203.0.113.9" || spy.requested.UserAgent != "link-http-test" {
				t.Errorf("the command got %+v, want the signed-in account and its origin", spy.requested)
			}
			if c.code != "" {
				var decoded answered
				if err := json.Unmarshal(got.body, &decoded); err != nil || decoded.Error.Code != c.code {
					t.Errorf("answered %s (%v), want %s", got.body, err, c.code)
				}
			}
		})
	}
}

func TestAServerThatCannotSendALinkAnswers501(t *testing.T) {
	h := identityhttp.NewIdentity(linkApp(&linkSpy{}, linkOutcome{err: domain.ErrSetPasswordUnavailable}), time.Hour, false, nil)

	got := serve(t, "", uuid.NewString(), requestLink(h))

	if !errors.Is(got.err, httpx.ErrNotImplemented) {
		t.Fatalf("err = %v, want httpx.ErrNotImplemented, which the error handler answers as 501", got.err)
	}
}

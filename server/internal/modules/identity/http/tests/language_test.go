package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

func refusingEverything(err error) *application.Application {
	return &application.Application{Commands: application.Commands{
		Login: cqrs.HandlerFunc[command.Login, model.Session](func(context.Context, command.Login) (model.Session, error) {
			return model.Session{}, err
		}),
		Refresh: cqrs.HandlerFunc[command.Refresh, model.RefreshResult](func(context.Context, command.Refresh) (model.RefreshResult, error) {
			return model.RefreshResult{}, err
		}),
		ChangePassword: cqrs.HandlerFunc[command.ChangePassword, cqrs.Nothing](func(context.Context, command.ChangePassword) (cqrs.Nothing, error) {
			return cqrs.Nothing{}, err
		}),
		GoogleSignIn: cqrs.HandlerFunc[command.GoogleSignIn, model.GoogleSignInResult](func(context.Context, command.GoogleSignIn) (model.GoogleSignInResult, error) {
			return model.GoogleSignInResult{}, err
		}),
		UpdateStudent: cqrs.HandlerFunc[command.UpdateStudent, domain.Student](func(context.Context, command.UpdateStudent) (domain.Student, error) {
			return domain.Student{}, err
		}),
		ResetStudentPassword: cqrs.HandlerFunc[command.ResetStudentPassword, string](func(context.Context, command.ResetStudentPassword) (string, error) {
			return "", err
		}),
	}}
}

func render[R any](w http.ResponseWriter, visit func(R, http.ResponseWriter) error, response R, err error) error {
	if err != nil {
		return err
	}
	return visit(response, w)
}

type answered struct {
	status int
	Error  struct {
		Code    openapi.ErrorCode `json:"code"`
		Message string            `json:"message"`
	} `json:"error"`
}

func answerIn(t *testing.T, acceptLanguage string, call func(context.Context, http.ResponseWriter) error) answered {
	t.Helper()
	caller := uuid.NewString()
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: caller}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := call(r.Context(), w); err != nil {
				t.Fatal(err)
			}
		})))
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	var body answered
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("the answer is not the error envelope: %v: %s", err, response.Body.String())
	}
	body.status = response.Code
	return body
}

func TestIdentityRefusalsSpeakTheCallersLanguage(t *testing.T) {
	id := uuid.New()
	email := openapi_types.Email("moi@example.test")
	for _, c := range []struct {
		name   string
		err    error
		call   func(context.Context, identityhttp.Identity, http.ResponseWriter) error
		status int
		code   openapi.ErrorCode
		vi     string
		en     string
	}{
		{
			name: "a sign-in with the wrong password",
			err:  domain.ErrInvalidCredentials,
			call: func(ctx context.Context, h identityhttp.Identity, w http.ResponseWriter) error {
				response, err := h.Login(ctx, openapi.LoginRequestObject{
					Body: &openapi.LoginJSONRequestBody{Email: openapi_types.Email("a@example.com"), Password: "mật-khẩu"},
				})
				return render(w, openapi.LoginResponseObject.VisitLoginResponse, response, err)
			},
			status: http.StatusUnauthorized,
			code:   openapi.INVALIDCREDENTIALS,
			vi:     "Email hoặc mật khẩu không đúng.",
			en:     "The email or password is incorrect.",
		},
		{
			name: "a refresh that is rejected",
			err:  domain.ErrRefreshRejected,
			call: func(ctx context.Context, h identityhttp.Identity, w http.ResponseWriter) error {
				response, err := h.RefreshSession(ctx, openapi.RefreshSessionRequestObject{})
				return render(w, openapi.RefreshSessionResponseObject.VisitRefreshSessionResponse, response, err)
			},
			status: http.StatusUnauthorized,
			code:   openapi.REFRESHTOKENINVALID,
			vi:     "Phiên đăng nhập đã hết hạn. Vui lòng đăng nhập lại.",
			en:     "Your session has expired. Please sign in again.",
		},
		{
			name: "a refresh token used twice",
			err:  domain.ErrRefreshReused,
			call: func(ctx context.Context, h identityhttp.Identity, w http.ResponseWriter) error {
				response, err := h.RefreshSession(ctx, openapi.RefreshSessionRequestObject{})
				return render(w, openapi.RefreshSessionResponseObject.VisitRefreshSessionResponse, response, err)
			},
			status: http.StatusUnauthorized,
			code:   openapi.REFRESHTOKENREUSED,
			vi:     "Phiên đăng nhập này đã được sử dụng ở nơi khác. Vì lý do an toàn, vui lòng đăng nhập lại.",
			en:     "This session was used somewhere else. For your safety, please sign in again.",
		},
		{
			name: "a password change with the wrong current password",
			err:  domain.ErrInvalidCredentials,
			call: func(ctx context.Context, h identityhttp.Identity, w http.ResponseWriter) error {
				response, err := h.ChangePassword(ctx, openapi.ChangePasswordRequestObject{
					Body: &openapi.ChangePasswordJSONRequestBody{NewPassword: "mật-khẩu-mới"},
				})
				return render(w, openapi.ChangePasswordResponseObject.VisitChangePasswordResponse, response, err)
			},
			status: http.StatusBadRequest,
			code:   openapi.INVALIDCREDENTIALS,
			vi:     "Mật khẩu hiện tại không đúng.",
			en:     "The current password is incorrect.",
		},
		{
			name: "a password change to the same password",
			err:  domain.ErrPasswordUnchanged,
			call: func(ctx context.Context, h identityhttp.Identity, w http.ResponseWriter) error {
				response, err := h.ChangePassword(ctx, openapi.ChangePasswordRequestObject{
					Body: &openapi.ChangePasswordJSONRequestBody{NewPassword: "mật-khẩu-mới"},
				})
				return render(w, openapi.ChangePasswordResponseObject.VisitChangePasswordResponse, response, err)
			},
			status: http.StatusBadRequest,
			code:   openapi.PASSWORDUNCHANGED,
			vi:     "Mật khẩu mới phải khác mật khẩu hiện tại.",
			en:     "The new password must be different from the current one.",
		},
		{
			name: "a Google sign-in to an account nobody provisioned",
			err:  domain.ErrAccountNotProvisioned,
			call: func(ctx context.Context, h identityhttp.Identity, w http.ResponseWriter) error {
				response, err := h.GoogleAuth(ctx, openapi.GoogleAuthRequestObject{
					Body: &openapi.GoogleAuthJSONRequestBody{Code: "code", CodeVerifier: "verifier", RedirectUri: "http://localhost:5173/auth/callback"},
				})
				return render(w, openapi.GoogleAuthResponseObject.VisitGoogleAuthResponse, response, err)
			},
			status: http.StatusForbidden,
			code:   openapi.ACCOUNTNOTPROVISIONED,
			vi:     "Tài khoản này chưa được đăng ký. Bạn cần mã lớp từ giáo viên để tham gia.",
			en:     "This account is not registered. You need a class code from your teacher to join.",
		},
		{
			name: "a Google sign-in to a disabled account",
			err:  domain.ErrAccountDisabled,
			call: func(ctx context.Context, h identityhttp.Identity, w http.ResponseWriter) error {
				response, err := h.GoogleAuth(ctx, openapi.GoogleAuthRequestObject{
					Body: &openapi.GoogleAuthJSONRequestBody{Code: "code", CodeVerifier: "verifier", RedirectUri: "http://localhost:5173/auth/callback"},
				})
				return render(w, openapi.GoogleAuthResponseObject.VisitGoogleAuthResponse, response, err)
			},
			status: http.StatusForbidden,
			code:   openapi.ACCOUNTDISABLED,
			vi:     "Tài khoản của bạn đã bị vô hiệu hoá. Vui lòng liên hệ giáo viên.",
			en:     "Your account has been disabled. Please contact your teacher.",
		},
		{
			name: "a student moved to an email that is taken",
			err:  domain.ErrEmailTaken,
			call: func(ctx context.Context, h identityhttp.Identity, w http.ResponseWriter) error {
				response, err := h.UpdateStudent(ctx, openapi.UpdateStudentRequestObject{
					Id: id, Body: &openapi.UpdateStudentJSONRequestBody{Email: &email},
				})
				return render(w, openapi.UpdateStudentResponseObject.VisitUpdateStudentResponse, response, err)
			},
			status: http.StatusConflict,
			code:   openapi.EMAILTAKEN,
			vi:     "Địa chỉ email này đã được dùng cho một tài khoản khác.",
			en:     "This email address is already used by another account.",
		},
		{
			name: "an email change on a student someone else also reaches",
			err:  domain.ErrStudentShared,
			call: func(ctx context.Context, h identityhttp.Identity, w http.ResponseWriter) error {
				response, err := h.UpdateStudent(ctx, openapi.UpdateStudentRequestObject{
					Id: id, Body: &openapi.UpdateStudentJSONRequestBody{Email: &email},
				})
				return render(w, openapi.UpdateStudentResponseObject.VisitUpdateStudentResponse, response, err)
			},
			status: http.StatusForbidden,
			code:   openapi.STUDENTSHARED,
			vi:     "Học viên này còn thuộc lớp hoặc bài giao của giáo viên khác, hoặc do người khác tạo, nên chỉ quản trị viên mới đổi được email.",
			en:     "This student is still in another teacher's class or assignment, or was created by someone else, so only an admin can change the email.",
		},
		{
			name: "a password reset for a student who is not there",
			err:  domain.ErrStudentNotFound,
			call: func(ctx context.Context, h identityhttp.Identity, w http.ResponseWriter) error {
				response, err := h.ResetStudentPassword(ctx, openapi.ResetStudentPasswordRequestObject{Id: id})
				return render(w, openapi.ResetStudentPasswordResponseObject.VisitResetStudentPasswordResponse, response, err)
			},
			status: http.StatusNotFound,
			code:   openapi.NOTFOUND,
			vi:     "Không tìm thấy học viên.",
			en:     "The student was not found.",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := identityhttp.NewIdentity(refusingEverything(c.err), 0, false, nil)
			for _, language := range []struct {
				accept string
				want   string
			}{{"", c.vi}, {"en", c.en}} {
				got := answerIn(t, language.accept, func(ctx context.Context, w http.ResponseWriter) error {
					return c.call(ctx, h, w)
				})
				if got.status != c.status || got.Error.Code != c.code {
					t.Errorf("Accept-Language %q answered %d %s, want %d %s", language.accept, got.status, got.Error.Code, c.status, c.code)
				}
				if got.Error.Message != language.want {
					t.Errorf("Accept-Language %q answered %q, want %q", language.accept, got.Error.Message, language.want)
				}
			}
		})
	}
}

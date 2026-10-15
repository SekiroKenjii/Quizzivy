package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/validation"
)

type createUserSpy struct {
	got command.CreateUser
}

func createUserApp(spy *createUserSpy, result command.CreateUserResult, err error) *application.Application {
	return &application.Application{Commands: application.Commands{
		CreateUser: cqrs.HandlerFunc[command.CreateUser, command.CreateUserResult](func(_ context.Context, cmd command.CreateUser) (command.CreateUserResult, error) {
			spy.got = cmd
			return result, err
		}),
	}}
}

func createdAs(t *testing.T, principal access.Principal, accept string, h identityhttp.Identity, body *openapi.CreateUserJSONRequestBody) served {
	t.Helper()
	var result served
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, err := h.CreateUser(r.Context(), openapi.CreateUserRequestObject{Body: body})
		result.err = render(w, openapi.CreateUserResponseObject.VisitCreateUserResponse, response, err)
	})
	verify := func(string) (httpx.Principal, error) {
		return httpx.Principal{UserID: principal.UserID, Access: principal}, nil
	}
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(httpx.RequireAuth(nil, verify)(inner))
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	request.Header.Set("User-Agent", "users-http-test")
	if accept != "" {
		request.Header.Set("Accept-Language", accept)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	result.status, result.header, result.body = recorder.Code, recorder.Header(), recorder.Body.Bytes()
	return result
}

func newAccountBody(delivery openapi.UserCreateDelivery) *openapi.CreateUserJSONRequestBody {
	return &openapi.CreateUserJSONRequestBody{
		Email:    openapi_types.Email("moi@example.test"),
		FullName: "Nguyễn Thị Mai",
		RoleId:   uuid.New(),
		Delivery: delivery,
	}
}

func anAdmin() access.Principal {
	return access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)}
}

func createdAccount() domain.AdminUser {
	return domain.AdminUser{
		ID:        uuid.NewString(),
		FullName:  "Nguyễn Thị Mai",
		Email:     "moi@example.test",
		Role:      domain.AdminRole{ID: uuid.NewString(), Name: "Giáo viên", BuiltinKey: "teacher", Icon: "graduation-cap", Color: "info"},
		Status:    domain.AccountMustChangePassword,
		CreatedAt: time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC),
	}
}

func TestCreateUserPassesTheCallerTheRoleTheClassAndTheDeliveryOn(t *testing.T) {
	admin := anAdmin()
	classID := uuid.New()
	body := newAccountBody(openapi.UserCreateDeliveryTemporaryPassword)
	body.ClassId = &classID
	spy := &createUserSpy{}
	h := identityhttp.NewIdentity(createUserApp(spy, command.CreateUserResult{User: createdAccount()}, nil), 0, false, nil)

	got := createdAs(t, admin, "", h, body)
	if got.err != nil || got.status != http.StatusCreated {
		t.Fatalf("answered %d, %v: %s", got.status, got.err, got.body)
	}
	req, in := spy.got.Request, spy.got.Input
	if req.ActorID != admin.UserID || !req.All || req.IP != "203.0.113.9" || req.UserAgent != "users-http-test" {
		t.Errorf("request = %+v, want the Admin, scope.all and the request's origin", req)
	}
	if !req.Grants.Has(access.PeopleUsersManage) {
		t.Error("the caller's grants did not reach the command: the subset rule has nothing to compare")
	}
	if in.Email != "moi@example.test" || in.FullName != "Nguyễn Thị Mai" || in.RoleID != body.RoleId.String() || in.ClassID != classID.String() || in.Delivery != domain.DeliveryTemporaryPassword {
		t.Errorf("input = %+v", in)
	}
}

func TestCreateUserWithoutAClassSendsNone(t *testing.T) {
	spy := &createUserSpy{}
	h := identityhttp.NewIdentity(createUserApp(spy, command.CreateUserResult{User: createdAccount()}, nil), 0, false, nil)
	if got := createdAs(t, anAdmin(), "", h, newAccountBody(openapi.UserCreateDeliveryEmailLink)); got.status != http.StatusCreated {
		t.Fatalf("answered %d: %s", got.status, got.body)
	}
	if spy.got.Input.ClassID != "" {
		t.Errorf("class = %q, want none", spy.got.Input.ClassID)
	}
}

func TestACreatedAccountIsNeverCachedAndShowsItsPasswordOnce(t *testing.T) {
	const password = "Tam-thoi-9Kq2"
	account := createdAccount()
	for _, c := range []struct {
		name     string
		delivery openapi.UserCreateDelivery
		result   command.CreateUserResult
		wantPass bool
	}{
		{"a temporary password", openapi.UserCreateDeliveryTemporaryPassword, command.CreateUserResult{User: account, TemporaryPassword: password}, true},
		{"an email link", openapi.UserCreateDeliveryEmailLink, command.CreateUserResult{User: account}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := identityhttp.NewIdentity(createUserApp(&createUserSpy{}, c.result, nil), 0, false, nil)
			got := createdAs(t, anAdmin(), "", h, newAccountBody(c.delivery))
			if got.err != nil || got.status != http.StatusCreated {
				t.Fatalf("answered %d, %v: %s", got.status, got.err, got.body)
			}
			if cc := got.header.Get("Cache-Control"); cc != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", cc)
			}
			var out struct {
				User              map[string]any `json:"user"`
				TemporaryPassword *string        `json:"temporaryPassword"`
			}
			if err := json.Unmarshal(got.body, &out); err != nil {
				t.Fatal(err)
			}
			if c.wantPass != (out.TemporaryPassword != nil) || (c.wantPass && *out.TemporaryPassword != password) {
				t.Errorf("temporaryPassword = %v, want shown %v", out.TemporaryPassword, c.wantPass)
			}
			if !c.wantPass && strings.Contains(string(got.body), "temporaryPassword") {
				t.Errorf("an email link answer names a temporary password: %s", got.body)
			}
			for _, key := range []string{"id", "fullName", "email", "role", "signIn", "classes", "status", "createdAt"} {
				if _, ok := out.User[key]; !ok {
					t.Errorf("the account row lacks %q: %s", key, got.body)
				}
			}
			for _, leak := range []string{"password_hash", "passwordHash", "hash", "argon2"} {
				if strings.Contains(strings.ToLower(string(got.body)), strings.ToLower(leak)) {
					t.Errorf("the answer carries %q: %s", leak, got.body)
				}
			}
		})
	}
}

func TestACreatedAccountRowTellsHowItSignsIn(t *testing.T) {
	account := createdAccount()
	account.HasGoogle = true
	account.ClassCount, account.FirstClass = 2, strPtr("Lớp 10A")
	h := identityhttp.NewIdentity(createUserApp(&createUserSpy{}, command.CreateUserResult{User: account}, nil), 0, false, nil)
	got := createdAs(t, anAdmin(), "", h, newAccountBody(openapi.UserCreateDeliveryEmailLink))
	var out struct {
		User struct {
			SignIn  struct{ Password, Google bool } `json:"signIn"`
			Classes struct {
				Count     int     `json:"count"`
				FirstName *string `json:"firstName"`
			} `json:"classes"`
			Status string `json:"status"`
			Role   struct {
				BuiltinKey string `json:"builtinKey"`
			} `json:"role"`
		} `json:"user"`
	}
	if err := json.Unmarshal(got.body, &out); err != nil {
		t.Fatal(err)
	}
	if out.User.SignIn.Password || !out.User.SignIn.Google || out.User.Classes.Count != 2 || out.User.Classes.FirstName == nil || *out.User.Classes.FirstName != "Lớp 10A" ||
		out.User.Status != "must_change_password" || out.User.Role.BuiltinKey != "teacher" {
		t.Errorf("row = %+v", out.User)
	}
}

func strPtr(s string) *string { return &s }

func TestEveryCreateUserRefusalHasItsStatusCodeAndBothLanguages(t *testing.T) {
	for _, c := range []struct {
		name   string
		err    error
		status int
		code   openapi.ErrorCode
		vi, en string
	}{
		{
			name: "a name or an email the command rejects", err: &validation.Error{Fields: []validation.Field{{Field: "fullName", Message: "required"}}},
			status: http.StatusBadRequest, code: openapi.VALIDATIONFAILED,
			vi: "Dữ liệu tài khoản không hợp lệ.", en: "The account data is not valid.",
		},
		{
			name: "a class given to a role that is not a student target", err: domain.ErrClassNotAllowed,
			status: http.StatusBadRequest, code: openapi.VALIDATIONFAILED,
			vi: "Chỉ thêm được vào lớp khi vai trò là học viên và bạn có quyền trên mọi lớp.",
			en: "A class can be set only for a student role, and only by someone who reaches every class.",
		},
		{
			name: "a role that does not exist", err: accessdomain.ErrRoleNotFound,
			status: http.StatusNotFound, code: openapi.NOTFOUND,
			vi: "Không tìm thấy vai trò này.", en: "No such role.",
		},
		{
			name: "a class that does not exist or is archived", err: domain.ErrClassNotFound,
			status: http.StatusNotFound, code: openapi.NOTFOUND,
			vi: "Không tìm thấy lớp học này.", en: "No such class.",
		},
		{
			name: "a role changed after the check", err: domain.ErrRoleChanged,
			status: http.StatusConflict, code: openapi.STALEWRITE,
			vi: "Vai trò vừa được thay đổi. Hãy thử lại.", en: "The role was just changed. Try again.",
		},
		{
			name: "a role the caller cannot give", err: accessdomain.ErrTargetRoleNotAllowed,
			status: http.StatusForbidden, code: openapi.TARGETROLENOTALLOWED,
			vi: "Bạn chỉ giao được những quyền bạn có.", en: "You can only give what you can do yourself.",
		},
		{
			name: "an email already in use", err: domain.ErrEmailTaken,
			status: http.StatusConflict, code: openapi.EMAILTAKEN,
			vi: "Địa chỉ email này đã được dùng cho một tài khoản khác.", en: "This email address is already used by another account.",
		},
		{
			name: "the Assistant role", err: accessdomain.ErrNotAssignable,
			status: http.StatusUnprocessableEntity, code: openapi.ROLENOTASSIGNABLE,
			vi: "Không thể chọn vai trò này.", en: "This role cannot be chosen.",
		},
		{
			name: "a temporary password while password sign-in is off", err: domain.ErrPasswordSignInDisabled,
			status: http.StatusUnprocessableEntity, code: openapi.PASSWORDSIGNINDISABLED,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := identityhttp.NewIdentity(createUserApp(&createUserSpy{}, command.CreateUserResult{}, c.err), 0, false, nil)
			for _, language := range []struct{ accept, want string }{{"", c.vi}, {"en", c.en}} {
				got := createdAs(t, anAdmin(), language.accept, h, newAccountBody(openapi.UserCreateDeliveryTemporaryPassword))
				if got.err != nil || got.status != c.status {
					t.Fatalf("answered %d, %v: %s", got.status, got.err, got.body)
				}
				var envelope struct {
					Error struct {
						Code    openapi.ErrorCode `json:"code"`
						Message string            `json:"message"`
					} `json:"error"`
				}
				if err := json.Unmarshal(got.body, &envelope); err != nil {
					t.Fatalf("the answer is not the error envelope: %v: %s", err, got.body)
				}
				if envelope.Error.Code != c.code {
					t.Errorf("code = %s, want %s", envelope.Error.Code, c.code)
				}
				if language.want != "" && envelope.Error.Message != language.want {
					t.Errorf("message (%q) = %q, want %q", language.accept, envelope.Error.Message, language.want)
				}
				if envelope.Error.Message == "" {
					t.Errorf("message (%q) is empty", language.accept)
				}
				if cc := got.header.Get("Cache-Control"); strings.Contains(cc, "public") {
					t.Errorf("Cache-Control = %q", cc)
				}
			}
		})
	}
}

func TestACreatedAccountOnAServerThatCannotSendMailIsNotImplemented(t *testing.T) {
	h := identityhttp.NewIdentity(createUserApp(&createUserSpy{}, command.CreateUserResult{}, domain.ErrSetPasswordUnavailable), 0, false, nil)
	got := createdAs(t, anAdmin(), "", h, newAccountBody(openapi.UserCreateDeliveryEmailLink))
	if got.err == nil || !errors.Is(got.err, httpx.ErrNotImplemented) {
		t.Fatalf("answered %d, err = %v, want httpx.ErrNotImplemented for the 501 mapper", got.status, got.err)
	}
}

func TestAnUnexpectedFailureOfCreateUserIsNotTranslated(t *testing.T) {
	boom := errors.New("the database is gone")
	h := identityhttp.NewIdentity(createUserApp(&createUserSpy{}, command.CreateUserResult{}, boom), 0, false, nil)
	got := createdAs(t, anAdmin(), "", h, newAccountBody(openapi.UserCreateDeliveryEmailLink))
	if !errors.Is(got.err, boom) {
		t.Fatalf("err = %v, want the failure passed on to the 500 mapper", got.err)
	}
}

func TestCreateUserIsNotImplementedWithoutItsCommandOrBody(t *testing.T) {
	for name, h := range map[string]identityhttp.Identity{
		"no application": identityhttp.NewIdentity(nil, 0, false, nil),
		"no command":     identityhttp.NewIdentity(&application.Application{}, 0, false, nil),
	} {
		t.Run(name, func(t *testing.T) {
			got := createdAs(t, anAdmin(), "", h, newAccountBody(openapi.UserCreateDeliveryEmailLink))
			if !errors.Is(got.err, httpx.ErrNotImplemented) {
				t.Errorf("err = %v, want httpx.ErrNotImplemented", got.err)
			}
		})
	}
	h := identityhttp.NewIdentity(createUserApp(&createUserSpy{}, command.CreateUserResult{}, nil), 0, false, nil)
	if got := createdAs(t, anAdmin(), "", h, nil); !errors.Is(got.err, httpx.ErrNotImplemented) {
		t.Errorf("a missing body: err = %v, want httpx.ErrNotImplemented", got.err)
	}
}

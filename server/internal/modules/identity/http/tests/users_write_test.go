package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"

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

type accountStubs struct {
	err      error
	user     domain.AdminUser
	disabled domain.BulkDisable
	resets   domain.BulkUserReset
	password string

	role    command.ChangeUserRole
	disable command.DisableUser
	bulk    command.DisableUsers
	enable  command.EnableUser
	signOut command.SignOutUser
	reset   command.ResetUserPassword
	many    command.ResetUsersPasswords
}

func (s *accountStubs) app() *application.Application {
	return &application.Application{Commands: application.Commands{
		ChangeUserRole: cqrs.HandlerFunc[command.ChangeUserRole, domain.AdminUser](func(_ context.Context, c command.ChangeUserRole) (domain.AdminUser, error) {
			s.role = c
			return s.user, s.err
		}),
		DisableUser: cqrs.HandlerFunc[command.DisableUser, domain.AdminUser](func(_ context.Context, c command.DisableUser) (domain.AdminUser, error) {
			s.disable = c
			return s.user, s.err
		}),
		DisableUsers: cqrs.HandlerFunc[command.DisableUsers, domain.BulkDisable](func(_ context.Context, c command.DisableUsers) (domain.BulkDisable, error) {
			s.bulk = c
			return s.disabled, s.err
		}),
		EnableUser: cqrs.HandlerFunc[command.EnableUser, domain.AdminUser](func(_ context.Context, c command.EnableUser) (domain.AdminUser, error) {
			s.enable = c
			return s.user, s.err
		}),
		SignOutUser: cqrs.HandlerFunc[command.SignOutUser, cqrs.Nothing](func(_ context.Context, c command.SignOutUser) (cqrs.Nothing, error) {
			s.signOut = c
			return cqrs.Nothing{}, s.err
		}),
		ResetUserPassword: cqrs.HandlerFunc[command.ResetUserPassword, command.ResetUserPasswordResult](func(_ context.Context, c command.ResetUserPassword) (command.ResetUserPasswordResult, error) {
			s.reset = c
			return command.ResetUserPasswordResult{TemporaryPassword: s.password}, s.err
		}),
		ResetUsersPasswords: cqrs.HandlerFunc[command.ResetUsersPasswords, domain.BulkUserReset](func(_ context.Context, c command.ResetUsersPasswords) (domain.BulkUserReset, error) {
			s.many = c
			return s.resets, s.err
		}),
	}}
}

func (s *accountStubs) handler() identityhttp.Identity {
	return identityhttp.NewIdentity(s.app(), 0, false, nil)
}

type accountCall struct {
	name string
	run  func(h identityhttp.Identity, ctx context.Context, w http.ResponseWriter, id uuid.UUID) error
}

func theSingleAccountCalls() []accountCall {
	return []accountCall{
		{"changeUserRole", func(h identityhttp.Identity, ctx context.Context, w http.ResponseWriter, id uuid.UUID) error {
			response, err := h.ChangeUserRole(ctx, openapi.ChangeUserRoleRequestObject{Id: id, Body: &openapi.ChangeUserRoleJSONRequestBody{RoleId: uuid.New()}})
			return render(w, openapi.ChangeUserRoleResponseObject.VisitChangeUserRoleResponse, response, err)
		}},
		{"disableUser", func(h identityhttp.Identity, ctx context.Context, w http.ResponseWriter, id uuid.UUID) error {
			response, err := h.DisableUser(ctx, openapi.DisableUserRequestObject{Id: id})
			return render(w, openapi.DisableUserResponseObject.VisitDisableUserResponse, response, err)
		}},
		{"enableUser", func(h identityhttp.Identity, ctx context.Context, w http.ResponseWriter, id uuid.UUID) error {
			response, err := h.EnableUser(ctx, openapi.EnableUserRequestObject{Id: id})
			return render(w, openapi.EnableUserResponseObject.VisitEnableUserResponse, response, err)
		}},
		{"signOutUser", func(h identityhttp.Identity, ctx context.Context, w http.ResponseWriter, id uuid.UUID) error {
			response, err := h.SignOutUser(ctx, openapi.SignOutUserRequestObject{Id: id})
			return render(w, openapi.SignOutUserResponseObject.VisitSignOutUserResponse, response, err)
		}},
		{"resetUserPassword", func(h identityhttp.Identity, ctx context.Context, w http.ResponseWriter, id uuid.UUID) error {
			response, err := h.ResetUserPassword(ctx, openapi.ResetUserPasswordRequestObject{Id: id, Body: &openapi.ResetUserPasswordJSONRequestBody{Delivery: openapi.UserResetDeliveryTemporaryPassword}})
			return render(w, openapi.ResetUserPasswordResponseObject.VisitResetUserPasswordResponse, response, err)
		}},
	}
}

func (c accountCall) as(t *testing.T, accept string, s *accountStubs, id uuid.UUID) served {
	t.Helper()
	h := s.handler()
	return readAs(t, anAdmin(), accept, func(ctx context.Context, w http.ResponseWriter) error { return c.run(h, ctx, w, id) })
}

func codeOf(t *testing.T, body []byte) (string, map[string]any) {
	t.Helper()
	var out struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("not an error envelope: %s", body)
	}
	return out.Error.Code, out.Error.Details
}

func TestEveryAccountWriteAnswersEachRefusalWithItsStatusAndCode(t *testing.T) {
	type refusal struct {
		err    error
		status int
		code   string
		skip   map[string]bool
	}
	for _, r := range []refusal{
		{domain.ErrUserNotFound, http.StatusNotFound, "NOT_FOUND", nil},
		{accessdomain.ErrTargetRoleNotAllowed, http.StatusForbidden, "TARGET_ROLE_NOT_ALLOWED", nil},
		{domain.ErrCannotActOnSelf, http.StatusUnprocessableEntity, "CANNOT_ACT_ON_SELF", map[string]bool{"enableUser": true, "signOutUser": true}},
		{domain.ErrLastAdmin, http.StatusConflict, "LAST_ADMIN", map[string]bool{"enableUser": true, "signOutUser": true, "resetUserPassword": true}},
		{domain.ErrRoleChanged, http.StatusConflict, "STALE_WRITE", nil},
		{fmt.Errorf("%w: deadlock detected", domain.ErrWriteContended), http.StatusConflict, "STALE_WRITE", nil},
		{domain.ErrAccountDisabled, http.StatusConflict, "ACCOUNT_DISABLED", map[string]bool{"changeUserRole": true, "disableUser": true, "enableUser": true, "signOutUser": true}},
		{domain.ErrPasswordSignInDisabled, http.StatusUnprocessableEntity, "PASSWORD_SIGN_IN_DISABLED", map[string]bool{"changeUserRole": true, "disableUser": true, "enableUser": true, "signOutUser": true}},
	} {
		for _, call := range theSingleAccountCalls() {
			if r.skip[call.name] {
				continue
			}
			t.Run(call.name+" "+r.code, func(t *testing.T) {
				got := call.as(t, "", &accountStubs{err: r.err}, uuid.New())
				if got.err != nil || got.status != r.status {
					t.Fatalf("answered %d, %v: %s", got.status, got.err, got.body)
				}
				if code, _ := codeOf(t, got.body); code != r.code {
					t.Errorf("code = %s, want %s", code, r.code)
				}
			})
		}
	}
}

func TestAnUnknownRoleAnAssistantAndAnAccountThatOwnsContentAreTheirOwnAnswers(t *testing.T) {
	role := theSingleAccountCalls()[0]

	got := role.as(t, "", &accountStubs{err: accessdomain.ErrRoleNotFound}, uuid.New())
	if code, _ := codeOf(t, got.body); got.status != http.StatusNotFound || code != "NOT_FOUND" {
		t.Errorf("unknown role: %d %s, want 404 NOT_FOUND", got.status, code)
	}
	got = role.as(t, "", &accountStubs{err: accessdomain.ErrNotAssignable}, uuid.New())
	if code, _ := codeOf(t, got.body); got.status != http.StatusUnprocessableEntity || code != "ROLE_NOT_ASSIGNABLE" {
		t.Errorf("assistant: %d %s, want 422 ROLE_NOT_ASSIGNABLE", got.status, code)
	}
	owned := domain.OwnedContent{Classes: 2, Tests: 3, Questions: 40, Media: 5, Assignments: 7}
	got = role.as(t, "en", &accountStubs{err: &domain.OwnsContentError{Owned: owned}}, uuid.New())
	code, details := codeOf(t, got.body)
	if got.status != http.StatusConflict || code != "USER_OWNS_CONTENT" {
		t.Fatalf("owner: %d %s, want 409 USER_OWNS_CONTENT", got.status, code)
	}
	want := map[string]any{"classes": 2.0, "tests": 3.0, "questions": 40.0, "media": 5.0, "assignments": 7.0}
	for key, n := range want {
		if details[key] != n {
			t.Errorf("details[%s] = %v, want %v: %v", key, details[key], n, details)
		}
	}
}

func TestAFaultThatIsNotARefusalIsLeftToTheServerErrorPath(t *testing.T) {
	for _, call := range theSingleAccountCalls() {
		t.Run(call.name, func(t *testing.T) {
			boom := errors.New("pq: connection reset")
			got := call.as(t, "", &accountStubs{err: boom}, uuid.New())
			if !errors.Is(got.err, boom) {
				t.Errorf("answered %d with err %v, want the fault returned for the 500 path", got.status, got.err)
			}
		})
	}
}

func TestTheSingleWritesPassTheCallerTheAccountAndWhatTheyWereAskedToDo(t *testing.T) {
	id, role := uuid.New(), uuid.New()
	s := &accountStubs{user: createdAccount(), password: "Tam-thoi-9Kq2"}
	h := s.handler()
	admin := anAdmin()
	drive := func(call func(ctx context.Context, w http.ResponseWriter) error) served {
		return readAs(t, admin, "", call)
	}

	drive(func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.ChangeUserRole(ctx, openapi.ChangeUserRoleRequestObject{Id: id, Body: &openapi.ChangeUserRoleJSONRequestBody{RoleId: role}})
		return render(w, openapi.ChangeUserRoleResponseObject.VisitChangeUserRoleResponse, response, err)
	})
	drive(func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.ResetUserPassword(ctx, openapi.ResetUserPasswordRequestObject{Id: id, Body: &openapi.ResetUserPasswordJSONRequestBody{Delivery: openapi.UserResetDeliveryEmailLink}})
		return render(w, openapi.ResetUserPasswordResponseObject.VisitResetUserPasswordResponse, response, err)
	})
	drive(func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.SignOutUser(ctx, openapi.SignOutUserRequestObject{Id: id})
		return render(w, openapi.SignOutUserResponseObject.VisitSignOutUserResponse, response, err)
	})

	if s.role.ID != id.String() || s.role.RoleID != role.String() || s.role.Request.ActorID != admin.UserID || !s.role.Request.All ||
		s.role.Request.IP != "203.0.113.9" || s.role.Request.UserAgent != "users-http-test" || !s.role.Request.Grants.Has(access.PeopleUsersManage) {
		t.Errorf("role change = %+v, want the account, the role and the caller's origin and grants", s.role)
	}
	if s.reset.ID != id.String() || s.reset.Delivery != domain.DeliveryEmailLink {
		t.Errorf("reset = %+v", s.reset)
	}
	if s.signOut.ID != id.String() {
		t.Errorf("sign-out = %+v", s.signOut)
	}
}

func TestASuccessfulSingleWriteAnswersWithTheAccountRowOr204(t *testing.T) {
	calls := theSingleAccountCalls()
	byName := map[string]accountCall{}
	for _, c := range calls {
		byName[c.name] = c
	}
	for _, name := range []string{"changeUserRole", "disableUser", "enableUser"} {
		got := byName[name].as(t, "", &accountStubs{user: createdAccount()}, uuid.New())
		var row map[string]any
		if err := json.Unmarshal(got.body, &row); err != nil || got.status != http.StatusOK || row["id"] == nil || row["role"] == nil {
			t.Errorf("%s answered %d: %s", name, got.status, got.body)
		}
	}
	if got := byName["signOutUser"].as(t, "", &accountStubs{}, uuid.New()); got.status != http.StatusNoContent || len(got.body) != 0 {
		t.Errorf("signOutUser answered %d %q, want 204 and nothing", got.status, got.body)
	}
}

func TestASingleResetShowsTheTemporaryPasswordOnceAndNeverCachesIt(t *testing.T) {
	reset := theSingleAccountCalls()[4]

	got := reset.as(t, "", &accountStubs{password: "Tam-thoi-9Kq2"}, uuid.New())
	var out map[string]any
	if err := json.Unmarshal(got.body, &out); err != nil || got.status != http.StatusOK || out["temporaryPassword"] != "Tam-thoi-9Kq2" || got.header.Get("Cache-Control") != "no-store" {
		t.Errorf("a temporary password reset: %d %q cache %q: %s", got.status, out["temporaryPassword"], got.header.Get("Cache-Control"), got.body)
	}
	got = reset.as(t, "", &accountStubs{}, uuid.New())
	if got.status != http.StatusOK || got.header.Get("Cache-Control") != "no-store" || string(got.body) != "{}\n" && string(got.body) != "{}" {
		t.Errorf("a link reset: %d cache %q body %s, want an empty object, still not cached", got.status, got.header.Get("Cache-Control"), got.body)
	}
}

func TestAServerWithoutALinkSenderAnswers501ForALinkReset(t *testing.T) {
	reset := theSingleAccountCalls()[4]
	got := reset.as(t, "", &accountStubs{err: domain.ErrSetPasswordUnavailable}, uuid.New())
	if !errors.Is(got.err, httpx.ErrNotImplemented) {
		t.Errorf("answered %d, %v, want the not-implemented error the server turns into 501", got.status, got.err)
	}
}

func TestWritesAnswer501WhereTheUsersDirectoryIsNotAttached(t *testing.T) {
	h := identityhttp.NewIdentity(&application.Application{}, 0, false, nil)
	for _, call := range theSingleAccountCalls() {
		got := readAs(t, anAdmin(), "", func(ctx context.Context, w http.ResponseWriter) error { return call.run(h, ctx, w, uuid.New()) })
		if !errors.Is(got.err, httpx.ErrNotImplemented) {
			t.Errorf("%s answered %d, %v, want 501", call.name, got.status, got.err)
		}
	}
}

func TestABulkDisableAnswersWhoWasDisabledAndWhyTheOthersWereNot(t *testing.T) {
	one, two, bad, gone, boom, held, moving := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	s := &accountStubs{disabled: domain.BulkDisable{
		Disabled: []string{one.String(), two.String()},
		Failed: []domain.UserFailure{
			{UserID: bad.String(), Reason: domain.ErrLastAdmin},
			{UserID: gone.String(), Reason: domain.ErrUserNotFound},
			{UserID: boom.String(), Reason: domain.ErrActionFailed},
			{UserID: held.String(), Reason: domain.ErrWriteContended},
			{UserID: moving.String(), Reason: domain.ErrRoleChanged},
		},
	}}
	h := s.handler()

	got := readAs(t, anAdmin(), "", func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.DisableUsers(ctx, openapi.DisableUsersRequestObject{Body: &openapi.DisableUsersJSONRequestBody{UserIds: []uuid.UUID{one, two, bad, gone, boom, held, moving}}})
		return render(w, openapi.DisableUsersResponseObject.VisitDisableUsersResponse, response, err)
	})

	var out struct {
		Disabled []string `json:"disabled"`
		Failed   []struct {
			UserId string `json:"userId"`
			Code   string `json:"code"`
		} `json:"failed"`
	}
	if err := json.Unmarshal(got.body, &out); err != nil || got.status != http.StatusOK {
		t.Fatalf("answered %d, %v: %s", got.status, err, got.body)
	}
	if !slices.Equal(out.Disabled, []string{one.String(), two.String()}) {
		t.Errorf("disabled = %v", out.Disabled)
	}
	wantCodes := []string{"LAST_ADMIN", "NOT_FOUND", "INTERNAL", "STALE_WRITE", "STALE_WRITE"}
	if len(out.Failed) != len(wantCodes) {
		t.Fatalf("failed = %+v", out.Failed)
	}
	for i, code := range wantCodes {
		if out.Failed[i].Code != code {
			t.Errorf("failure %d = %+v, want %s", i, out.Failed[i], code)
		}
	}
	if !slices.Equal(s.bulk.IDs, []string{one.String(), two.String(), bad.String(), gone.String(), boom.String(), held.String(), moving.String()}) {
		t.Errorf("the command was given %v, want the ids in the order named", s.bulk.IDs)
	}
}

func TestABulkDisableWithNoFailureAnswersAnEmptyListNotNull(t *testing.T) {
	one := uuid.New()
	s := &accountStubs{disabled: domain.BulkDisable{Disabled: []string{one.String()}}}
	h := s.handler()

	got := readAs(t, anAdmin(), "", func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.DisableUsers(ctx, openapi.DisableUsersRequestObject{Body: &openapi.DisableUsersJSONRequestBody{UserIds: []uuid.UUID{one}}})
		return render(w, openapi.DisableUsersResponseObject.VisitDisableUsersResponse, response, err)
	})

	var out map[string]any
	if err := json.Unmarshal(got.body, &out); err != nil {
		t.Fatal(err)
	}
	if failed, ok := out["failed"].([]any); !ok || len(failed) != 0 {
		t.Errorf("failed = %v in %s, want []", out["failed"], got.body)
	}
}

func TestTheBulkFormsRefuseAnAccountNamedTwiceBeforeAnythingRuns(t *testing.T) {
	id := uuid.New()
	s := &accountStubs{}
	h := s.handler()

	disable := readAs(t, anAdmin(), "", func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.DisableUsers(ctx, openapi.DisableUsersRequestObject{Body: &openapi.DisableUsersJSONRequestBody{UserIds: []uuid.UUID{id, uuid.New(), id}}})
		return render(w, openapi.DisableUsersResponseObject.VisitDisableUsersResponse, response, err)
	})
	reset := readAs(t, anAdmin(), "", func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.ResetUsersPasswords(ctx, openapi.ResetUsersPasswordsRequestObject{Body: &openapi.ResetUsersPasswordsJSONRequestBody{UserIds: []uuid.UUID{id, id}, Delivery: openapi.UserResetDeliveryTemporaryPassword}})
		return render(w, openapi.ResetUsersPasswordsResponseObject.VisitResetUsersPasswordsResponse, response, err)
	})

	for name, got := range map[string]served{"disable": disable, "reset": reset} {
		if code, _ := codeOf(t, got.body); got.status != http.StatusBadRequest || code != "VALIDATION_FAILED" {
			t.Errorf("%s with a repeated id: %d %s: %s", name, got.status, code, got.body)
		}
	}
	if len(s.bulk.IDs) != 0 || len(s.many.IDs) != 0 {
		t.Errorf("a refused request reached the commands: %v %v", s.bulk.IDs, s.many.IDs)
	}
}

func TestABulkResetAnswersEachPasswordWithItsAccountAndNeverCachesIt(t *testing.T) {
	one, bad := uuid.New(), uuid.New()
	s := &accountStubs{resets: domain.BulkUserReset{
		Reset:  []domain.UserReset{{UserID: one.String(), FullName: "Nguyễn Văn An", Email: "an@example.test", TemporaryPassword: "Tam-thoi-9Kq2"}},
		Failed: []domain.UserFailure{{UserID: bad.String(), Reason: domain.ErrAccountDisabled}},
	}}
	h := s.handler()

	got := readAs(t, anAdmin(), "", func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.ResetUsersPasswords(ctx, openapi.ResetUsersPasswordsRequestObject{Body: &openapi.ResetUsersPasswordsJSONRequestBody{UserIds: []uuid.UUID{one, bad}, Delivery: openapi.UserResetDeliveryTemporaryPassword}})
		return render(w, openapi.ResetUsersPasswordsResponseObject.VisitResetUsersPasswordsResponse, response, err)
	})

	var out struct {
		Items []struct {
			UserId            string  `json:"userId"`
			FullName          string  `json:"fullName"`
			Email             string  `json:"email"`
			TemporaryPassword *string `json:"temporaryPassword"`
		} `json:"items"`
		Failed []struct {
			UserId string `json:"userId"`
			Code   string `json:"code"`
		} `json:"failed"`
	}
	if err := json.Unmarshal(got.body, &out); err != nil || got.status != http.StatusOK {
		t.Fatalf("answered %d, %v: %s", got.status, err, got.body)
	}
	if got.header.Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got.header.Get("Cache-Control"))
	}
	if len(out.Items) != 1 || out.Items[0].UserId != one.String() || out.Items[0].FullName != "Nguyễn Văn An" || out.Items[0].Email != "an@example.test" ||
		out.Items[0].TemporaryPassword == nil || *out.Items[0].TemporaryPassword != "Tam-thoi-9Kq2" {
		t.Errorf("items = %+v", out.Items)
	}
	if len(out.Failed) != 1 || out.Failed[0].UserId != bad.String() || out.Failed[0].Code != "ACCOUNT_DISABLED" {
		t.Errorf("failed = %+v", out.Failed)
	}
	if s.many.Delivery != domain.DeliveryTemporaryPassword || !slices.Equal(s.many.IDs, []string{one.String(), bad.String()}) {
		t.Errorf("the command was given %+v", s.many)
	}
}

func TestABulkLinkResetNamesNoTemporaryPassword(t *testing.T) {
	one := uuid.New()
	s := &accountStubs{resets: domain.BulkUserReset{Reset: []domain.UserReset{{UserID: one.String(), FullName: "An", Email: "an@example.test"}}}}
	h := s.handler()

	got := readAs(t, anAdmin(), "", func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.ResetUsersPasswords(ctx, openapi.ResetUsersPasswordsRequestObject{Body: &openapi.ResetUsersPasswordsJSONRequestBody{UserIds: []uuid.UUID{one}, Delivery: openapi.UserResetDeliveryEmailLink}})
		return render(w, openapi.ResetUsersPasswordsResponseObject.VisitResetUsersPasswordsResponse, response, err)
	})

	if got.status != http.StatusOK || string(got.body) == "" || bytes.Contains(got.body, []byte("temporaryPassword")) {
		t.Errorf("answered %d: %s, want no temporaryPassword key at all", got.status, got.body)
	}
}

func TestABulkResetOverTheLimitOrWithPasswordsOffIsRefusedWholesale(t *testing.T) {
	for name, c := range map[string]struct {
		err    error
		status int
		code   string
	}{
		"too many accounts":    {func() error { var v validation.Error; v.Add("userIds", "quá nhiều"); return &v }(), http.StatusBadRequest, "VALIDATION_FAILED"},
		"password sign-in off": {domain.ErrPasswordSignInDisabled, http.StatusUnprocessableEntity, "PASSWORD_SIGN_IN_DISABLED"},
	} {
		t.Run(name, func(t *testing.T) {
			s := &accountStubs{err: c.err}
			h := s.handler()
			got := readAs(t, anAdmin(), "", func(ctx context.Context, w http.ResponseWriter) error {
				response, err := h.ResetUsersPasswords(ctx, openapi.ResetUsersPasswordsRequestObject{Body: &openapi.ResetUsersPasswordsJSONRequestBody{UserIds: []uuid.UUID{uuid.New()}, Delivery: openapi.UserResetDeliveryTemporaryPassword}})
				return render(w, openapi.ResetUsersPasswordsResponseObject.VisitResetUsersPasswordsResponse, response, err)
			})
			if code, _ := codeOf(t, got.body); got.status != c.status || code != c.code {
				t.Errorf("answered %d %s: %s, want %d %s", got.status, code, got.body, c.status, c.code)
			}
		})
	}
}

package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/access/application"
	"quizzivy/internal/modules/access/domain"
	accesshttp "quizzivy/internal/modules/access/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

type refusing struct {
	domain.Repository
	err error
}

func (r refusing) InTx(context.Context, func(context.Context, domain.RoleTx) error) error {
	return r.err
}

func answer(t *testing.T, w http.ResponseWriter, visit func(http.ResponseWriter) error) {
	t.Helper()
	if err := visit(w); err != nil {
		t.Fatal(err)
	}
}

func call(t *testing.T, name string, refused error, acceptLanguage string) *httptest.ResponseRecorder {
	t.Helper()
	app := application.New(refusing{err: refused}).WithRoleStore(refusing{err: refused})
	transport := accesshttp.NewAccess(app)
	user := uuid.NewString()
	renamed := "Tổ chấm"
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: user, Access: access.Principal{UserID: user, RoleID: uuid.NewString(), Permissions: access.NewSet(access.PeopleRolesManage)}}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			switch name {
			case "create":
				out, err := transport.CreateRole(ctx, openapi.CreateRoleRequestObject{Body: &openapi.CreateRoleJSONRequestBody{Name: renamed}})
				if err != nil {
					t.Fatal(err)
				}
				answer(t, w, out.VisitCreateRoleResponse)
			case "createBlank":
				out, err := transport.CreateRole(ctx, openapi.CreateRoleRequestObject{Body: &openapi.CreateRoleJSONRequestBody{Name: "  "}})
				if err != nil {
					t.Fatal(err)
				}
				answer(t, w, out.VisitCreateRoleResponse)
			case "update":
				out, err := transport.UpdateRole(ctx, openapi.UpdateRoleRequestObject{Id: uuid.New(), Body: &openapi.UpdateRoleJSONRequestBody{Name: &renamed, ExpectedRevision: 1}})
				if err != nil {
					t.Fatal(err)
				}
				answer(t, w, out.VisitUpdateRoleResponse)
			case "save":
				out, err := transport.SetRolePermissions(ctx, openapi.SetRolePermissionsRequestObject{Body: &openapi.SetRolePermissionsJSONRequestBody{
					Changes:           []openapi.RolePermissionChange{{RoleId: uuid.New(), Key: openapi.PermissionKey(access.TeachingGrading), Granted: true}},
					ExpectedRevisions: []openapi.RoleRevision{},
				}})
				if err != nil {
					t.Fatal(err)
				}
				answer(t, w, out.VisitSetRolePermissionsResponse)
			}
		})))
	request := httptest.NewRequest(http.MethodPost, "/admin/roles", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func envelope(t *testing.T, response *httptest.ResponseRecorder) (code, message string) {
	t.Helper()
	var body openapi.ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("%d %q: %v", response.Code, response.Body.String(), err)
	}
	return string(body.Error.Code), body.Error.Message
}

func TestEachRefusalAnswersWithItsStatusAndCode(t *testing.T) {
	for _, c := range []struct {
		refused error
		ops     []string
		status  int
		code    string
	}{
		{domain.ErrRoleNotFound, []string{"create", "update", "save"}, http.StatusNotFound, "NOT_FOUND"},
		{domain.ErrNameTaken, []string{"create", "update"}, http.StatusConflict, "ROLE_NAME_TAKEN"},
		{domain.ErrStaleWrite, []string{"update", "save"}, http.StatusConflict, "STALE_WRITE"},
		{domain.ErrRoleLimit, []string{"create"}, http.StatusUnprocessableEntity, "ROLE_LIMIT_REACHED"},
		{domain.ErrNotAssignable, []string{"create"}, http.StatusUnprocessableEntity, "ROLE_NOT_ASSIGNABLE"},
		{domain.ErrRoleLocked, []string{"save"}, http.StatusUnprocessableEntity, "ROLE_LOCKED"},
		{domain.ErrStudentNeedsTakeTests, []string{"save"}, http.StatusUnprocessableEntity, "STUDENT_NEEDS_TAKE_TESTS"},
		{domain.ErrKeyNotGrantable, []string{"save"}, http.StatusUnprocessableEntity, "KEY_NOT_GRANTABLE"},
		{domain.ErrCannotActOnSelf, []string{"save"}, http.StatusUnprocessableEntity, "CANNOT_ACT_ON_SELF"},
		{domain.ErrTargetRoleNotAllowed, []string{"create", "update", "save"}, http.StatusForbidden, "TARGET_ROLE_NOT_ALLOWED"},
	} {
		for _, op := range c.ops {
			response := call(t, op, c.refused, "")
			code, message := envelope(t, response)
			if response.Code != c.status || code != c.code || message == "" {
				t.Errorf("%s refused with %v: %d %s %q, want %d %s", op, c.refused, response.Code, code, message, c.status, c.code)
			}
		}
	}
}

func TestTheMessageFollowsTheAcceptedLanguage(t *testing.T) {
	_, vi := envelope(t, call(t, "save", domain.ErrTargetRoleNotAllowed, ""))
	_, en := envelope(t, call(t, "save", domain.ErrTargetRoleNotAllowed, "en"))
	if vi != "Bạn chỉ đổi được những quyền bạn có." || en != "You can only change what you can do yourself." {
		t.Errorf("vi %q, en %q", vi, en)
	}
}

func TestBadInputAnswers400WithTheField(t *testing.T) {
	response := call(t, "createBlank", nil, "")
	var body struct {
		Error struct {
			Code    string            `json:"code"`
			Details map[string]string `json:"details"`
		} `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadRequest || body.Error.Code != "VALIDATION_FAILED" || body.Error.Details["name"] == "" {
		t.Errorf("%d %+v, want 400 VALIDATION_FAILED naming the name", response.Code, body.Error)
	}
}

func TestAnUnexpectedFailureIsNotDisguised(t *testing.T) {
	app := application.New(refusing{err: errors.New("disk")}).WithRoleStore(refusing{err: errors.New("disk")})
	transport := accesshttp.NewAccess(app)
	user := uuid.NewString()
	handler := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
		return httpx.Principal{UserID: user, Access: access.Principal{UserID: user}}, nil
	})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, err := transport.CreateRole(r.Context(), openapi.CreateRoleRequestObject{Body: &openapi.CreateRoleJSONRequestBody{Name: "Mới"}})
		if err == nil {
			t.Error("a store failure was answered as a refusal")
		}
	}))
	request := httptest.NewRequest(http.MethodPost, "/admin/roles", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	handler.ServeHTTP(httptest.NewRecorder(), request)
}

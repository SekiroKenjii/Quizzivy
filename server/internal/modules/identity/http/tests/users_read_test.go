package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/paging"
)

type listUsersSpy struct {
	got query.ListUsers
}

type getUserSpy struct {
	got query.GetUser
}

func readApp(list *listUsersSpy, listed query.ListUsersResult, listErr error, get *getUserSpy, found query.GetUserResult, getErr error) *application.Application {
	return &application.Application{Queries: application.Queries{
		ListUsers: cqrs.HandlerFunc[query.ListUsers, query.ListUsersResult](func(_ context.Context, q query.ListUsers) (query.ListUsersResult, error) {
			list.got = q
			return listed, listErr
		}),
		GetUser: cqrs.HandlerFunc[query.GetUser, query.GetUserResult](func(_ context.Context, q query.GetUser) (query.GetUserResult, error) {
			get.got = q
			return found, getErr
		}),
	}}
}

func readAs(t *testing.T, principal access.Principal, accept string, call func(context.Context, http.ResponseWriter) error) served {
	t.Helper()
	var result served
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { result.err = call(r.Context(), w) })
	verify := func(string) (httpx.Principal, error) {
		return httpx.Principal{UserID: principal.UserID, Access: principal}, nil
	}
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(httpx.RequireAuth(nil, verify)(inner))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
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

func listUsersAs(t *testing.T, h identityhttp.Identity, params openapi.ListUsersParams) served {
	t.Helper()
	return readAs(t, anAdmin(), "", func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.ListUsers(ctx, openapi.ListUsersRequestObject{Params: params})
		return render(w, openapi.ListUsersResponseObject.VisitListUsersResponse, response, err)
	})
}

func getUserAs(t *testing.T, principal access.Principal, accept string, h identityhttp.Identity, id uuid.UUID) served {
	t.Helper()
	return readAs(t, principal, accept, func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.GetUser(ctx, openapi.GetUserRequestObject{Id: id})
		return render(w, openapi.GetUserResponseObject.VisitGetUserResponse, response, err)
	})
}

func TestListUsersPassesTheFiltersOnAndStartsOnTheAllTab(t *testing.T) {
	spy := &listUsersSpy{}
	h := identityhttp.NewIdentity(readApp(spy, query.ListUsersResult{}, nil, &getUserSpy{}, query.GetUserResult{}, nil), 0, false, nil)

	if got := listUsersAs(t, h, openapi.ListUsersParams{}); got.err != nil || got.status != http.StatusOK {
		t.Fatalf("answered %d, %v: %s", got.status, got.err, got.body)
	}
	if spy.got.Query != (domain.AdminUserQuery{Tab: domain.TabAll}) {
		t.Errorf("no parameters: query = %+v, want the All tab and nothing else", spy.got.Query)
	}

	tab, status, q, email := openapi.AdminUserTabStudents, openapi.ListUsersParamsStatusNeverSignedIn, openapi.Query("phát âm"), openapi_types.Email("Mai@Example.test")
	page, limit := openapi.Page(3), openapi.ListUsersParamsLimit(50)
	listUsersAs(t, h, openapi.ListUsersParams{Tab: &tab, Status: &status, Q: &q, Email: &email, Page: &page, Limit: &limit})
	want := domain.AdminUserQuery{Tab: domain.TabStudents, Status: domain.AccountNeverSignedIn, Query: "phát âm", Email: "Mai@Example.test", Page: 3, Limit: 50}
	if spy.got.Query != want {
		t.Errorf("query = %+v, want %+v", spy.got.Query, want)
	}
}

func TestListUsersAnswersThePageItsPositionAndEveryTabsCount(t *testing.T) {
	account := createdAccount()
	account.HasPassword, account.ClassCount, account.FirstClass = true, 3, strPtr("Lớp 10A")
	listed := query.ListUsersResult{
		Items:  []domain.AdminUser{account},
		Page:   paging.Page{Number: 2, Size: 10, Total: 11},
		Facets: domain.AdminUserFacets{All: 40, Teachers: 7, Students: 30, Admins: 2, Disabled: 1},
	}
	h := identityhttp.NewIdentity(readApp(&listUsersSpy{}, listed, nil, &getUserSpy{}, query.GetUserResult{}, nil), 0, false, nil)

	got := listUsersAs(t, h, openapi.ListUsersParams{})
	var out struct {
		Items    []map[string]any `json:"items"`
		Page     int              `json:"page"`
		PageSize int              `json:"pageSize"`
		Total    int              `json:"total"`
		Facets   map[string]int   `json:"facets"`
	}
	if err := json.Unmarshal(got.body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 || out.Items[0]["id"] != account.ID || out.Page != 2 || out.PageSize != 10 || out.Total != 11 {
		t.Errorf("answer = %s", got.body)
	}
	for key, want := range map[string]int{"all": 40, "teachers": 7, "students": 30, "admins": 2, "disabled": 1} {
		if out.Facets[key] != want {
			t.Errorf("facets[%s] = %d, want %d", key, out.Facets[key], want)
		}
	}
}

func TestAnEmptyPageOfUsersIsAnEmptyListNotNull(t *testing.T) {
	h := identityhttp.NewIdentity(readApp(&listUsersSpy{}, query.ListUsersResult{Page: paging.Page{Number: 1, Size: 20}}, nil, &getUserSpy{}, query.GetUserResult{}, nil), 0, false, nil)
	var out struct {
		Items *[]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(listUsersAs(t, h, openapi.ListUsersParams{}).body, &out); err != nil {
		t.Fatal(err)
	}
	if out.Items == nil || len(*out.Items) != 0 {
		t.Errorf("items = %v, want []", out.Items)
	}
}

func theSheet() query.GetUserResult {
	account := createdAccount()
	creator := domain.AdminUserRef{ID: uuid.NewString(), FullName: "Trần Văn Quản"}
	signedIn := time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC)
	return query.GetUserResult{
		Detail: domain.AdminUserDetail{
			User:      account,
			CreatedBy: &creator,
			Classes:   []domain.AdminClassRef{{ID: uuid.NewString(), Name: "Lớp 10A"}, {ID: uuid.NewString(), Name: "Lớp 10B"}},
			Owned:     domain.OwnedContent{Classes: 2, Tests: 3, Questions: 40, Media: 5, Assignments: 7},
			Events: []domain.UserEvent{
				{Kind: domain.EventSignIn, At: signedIn, Detail: strPtr("google")},
				{Kind: domain.EventClassJoined, At: signedIn.Add(-time.Hour), Detail: strPtr("Lớp 10A")},
				{Kind: domain.EventAudit, At: signedIn.Add(-2 * time.Hour)},
			},
			InProgressAttempts: 1,
		},
		Can: domain.UserCan{ChangeRole: true, SignOut: true},
	}
}

func TestGetUserAnswersTheSheetAsOneFlatObject(t *testing.T) {
	sheet := theSheet()
	spy := &getUserSpy{}
	caller := anAdmin()
	h := identityhttp.NewIdentity(readApp(&listUsersSpy{}, query.ListUsersResult{}, nil, spy, sheet, nil), 0, false, nil)
	id := uuid.MustParse(sheet.Detail.User.ID)

	got := getUserAs(t, caller, "", h, id)
	if got.err != nil || got.status != http.StatusOK {
		t.Fatalf("answered %d, %v: %s", got.status, got.err, got.body)
	}
	if spy.got.ID != id.String() || spy.got.Request.ActorID != caller.UserID || !spy.got.Request.Grants.Has(access.PeopleUsersManage) {
		t.Errorf("the query = %+v, want the account and the caller's id and grants", spy.got)
	}

	var out struct {
		User       map[string]any                `json:"user"`
		CreatedBy  struct{ ID, FullName string } `json:"createdBy"`
		Classes    []struct{ ID, Name string }   `json:"classes"`
		Owned      map[string]int                `json:"ownedContent"`
		InProgress int                           `json:"inProgressAttempts"`
		Activity   []struct {
			Kind   string  `json:"kind"`
			Detail *string `json:"detail"`
		} `json:"activity"`
		Can map[string]bool `json:"can"`
	}
	if err := json.Unmarshal(got.body, &out); err != nil {
		t.Fatal(err)
	}
	if out.User["id"] != sheet.Detail.User.ID || out.CreatedBy.FullName != "Trần Văn Quản" || len(out.Classes) != 2 || out.Classes[1].Name != "Lớp 10B" || out.InProgress != 1 {
		t.Errorf("sheet = %s", got.body)
	}
	if out.Owned["classes"] != 2 || out.Owned["tests"] != 3 || out.Owned["questions"] != 40 || out.Owned["media"] != 5 || out.Owned["assignments"] != 7 {
		t.Errorf("ownedContent = %v", out.Owned)
	}
	kinds := make([]string, len(out.Activity))
	for i, e := range out.Activity {
		kinds[i] = e.Kind
	}
	if !slices.Equal(kinds, []string{"sign_in", "class_joined", "audit"}) || *out.Activity[0].Detail != "google" || out.Activity[2].Detail != nil {
		t.Errorf("activity = %+v, want the events in the order given, the audit one without a detail", out.Activity)
	}
	if want := map[string]bool{"changeRole": true, "resetPassword": false, "disable": false, "signOut": true, "transfer": false}; !equalFlags(out.Can, want) {
		t.Errorf("can = %v, want %v", out.Can, want)
	}
}

func equalFlags(got, want map[string]bool) bool {
	if len(got) != len(want) {
		return false
	}
	for key, v := range want {
		if g, ok := got[key]; !ok || g != v {
			return false
		}
	}
	return true
}

func TestAnAccountNoOneCreatedHasANullCreator(t *testing.T) {
	sheet := theSheet()
	sheet.Detail.CreatedBy, sheet.Detail.Classes, sheet.Detail.Events = nil, nil, nil
	h := identityhttp.NewIdentity(readApp(&listUsersSpy{}, query.ListUsersResult{}, nil, &getUserSpy{}, sheet, nil), 0, false, nil)

	var out map[string]json.RawMessage
	if err := json.Unmarshal(getUserAs(t, anAdmin(), "", h, uuid.MustParse(sheet.Detail.User.ID)).body, &out); err != nil {
		t.Fatal(err)
	}
	if string(out["createdBy"]) != "null" || string(out["classes"]) != "[]" || string(out["activity"]) != "[]" {
		t.Errorf("createdBy %s, classes %s, activity %s, want null, [] and []", out["createdBy"], out["classes"], out["activity"])
	}
}

func TestAnUnknownAccountIsNotFoundInBothLanguages(t *testing.T) {
	h := identityhttp.NewIdentity(readApp(&listUsersSpy{}, query.ListUsersResult{}, nil, &getUserSpy{}, query.GetUserResult{}, domain.ErrUserNotFound), 0, false, nil)
	for accept, want := range map[string]string{"vi": "Không tìm thấy người dùng.", "en": "The user was not found."} {
		got := getUserAs(t, anAdmin(), accept, h, uuid.New())
		var out struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(got.body, &out); err != nil {
			t.Fatal(err)
		}
		if got.status != http.StatusNotFound || out.Error.Code != "NOT_FOUND" || out.Error.Message != want {
			t.Errorf("%s: %d %s %q, want 404 NOT_FOUND %q", accept, got.status, out.Error.Code, out.Error.Message, want)
		}
	}
}

func TestAnUnexpectedFailureOfTheUsersReadsIsNotTranslated(t *testing.T) {
	boom := errors.New("the database is gone")
	h := identityhttp.NewIdentity(readApp(&listUsersSpy{}, query.ListUsersResult{}, boom, &getUserSpy{}, query.GetUserResult{}, boom), 0, false, nil)
	if got := listUsersAs(t, h, openapi.ListUsersParams{}); !errors.Is(got.err, boom) {
		t.Errorf("list: err = %v, want the failure passed on to the 500 mapper", got.err)
	}
	if got := getUserAs(t, anAdmin(), "", h, uuid.New()); !errors.Is(got.err, boom) {
		t.Errorf("detail: err = %v, want the failure passed on to the 500 mapper", got.err)
	}
}

func TestTheUsersReadsAreNotImplementedWithoutTheirQueries(t *testing.T) {
	for name, h := range map[string]identityhttp.Identity{
		"no application": identityhttp.NewIdentity(nil, 0, false, nil),
		"no queries":     identityhttp.NewIdentity(&application.Application{}, 0, false, nil),
	} {
		t.Run(name, func(t *testing.T) {
			if got := listUsersAs(t, h, openapi.ListUsersParams{}); !errors.Is(got.err, httpx.ErrNotImplemented) {
				t.Errorf("list: err = %v, want httpx.ErrNotImplemented", got.err)
			}
			if got := getUserAs(t, anAdmin(), "", h, uuid.New()); !errors.Is(got.err, httpx.ErrNotImplemented) {
				t.Errorf("detail: err = %v, want httpx.ErrNotImplemented", got.err)
			}
		})
	}
}

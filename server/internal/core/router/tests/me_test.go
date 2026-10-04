package router_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/internal/core/router"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	notificationsapp "quizzivy/internal/modules/notifications/application"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	notificationshttp "quizzivy/internal/modules/notifications/http"
)

type meStore struct {
	mu    sync.Mutex
	calls []string
	users []string
	query notificationsdomain.ListQuery
	ids   []string
	saved []notificationsdomain.Preference
}

func (s *meStore) record(call, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call)
	s.users = append(s.users, userID)
}

func (s *meStore) Upsert(_ context.Context, n notificationsdomain.Notice) (bool, error) {
	s.record("Upsert", n.UserID)
	return true, nil
}

func (s *meStore) List(_ context.Context, q notificationsdomain.ListQuery) (notificationsdomain.Page, error) {
	s.record("List", q.UserID)
	s.query = q
	return notificationsdomain.Page{}, nil
}

func (s *meStore) MarkRead(_ context.Context, userID string, ids []string) error {
	s.record("MarkRead", userID)
	s.ids = ids
	return nil
}

func (s *meStore) MarkAllRead(_ context.Context, userID string) error {
	s.record("MarkAllRead", userID)
	return nil
}

func (s *meStore) Unread(_ context.Context, userID string) (int, error) {
	s.record("Unread", userID)
	return 2, nil
}

func (s *meStore) Preferences(_ context.Context, userID string) ([]notificationsdomain.Preference, error) {
	s.record("Preferences", userID)
	return nil, nil
}

func (s *meStore) SavePreferences(_ context.Context, userID string, prefs []notificationsdomain.Preference) ([]notificationsdomain.Preference, error) {
	s.record("SavePreferences", userID)
	s.saved = prefs
	return prefs, nil
}

func (s *meStore) DeleteBefore(context.Context, time.Time) (int64, error) {
	s.record("DeleteBefore", "")
	return 0, nil
}

func meRouter(t *testing.T, issuer *identitytoken.Issuer, store *meStore) http.Handler {
	t.Helper()
	modules := router.Modules{Notifications: notificationshttp.NewNotifications(notificationsapp.New(store))}
	h, err := router.New(router.Deps{Modules: modules, Principals: rolePrincipals(), DB: fakeDB{}, Tokens: issuer},
		slog.New(slog.NewTextHandler(io.Discard, nil)), []string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func switches(events ...string) string {
	items := make([]map[string]any, len(events))
	for i, event := range events {
		items[i] = map[string]any{"event": event, "inApp": i%2 == 0, "email": false}
	}
	raw, _ := json.Marshal(items)
	return string(raw)
}

func someIDs(n int) string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = uuid.NewString()
	}
	raw, _ := json.Marshal(map[string]any{"ids": ids})
	return string(raw)
}

const outOfOrder = `[{"event":"result.ready","inApp":false,"email":true},{"event":"assignment.due_soon","inApp":true,"email":false},` +
	`{"event":"assignment.closing","inApp":true,"email":false},{"event":"attempt.flagged","inApp":true,"email":false},` +
	`{"event":"attempt.submitted","inApp":true,"email":false}]`

var theFiveEvents = []string{"attempt.submitted", "attempt.flagged", "assignment.closing", "assignment.due_soon", "result.ready"}

var theMeTree = []struct{ method, path, body string }{
	{http.MethodGet, "/me/notifications", ""},
	{http.MethodPost, "/me/notifications/read", `{}`},
	{http.MethodGet, "/me/summary", ""},
	{http.MethodGet, "/me/notification-preferences", ""},
	{http.MethodPut, "/me/notification-preferences", switches(theFiveEvents...)},
}

func TestTheMeTreeRefusesWhatItsContractRefuses(t *testing.T) {
	issuer := testIssuer(t)
	for name, c := range map[string]struct{ method, path, body string }{
		"a limit over fifty":           {http.MethodGet, "/me/notifications?limit=51", ""},
		"a limit of zero":              {http.MethodGet, "/me/notifications?limit=0", ""},
		"a limit that is not a number": {http.MethodGet, "/me/notifications?limit=many", ""},
		"a cursor that is not a uuid":  {http.MethodGet, "/me/notifications?before=yesterday", ""},
		"a hundred and one ids":        {http.MethodPost, "/me/notifications/read", someIDs(101)},
		"an empty list of ids":         {http.MethodPost, "/me/notifications/read", `{"ids":[]}`},
		"an id that is not a uuid":     {http.MethodPost, "/me/notifications/read", `{"ids":["mine"]}`},
		"a user id beside the ids":     {http.MethodPost, "/me/notifications/read", `{"userId":"` + teacherUser + `"}`},
		"no body to mark with":         {http.MethodPost, "/me/notifications/read", ""},
		"four switches":                {http.MethodPut, "/me/notification-preferences", switches(theFiveEvents[:4]...)},
		"six switches":                 {http.MethodPut, "/me/notification-preferences", switches(append(slices.Clone(theFiveEvents), "attempt.flagged")...)},
		"an event that is not a switch": {http.MethodPut, "/me/notification-preferences",
			switches("attempt.submitted", "attempt.flagged", "assignment.closing", "assignment.due_soon", "class.joined")},
		"a switch without its email half": {http.MethodPut, "/me/notification-preferences",
			strings.Replace(switches(theFiveEvents...), `"email":false,`, "", 1)},
		"a switch for somebody else": {http.MethodPut, "/me/notification-preferences",
			strings.Replace(switches(theFiveEvents...), `"email":false,`, `"email":false,"userId":"`+teacherUser+`",`, 1)},
		"switches that are not a list": {http.MethodPut, "/me/notification-preferences", `{"items":` + switches(theFiveEvents...) + `}`},
		"a repeated switch": {http.MethodPut, "/me/notification-preferences",
			switches("attempt.submitted", "attempt.flagged", "assignment.closing", "assignment.due_soon", "attempt.submitted")},
	} {
		t.Run(name, func(t *testing.T) {
			store := &meStore{}
			rec := sendAs(t, meRouter(t, issuer, store), issuer, c.method, c.path, studentUser, c.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("the refusal is cacheable: Cache-Control %q", got)
			}
			if code := errorCode(t, rec); code != "VALIDATION_FAILED" {
				t.Errorf("error code = %q, want VALIDATION_FAILED", code)
			}
			if len(store.calls) != 0 {
				t.Errorf("the store was reached: %v", store.calls)
			}
		})
	}
}

func TestARepeatedSwitchIsRefusedInWordsByTheHandler(t *testing.T) {
	issuer := testIssuer(t)
	body := switches("attempt.submitted", "attempt.flagged", "assignment.closing", "assignment.due_soon", "attempt.submitted")
	rec := sendAs(t, meRouter(t, issuer, &meStore{}), issuer, http.MethodPut, "/me/notification-preferences", teacherUser, body)
	var envelope struct {
		Error struct {
			Message string            `json:"message"`
			Details map[string]string `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	const sentence = "Mỗi loại thông báo phải xuất hiện đúng một lần."
	if rec.Code != http.StatusBadRequest || envelope.Error.Message != sentence || envelope.Error.Details["event"] != sentence {
		t.Errorf("answered %d %+v, want 400 and %q for the event", rec.Code, envelope.Error, sentence)
	}
}

func TestTheMeTreeServesEverySignedInRoleItsOwnRows(t *testing.T) {
	issuer := testIssuer(t)
	cursor := uuid.NewString()
	for _, user := range []string{studentUser, teacherUser, assistantUser, adminUser} {
		for name, c := range map[string]struct {
			method, path, body string
			status             int
			call               string
		}{
			"the first page":        {http.MethodGet, "/me/notifications", "", http.StatusOK, "List"},
			"the largest page":      {http.MethodGet, "/me/notifications?limit=50&before=" + cursor, "", http.StatusOK, "List"},
			"marking a hundred":     {http.MethodPost, "/me/notifications/read", someIDs(100), http.StatusNoContent, "MarkRead"},
			"marking everything":    {http.MethodPost, "/me/notifications/read", `{}`, http.StatusNoContent, "MarkAllRead"},
			"the summary":           {http.MethodGet, "/me/summary", "", http.StatusOK, "Unread"},
			"reading the switches":  {http.MethodGet, "/me/notification-preferences", "", http.StatusOK, "Preferences"},
			"saving the switches":   {http.MethodPut, "/me/notification-preferences", switches(theFiveEvents...), http.StatusOK, "SavePreferences"},
			"switches out of order": {http.MethodPut, "/me/notification-preferences", outOfOrder, http.StatusOK, "SavePreferences"},
		} {
			store := &meStore{}
			rec := sendAs(t, meRouter(t, issuer, store), issuer, c.method, c.path, user, c.body)
			if rec.Code != c.status {
				t.Errorf("%s as %s: %d, want %d: %s", name, user, rec.Code, c.status, rec.Body.String())
				continue
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("%s as %s is cacheable: Cache-Control %q", name, user, got)
			}
			if !slices.Equal(store.calls, []string{c.call}) || !slices.Equal(store.users, []string{user}) {
				t.Errorf("%s as %s reached %v for %v, want %s for the caller alone", name, user, store.calls, store.users, c.call)
			}
			switch name {
			case "the first page":
				if store.query.Size() != 20 || store.query.Before != "" {
					t.Errorf("the first page asked %+v, want twenty from the top", store.query)
				}
			case "the largest page":
				if store.query.Size() != 50 || store.query.Before != cursor {
					t.Errorf("the largest page asked %+v, want fifty before the cursor", store.query)
				}
			case "marking a hundred":
				if len(store.ids) != 100 {
					t.Errorf("%d ids reached the store, want 100", len(store.ids))
				}
			case "switches out of order":
				want := []notificationsdomain.Preference{
					{Event: "attempt.submitted", InApp: true}, {Event: "attempt.flagged", InApp: true}, {Event: "assignment.closing", InApp: true},
					{Event: "assignment.due_soon", InApp: true}, {Event: "result.ready", InApp: false, Email: true},
				}
				if !slices.Equal(store.saved, want) {
					t.Errorf("the store was handed %+v, want the five in the switches' order with their own values", store.saved)
				}
			}
		}
	}
}

func TestTheSummaryAndTheSwitchesAnswerTheContractsBodies(t *testing.T) {
	issuer := testIssuer(t)
	h := meRouter(t, issuer, &meStore{})
	if got := sendAs(t, h, issuer, http.MethodGet, "/me/summary", studentUser, "").Body.String(); got != `{"unreadNotifications":2}`+"\n" {
		t.Errorf("the summary is %s", got)
	}
	var defaults []map[string]any
	if err := json.Unmarshal(sendAs(t, h, issuer, http.MethodGet, "/me/notification-preferences", studentUser, "").Body.Bytes(), &defaults); err != nil {
		t.Fatal(err)
	}
	if len(defaults) != 5 {
		t.Fatalf("%d switches, want five", len(defaults))
	}
	for i, event := range theFiveEvents {
		if defaults[i]["event"] != event || defaults[i]["inApp"] != true || defaults[i]["email"] != false {
			t.Errorf("switch %d is %v, want %s in the app and not by email", i, defaults[i], event)
		}
	}
}

func TestAnAnonymousCallerIsRefusedOnTheWholeMeTree(t *testing.T) {
	issuer := testIssuer(t)
	spec := freshSpec(t)
	served := 0
	for path, item := range spec.Paths.Map() {
		if strings.HasPrefix(path, "/me/") {
			served += len(item.Operations())
		}
	}
	if served != len(theMeTree) {
		t.Fatalf("the contract has %d operations under /me/, this test knows %d", served, len(theMeTree))
	}
	for _, c := range theMeTree {
		store := &meStore{}
		rec := sendAs(t, meRouter(t, issuer, store), issuer, c.method, c.path, "", c.body)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without a token: %d, want 401", c.method, c.path, rec.Code)
			continue
		}
		if code := errorCode(t, rec); code != "UNAUTHORIZED" {
			t.Errorf("%s %s without a token: code %q, want UNAUTHORIZED", c.method, c.path, code)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s %s without a token is cacheable: Cache-Control %q", c.method, c.path, got)
		}
		if len(store.calls) != 0 {
			t.Errorf("%s %s without a token reached the store: %v", c.method, c.path, store.calls)
		}
		forged := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		forged.Header.Set("Authorization", "Bearer not-a-token")
		forged.Header.Set("Content-Type", "application/json")
		refused := httptest.NewRecorder()
		meRouter(t, issuer, store).ServeHTTP(refused, forged)
		if refused.Code != http.StatusUnauthorized || len(store.calls) != 0 {
			t.Errorf("%s %s with a forged token: %d, store %v", c.method, c.path, refused.Code, store.calls)
		}
	}
}

func TestTheMeTreeNamesWhatItsIdsAre(t *testing.T) {
	spec := freshSpec(t)
	want := map[string][]resourceEntry{
		"GET /me/notifications":            {{In: "query", Name: "before", Kind: "none"}},
		"POST /me/notifications/read":      {{In: "body", Name: "/ids/-", Kind: "notification"}},
		"GET /me/summary":                  nil,
		"GET /me/notification-preferences": nil,
		"PUT /me/notification-preferences": nil,
	}
	for pattern, entries := range want {
		method, path, _ := strings.Cut(pattern, " ")
		op := spec.Paths.Find(path).GetOperation(method)
		if got := declaredResources(t, pattern, op); !slices.Equal(got, entries) {
			t.Errorf("%s declares %+v, want %+v", pattern, got, entries)
		}
		if _, lists := op.Extensions["x-resource-list"]; lists {
			t.Errorf("%s declares x-resource-list, which would put a cursor filled at random into the isolation suite's views", pattern)
		}
		if id := strings.ToLower(op.OperationID[:1]) + op.OperationID[1:]; strings.HasPrefix(id, "listMy") {
			t.Errorf("%s is named %s: an id starting listMy puts it into the isolation suite's views", pattern, id)
		}
	}
}

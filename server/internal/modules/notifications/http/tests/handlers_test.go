package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/notifications/application"
	"quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/modules/notifications/application/query"
	"quizzivy/internal/modules/notifications/domain"
	notificationshttp "quizzivy/internal/modules/notifications/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

const (
	firstID    = "01935000-0000-7000-8000-000000000001"
	secondID   = "01935000-0000-7000-8000-000000000002"
	assignment = "01935000-0000-7000-8000-00000000a551"
	attempt    = "01935000-0000-7000-8000-00000000a77e"
)

func responseSchema(t *testing.T, path, method string) *openapi3.Schema {
	t.Helper()
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	return spec.Paths.Find(path).GetOperation(method).Responses.Status(http.StatusOK).Value.Content.Get("application/json").Schema.Value
}

func listing(page domain.Page, asked *query.List) *application.Application {
	return &application.Application{Queries: application.Queries{
		List: cqrs.HandlerFunc[query.List, domain.Page](func(_ context.Context, q query.List) (domain.Page, error) {
			*asked = q
			return page, nil
		}),
	}}
}

func TestTheListIsSentInTheContractsShape(t *testing.T) {
	written := time.Date(2026, 10, 4, 13, 2, 0, 0, time.UTC)
	read := written.Add(5 * time.Minute)
	page := domain.Page{
		Items: []domain.Notification{
			{
				ID: secondID, Kind: domain.AttemptFlagged, CreatedAt: written,
				Params: json.RawMessage(`{"studentName":"Lê Hoàng Nam","title":"Mid-term Reading Mock","focusLost":3}`),
				Target: &domain.Target{Route: domain.RouteAttempt, AssignmentID: assignment, AttemptID: attempt},
			},
			{
				ID: firstID, Kind: domain.ClassJoined, CreatedAt: written.Add(-time.Hour), ReadAt: &read,
				Params: json.RawMessage(`{"studentName":"Nguyễn Gia Bảo","className":"IELTS Foundation A","score":9.5}`),
			},
		},
		NextBefore: firstID,
	}
	var asked query.List
	transport := notificationshttp.NewNotifications(listing(page, &asked))
	before, limit := uuid.MustParse(secondID), 2
	response := answered(t, "", func(ctx context.Context, w http.ResponseWriter) error {
		out, err := transport.ListNotifications(ctx, openapi.ListNotificationsRequestObject{
			Params: openapi.ListNotificationsParams{Before: &before, Limit: &limit},
		})
		if err != nil {
			return err
		}
		return out.VisitListNotificationsResponse(w)
	})
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	if asked != (query.List{UserID: caller, Before: secondID, Limit: 2}) {
		t.Errorf("the query was %+v, want the caller's own page", asked)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if err := responseSchema(t, "/me/notifications", http.MethodGet).VisitJSON(body); err != nil {
		t.Errorf("the contract refuses the answer: %v\n%s", err, response.Body.String())
	}
	want := map[string]any{
		"nextBefore": firstID,
		"items": []any{
			map[string]any{
				"id": secondID, "kind": "attempt.flagged", "createdAt": "2026-10-04T13:02:00Z", "readAt": nil,
				"params": map[string]any{"studentName": "Lê Hoàng Nam", "title": "Mid-term Reading Mock", "focusLost": float64(3)},
				"target": map[string]any{"route": "attempt", "assignmentId": assignment, "attemptId": attempt},
			},
			map[string]any{
				"id": firstID, "kind": "class.joined", "createdAt": "2026-10-04T12:02:00Z", "readAt": "2026-10-04T13:07:00Z",
				"params": map[string]any{"studentName": "Nguyễn Gia Bảo", "className": "IELTS Foundation A"},
				"target": nil,
			},
		},
	}
	got, _ := json.Marshal(body)
	expected, _ := json.Marshal(want)
	if string(got) != string(expected) {
		t.Errorf("the answer is\n  %s\nwant\n  %s", got, expected)
	}
}

func TestTheLastPageNamesNoNextOne(t *testing.T) {
	var asked query.List
	transport := notificationshttp.NewNotifications(listing(domain.Page{}, &asked))
	response := answered(t, "", func(ctx context.Context, w http.ResponseWriter) error {
		out, err := transport.ListNotifications(ctx, openapi.ListNotificationsRequestObject{})
		if err != nil {
			return err
		}
		return out.VisitListNotificationsResponse(w)
	})
	if got := response.Body.String(); got != `{"items":[],"nextBefore":null}`+"\n" {
		t.Errorf("an empty page is %s", got)
	}
	if asked != (query.List{UserID: caller}) {
		t.Errorf("the query was %+v, want no cursor and no size", asked)
	}
}

func TestStoredParamsThatAreNotAnObjectAreAnErrorNotAnAnswer(t *testing.T) {
	var asked query.List
	page := domain.Page{Items: []domain.Notification{{ID: firstID, Kind: domain.ResultReady, Params: json.RawMessage(`"<b>Đề</b>"`)}}}
	transport := notificationshttp.NewNotifications(listing(page, &asked))
	answered(t, "", func(ctx context.Context, _ http.ResponseWriter) error {
		if out, err := transport.ListNotifications(ctx, openapi.ListNotificationsRequestObject{}); err == nil {
			t.Errorf("params that are a bare string were answered: %+v", out)
		}
		return nil
	})
}

func TestMarkingReadNamesTheCallerAndTheIdsOrEverything(t *testing.T) {
	var marked []command.MarkRead
	var all []command.MarkAllRead
	transport := notificationshttp.NewNotifications(&application.Application{Commands: application.Commands{
		MarkRead: cqrs.HandlerFunc[command.MarkRead, cqrs.Nothing](func(_ context.Context, cmd command.MarkRead) (cqrs.Nothing, error) {
			marked = append(marked, cmd)
			return cqrs.Nothing{}, nil
		}),
		MarkAllRead: cqrs.HandlerFunc[command.MarkAllRead, cqrs.Nothing](func(_ context.Context, cmd command.MarkAllRead) (cqrs.Nothing, error) {
			all = append(all, cmd)
			return cqrs.Nothing{}, nil
		}),
	}})
	send := func(body openapi.MarkNotificationsReadJSONRequestBody) int {
		return answered(t, "", func(ctx context.Context, w http.ResponseWriter) error {
			out, err := transport.MarkNotificationsRead(ctx, openapi.MarkNotificationsReadRequestObject{Body: &body})
			if err != nil {
				return err
			}
			return out.VisitMarkNotificationsReadResponse(w)
		}).Code
	}
	ids := []openapi.Uuid{uuid.MustParse(firstID), uuid.MustParse(secondID)}
	if code := send(openapi.MarkNotificationsReadJSONRequestBody{Ids: &ids}); code != http.StatusNoContent {
		t.Errorf("marking two: %d, want 204", code)
	}
	if len(marked) != 1 || marked[0].UserID != caller || !slices.Equal(marked[0].IDs, []string{firstID, secondID}) || len(all) != 0 {
		t.Errorf("marking two ran %+v and %+v", marked, all)
	}
	if code := send(openapi.MarkNotificationsReadJSONRequestBody{}); code != http.StatusNoContent {
		t.Errorf("marking all: %d, want 204", code)
	}
	if len(marked) != 1 || len(all) != 1 || all[0].UserID != caller {
		t.Errorf("marking all ran %+v and %+v", marked, all)
	}
}

func TestTheSummaryAndTheSwitchesAreTheCallersOwn(t *testing.T) {
	var summarised query.Summary
	var read query.Preferences
	var saved command.UpdatePreferences
	stored := domain.WithDefaults([]domain.Preference{{Event: domain.EventAttemptFlagged, InApp: false, Email: true}})
	transport := notificationshttp.NewNotifications(&application.Application{
		Queries: application.Queries{
			Summary: cqrs.HandlerFunc[query.Summary, domain.Summary](func(_ context.Context, q query.Summary) (domain.Summary, error) {
				summarised = q
				return domain.Summary{UnreadNotifications: 4}, nil
			}),
			Preferences: cqrs.HandlerFunc[query.Preferences, []domain.Preference](func(_ context.Context, q query.Preferences) ([]domain.Preference, error) {
				read = q
				return stored, nil
			}),
		},
		Commands: application.Commands{
			UpdatePreferences: cqrs.HandlerFunc[command.UpdatePreferences, []domain.Preference](func(_ context.Context, cmd command.UpdatePreferences) ([]domain.Preference, error) {
				saved = cmd
				return cmd.Preferences, nil
			}),
		},
	})

	summary := answered(t, "", func(ctx context.Context, w http.ResponseWriter) error {
		out, err := transport.GetMySummary(ctx, openapi.GetMySummaryRequestObject{})
		if err != nil {
			return err
		}
		return out.VisitGetMySummaryResponse(w)
	})
	if got := summary.Body.String(); got != `{"unreadNotifications":4}`+"\n" || summarised.UserID != caller {
		t.Errorf("the summary is %s for %q", got, summarised.UserID)
	}

	const switches = `[{"email":false,"event":"attempt.submitted","inApp":true},{"email":true,"event":"attempt.flagged","inApp":false},` +
		`{"email":false,"event":"assignment.closing","inApp":true},{"email":false,"event":"assignment.due_soon","inApp":true},` +
		`{"email":false,"event":"result.ready","inApp":true}]` + "\n"
	preferences := answered(t, "", func(ctx context.Context, w http.ResponseWriter) error {
		out, err := transport.GetNotificationPreferences(ctx, openapi.GetNotificationPreferencesRequestObject{})
		if err != nil {
			return err
		}
		return out.VisitGetNotificationPreferencesResponse(w)
	})
	if got := preferences.Body.String(); got != switches || read.UserID != caller {
		t.Errorf("the switches are %s for %q, want %s", got, read.UserID, switches)
	}
	var decoded []any
	if err := json.Unmarshal(preferences.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if err := responseSchema(t, "/me/notification-preferences", http.MethodGet).VisitJSON(decoded); err != nil {
		t.Errorf("the contract refuses the switches: %v", err)
	}

	var body openapi.UpdateNotificationPreferencesJSONRequestBody
	if err := json.Unmarshal([]byte(switches), &body); err != nil {
		t.Fatal(err)
	}
	updated := answered(t, "", func(ctx context.Context, w http.ResponseWriter) error {
		out, err := transport.UpdateNotificationPreferences(ctx, openapi.UpdateNotificationPreferencesRequestObject{Body: &body})
		if err != nil {
			return err
		}
		return out.VisitUpdateNotificationPreferencesResponse(w)
	})
	if updated.Code != http.StatusOK || updated.Body.String() != switches {
		t.Errorf("saving answered %d %s, want the stored switches", updated.Code, updated.Body.String())
	}
	if saved.UserID != caller || !slices.Equal(saved.Preferences, stored) {
		t.Errorf("the command was %+v, want the caller's five switches as sent", saved)
	}
}

func TestNothingIsServedWithoutACallerOrAModule(t *testing.T) {
	reached := false
	app := &application.Application{Queries: application.Queries{
		Summary: cqrs.HandlerFunc[query.Summary, domain.Summary](func(context.Context, query.Summary) (domain.Summary, error) {
			reached = true
			return domain.Summary{}, nil
		}),
	}}
	ids := []openapi.Uuid{uuid.New()}
	refused := func(ctx context.Context, label string, transport notificationshttp.Notifications) {
		_, list := transport.ListNotifications(ctx, openapi.ListNotificationsRequestObject{})
		_, mark := transport.MarkNotificationsRead(ctx, openapi.MarkNotificationsReadRequestObject{Body: &openapi.MarkNotificationsReadJSONRequestBody{Ids: &ids}})
		_, summary := transport.GetMySummary(ctx, openapi.GetMySummaryRequestObject{})
		_, get := transport.GetNotificationPreferences(ctx, openapi.GetNotificationPreferencesRequestObject{})
		_, put := transport.UpdateNotificationPreferences(ctx, openapi.UpdateNotificationPreferencesRequestObject{Body: &openapi.UpdateNotificationPreferencesJSONRequestBody{}})
		for operation, err := range map[string]error{"list": list, "mark": mark, "summary": summary, "get": get, "put": put} {
			if !errors.Is(err, httpx.ErrNotImplemented) {
				t.Errorf("%s, %s: err = %v, want ErrNotImplemented", label, operation, err)
			}
		}
	}
	refused(context.Background(), "no caller", notificationshttp.NewNotifications(app))
	answered(t, "", func(ctx context.Context, _ http.ResponseWriter) error {
		refused(ctx, "no module", notificationshttp.NewNotifications(nil))
		return nil
	})
	if reached {
		t.Error("a query ran without a caller")
	}
}

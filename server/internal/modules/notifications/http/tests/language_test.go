package http_test

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/notifications/application"
	"quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/modules/notifications/domain"
	notificationshttp "quizzivy/internal/modules/notifications/http"
	"quizzivy/internal/shared/cqrs"
)

func updating(outcome error) serving {
	transport := notificationshttp.NewNotifications(&application.Application{Commands: application.Commands{
		UpdatePreferences: cqrs.HandlerFunc[command.UpdatePreferences, []domain.Preference](func(context.Context, command.UpdatePreferences) ([]domain.Preference, error) {
			return nil, outcome
		}),
	}})
	body := openapi.UpdateNotificationPreferencesJSONRequestBody{}
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.UpdateNotificationPreferences(ctx, openapi.UpdateNotificationPreferencesRequestObject{Body: &body})
		if err != nil {
			return err
		}
		return response.VisitUpdateNotificationPreferencesResponse(w)
	}
}

func marking(outcome error) serving {
	transport := notificationshttp.NewNotifications(&application.Application{Commands: application.Commands{
		MarkRead: cqrs.HandlerFunc[command.MarkRead, cqrs.Nothing](func(context.Context, command.MarkRead) (cqrs.Nothing, error) {
			return cqrs.Nothing{}, outcome
		}),
	}})
	ids := []openapi.Uuid{uuid.New()}
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.MarkNotificationsRead(ctx, openapi.MarkNotificationsReadRequestObject{
			Body: &openapi.MarkNotificationsReadJSONRequestBody{Ids: &ids},
		})
		if err != nil {
			return err
		}
		return response.VisitMarkNotificationsReadResponse(w)
	}
}

func TestNotificationRefusalsSpeakTheCallersLanguage(t *testing.T) {
	for _, c := range []struct {
		name  string
		serve serving
		field string
		vi    string
		en    string
	}{
		{
			name:  "saving a switch twice",
			serve: updating(domain.ErrEventsNotOnceEach),
			field: "event",
			vi:    "Mỗi loại thông báo phải xuất hiện đúng một lần.",
			en:    "Each notification type must appear exactly once.",
		},
		{
			name:  "saving a switch that does not exist",
			serve: updating(domain.ErrUnknownEvent),
			field: "event",
			vi:    "Loại thông báo không hợp lệ.",
			en:    "The notification type is not valid.",
		},
		{
			name:  "marking no notification",
			serve: marking(domain.ErrNoIDs),
			field: "ids",
			vi:    "Cần chọn ít nhất một thông báo.",
			en:    "At least one notification is needed.",
		},
		{
			name:  "marking too many notifications",
			serve: marking(domain.ErrTooManyIDs),
			field: "ids",
			vi:    "Chỉ đánh dấu được tối đa 100 thông báo mỗi lần.",
			en:    "At most 100 notifications can be marked at a time.",
		},
	} {
		for _, language := range []struct{ accept, message string }{{"", c.vi}, {"vi", c.vi}, {"en", c.en}, {"en-GB,en;q=0.9,vi;q=0.5", c.en}} {
			response := answered(t, language.accept, c.serve)
			var body openapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("%s with Accept-Language %q: %v in %s", c.name, language.accept, err, response.Body.String())
			}
			if response.Code != http.StatusBadRequest || body.Error.Code != openapi.VALIDATIONFAILED {
				t.Errorf("%s with Accept-Language %q answered %d %s, want 400 VALIDATION_FAILED", c.name, language.accept, response.Code, body.Error.Code)
			}
			if body.Error.Message != language.message {
				t.Errorf("%s with Accept-Language %q answered %q, want %q", c.name, language.accept, body.Error.Message, language.message)
			}
			var details map[string]interface{}
			if body.Error.Details != nil {
				details = *body.Error.Details
			}
			if want := map[string]interface{}{c.field: language.message}; !maps.Equal(details, want) {
				t.Errorf("%s with Accept-Language %q carries details %v, want %v", c.name, language.accept, details, want)
			}
		}
	}
}

package http_test

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"testing"
	"time"
)

func TestPreferencesTransportUsesSelfFalseAndReturnsMergedStorage(t *testing.T) {
	var got command.UpdatePreferences
	stored := domain.Preferences{Theme: profileHTTPString("dark"), CompactTables: profileHTTPBool(false)}
	app := &application.Application{Commands: application.Commands{UpdatePreferences: cqrs.HandlerFunc[command.UpdatePreferences, domain.Preferences](func(_ context.Context, in command.UpdatePreferences) (domain.Preferences, error) {
		got = in
		return stored, nil
	})}}
	h := identityhttp.NewIdentity(app, time.Hour, false, nil)
	r, err := h.UpdatePreferences(contextAs(t, access.Principal{UserID: profileHTTPUser().ID}), openapi.UpdatePreferencesRequestObject{Body: &openapi.UpdatePreferencesJSONRequestBody{CompactTables: profileHTTPBool(false)}})
	if err != nil {
		t.Fatal(err)
	}
	body, ok := r.(openapi.UpdatePreferences200JSONResponse)
	if !ok || body.Theme == nil || string(*body.Theme) != "dark" || body.CompactTables == nil || *body.CompactTables {
		t.Fatalf("merged=%+v", r)
	}
	if got.UserID != profileHTTPUser().ID || got.Preferences.CompactTables == nil || *got.Preferences.CompactTables || got.Preferences.Theme != nil {
		t.Fatalf("patch=%+v", got)
	}
}

func TestPreferencesTransportMapsNamedCapAndDisabledErrors(t *testing.T) {
	for _, failure := range []error{domain.ErrPreferencesTooLarge, domain.ErrPreferencesInvalid, domain.ErrAccountDisabled, domain.ErrUserNotFound} {
		app := &application.Application{Commands: application.Commands{UpdatePreferences: cqrs.HandlerFunc[command.UpdatePreferences, domain.Preferences](func(context.Context, command.UpdatePreferences) (domain.Preferences, error) {
			return domain.Preferences{}, failure
		})}}
		h := identityhttp.NewIdentity(app, time.Hour, false, nil)
		r, err := h.UpdatePreferences(contextAs(t, access.Principal{UserID: profileHTTPUser().ID}), openapi.UpdatePreferencesRequestObject{Body: &openapi.UpdatePreferencesJSONRequestBody{}})
		if err != nil {
			t.Fatal(err)
		}
		switch failure {
		case domain.ErrPreferencesTooLarge, domain.ErrPreferencesInvalid:
			if _, ok := r.(openapi.UpdatePreferences400JSONResponse); !ok {
				t.Fatalf("cap/shape=%T", r)
			}
		default:
			if _, ok := r.(openapi.UpdatePreferences401JSONResponse); !ok {
				t.Fatalf("disabled/missing=%T", r)
			}
		}
	}
	h := identityhttp.NewIdentity(nil, time.Hour, false, nil)
	if _, err := h.UpdatePreferences(context.Background(), openapi.UpdatePreferencesRequestObject{}); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Fatal(err)
	}
}

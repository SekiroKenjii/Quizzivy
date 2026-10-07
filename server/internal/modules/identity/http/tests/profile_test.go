package http_test

import (
	"context"
	"encoding/json"
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

func TestProfileTransportPreservesNullablePresenceAndSelfActor(t *testing.T) {
	for _, body := range []openapi.UpdateCurrentUserJSONRequestBody{{FullName: profileHTTPString("Name")}, {DisplayName: json.RawMessage(`null`)}, {Phone: json.RawMessage(`null`)}, {DisplayName: json.RawMessage(`"Public"`), Phone: json.RawMessage(`"123456"`)}} {
		user := profileHTTPUser()
		var got command.UpdateProfile
		app := &application.Application{Commands: application.Commands{UpdateProfile: cqrs.HandlerFunc[command.UpdateProfile, domain.User](func(_ context.Context, in command.UpdateProfile) (domain.User, error) { got = in; return user, nil })}}
		h := identityhttp.NewIdentity(app, time.Hour, false, nil)
		r, err := h.UpdateCurrentUser(contextAs(t, access.Principal{UserID: user.ID}), openapi.UpdateCurrentUserRequestObject{Body: &body})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := r.(openapi.UpdateCurrentUser200JSONResponse); !ok {
			t.Fatalf("response=%T", r)
		}
		if got.UserID != user.ID || got.Patch.DisplayNameSet != (len(body.DisplayName) > 0) || got.Patch.PhoneSet != (len(body.Phone) > 0) {
			t.Fatalf("patch=%+v", got)
		}
		if string(body.DisplayName) == "null" && got.Patch.DisplayName != nil || string(body.Phone) == "null" && got.Patch.Phone != nil {
			t.Fatal("clear became string/default")
		}
	}
}

func TestProfileTransportNamesSemanticZoneErrorAndKeepsDisabledUnauthorized(t *testing.T) {
	for _, failure := range []error{domain.ErrTimeZoneInvalid, domain.ErrAccountDisabled, domain.ErrUserNotFound} {
		app := &application.Application{Commands: application.Commands{UpdateProfile: cqrs.HandlerFunc[command.UpdateProfile, domain.User](func(context.Context, command.UpdateProfile) (domain.User, error) { return domain.User{}, failure })}}
		h := identityhttp.NewIdentity(app, time.Hour, false, nil)
		r, err := h.UpdateCurrentUser(contextAs(t, access.Principal{UserID: profileHTTPUser().ID}), openapi.UpdateCurrentUserRequestObject{Body: &openapi.UpdateCurrentUserJSONRequestBody{TimeZone: profileHTTPString("Mars")}})
		if err != nil {
			t.Fatal(err)
		}
		if errors.Is(failure, domain.ErrTimeZoneInvalid) {
			bad, ok := r.(openapi.UpdateCurrentUser400JSONResponse)
			if !ok || bad.Error.Details == nil || (*bad.Error.Details)["timeZone"] == nil {
				t.Fatalf("response=%+v", r)
			}
		} else {
			if _, ok := r.(openapi.UpdateCurrentUser401JSONResponse); !ok {
				t.Fatalf("disabled/missing=%T", r)
			}
		}
	}
	h := identityhttp.NewIdentity(nil, time.Hour, false, nil)
	if _, err := h.UpdateCurrentUser(context.Background(), openapi.UpdateCurrentUserRequestObject{}); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Fatal(err)
	}
}

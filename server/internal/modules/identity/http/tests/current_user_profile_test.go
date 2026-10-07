package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"testing"
	"time"
)

func profileHTTPString(s string) *string { return &s }
func profileHTTPBool(b bool) *bool       { return &b }
func profileHTTPUser() domain.User {
	return domain.User{ID: "01935000-0000-7000-8000-0000000000a1", Email: "profile@example.com", FullName: "Private name", Role: "student", DisplayName: profileHTTPString("Public name"), Phone: profileHTTPString("+84 123456"), Locale: profileHTTPString("en"), TimeZone: profileHTTPString("UTC"), Preferences: domain.Preferences{Theme: profileHTTPString("dark"), CompactTables: profileHTTPBool(false)}, CreatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
}

func TestEverySuccessfulCurrentUserWirePathCarriesOnlyCallerProfile(t *testing.T) {
	user := profileHTTPUser()
	permissions := access.NewSet(access.LearningTakeTests)
	app := currentUserApp(user)
	session := model.Session{User: user, Permissions: permissions, AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 900}
	app.Commands = application.Commands{
		Login: cqrs.HandlerFunc[command.Login, model.Session](func(context.Context, command.Login) (model.Session, error) { return session, nil }),
		GoogleSignIn: cqrs.HandlerFunc[command.GoogleSignIn, model.GoogleSignInResult](func(context.Context, command.GoogleSignIn) (model.GoogleSignInResult, error) {
			return model.GoogleSignInResult{Session: session}, nil
		}),
		LinkGoogle:    cqrs.HandlerFunc[command.LinkGoogle, domain.User](func(context.Context, command.LinkGoogle) (domain.User, error) { return user, nil }),
		UpdateProfile: cqrs.HandlerFunc[command.UpdateProfile, domain.User](func(context.Context, command.UpdateProfile) (domain.User, error) { return user, nil }),
		Refresh: cqrs.HandlerFunc[command.Refresh, model.RefreshResult](func(context.Context, command.Refresh) (model.RefreshResult, error) {
			return model.RefreshResult{User: user, AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 900}, nil
		}),
	}
	h := identityhttp.NewIdentity(app, time.Hour, false, nil)
	ctx := contextAs(t, access.Principal{UserID: user.ID, Permissions: permissions})
	writes := map[string]func(http.ResponseWriter) error{
		"login": func(w http.ResponseWriter) error {
			r, e := h.Login(ctx, openapi.LoginRequestObject{Body: &openapi.LoginJSONRequestBody{Email: "profile@example.com", Password: "password"}})
			if e != nil {
				return e
			}
			return r.VisitLoginResponse(w)
		},
		"google": func(w http.ResponseWriter) error {
			r, e := h.GoogleAuth(ctx, openapi.GoogleAuthRequestObject{Body: &openapi.GoogleAuthJSONRequestBody{Code: "code", CodeVerifier: "verifier", RedirectUri: "https://example.com"}})
			if e != nil {
				return e
			}
			return r.VisitGoogleAuthResponse(w)
		},
		"get": func(w http.ResponseWriter) error {
			r, e := h.GetCurrentUser(ctx, openapi.GetCurrentUserRequestObject{})
			if e != nil {
				return e
			}
			return r.VisitGetCurrentUserResponse(w)
		},
		"patch": func(w http.ResponseWriter) error {
			r, e := h.UpdateCurrentUser(ctx, openapi.UpdateCurrentUserRequestObject{Body: &openapi.UpdateCurrentUserJSONRequestBody{FullName: profileHTTPString("Private name")}})
			if e != nil {
				return e
			}
			return r.VisitUpdateCurrentUserResponse(w)
		},
		"link": func(w http.ResponseWriter) error {
			r, e := h.LinkGoogle(ctx, openapi.LinkGoogleRequestObject{Body: &openapi.LinkGoogleJSONRequestBody{Code: "code", CodeVerifier: "verifier", RedirectUri: "https://example.com"}})
			if e != nil {
				return e
			}
			return r.VisitLinkGoogleResponse(w)
		},
	}
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			if err := write(rec); err != nil {
				t.Fatal(err)
			}
			if rec.Code != 200 {
				t.Fatal(rec.Code)
			}
			var body map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if raw, ok := body["user"]; ok {
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatal(err)
				}
			}
			for key, want := range map[string]string{"displayName": `"Public name"`, "phone": `"+84 123456"`, "locale": `"en"`, "timeZone": `"UTC"`} {
				if string(body[key]) != want {
					t.Fatalf("%s=%s", key, body[key])
				}
			}
			if _, ok := body["preferences"]; !ok {
				t.Fatal("stored preferences absent")
			}
			for _, key := range []string{"avatar_key", "avatarKey", "avatarUrl", "passwordHash"} {
				if _, ok := body[key]; ok {
					t.Fatalf("unimplemented/private storage property %s", key)
				}
			}
		})
	}
	rec := httptest.NewRecorder()
	r, err := h.RefreshSession(ctx, openapi.RefreshSessionRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.VisitRefreshSessionResponse(rec); err != nil {
		t.Fatal(err)
	}
	var refresh map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &refresh); err != nil {
		t.Fatal(err)
	}
	if len(refresh) != 2 || refresh["accessToken"] == nil || refresh["expiresIn"] == nil {
		t.Fatalf("refresh wire expanded=%s", rec.Body.String())
	}
}

func TestCurrentUserUnsetOptionalProfileRemainsAbsentAndPreferencesRemainEmpty(t *testing.T) {
	user := profileHTTPUser()
	user.DisplayName = nil
	user.Phone = nil
	user.Locale = nil
	user.TimeZone = nil
	user.Preferences = domain.Preferences{}
	h := identityhttp.NewIdentity(currentUserApp(user), time.Hour, false, nil)
	r, err := h.GetCurrentUser(contextAs(t, access.Principal{UserID: user.ID}), openapi.GetCurrentUserRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := r.VisitGetCurrentUserResponse(rec); err != nil {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"displayName", "phone", "locale", "timeZone", "avatarUrl"} {
		if _, ok := body[key]; ok {
			t.Fatalf("unset %s=%s", key, body[key])
		}
	}
	if string(body["preferences"]) != "{}" {
		t.Fatalf("defaults materialized=%s", body["preferences"])
	}
}

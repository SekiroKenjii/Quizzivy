package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"strings"
	"testing"
	"time"
)

const avatarUserID = "01935000-0000-7000-8000-0000000000a1"

func avatarUser(key *string) domain.User {
	return domain.User{ID: avatarUserID, Email: "photo@example.com", FullName: "Có ảnh", Role: "student", AvatarKey: key, CreatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
}

func upload(t *testing.T, parts ...[2]string) *multipart.Reader {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for _, part := range parts {
		if part[0] == "file" {
			f, err := w.CreateFormFile("file", "chan-dung.png")
			if err != nil {
				t.Fatal(err)
			}
			_, _ = f.Write([]byte(part[1]))
			continue
		}
		if err := w.WriteField(part[0], part[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return multipart.NewReader(&body, w.Boundary())
}

type avatarCalls struct {
	set     []command.SetAvatar
	read    string
	removed []command.RemoveAvatar
	signed  []*string
}

func avatarApp(calls *avatarCalls, setErr error, signed string, signErr error) *application.Application {
	key := "avatars/" + avatarUserID + "/now.png"
	return &application.Application{
		Commands: application.Commands{
			SetAvatar: cqrs.HandlerFunc[command.SetAvatar, domain.User](func(_ context.Context, c command.SetAvatar) (domain.User, error) {
				data, _ := io.ReadAll(c.Body)
				calls.set, calls.read = append(calls.set, c), string(data)
				if setErr != nil {
					return domain.User{}, setErr
				}
				return avatarUser(&key), nil
			}),
			RemoveAvatar: cqrs.HandlerFunc[command.RemoveAvatar, domain.User](func(_ context.Context, c command.RemoveAvatar) (domain.User, error) {
				calls.removed = append(calls.removed, c)
				return avatarUser(nil), setErr
			}),
		},
		Queries: application.Queries{
			AvatarURL: cqrs.HandlerFunc[query.AvatarURL, string](func(_ context.Context, q query.AvatarURL) (string, error) {
				calls.signed = append(calls.signed, q.Key)
				return signed, signErr
			}),
			CurrentUser: cqrs.HandlerFunc[query.CurrentUser, domain.User](func(context.Context, query.CurrentUser) (domain.User, error) {
				return avatarUser(&key), nil
			}),
		},
	}
}

func avatarCtx(t *testing.T) context.Context {
	return contextAs(t, access.Principal{UserID: avatarUserID, Permissions: access.NewSet(access.LearningTakeTests)})
}

func envelope(t *testing.T, rec *httptest.ResponseRecorder) (string, string) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%v: %s", err, rec.Body.String())
	}
	return body.Error.Code, body.Error.Message
}

func TestSettingAPhotoAnswersTheWholeCallerWithASignedUrlAndNoKeyField(t *testing.T) {
	calls := &avatarCalls{}
	h := identityhttp.NewIdentity(avatarApp(calls, nil, "https://objects.example/signed?X-Amz-Expires=86400", nil), time.Hour, false, nil)

	out, err := h.SetAvatar(avatarCtx(t), openapi.SetAvatarRequestObject{Body: upload(t, [2]string{"note", "ignored"}, [2]string{"file", "the-image"})})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := out.VisitSetAvatarResponse(rec); err != nil {
		t.Fatal(err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["avatarUrl"] != "https://objects.example/signed?X-Amz-Expires=86400" || body["email"] != "photo@example.com" || body["permissions"] == nil {
		t.Errorf("the body is %v, want the whole caller with the signed url", body)
	}
	for _, key := range []string{"avatarKey", "avatar_key", "passwordHash"} {
		if _, present := body[key]; present {
			t.Errorf("the body carries %s: %s", key, rec.Body)
		}
	}
	if len(calls.set) != 1 || calls.set[0].UserID != avatarUserID || calls.read != "the-image" {
		t.Errorf("the command was %+v and read %q, want the caller's file part and nothing else", calls.set, calls.read)
	}
}

func TestEachRefusedPhotoAnswersItsOwnStatusAndCode(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"too large", domain.ErrAvatarTooLarge, 413, "MEDIA_TOO_LARGE"},
		{"too large by the request limit", &http.MaxBytesError{Limit: 2 << 20}, 413, "MEDIA_TOO_LARGE"},
		{"not a png or a jpeg", domain.ErrAvatarUnsupported, 415, "MEDIA_TYPE_UNSUPPORTED"},
		{"unreadable", domain.ErrAvatarUnreadable, 415, "MEDIA_UNREADABLE"},
		{"out of range", domain.ErrAvatarDimensions, 415, "IMAGE_DIMENSIONS"},
		{"a disabled account", domain.ErrAccountDisabled, 401, "UNAUTHORIZED"},
	}
	for _, c := range cases {
		h := identityhttp.NewIdentity(avatarApp(&avatarCalls{}, c.err, "", nil), time.Hour, false, nil)
		out, err := h.SetAvatar(avatarCtx(t), openapi.SetAvatarRequestObject{Body: upload(t, [2]string{"file", "x"})})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		rec := httptest.NewRecorder()
		if err := out.VisitSetAvatarResponse(rec); err != nil {
			t.Fatal(err)
		}
		if code, message := envelope(t, rec); rec.Code != c.status || code != c.code || message == "" {
			t.Errorf("%s answered %d %s %q, want %d %s", c.name, rec.Code, code, message, c.status, c.code)
		}
	}
}

func TestADeploymentWithoutObjectStorageAnswersNotImplementedAndAnythingElseFailsAsItIs(t *testing.T) {
	unavailable := identityhttp.NewIdentity(avatarApp(&avatarCalls{}, domain.ErrAvatarsUnavailable, "", nil), time.Hour, false, nil)
	if _, err := unavailable.SetAvatar(avatarCtx(t), openapi.SetAvatarRequestObject{Body: upload(t, [2]string{"file", "x"})}); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Errorf("answered %v, want ErrNotImplemented", err)
	}
	boom := errors.New("bucket down")
	broken := identityhttp.NewIdentity(avatarApp(&avatarCalls{}, boom, "", nil), time.Hour, false, nil)
	if _, err := broken.SetAvatar(avatarCtx(t), openapi.SetAvatarRequestObject{Body: upload(t, [2]string{"file", "x"})}); !errors.Is(err, boom) {
		t.Errorf("answered %v, want the command's error", err)
	}
}

func TestARequestWithNoFilePartAnswers400OnTheFileField(t *testing.T) {
	calls := &avatarCalls{}
	h := identityhttp.NewIdentity(avatarApp(calls, nil, "", nil), time.Hour, false, nil)

	out, err := h.SetAvatar(avatarCtx(t), openapi.SetAvatarRequestObject{Body: upload(t, [2]string{"note", "only text"})})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := out.VisitSetAvatarResponse(rec); err != nil {
		t.Fatal(err)
	}

	if code, _ := envelope(t, rec); rec.Code != 400 || code != "VALIDATION_FAILED" || !strings.Contains(rec.Body.String(), `"file"`) {
		t.Errorf("answered %d %s: %s", rec.Code, code, rec.Body)
	}
	if len(calls.set) != 0 {
		t.Error("the command ran without a file")
	}
}

func TestRemovingAPhotoAnswersTheWholeCallerWithoutAnUrl(t *testing.T) {
	calls := &avatarCalls{}
	h := identityhttp.NewIdentity(avatarApp(calls, nil, "https://objects.example/never", nil), time.Hour, false, nil)

	out, err := h.DeleteAvatar(avatarCtx(t), openapi.DeleteAvatarRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := out.VisitDeleteAvatarResponse(rec); err != nil {
		t.Fatal(err)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || body["email"] != "photo@example.com" {
		t.Fatalf("answered %d: %s", rec.Code, rec.Body)
	}
	if _, present := body["avatarUrl"]; present || len(calls.removed) != 1 || calls.removed[0].UserID != avatarUserID {
		t.Errorf("body %s after %+v, want no avatarUrl and one removal for the caller", rec.Body, calls.removed)
	}
}

func TestBothOperationsAnswer401WhenNoCallerIsResolved(t *testing.T) {
	h := identityhttp.NewIdentity(avatarApp(&avatarCalls{}, nil, "", nil), time.Hour, false, nil)

	set, err := h.SetAvatar(context.Background(), openapi.SetAvatarRequestObject{Body: upload(t, [2]string{"file", "x"})})
	if err != nil {
		t.Fatal(err)
	}
	remove, err := h.DeleteAvatar(context.Background(), openapi.DeleteAvatarRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	for name, visit := range map[string]func(http.ResponseWriter) error{"set": set.VisitSetAvatarResponse, "delete": remove.VisitDeleteAvatarResponse} {
		rec := httptest.NewRecorder()
		if err := visit(rec); err != nil {
			t.Fatal(err)
		}
		if rec.Code != 401 {
			t.Errorf("%s answered %d", name, rec.Code)
		}
	}
}

func TestAUserWithAPhotoSeesItsSignedUrlOnGetAndALostSigningLeavesItOutInsteadOfFailing(t *testing.T) {
	calls := &avatarCalls{}
	h := identityhttp.NewIdentity(avatarApp(calls, nil, "https://objects.example/photo", nil), time.Hour, false, nil)
	out, err := h.GetCurrentUser(avatarCtx(t), openapi.GetCurrentUserRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	if err := out.VisitGetCurrentUserResponse(rec); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec.Body.String(), `"avatarUrl":"https://objects.example/photo"`) || len(calls.signed) != 1 || calls.signed[0] == nil {
		t.Errorf("get answered %s after signing %v", rec.Body, calls.signed)
	}

	failing := identityhttp.NewIdentity(avatarApp(&avatarCalls{}, nil, "", errors.New("signer down")), time.Hour, false, nil)
	out, err = failing.GetCurrentUser(avatarCtx(t), openapi.GetCurrentUserRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	if err := out.VisitGetCurrentUserResponse(rec); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "avatarUrl") {
		t.Errorf("with the signer down, get answered %d: %s", rec.Code, rec.Body)
	}
}

func TestEverySuccessfulCurrentUserWirePathCarriesTheSignedUrlOfAUserWithAPhoto(t *testing.T) {
	key := "avatars/" + avatarUserID + "/now.png"
	user := avatarUser(&key)
	permissions := access.NewSet(access.LearningTakeTests)
	session := model.Session{User: user, Permissions: permissions, AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 900}
	app := avatarApp(&avatarCalls{}, nil, "https://objects.example/photo", nil)
	app.Commands.Login = cqrs.HandlerFunc[command.Login, model.Session](func(context.Context, command.Login) (model.Session, error) { return session, nil })
	app.Commands.GoogleSignIn = cqrs.HandlerFunc[command.GoogleSignIn, model.GoogleSignInResult](func(context.Context, command.GoogleSignIn) (model.GoogleSignInResult, error) {
		return model.GoogleSignInResult{Session: session}, nil
	})
	app.Commands.LinkGoogle = cqrs.HandlerFunc[command.LinkGoogle, domain.User](func(context.Context, command.LinkGoogle) (domain.User, error) { return user, nil })
	app.Commands.UpdateProfile = cqrs.HandlerFunc[command.UpdateProfile, domain.User](func(context.Context, command.UpdateProfile) (domain.User, error) { return user, nil })
	h := identityhttp.NewIdentity(app, time.Hour, false, nil)
	ctx := contextAs(t, access.Principal{UserID: user.ID, Permissions: permissions})
	name := "Có ảnh"
	paths := map[string]func(http.ResponseWriter) error{
		"login": func(w http.ResponseWriter) error {
			r, e := h.Login(ctx, openapi.LoginRequestObject{Body: &openapi.LoginJSONRequestBody{Email: "photo@example.com", Password: "password"}})
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
			r, e := h.UpdateCurrentUser(ctx, openapi.UpdateCurrentUserRequestObject{Body: &openapi.UpdateCurrentUserJSONRequestBody{FullName: &name}})
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
	for path, write := range paths {
		rec := httptest.NewRecorder()
		if err := write(rec); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"avatarUrl":"https://objects.example/photo"`) {
			t.Errorf("%s answered %d without the signed url: %s", path, rec.Code, rec.Body)
		}
	}
}

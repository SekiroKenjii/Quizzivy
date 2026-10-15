package http_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/organization/application"
	"quizzivy/internal/modules/organization/application/ports"
	"quizzivy/internal/modules/organization/domain"
	organizationhttp "quizzivy/internal/modules/organization/http"
	"quizzivy/internal/platform/httpx"
)

type repo struct {
	mu       sync.Mutex
	settings domain.Settings
	impact   domain.Impact
}

func (r *repo) Read(context.Context) (domain.Settings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.settings, nil
}

func (r *repo) Save(_ context.Context, in domain.Save) (domain.Settings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if in.ExpectedRevision != r.settings.Revision {
		return domain.Settings{}, domain.ErrStale
	}
	if r.impact.Lockout() {
		return domain.Settings{}, &domain.LockedOut{Impact: r.impact}
	}
	saved := in.Settings
	saved.Revision = r.settings.Revision + 1
	saved.UpdatedAt = in.Now
	r.settings = saved
	return saved, nil
}

func (r *repo) Impact(context.Context, domain.SignIn, string, bool) (domain.Impact, error) {
	return r.impact, nil
}

type store struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (s *store) Put(_ context.Context, key string, _ domain.LogoType, body, _ []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = body
	return nil
}

func (s *store) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	body, ok := s.objects[key]
	if !ok {
		return nil, domain.ErrNoLogo
	}
	return body, nil
}

func (s *store) Exists(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.objects[key]
	return ok, nil
}

func (s *store) List(context.Context, string, int) ([]ports.StoredLogo, error) { return nil, nil }

func (s *store) Delete(context.Context, string) error { return nil }

type checker struct {
	file domain.LogoFile
	err  error
}

func (c checker) Check(_ context.Context, body io.Reader) (domain.LogoFile, error) {
	if _, err := io.ReadAll(body); err != nil {
		return domain.LogoFile{}, err
	}
	return c.file, c.err
}

var now = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func stored() domain.Settings {
	return domain.Settings{
		Profile:   domain.Profile{Name: "Quizzivy", ContactEmail: "hello@quizzivy.com"},
		SignIn:    domain.SignIn{Google: true, Password: true, ForcePasswordChange: true, JoinCodes: true, AllowedGoogleDomains: []string{}},
		Retention: domain.Retention{KeepFocusEventsMonths: 13},
		Revision:  1,
		UpdatedAt: now,
	}
}

func handler(t *testing.T, r *repo, s *store, c checker) organizationhttp.Organization {
	t.Helper()
	app := application.New(r, slog.New(slog.DiscardHandler), true)
	app.SetClock(func() time.Time { return now })
	if s != nil {
		app.WithLogos(s, c)
	}
	return organizationhttp.NewOrganization(app)
}

func contextAs(t *testing.T, acceptLanguage string) context.Context {
	t.Helper()
	var ctx context.Context
	verify := func(string) (httpx.Principal, error) {
		return httpx.Principal{UserID: "019535d9-3df7-79fb-b466-fa907fa17f9e"}, nil
	}
	h := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, verify)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	req := httptest.NewRequest(http.MethodPut, "/admin/settings", nil)
	req.Header.Set("Authorization", "Bearer token")
	if acceptLanguage != "" {
		req.Header.Set("Accept-Language", acceptLanguage)
	}
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ctx == nil {
		t.Fatal("the request did not reach the handler")
	}
	return ctx
}

func input(revision int64) openapi.OrganizationSettingsInput {
	var in openapi.OrganizationSettingsInput
	in.ExpectedRevision = revision
	in.Organization.Name = " Trung tâm Quizzivy "
	in.SignIn = openapi.SignInPolicy{Google: true, Password: true, ForcePasswordChange: true, JoinCodes: true, AllowedGoogleDomains: []string{"Quizzivy.com"}}
	in.Retention.KeepFocusEventsMonths = openapi.RetentionPolicyKeepFocusEventsMonthsN13
	return in
}

func TestThePublicOrganizationHoldsExactlyWhatTheFrontDoorNeeds(t *testing.T) {
	s := stored()
	s.Profile.FrontDeskPhone = "028 1234 5678"
	s.Profile.LightLogo = &domain.Logo{SHA256: bytes.Repeat([]byte{0xab}, 32), Type: domain.LogoPNG}
	s.SignIn.AllowedGoogleDomains = []string{"corp.example"}
	s.SignIn.ForcePasswordChange = false
	s.Retention.KeepAnswersMonths = new(12)
	out, err := handler(t, &repo{settings: s}, nil, checker{}).GetPublicOrganization(context.Background(), openapi.GetPublicOrganizationRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	if err := out.VisitGetPublicOrganizationResponse(response); err != nil {
		t.Fatal(err)
	}
	if got := response.Header().Get("Cache-Control"); got != "public, max-age=300" {
		t.Errorf("Cache-Control %q, want public, max-age=300", got)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(body))
	for k := range body {
		keys = append(keys, k)
	}
	want := map[string]bool{"name": true, "shortName": true, "website": true, "contactEmail": true, "frontDeskPhone": true,
		"frontDeskHours": true, "address": true, "logo": true, "signIn": true, "joinCodes": true}
	if len(keys) != len(want) {
		t.Errorf("keys %v, want exactly %v", keys, want)
	}
	for _, k := range keys {
		if !want[k] {
			t.Errorf("the public document names %q", k)
		}
	}
	signIn, _ := body["signIn"].(map[string]any)
	if len(signIn) != 2 {
		t.Errorf("signIn %v, want google and password only", signIn)
	}
	for _, leak := range []string{"allowedGoogleDomains", "corp.example", "keepAnswersMonths", "revision", "updatedBy", "forcePasswordChange"} {
		if strings.Contains(response.Body.String(), leak) {
			t.Errorf("the public document leaks %q: %s", leak, response.Body.String())
		}
	}
	logo, _ := body["logo"].(map[string]any)
	if logo["lightUrl"] != "/public/organization/logo?v="+strings.Repeat("ab", 32)+"&variant=light" || logo["darkUrl"] != nil {
		t.Errorf("logo %v", logo)
	}
}

func TestAnAbsentOptionalFieldIsNull(t *testing.T) {
	out, err := handler(t, &repo{settings: stored()}, nil, checker{}).GetPublicOrganization(context.Background(), openapi.GetPublicOrganizationRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	if err := out.VisitGetPublicOrganizationResponse(response); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"shortName", "website", "frontDeskPhone", "frontDeskHours", "address"} {
		if v, ok := body[k]; !ok || v != nil {
			t.Errorf("%s = %v (present %v), want null", k, v, ok)
		}
	}
}

func TestWithoutTheModuleEveryOperationIsNotImplemented(t *testing.T) {
	h := organizationhttp.NewOrganization(nil)
	ctx := context.Background()
	if _, err := h.GetPublicOrganization(ctx, openapi.GetPublicOrganizationRequestObject{}); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Errorf("public read: %v", err)
	}
	if _, err := h.GetOrganizationLogo(ctx, openapi.GetOrganizationLogoRequestObject{}); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Errorf("logo: %v", err)
	}
	if _, err := h.GetOrganizationSettings(ctx, openapi.GetOrganizationSettingsRequestObject{}); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Errorf("settings: %v", err)
	}
}

func TestASaveIsNormalisedAndAnswersTheStoredDocument(t *testing.T) {
	r := &repo{settings: stored(), impact: domain.Impact{ActiveAdmins: 2}}
	in := input(1)
	out, err := handler(t, r, nil, checker{}).UpdateOrganizationSettings(contextAs(t, ""), openapi.UpdateOrganizationSettingsRequestObject{Body: &in})
	if err != nil {
		t.Fatal(err)
	}
	saved, ok := out.(openapi.UpdateOrganizationSettings200JSONResponse)
	if !ok {
		t.Fatalf("answered %T", out)
	}
	if saved.Organization.Name != "Trung tâm Quizzivy" || saved.Revision != 2 {
		t.Errorf("saved %q at revision %d", saved.Organization.Name, saved.Revision)
	}
	if got := saved.SignIn.AllowedGoogleDomains; len(got) != 1 || got[0] != "quizzivy.com" {
		t.Errorf("domains %v, want lower-cased", got)
	}
}

func TestASaveOfAnOldRevisionAnswersStaleWrite(t *testing.T) {
	r := &repo{settings: stored(), impact: domain.Impact{ActiveAdmins: 2}}
	in := input(7)
	out, err := handler(t, r, nil, checker{}).UpdateOrganizationSettings(contextAs(t, ""), openapi.UpdateOrganizationSettingsRequestObject{Body: &in})
	if err != nil {
		t.Fatal(err)
	}
	conflict, ok := out.(openapi.UpdateOrganizationSettings409JSONResponse)
	if !ok {
		t.Fatalf("answered %T", out)
	}
	if conflict.Error.Code != openapi.STALEWRITE {
		t.Errorf("code %q", conflict.Error.Code)
	}
}

func TestAnInvalidDocumentNamesEveryFieldInTheCallersLanguage(t *testing.T) {
	r := &repo{settings: stored(), impact: domain.Impact{ActiveAdmins: 2}}
	in := input(1)
	in.Organization.Name = "  "
	in.SignIn.AllowedGoogleDomains = []string{"not a domain"}
	out, err := handler(t, r, nil, checker{}).UpdateOrganizationSettings(contextAs(t, "en"), openapi.UpdateOrganizationSettingsRequestObject{Body: &in})
	if err != nil {
		t.Fatal(err)
	}
	invalid, ok := out.(openapi.UpdateOrganizationSettings400JSONResponse)
	if !ok {
		t.Fatalf("answered %T", out)
	}
	raw, err := json.Marshal(invalid)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"organization.name", "Cannot be empty.", "signIn.allowedGoogleDomains", "Not a valid domain."} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the refusal lacks %q: %s", want, raw)
		}
	}
}

func TestALockoutAnswers422WithTheCounts(t *testing.T) {
	r := &repo{settings: stored(), impact: domain.Impact{AffectedUsers: 5, AffectedAdmins: 1, ActiveAdmins: 1}}
	in := input(1)
	out, err := handler(t, r, nil, checker{}).UpdateOrganizationSettings(contextAs(t, "en"), openapi.UpdateOrganizationSettingsRequestObject{Body: &in})
	if err != nil {
		t.Fatal(err)
	}
	locked, ok := out.(openapi.UpdateOrganizationSettings422JSONResponse)
	if !ok {
		t.Fatalf("answered %T", out)
	}
	if locked.Error.Code != openapi.SIGNINPOLICYLOCKOUT {
		t.Errorf("code %q", locked.Error.Code)
	}
	raw, _ := json.Marshal(locked)
	for _, want := range []string{`"affectedUsers":5`, `"affectedAdmins":1`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the refusal lacks %s: %s", want, raw)
		}
	}
}

func TestTheImpactAnswersWhetherThePolicyIsALockout(t *testing.T) {
	r := &repo{settings: stored(), impact: domain.Impact{AffectedUsers: 3, AffectedAdmins: 0, ActiveAdmins: 2, ActorLocked: true}}
	out, err := handler(t, r, nil, checker{}).GetSignInImpact(contextAs(t, ""), openapi.GetSignInImpactRequestObject{
		Body: &openapi.GetSignInImpactJSONRequestBody{SignIn: openapi.SignInPolicy{Google: false, Password: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.(openapi.GetSignInImpact200JSONResponse)
	if !ok {
		t.Fatalf("answered %T", out)
	}
	if got.AffectedUsers != 3 || !got.ActorLocked || !got.Lockout {
		t.Errorf("impact %+v", got)
	}
}

func upload(t *testing.T, variant string, file []byte) *multipart.Reader {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if variant != "" {
		if err := w.WriteField("variant", variant); err != nil {
			t.Fatal(err)
		}
	}
	if file != nil {
		part, err := w.CreateFormFile("file", "logo.png")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(file)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return multipart.NewReader(&body, w.Boundary())
}

func TestAnUploadIsStoredUnderItsHashAndAnswersIt(t *testing.T) {
	png := []byte("\x89PNG pretend bytes")
	s := &store{objects: map[string][]byte{}}
	h := handler(t, &repo{settings: stored()}, s, checker{file: domain.LogoFile{Bytes: png, Type: domain.LogoPNG, Width: 512, Height: 512}})
	out, err := h.UploadOrganizationLogo(contextAs(t, ""), openapi.UploadOrganizationLogoRequestObject{Body: upload(t, "light", png)})
	if err != nil {
		t.Fatal(err)
	}
	ok, isOK := out.(openapi.UploadOrganizationLogo200JSONResponse)
	if !isOK {
		t.Fatalf("answered %T", out)
	}
	sum := sha256.Sum256(png)
	if ok.Sha256 != hex.EncodeToString(sum[:]) || ok.Width != 512 || ok.Height != 512 || ok.Type != openapi.OrganizationLogoUploadTypeImagepng {
		t.Errorf("answer %+v", ok)
	}
	if _, found := s.objects["branding/"+ok.Sha256+".png"]; !found {
		t.Errorf("the store holds %v", s.objects)
	}
}

func TestAnUploadWithoutAFileOrAVariantIsABadRequest(t *testing.T) {
	s := &store{objects: map[string][]byte{}}
	h := handler(t, &repo{settings: stored()}, s, checker{})
	for name, body := range map[string]*multipart.Reader{
		"no file":         upload(t, "light", nil),
		"a wrong variant": upload(t, "sepia", []byte("x")),
		"no variant":      upload(t, "", []byte("x")),
		"no body":         nil,
	} {
		out, err := h.UploadOrganizationLogo(contextAs(t, ""), openapi.UploadOrganizationLogoRequestObject{Body: body})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, ok := out.(openapi.UploadOrganizationLogo400JSONResponse); !ok {
			t.Errorf("%s: answered %T", name, out)
		}
	}
	if len(s.objects) != 0 {
		t.Errorf("a refused upload stored %v", s.objects)
	}
}

func TestAnUploadTheCheckerRefusesIsMappedToItsStatus(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want any
	}{
		"too large":   {domain.ErrLogoTooLarge, openapi.UploadOrganizationLogo413JSONResponse{}},
		"unsupported": {domain.ErrLogoUnsupported, openapi.UploadOrganizationLogo415JSONResponse{}},
		"unreadable":  {domain.ErrLogoUnreadable, openapi.UploadOrganizationLogo415JSONResponse{}},
		"dimensions":  {domain.ErrLogoDimensions, openapi.UploadOrganizationLogo415JSONResponse{}},
	} {
		t.Run(name, func(t *testing.T) {
			s := &store{objects: map[string][]byte{}}
			h := handler(t, &repo{settings: stored()}, s, checker{err: c.err})
			out, err := h.UploadOrganizationLogo(contextAs(t, ""), openapi.UploadOrganizationLogoRequestObject{Body: upload(t, "dark", []byte("x"))})
			if err != nil {
				t.Fatal(err)
			}
			if got, want := typeName(out), typeName(c.want); got != want {
				t.Errorf("answered %s, want %s", got, want)
			}
			if len(s.objects) != 0 {
				t.Errorf("a refused upload stored %v", s.objects)
			}
		})
	}
}

func TestAnSVGUploadIsRefusedAs415MediaTypeUnsupported(t *testing.T) {
	h := handler(t, &repo{settings: stored()}, &store{objects: map[string][]byte{}}, checker{err: domain.ErrLogoUnsupported})

	out, err := h.UploadOrganizationLogo(contextAs(t, ""), openapi.UploadOrganizationLogoRequestObject{
		Body: upload(t, "light", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"/>`)),
	})

	if err != nil {
		t.Fatal(err)
	}
	refusal, ok := out.(openapi.UploadOrganizationLogo415JSONResponse)
	if !ok || refusal.Error.Code != openapi.MEDIATYPEUNSUPPORTED {
		t.Errorf("answered %T %+v, want 415 MEDIA_TYPE_UNSUPPORTED", out, refusal)
	}
}

func typeName(v any) string { return fmt.Sprintf("%T", v) }

func TestAServerWithoutStorageAnswers501ToAnUpload(t *testing.T) {
	out, err := handler(t, &repo{settings: stored()}, nil, checker{}).UploadOrganizationLogo(contextAs(t, ""),
		openapi.UploadOrganizationLogoRequestObject{Body: upload(t, "light", []byte("x"))})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.(openapi.UploadOrganizationLogo501JSONResponse); !ok {
		t.Errorf("answered %T", out)
	}
}

func logoFor(t *testing.T, variant string, v *string, s domain.Settings, objects map[string][]byte) *httptest.ResponseRecorder {
	t.Helper()
	h := handler(t, &repo{settings: s}, &store{objects: objects}, checker{})
	out, err := h.GetOrganizationLogo(context.Background(), openapi.GetOrganizationLogoRequestObject{
		Params: openapi.GetOrganizationLogoParams{Variant: openapi.GetOrganizationLogoParamsVariant(variant), V: v},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	if err := out.VisitGetOrganizationLogoResponse(response); err != nil {
		t.Fatal(err)
	}
	return response
}

func TestALogoIsServedAsAPNGAttachment(t *testing.T) {
	digest := bytes.Repeat([]byte{0x11}, 32)
	hexed := hex.EncodeToString(digest)
	body := []byte("\x89PNG pretend")
	s := stored()
	s.Profile.DarkLogo = &domain.Logo{SHA256: digest, Type: domain.LogoPNG}
	objects := map[string][]byte{"branding/" + hexed + ".png": body}

	versioned := logoFor(t, "dark", &hexed, s, objects)
	if versioned.Code != http.StatusOK || !bytes.Equal(versioned.Body.Bytes(), body) {
		t.Fatalf("status %d body %q", versioned.Code, versioned.Body.String())
	}
	for header, want := range map[string]string{
		"Content-Type":           "image/png",
		"X-Content-Type-Options": "nosniff",
		"Content-Disposition":    `attachment; filename="logo.png"`,
		"Cache-Control":          "public, max-age=31536000, immutable",
	} {
		if got := versioned.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	other := strings.Repeat("22", 32)
	for name, v := range map[string]*string{"no version": nil, "a stale version": &other} {
		if got := logoFor(t, "dark", v, s, objects).Header().Get("Cache-Control"); got != "public, max-age=300" {
			t.Errorf("%s: Cache-Control %q, want public, max-age=300", name, got)
		}
	}
}

func TestALogoTheCentreDoesNotHaveIs404(t *testing.T) {
	response := logoFor(t, "light", nil, stored(), map[string][]byte{})
	if response.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", response.Code)
	}
}

func TestALogoWhoseBytesAreGoneFromTheStoreIs404NotA500(t *testing.T) {
	s := stored()
	s.Profile.LightLogo = &domain.Logo{SHA256: bytes.Repeat([]byte{0x33}, 32), Type: domain.LogoPNG}
	h := handler(t, &repo{settings: s}, &store{objects: map[string][]byte{}}, checker{})
	out, err := h.GetOrganizationLogo(context.Background(), openapi.GetOrganizationLogoRequestObject{
		Params: openapi.GetOrganizationLogoParams{Variant: openapi.GetOrganizationLogoParamsVariant("light")},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	if err := out.VisitGetOrganizationLogoResponse(response); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", response.Code)
	}
}

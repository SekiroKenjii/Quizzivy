package router_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizzivy/internal/core/router"
	importsapp "quizzivy/internal/modules/imports/application"
	importshttp "quizzivy/internal/modules/imports/http"
)

func importRouter(t *testing.T, imports importshttp.Imports) func(method, path, body, language string) *httptest.ResponseRecorder {
	t.Helper()
	issuer := testIssuer(t)
	handler, err := router.New(router.Deps{DB: fakeDB{}, Tokens: issuer, Modules: router.Modules{Imports: imports}},
		slog.New(slog.NewTextHandler(io.Discard, nil)), []string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatal(err)
	}
	token, err := issuer.Issue("01935000-0000-7000-8000-0000000000a1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	return func(method, path, body, language string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if language != "" {
			req.Header.Set("Accept-Language", language)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
}

func capabilities(t *testing.T, send func(method, path, body, language string) *httptest.ResponseRecorder) (intake, processing bool) {
	t.Helper()
	rec := send(http.MethodGet, "/admin/imports/capabilities", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("capabilities status = %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		IntakeEnabled     *bool `json:"intakeEnabled"`
		ProcessingEnabled *bool `json:"processingEnabled"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || body.IntakeEnabled == nil || body.ProcessingEnabled == nil {
		t.Fatalf("capabilities body %+v: %v", body, err)
	}
	return *body.IntakeEnabled, *body.ProcessingEnabled
}

func TestImportCapabilitiesAnswerWhereWordImportIsNotConfigured(t *testing.T) {
	send := importRouter(t, importshttp.New(nil))
	if intake, processing := capabilities(t, send); intake || processing {
		t.Fatalf("unconfigured import reported intake %v, processing %v", intake, processing)
	}
	if rec := send(http.MethodGet, "/admin/imports/limits", "", ""); rec.Code != http.StatusNotImplemented {
		t.Fatalf("limits without import storage = %d, want 501", rec.Code)
	}
}

func TestImportCapabilitiesFollowTheProcessingSwitch(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		send := importRouter(t, importshttp.New(importsapp.New(importsapp.Dependencies{Processing: enabled})))
		if intake, processing := capabilities(t, send); !intake || processing != enabled {
			t.Fatalf("processing %v reported as intake %v, processing %v", enabled, intake, processing)
		}
	}
}

func TestProcessingWithoutAWorkerIsRefusedInTheEnvelope(t *testing.T) {
	send := importRouter(t, importshttp.New(importsapp.New(importsapp.Dependencies{})))
	body := `{"requestId":"01935000-0000-7000-8000-000000000009","expectedRevision":1}`
	for language, want := range map[string]string{"": "Máy chủ này chưa bật xử lý tài liệu Word", "en": "Word processing is not enabled on this server"} {
		rec := send(http.MethodPost, "/admin/imports/01935000-0000-7000-8000-000000000001/process", body, language)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("process without a worker = %d, want 503: %s", rec.Code, rec.Body.String())
		}
		message := rec.Body.String()
		if code := errorCode(t, rec); code != "IMPORT_PROCESSING_UNAVAILABLE" {
			t.Fatalf("code = %q, want IMPORT_PROCESSING_UNAVAILABLE", code)
		}
		if !strings.Contains(message, want) {
			t.Fatalf("Accept-Language %q message = %s", language, message)
		}
	}
}

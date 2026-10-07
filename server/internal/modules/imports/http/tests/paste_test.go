package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/core/router"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/domain"
	importshttp "quizzivy/internal/modules/imports/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"strings"
	"testing"
	"time"
)

func TestPasteTransportCarriesCallerScopeAndTextAndMapsPrivateErrors(t *testing.T) {
	for _, all := range []bool{false, true} {
		principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
		if all {
			principal.Permissions = access.NewSet(access.All()...)
		}
		id, upload := uuid.New(), uuid.New()
		var seen command.Paste
		app := &application.Application{Commands: application.Commands{Paste: cqrs.HandlerFunc[command.Paste, domain.Receipt](func(_ context.Context, in command.Paste) (domain.Receipt, error) {
			seen = in
			return domain.Receipt{}, domain.ErrNotFound
		})}}
		response, err := importshttp.New(app).PasteImportSource(signedIn(t, principal), openapi.PasteImportSourceRequestObject{Id: id, Body: &openapi.PasteImportSourceJSONRequestBody{UploadId: upload, ExpectedRevision: 9, Text: "plain"}})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		if err := response.VisitPasteImportSourceResponse(recorder); err != nil {
			t.Fatal(err)
		}
		if recorder.Code != 404 || recorder.Header().Get("Cache-Control") != "no-store" || seen.ImportID != id.String() || seen.UploadID != upload.String() || seen.Text != "plain" || seen.ExpectedRevision != 9 || seen.Actor.ID != principal.UserID || seen.Actor.Scope.All != all {
			t.Fatalf("paste transport %+v %s", seen, recorder.Body.String())
		}
	}
	for _, test := range []struct {
		err    error
		status int
		code   string
	}{{domain.ErrTooLarge, 413, "IMPORT_SOURCE_TOO_LARGE"}, {domain.ErrInvalid, 415, "IMPORT_SOURCE_INVALID"}, {domain.ErrBusy, 429, "IMPORT_BUSY"}, {domain.ErrQuota, 429, "IMPORT_QUOTA_EXCEEDED"}, {domain.ErrConflict, 409, "IMPORT_CONFLICT"}} {
		app := &application.Application{Commands: application.Commands{Paste: cqrs.HandlerFunc[command.Paste, domain.Receipt](func(context.Context, command.Paste) (domain.Receipt, error) { return domain.Receipt{}, test.err })}}
		response, err := importshttp.New(app).PasteImportSource(signedIn(t, access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}), openapi.PasteImportSourceRequestObject{Body: &openapi.PasteImportSourceJSONRequestBody{}})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		if err := response.VisitPasteImportSourceResponse(recorder); err != nil {
			t.Fatal(err)
		}
		if recorder.Code != test.status || !strings.Contains(recorder.Body.String(), test.code) || recorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("error response %d %s", recorder.Code, recorder.Body.String())
		}
	}
	if _, err := importshttp.New(nil).PasteImportSource(context.Background(), openapi.PasteImportSourceRequestObject{}); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Fatalf("disabled intake: %v", err)
	}
}

func TestPasteRuntimeSchemaAndPermissionGatePrecedeDisabledIntake(t *testing.T) {
	user := uuid.NewString()
	issuer, err := identitytoken.NewIssuer([]byte(strings.Repeat("k", 32)), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	token, err := issuer.Issue(user, "teacher", 0)
	if err != nil {
		t.Fatal(err)
	}
	principals := scopedPrincipals{user: {UserID: user, BuiltinKey: access.BuiltinTeacher, Permissions: access.NewSet(access.ContentTestsWrite)}}
	handler, err := router.New(router.Deps{Tokens: issuer, Principals: principals, Modules: router.Modules{Imports: importshttp.New(nil)}}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, "")
	if err != nil {
		t.Fatal(err)
	}
	path := "/teacher/imports/" + uuid.NewString() + "/sources/text"
	for _, test := range []struct {
		name string
		body any
		want int
	}{
		{"valid", map[string]any{"uploadId": uuid.NewString(), "expectedRevision": 1, "text": "plain"}, 501},
		{"extra property", map[string]any{"uploadId": uuid.NewString(), "expectedRevision": 1, "text": "plain", "role": "answer_key"}, 400},
		{"missing upload", map[string]any{"expectedRevision": 1, "text": "plain"}, 400},
		{"blank", map[string]any{"uploadId": uuid.NewString(), "expectedRevision": 1, "text": " \n\t"}, 400},
		{"maximum Unicode", map[string]any{"uploadId": uuid.NewString(), "expectedRevision": 1, "text": strings.Repeat("😀", 100000)}, 501},
		{"extra Unicode", map[string]any{"uploadId": uuid.NewString(), "expectedRevision": 1, "text": strings.Repeat("😀", 100001)}, 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, err := json.Marshal(test.body)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(data)))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("runtime validation: %d %s", response.Code, response.Body.String())
			}
		})
	}
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader("invalid"))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 401 {
		t.Fatalf("body read before auth: %d", response.Code)
	}
}

func TestPasteReceiptCharactersAgreeWithActualClosedSourceSchema(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
	now := time.Now().UTC()
	n := 1
	for _, format := range []string{"text", "docx"} {
		source := domain.Source{ID: uuid.NewString(), Role: "exam", Filename: "source", Format: format, Bytes: 1, SHA256: make([]byte, 32), UploadedBy: principal.UserID, CreatedAt: now, SourceRevision: 1}
		if format == "text" {
			source.Characters = &n
		}
		app := &application.Application{Commands: application.Commands{Paste: cqrs.HandlerFunc[command.Paste, domain.Receipt](func(context.Context, command.Paste) (domain.Receipt, error) {
			return domain.Receipt{Source: source, Import: domain.Import{ID: uuid.NewString(), Title: "Private text", Status: "awaiting_sources", Revision: 2, SourceRevision: 1, Sources: []domain.Source{source}, CreatedBy: principal.UserID, CreatedAt: now, UpdatedAt: now}}, nil
		})}}
		response, err := importshttp.New(app).PasteImportSource(signedIn(t, principal), openapi.PasteImportSourceRequestObject{Body: &openapi.PasteImportSourceJSONRequestBody{}})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		if err := response.VisitPasteImportSourceResponse(recorder); err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		spec, err := openapi.GetSpec()
		if err != nil {
			t.Fatal(err)
		}
		if err := spec.Components.Schemas["ImportUploadReceipt"].Value.VisitJSON(body); err != nil {
			t.Fatalf("closed receipt: %v %s", err, recorder.Body.String())
		}
		src := body["source"].(map[string]any)
		_, has := src["characters"]
		if has != (format == "text") {
			t.Fatalf("character presence %v", src)
		}
	}
}

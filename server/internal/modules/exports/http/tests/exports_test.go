package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/exports/application"
	"quizzivy/internal/modules/exports/domain"
	exportshttp "quizzivy/internal/modules/exports/http"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

func TestWithoutAnApplicationEveryOperationAnswersExportOff(t *testing.T) {
	h := exportshttp.NewExports(nil)
	ctx := context.Background()
	id := httpapi.ParseUUID("0198f000-0000-7000-8000-000000000001")
	cases := map[string]func() (visitor, error){
		"requestDataExport": func() (visitor, error) {
			out, err := h.RequestDataExport(ctx, openapi.RequestDataExportRequestObject{Body: &openapi.DataExportRequest{}})
			return func(w http.ResponseWriter) error { return out.VisitRequestDataExportResponse(w) }, err
		},
		"getDataExport": func() (visitor, error) {
			out, err := h.GetDataExport(ctx, openapi.GetDataExportRequestObject{Id: id})
			return func(w http.ResponseWriter) error { return out.VisitGetDataExportResponse(w) }, err
		},
		"downloadDataExport": func() (visitor, error) {
			out, err := h.DownloadDataExport(ctx, openapi.DownloadDataExportRequestObject{Id: id})
			return func(w http.ResponseWriter) error { return out.VisitDownloadDataExportResponse(w) }, err
		},
	}
	for name, call := range cases {
		visit, err := call()
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		response := httptest.NewRecorder()
		if err := visit(response); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if code, _ := envelope(t, response); response.Code != http.StatusNotImplemented || code != "EXPORT_OFF" {
			t.Errorf("%s answered %d %s, want 501 EXPORT_OFF", name, response.Code, code)
		}
	}
}

type visitor func(http.ResponseWriter) error

type repo struct {
	domain.Repository
	err error
}

func (r repo) Request(_ context.Context, in domain.Request) (domain.Export, error) {
	if r.err != nil {
		return domain.Export{}, r.err
	}
	return domain.Export{ID: "0198f000-0000-7000-8000-000000000001", RequestedBy: in.ActorID, Status: domain.Queued, CreatedAt: time.Now()}, nil
}

type store struct{}

func (store) Put(context.Context, string, string, io.Reader, int64) error { return nil }

func (store) SignedDownloadURL(context.Context, string, string, time.Duration) (string, error) {
	return "https://objects.example/x", nil
}

type wake struct{}

func (wake) Wake() {}

func request(t *testing.T, fail error, acceptLanguage string) *httptest.ResponseRecorder {
	t.Helper()
	app := application.New(repo{err: fail}, store{}, wake{}, time.UTC)
	transport := exportshttp.NewExports(app)
	admin := uuid.NewString()
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: admin, Access: access.Principal{UserID: admin, Permissions: access.NewSet()}}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			out, err := transport.RequestDataExport(r.Context(), openapi.RequestDataExportRequestObject{Body: &openapi.DataExportRequest{IncludeAnswers: true}})
			if err != nil {
				t.Fatal(err)
			}
			if err := out.VisitRequestDataExportResponse(w); err != nil {
				t.Fatal(err)
			}
		})))
	req := httptest.NewRequest(http.MethodPost, "/admin/data-exports", nil)
	req.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		req.Header.Set("Accept-Language", acceptLanguage)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func envelope(t *testing.T, response *httptest.ResponseRecorder) (code, message string) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("not an error envelope: %s", response.Body.String())
	}
	return body.Error.Code, body.Error.Message
}

func TestAnAdminWhoAskedWithinTheHourIsToldWhenToComeBack(t *testing.T) {
	response := request(t, &domain.TooSoonError{RetryAfter: 41*time.Minute + 20*time.Second}, "en")
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Retry-After"); got != "2480" {
		t.Errorf("Retry-After = %q, want 2480", got)
	}
	code, message := envelope(t, response)
	if code != "RATE_LIMITED" || message != "Only one export can be requested an hour; a failed one does not count. Try again later." {
		t.Errorf("error = %s %q", code, message)
	}
}

func TestTheRefusalForAnExportInProgressIsNotARateLimit(t *testing.T) {
	response := request(t, domain.ErrInProgress, "vi")
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Retry-After"); got != "" {
		t.Errorf("a 409 carries Retry-After %q; it spends no time", got)
	}
	if code, _ := envelope(t, response); code != "EXPORT_IN_PROGRESS" {
		t.Errorf("code = %s", code)
	}
}

func TestAQueuedExportAnswersAccepted(t *testing.T) {
	if response := request(t, nil, ""); response.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
}

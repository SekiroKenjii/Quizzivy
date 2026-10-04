package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/application/query"
	"quizzivy/internal/modules/imports/domain"
	importshttp "quizzivy/internal/modules/imports/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

func speaking(t *testing.T, acceptLanguage string) context.Context {
	t.Helper()
	var ctx context.Context
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: uuid.NewString()}, nil
		})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	request := httptest.NewRequest(http.MethodGet, "/teacher/imports", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if ctx == nil {
		t.Fatal("the request did not reach the handler")
	}
	return ctx
}

func TestImportRefusalsStillSpeakBothLanguages(t *testing.T) {
	transport := importshttp.New(&application.Application{
		Commands: application.Commands{Commit: cqrs.HandlerFunc[command.Commit, command.CommitResult](func(context.Context, command.Commit) (command.CommitResult, error) {
			return command.CommitResult{}, domain.ErrBadDraft
		})},
		Queries: application.Queries{Get: cqrs.HandlerFunc[query.Get, domain.Import](func(context.Context, query.Get) (domain.Import, error) {
			return domain.Import{}, domain.ErrNotFound
		})},
	})
	for _, c := range []struct {
		name   string
		serve  func(ctx context.Context, w http.ResponseWriter) error
		status int
		code   openapi.ErrorCode
		vi     string
		en     string
	}{
		{
			name: "a commit of a bad draft",
			serve: func(ctx context.Context, w http.ResponseWriter) error {
				response, err := transport.CommitWordImport(ctx, openapi.CommitWordImportRequestObject{Id: uuid.New(), Body: &openapi.CommitWordImportJSONRequestBody{RequestId: uuid.New(), DraftRevision: 1}})
				if err != nil {
					return err
				}
				return response.VisitCommitWordImportResponse(w)
			},
			status: http.StatusUnprocessableEntity,
			code:   openapi.VALIDATIONFAILED,
			vi:     "Bản rà soát gửi lên không hợp lệ. Hãy tải lại trang và thử lại.",
			en:     "The submitted review is malformed. Reload the page and try again.",
		},
		{
			name: "an import that is not there",
			serve: func(ctx context.Context, w http.ResponseWriter) error {
				response, err := transport.GetWordImport(ctx, openapi.GetWordImportRequestObject{Id: uuid.New()})
				if err != nil {
					return err
				}
				return response.VisitGetWordImportResponse(w)
			},
			status: http.StatusNotFound,
			code:   openapi.NOTFOUND,
			vi:     "Không tìm thấy lượt nhập hoặc tệp đã hoàn tất.",
			en:     "The import or completed source was not found.",
		},
	} {
		for _, language := range []struct {
			accept string
			want   string
		}{{"", c.vi}, {"en", c.en}} {
			rec := httptest.NewRecorder()
			if err := c.serve(speaking(t, language.accept), rec); err != nil {
				t.Fatalf("%s with Accept-Language %q: %v", c.name, language.accept, err)
			}
			var problem openapi.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
				t.Fatalf("%s with Accept-Language %q: %v: %s", c.name, language.accept, err, rec.Body.String())
			}
			if rec.Code != c.status || problem.Error.Code != c.code {
				t.Errorf("%s with Accept-Language %q answered %d %s, want %d %s", c.name, language.accept, rec.Code, problem.Error.Code, c.status, c.code)
			}
			if problem.Error.Message != language.want {
				t.Errorf("%s with Accept-Language %q answered %q, want %q", c.name, language.accept, problem.Error.Message, language.want)
			}
		}
	}
}

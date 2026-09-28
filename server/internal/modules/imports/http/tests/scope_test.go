package http_test

import (
	"bytes"
	"context"
	"mime/multipart"
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
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/cqrs"
)

type scopedPrincipals map[string]access.Principal

func (r scopedPrincipals) Resolve(_ context.Context, userID string) (access.Principal, error) {
	return r[userID], nil
}

func signedIn(t *testing.T, principal access.Principal) context.Context {
	t.Helper()
	var ctx context.Context
	verify := func(string) (httpx.Principal, error) { return httpx.Principal{UserID: principal.UserID}, nil }
	gate := httpx.RequirePermission(map[string]access.Requirement{"GET /teacher/imports": access.AnyOf(access.ContentTestsWrite)},
		scopedPrincipals{principal.UserID: principal})
	h := httpx.RequireAuth(nil, verify)(gate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	req := httptest.NewRequest(http.MethodGet, "/teacher/imports", nil)
	req.Pattern = "GET /teacher/imports"
	req.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ctx == nil {
		t.Fatal("the gate did not pass the request")
	}
	return ctx
}

func recordQuery[Q, R any](seen map[string]access.Scope, op string, scopeOf func(Q) access.Scope) cqrs.QueryHandler[Q, R] {
	return cqrs.HandlerFunc[Q, R](func(_ context.Context, q Q) (R, error) {
		var zero R
		seen[op] = scopeOf(q)
		if op == "list" {
			return zero, nil
		}
		return zero, domain.ErrNotFound
	})
}

func recordCommand[C, R any](seen map[string]access.Scope, op string, actorOf func(C) actor.Actor) cqrs.CommandHandler[C, R] {
	return cqrs.HandlerFunc[C, R](func(_ context.Context, c C) (R, error) {
		var zero R
		a := actorOf(c)
		seen[op] = access.Scope{UserID: a.ID, All: a.Scope.All}
		return zero, domain.ErrNotFound
	})
}

func TestEveryImportOperationCarriesTheCallersScope(t *testing.T) {
	for name, principal := range map[string]access.Principal{
		"a Teacher": {UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)},
		"an Admin":  {UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)},
	} {
		t.Run(name, func(t *testing.T) {
			seen := map[string]access.Scope{}
			app := &application.Application{
				Queries: application.Queries{
					Get:        recordQuery[query.Get, domain.Import](seen, "get", func(q query.Get) access.Scope { return q.Scope }),
					List:       recordQuery[query.List, domain.List](seen, "list", func(q query.List) access.Scope { return q.Scope }),
					Download:   recordQuery[query.Download, query.DownloadResult](seen, "download", func(q query.Download) access.Scope { return q.Scope }),
					Review:     recordQuery[query.Review, domain.ReviewState](seen, "review", func(q query.Review) access.Scope { return q.Scope }),
					SourceView: recordQuery[query.SourceView, query.SourceViewResult](seen, "source", func(q query.SourceView) access.Scope { return q.Scope }),
				},
				Commands: application.Commands{
					Create:     recordCommand[command.Create, domain.Import](seen, "create", func(c command.Create) actor.Actor { return c.Actor }),
					Upload:     recordCommand[command.Upload, domain.Receipt](seen, "upload", func(c command.Upload) actor.Actor { return c.Actor }),
					Process:    recordCommand[command.Process, domain.Import](seen, "process", func(c command.Process) actor.Actor { return c.Actor }),
					Cancel:     recordCommand[command.Cancel, domain.Import](seen, "cancel", func(c command.Cancel) actor.Actor { return c.Actor }),
					SaveReview: recordCommand[command.SaveReview, domain.ReviewState](seen, "save", func(c command.SaveReview) actor.Actor { return c.Actor }),
					Adopt:      recordCommand[command.Adopt, domain.ReviewState](seen, "adopt", func(c command.Adopt) actor.Actor { return c.Actor }),
					Commit:     recordCommand[command.Commit, command.CommitResult](seen, "commit", func(c command.Commit) actor.Actor { return c.Actor }),
				},
			}
			h := importshttp.New(app)
			ctx := signedIn(t, principal)
			id := uuid.New()
			calls := map[string]func() (any, error){
				"get":  func() (any, error) { return h.GetWordImport(ctx, openapi.GetWordImportRequestObject{Id: id}) },
				"list": func() (any, error) { return h.ListWordImports(ctx, openapi.ListWordImportsRequestObject{}) },
				"download": func() (any, error) {
					return h.DownloadImportSource(ctx, openapi.DownloadImportSourceRequestObject{Id: id, SourceId: uuid.New()})
				},
				"review": func() (any, error) {
					return h.GetWordImportReview(ctx, openapi.GetWordImportReviewRequestObject{Id: id})
				},
				"source": func() (any, error) {
					return h.GetWordImportSource(ctx, openapi.GetWordImportSourceRequestObject{Id: id, Params: openapi.GetWordImportSourceParams{Role: openapi.ImportSourceRole("exam")}})
				},
				"create": func() (any, error) {
					return h.CreateWordImport(ctx, openapi.CreateWordImportRequestObject{Body: &openapi.CreateWordImportJSONRequestBody{RequestId: uuid.New(), Title: "Đề"}})
				},
				"upload": func() (any, error) {
					var body bytes.Buffer
					form := multipart.NewWriter(&body)
					part, err := form.CreateFormFile("file", "de.docx")
					if err != nil {
						return nil, err
					}
					if _, err := part.Write([]byte("PK")); err != nil {
						return nil, err
					}
					if err := form.Close(); err != nil {
						return nil, err
					}
					return h.UploadImportSource(ctx, openapi.UploadImportSourceRequestObject{Id: id,
						Params: openapi.UploadImportSourceParams{Role: openapi.ImportSourceRole("exam"), UploadId: uuid.New(), ExpectedRevision: 1},
						Body:   multipart.NewReader(&body, form.Boundary())})
				},
				"process": func() (any, error) {
					return h.ProcessWordImport(ctx, openapi.ProcessWordImportRequestObject{Id: id, Body: &openapi.ProcessWordImportJSONRequestBody{RequestId: uuid.New(), ExpectedRevision: 1}})
				},
				"cancel": func() (any, error) {
					return h.CancelWordImport(ctx, openapi.CancelWordImportRequestObject{Id: id, Body: &openapi.CancelWordImportJSONRequestBody{ExpectedRevision: 1}})
				},
				"save": func() (any, error) {
					return h.SaveWordImportReview(ctx, openapi.SaveWordImportReviewRequestObject{Id: id, Body: &openapi.SaveWordImportReviewJSONRequestBody{ExpectedRevision: 1, Title: "Đề"}})
				},
				"adopt": func() (any, error) {
					return h.AdoptWordImportReprocessed(ctx, openapi.AdoptWordImportReprocessedRequestObject{Id: id, Body: &openapi.AdoptWordImportReprocessedJSONRequestBody{ExpectedRevision: 1}})
				},
				"commit": func() (any, error) {
					return h.CommitWordImport(ctx, openapi.CommitWordImportRequestObject{Id: id, Body: &openapi.CommitWordImportJSONRequestBody{RequestId: uuid.New(), DraftRevision: 1}})
				},
			}
			for op, call := range calls {
				if _, err := call(); err != nil {
					t.Fatalf("%s: %v", op, err)
				}
			}
			want := access.Scope{UserID: principal.UserID, All: principal.Permissions.Has(access.ScopeAll)}
			for op := range calls {
				if got, ok := seen[op]; !ok || got != want {
					t.Errorf("%s ran in %+v (reached %v), want %+v", op, got, ok, want)
				}
			}
		})
	}
}

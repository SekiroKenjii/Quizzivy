package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/application/query"
	"quizzivy/internal/modules/imports/domain"
	importshttp "quizzivy/internal/modules/imports/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/paging"
)

func TestHistoryCarriesRepeatedStatusesAndTruthfulCurrentCounts(t *testing.T) {
	by := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)}
	statuses := []openapi.ImportStatus{openapi.ImportStatusNeedsReview, openapi.ImportStatusFailed}
	var seen query.List
	now := time.Now().UTC()
	current := domain.Import{ID: uuid.NewString(), CreatedBy: by.UserID, Title: "Đề", Status: "needs_review", Revision: 1, CreatedAt: now, UpdatedAt: now, Sources: []domain.Source{}, ReviewCounts: &domain.ReviewCounts{NeedsAction: 32768, ToConfirm: 4}}
	legacy := current
	legacy.ID, legacy.ReviewCounts = uuid.NewString(), nil
	facets := domain.StatusFacets{All: 9, Processing: 4, NeedsReview: 2, Failed: 1, Committed: 1, Cancelled: 1}
	app := &application.Application{Queries: application.Queries{List: cqrs.HandlerFunc[query.List, domain.List](func(_ context.Context, in query.List) (domain.List, error) {
		seen = in
		return domain.List{Items: []domain.Import{current, legacy}, Page: paging.Page{Number: 2, Size: 2, Total: 3}, Facets: facets}, nil
	})}}
	response, err := importshttp.New(app).ListWordImports(signedIn(t, by), openapi.ListWordImportsRequestObject{Params: openapi.ListWordImportsParams{Status: &statuses}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seen.Status, []string{"needs_review", "failed"}) || seen.Scope.All || seen.Scope.UserID != by.UserID {
		t.Fatalf("history filter %+v", seen)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitListWordImportsResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var body struct {
		Items []struct {
			ReviewCounts *openapi.ImportReviewCounts `json:"reviewCounts"`
		}
		Facets openapi.ImportStatusFacets
		Total  int
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != 200 || body.Total != 3 || body.Facets.All != 9 || body.Facets.Processing != 4 || body.Facets.NeedsReview != 2 || body.Facets.Failed != 1 || body.Facets.Committed != 1 || body.Facets.Cancelled != 1 {
		t.Fatalf("history response %s", recorder.Body.String())
	}
	if len(body.Items) != 2 || body.Items[0].ReviewCounts == nil || body.Items[0].ReviewCounts.NeedsAction != 32768 || body.Items[0].ReviewCounts.ToConfirm != 4 || body.Items[1].ReviewCounts != nil {
		t.Fatalf("counts response %s", recorder.Body.String())
	}
	var raw map[string]json.RawMessage
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw["items"], &items); err != nil {
		t.Fatal(err)
	}
	if string(items[1]["reviewCounts"]) != "null" {
		t.Fatalf("legacy counts omitted or invented: %s", recorder.Body.String())
	}
}

func TestHistoryAcceptsOneStatusAndOtherResponsesDoNotInventCounts(t *testing.T) {
	by := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
	status := []openapi.ImportStatus{openapi.ImportStatusFailed}
	var seen []string
	app := &application.Application{Queries: application.Queries{
		List: cqrs.HandlerFunc[query.List, domain.List](func(_ context.Context, in query.List) (domain.List, error) {
			seen = in.Status
			return domain.List{}, nil
		}),
		Get: cqrs.HandlerFunc[query.Get, domain.Import](func(context.Context, query.Get) (domain.Import, error) {
			return domain.Import{ID: uuid.NewString(), CreatedBy: by.UserID, Title: "Đề", Status: "failed", ReviewCounts: &domain.ReviewCounts{NeedsAction: 7}}, nil
		}),
	}}
	handler := importshttp.New(app)
	if _, err := handler.ListWordImports(signedIn(t, by), openapi.ListWordImportsRequestObject{Params: openapi.ListWordImportsParams{Status: &status}}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(seen, []string{"failed"}) {
		t.Fatalf("single status %+v", seen)
	}
	response, err := handler.GetWordImport(signedIn(t, by), openapi.GetWordImportRequestObject{Id: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	if err := response.VisitGetWordImportResponse(recorder); err != nil {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, present := body["reviewCounts"]; present {
		t.Fatalf("detail fabricated history counts: %s", recorder.Body.String())
	}
}

type historyWireServer struct {
	openapi.ServerInterface
	handler importshttp.Imports
	ctx     context.Context
	test    *testing.T
}

func (s historyWireServer) ListWordImports(w http.ResponseWriter, _ *http.Request, params openapi.ListWordImportsParams) {
	response, err := s.handler.ListWordImports(s.ctx, openapi.ListWordImportsRequestObject{Params: params})
	if err != nil {
		s.test.Fatal(err)
	}
	if err := response.VisitListWordImportsResponse(w); err != nil {
		s.test.Fatal(err)
	}
}

func TestHistoryWireStillAcceptsSingleAndRepeatedQueryValues(t *testing.T) {
	for _, test := range []struct {
		query    string
		statuses []string
	}{
		{"?status=failed", []string{"failed"}},
		{"?status=failed&status=needs_review", []string{"failed", "needs_review"}},
		{"", nil},
	} {
		t.Run(test.query, func(t *testing.T) {
			by := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
			var seen query.List
			app := &application.Application{Queries: application.Queries{List: cqrs.HandlerFunc[query.List, domain.List](func(_ context.Context, in query.List) (domain.List, error) { seen = in; return domain.List{}, nil })}}
			handler := historyWireServer{handler: importshttp.New(app), ctx: signedIn(t, by), test: t}
			wrapper := openapi.ServerInterfaceWrapper{Handler: handler, ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) { t.Error(err); w.WriteHeader(400) }}
			recorder := httptest.NewRecorder()
			wrapper.ListWordImports(recorder, httptest.NewRequest(http.MethodGet, "/teacher/imports"+test.query, nil))
			if recorder.Code != 200 || !reflect.DeepEqual(seen.Status, test.statuses) {
				t.Fatalf("wire %s status%d filter %+v", test.query, recorder.Code, seen)
			}
		})
	}
}

package router_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	importsapp "quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/domain"
	importshttp "quizzivy/internal/modules/imports/http"
	"quizzivy/internal/shared/paging"
)

type listedImports struct {
	domain.Repository
	status string
}

func (r listedImports) item() domain.Import {
	at := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	return domain.Import{ID: "01935000-0000-7000-8000-000000000001", Title: "Đề", Status: r.status, Revision: 1, CreatedBy: "01935000-0000-7000-8000-0000000000a1", CreatedAt: at, UpdatedAt: at}
}

func (r listedImports) Get(context.Context, string) (domain.Import, error) { return r.item(), nil }

func (r listedImports) List(context.Context, domain.Filter) (domain.List, error) {
	return domain.List{Items: []domain.Import{r.item()}, Page: paging.Page{Number: 1, Size: 20, Total: 1}}, nil
}

type countedWakes struct{ n int }

func (c *countedWakes) Wake() { c.n++ }

func TestWatchingAQueuedImportNudgesTheWorker(t *testing.T) {
	for status, want := range map[string]int{"queued": 1, "processing": 0, "needs_review": 0} {
		for _, path := range []string{"/admin/imports/01935000-0000-7000-8000-000000000001", "/admin/imports"} {
			wakes := &countedWakes{}
			send := importRouter(t, importshttp.New(importsapp.New(importsapp.Dependencies{Repo: listedImports{status: status}, Worker: wakes, Processing: true})))
			if rec := send(http.MethodGet, path, "", ""); rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body.String())
			}
			if wakes.n != want {
				t.Fatalf("GET %s showing a %s import sent %d wakes, want %d", path, status, wakes.n, want)
			}
		}
	}
}

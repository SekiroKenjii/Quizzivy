package router_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	importsapp "quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/domain"
	importshttp "quizzivy/internal/modules/imports/http"
)

type removedImport struct{ listedImports }

func (r removedImport) Get(ctx context.Context, id string) (domain.Import, error) {
	v, _ := r.listedImports.Get(ctx, id)
	removed := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	v.FilesRemovedAt = &removed
	return v, nil
}

func TestAnImportWhoseFilesWereRemovedAnswersGone(t *testing.T) {
	send := importRouter(t, importshttp.New(importsapp.New(importsapp.Dependencies{Repo: removedImport{listedImports{status: "committed"}}})))
	base := "/admin/imports/01935000-0000-7000-8000-000000000001"
	for _, path := range []string{base + "/review", base + "/source?role=exam", base + "/sources/01935000-0000-7000-8000-000000000002/download"} {
		rec := send(http.MethodGet, path, "", "")
		if rec.Code != http.StatusGone {
			t.Fatalf("GET %s = %d, want 410: %s", path, rec.Code, rec.Body.String())
		}
		if code := errorCode(t, rec); code != "IMPORT_FILES_REMOVED" {
			t.Fatalf("GET %s code = %q", path, code)
		}
	}
	rec := send(http.MethodGet, base, "", "")
	var body struct {
		FilesRemovedAt *time.Time `json:"filesRemovedAt"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || body.FilesRemovedAt == nil {
		t.Fatalf("the import does not say its files were removed: %v", err)
	}
}

func TestCapabilitiesStateTheRetentionPolicy(t *testing.T) {
	for _, imports := range []importshttp.Imports{importshttp.New(nil), importshttp.New(importsapp.New(importsapp.Dependencies{}))} {
		rec := importRouter(t, imports)(http.MethodGet, "/admin/imports/capabilities", "", "")
		var body struct {
			Retention struct{ AfterCommitDays, AfterCancelDays, IdleDays int } `json:"retention"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if r := body.Retention; r.AfterCommitDays != 30 || r.AfterCancelDays != 7 || r.IdleDays != 60 {
			t.Fatalf("retention %+v, want 30/7/60 days", r)
		}
	}
}

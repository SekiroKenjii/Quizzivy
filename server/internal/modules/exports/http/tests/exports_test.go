package http_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/gen/openapi"
	exportshttp "quizzivy/internal/modules/exports/http"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

func TestWithoutAnApplicationEveryOperationAnswersNotImplemented(t *testing.T) {
	h := exportshttp.NewExports(nil)
	ctx := context.Background()
	id := httpapi.ParseUUID("0198f000-0000-7000-8000-000000000001")
	if _, err := h.RequestDataExport(ctx, openapi.RequestDataExportRequestObject{Body: &openapi.DataExportRequest{}}); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Errorf("requestDataExport: %v", err)
	}
	if _, err := h.GetDataExport(ctx, openapi.GetDataExportRequestObject{Id: id}); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Errorf("getDataExport: %v", err)
	}
	if _, err := h.DownloadDataExport(ctx, openapi.DownloadDataExportRequestObject{Id: id}); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Errorf("downloadDataExport: %v", err)
	}
}

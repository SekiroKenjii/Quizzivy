// Package http serves the data export under /admin/data-exports: asking for
// one, reading where it stands, and fetching its file once.
package http

import (
	"context"
	"errors"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/exports/application"
	"quizzivy/internal/modules/exports/application/command"
	"quizzivy/internal/modules/exports/application/query"
	"quizzivy/internal/modules/exports/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

// Exports is the transport of the data export.
type Exports struct {
	app *application.Application
}

// NewExports serves the exports through app. A nil app answers 501: the
// server has no object storage or no worker to build a file.
func NewExports(app *application.Application) Exports { return Exports{app: app} }

// RequestDataExport implements POST /admin/data-exports.
func (h Exports) RequestDataExport(ctx context.Context, request openapi.RequestDataExportRequestObject) (openapi.RequestDataExportResponseObject, error) {
	who, ok := httpapi.ActorFromContext(ctx)
	if h.app == nil || !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	queued, err := h.app.Commands.Request.Handle(ctx, command.Request{Actor: who, IncludeAnswers: request.Body.IncludeAnswers})
	if errors.Is(err, domain.ErrInProgress) {
		return openapi.RequestDataExport409JSONResponse(httpapi.Error(ctx, openapi.EXPORTINPROGRESS,
			httpx.Text(ctx, "Đang có một bản xuất dữ liệu được xử lý. Hãy đợi nó xong rồi yêu cầu lại.",
				"An export is already being prepared. Wait for it to finish, then ask again."))), nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.RequestDataExport202JSONResponse(body(queued)), nil
}

// GetDataExport implements GET /admin/data-exports/{id}.
func (h Exports) GetDataExport(ctx context.Context, request openapi.GetDataExportRequestObject) (openapi.GetDataExportResponseObject, error) {
	who, ok := httpapi.ActorFromContext(ctx)
	if h.app == nil || !ok {
		return nil, httpx.ErrNotImplemented
	}
	found, err := h.app.Queries.Get.Handle(ctx, query.Get{Actor: who, ID: request.Id.String()})
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.GetDataExport404JSONResponse{NotFoundJSONResponse: notFound(ctx)}, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.GetDataExport200JSONResponse(body(found)), nil
}

// DownloadDataExport implements POST /admin/data-exports/{id}/download.
func (h Exports) DownloadDataExport(ctx context.Context, request openapi.DownloadDataExportRequestObject) (openapi.DownloadDataExportResponseObject, error) {
	who, ok := httpapi.ActorFromContext(ctx)
	if h.app == nil || !ok {
		return nil, httpx.ErrNotImplemented
	}
	got, err := h.app.Commands.Download.Handle(ctx, command.Download{Actor: who, ID: request.Id.String()})
	switch {
	case err == nil:
		return openapi.DownloadDataExport200JSONResponse{
			Body:    openapi.DataExportDownload{Url: got.URL, ExpiresInSeconds: openapi.DataExportDownloadExpiresInSeconds(domain.URLTTL.Seconds())},
			Headers: openapi.DownloadDataExport200ResponseHeaders{CacheControl: "private, no-store"},
		}, nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.DownloadDataExport404JSONResponse{NotFoundJSONResponse: notFound(ctx)}, nil
	case errors.Is(err, domain.ErrUnavailable):
		return openapi.DownloadDataExport410JSONResponse(httpapi.Error(ctx, openapi.EXPORTUNAVAILABLE,
			httpx.Text(ctx, "Bản xuất này không còn tải được: đã tải một lần, đã hết hạn hoặc chưa sẵn sàng.",
				"This export can no longer be downloaded: it was fetched already, has expired or is not ready."))), nil
	default:
		return nil, err
	}
}

func notFound(ctx context.Context) openapi.NotFoundJSONResponse {
	return openapi.NotFoundJSONResponse(httpapi.NotFound(ctx,
		httpx.Text(ctx, "Không tìm thấy bản xuất dữ liệu.", "The export was not found.")))
}

func body(e domain.Export) openapi.DataExport {
	return openapi.DataExport{
		Id: httpapi.ParseUUID(e.ID), Status: openapi.DataExportStatus(e.Status), IncludeAnswers: e.IncludeAnswers, Bytes: e.Bytes,
		CreatedAt: e.CreatedAt, CompletedAt: e.CompletedAt, ExpiresAt: e.ExpiresAt, DownloadedAt: e.DownloadedAt,
	}
}

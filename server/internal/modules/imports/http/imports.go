// Package http exposes private Word intake only through authenticated teacher routes.
package http

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/application/query"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/actor"
)

type Imports struct {
	app   *application.Application
	slots chan struct{}
}

func New(app *application.Application) Imports {
	return Imports{app: app, slots: make(chan struct{}, 1)}
}

func (h Imports) CreateWordImport(ctx context.Context, request openapi.CreateWordImportRequestObject) (openapi.CreateWordImportResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	a, ok := httpapi.ActorFromContext(ctx)
	if !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	v, err := h.app.Commands.Create.Handle(ctx, command.Create{RequestID: request.Body.RequestId.String(), Title: request.Body.Title, Actor: actor.Actor(a)})
	if err != nil {
		return importFailure(ctx, err)
	}
	return openapi.CreateWordImport201JSONResponse(toImport(v)), nil
}
func (h Imports) GetWordImport(ctx context.Context, request openapi.GetWordImportRequestObject) (openapi.GetWordImportResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	v, err := h.app.Queries.Get.Handle(ctx, query.Get{ID: request.Id.String()})
	if err != nil {
		return importFailure(ctx, err)
	}
	return openapi.GetWordImport200JSONResponse(toImport(v)), nil
}
func (h Imports) ListWordImports(ctx context.Context, request openapi.ListWordImportsRequestObject) (openapi.ListWordImportsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	in := query.List{Search: httpapi.DerefString(request.Params.Q)}
	if request.Params.Status != nil {
		in.Status = string(*request.Params.Status)
	}
	if request.Params.Page != nil {
		in.Page = *request.Params.Page
	}
	if request.Params.Limit != nil {
		in.Limit = *request.Params.Limit
	}
	v, err := h.app.Queries.List.Handle(ctx, in)
	if err != nil {
		return nil, err
	}
	out := openapi.ListWordImports200JSONResponse{Items: make([]openapi.WordImport, len(v.Items)), Page: v.Page.Number, PageSize: v.Page.Size, Total: v.Page.Total}
	for i, v := range v.Items {
		out.Items[i] = toImport(v)
	}
	return out, nil
}
func (h Imports) UploadImportSource(ctx context.Context, request openapi.UploadImportSourceRequestObject) (openapi.UploadImportSourceResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	a, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	if request.Body == nil {
		return importFailure(ctx, errMultipart)
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		return importFailure(ctx, domain.ErrBusy)
	}
	part, err := request.Body.NextPart()
	if err != nil {
		return importFailure(ctx, multipartError(err))
	}
	if part.FormName() != "file" || part.FileName() == "" {
		return importFailure(ctx, errMultipart)
	}
	v, err := h.app.Commands.Upload.Handle(ctx, command.Upload{ImportID: request.Id.String(), UploadID: request.Params.UploadId.String(), ExpectedRevision: request.Params.ExpectedRevision, Role: string(request.Params.Role), Filename: part.FileName(), Body: part, Actor: actor.Actor(a), End: func() error { return endMultipart(request.Body) }})
	if err != nil {
		return importFailure(ctx, bodyError(err))
	}
	return openapi.UploadImportSource201JSONResponse{Import: toImport(v.Import), Source: toSource(v.Source), SourceRevision: v.Source.SourceRevision}, nil
}
func (h Imports) DownloadImportSource(ctx context.Context, request openapi.DownloadImportSourceRequestObject) (openapi.DownloadImportSourceResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	v, err := h.app.Queries.Download.Handle(ctx, query.Download{ImportID: request.Id.String(), SourceID: request.SourceId.String()})
	if err != nil {
		return importFailure(ctx, err)
	}
	return openapi.DownloadImportSource200JSONResponse{Url: v.URL, ExpiresAt: v.ExpiresAt}, nil
}

func endMultipart(reader *multipart.Reader) error {
	_, err := reader.NextPart()
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return multipartError(err)
	}
	return errMultipart
}
func multipartError(err error) error {
	if errors.Is(bodyError(err), domain.ErrTooLarge) {
		return domain.ErrTooLarge
	}
	return errMultipart
}
func bodyError(err error) error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return domain.ErrTooLarge
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return errMultipart
	}
	if errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return domain.ErrBusy
	}
	return err
}
func toImport(v domain.Import) openapi.WordImport {
	out := openapi.WordImport{Id: httpapi.ParseUUID(v.ID), Title: v.Title, Status: openapi.ImportStatus(v.Status), Revision: v.Revision, SourceRevision: v.SourceRevision, CreatedBy: httpapi.ParseUUID(v.CreatedBy), CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, PendingUploads: v.PendingUploads, Sources: make([]openapi.ImportSource, len(v.Sources))}
	for i, s := range v.Sources {
		out.Sources[i] = toSource(s)
	}
	if v.DraftRevision > 0 {
		revision := v.DraftRevision
		out.DraftRevision = &revision
	}
	if v.TestID != nil {
		id := httpapi.ParseUUID(*v.TestID)
		out.TestId = &id
	}
	if r := v.Run; r != nil {
		out.Run = &openapi.ImportRun{Id: httpapi.ParseUUID(r.ID), Status: openapi.ImportRunStatus(r.Status), Stage: openapi.ImportRunStage(r.Stage), Attempt: r.Attempt, MaxAttempts: r.MaxAttempts, ErrorCode: r.ErrorCode, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
		if r.Profile.KeyPaper > 0 {
			out.Run.KeyPaper = &r.Profile.KeyPaper
		}
	}
	return out
}
func toSource(s domain.Source) openapi.ImportSource {
	return openapi.ImportSource{Id: httpapi.ParseUUID(s.ID), Role: openapi.ImportSourceRole(s.Role), Filename: s.Filename, Format: openapi.ImportSourceFormat(s.Format), Bytes: s.Bytes, Sha256: hex.EncodeToString(s.SHA256), UploadedBy: httpapi.ParseUUID(s.UploadedBy), CreatedAt: s.CreatedAt}
}

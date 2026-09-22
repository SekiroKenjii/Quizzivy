package http

import (
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

func (h Tests) DeleteTestVersion(ctx context.Context, request openapi.DeleteTestVersionRequestObject) (openapi.DeleteTestVersionResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := testRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	_, err := h.app.Commands.DeleteVersion.Handle(ctx, command.DeleteVersion{Request: domain.VersionRequest{Request: req, Version: request.Version}})
	if err == nil {
		return openapi.DeleteTestVersion204Response{}, nil
	}
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.DeleteTestVersion404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgTestNotFound))}, nil
	}
	if response, ok := versionConflict(ctx, err); ok {
		return openapi.DeleteTestVersion409JSONResponse(response), nil
	}
	return nil, err
}

func (h Tests) SetCurrentTestVersion(ctx context.Context, request openapi.SetCurrentTestVersionRequestObject) (openapi.SetCurrentTestVersionResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := testRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	result, err := h.app.Commands.SetCurrentVersion.Handle(ctx, command.SetCurrentVersion{Request: domain.VersionRequest{Request: req, Version: request.Version, ExpectedUpdatedAt: request.Body.ExpectedUpdatedAt}})
	return versionReply(ctx, result, err)
}

func (h Tests) CreateDraftFromTestVersion(ctx context.Context, request openapi.CreateDraftFromTestVersionRequestObject) (openapi.CreateDraftFromTestVersionResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := testRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	result, err := h.app.Commands.CreateDraftFromVersion.Handle(ctx, command.CreateDraftFromVersion{Request: domain.VersionRequest{Request: req, Version: request.Version, ExpectedUpdatedAt: request.Body.ExpectedUpdatedAt}})
	return versionReply(ctx, result, err)
}

type versionChangeResponse struct {
	status  int
	test    *openapi.Test
	problem *openapi.ErrorResponse
}

func versionReply(ctx context.Context, result domain.Test, err error) (*versionChangeResponse, error) {
	if errors.Is(err, domain.ErrNotFound) {
		problem := httpapi.NotFound(ctx, msgTestNotFound)
		return &versionChangeResponse{status: nethttp.StatusNotFound, problem: &problem}, nil
	}
	if problem, ok := versionConflict(ctx, err); ok {
		return &versionChangeResponse{status: nethttp.StatusConflict, problem: &problem}, nil
	}
	if err != nil {
		return nil, err
	}
	out, err := toAPITest(result)
	return &versionChangeResponse{status: nethttp.StatusOK, test: &out}, err
}

func (r *versionChangeResponse) VisitSetCurrentTestVersionResponse(w nethttp.ResponseWriter) error {
	return r.write(w)
}

func (r *versionChangeResponse) VisitCreateDraftFromTestVersionResponse(w nethttp.ResponseWriter) error {
	return r.write(w)
}

func (r *versionChangeResponse) write(w nethttp.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(r.status)
	if r.problem != nil {
		return json.NewEncoder(w).Encode(r.problem)
	}
	return json.NewEncoder(w).Encode(r.test)
}

func versionConflict(ctx context.Context, err error) (openapi.ErrorResponse, bool) {
	switch {
	case errors.Is(err, domain.ErrStaleWrite):
		return httpapi.Error(ctx, openapi.STALEWRITE, "Đề đã được sửa ở nơi khác. Vui lòng tải lại trước khi tiếp tục."), true
	case errors.Is(err, domain.ErrCurrentVersion):
		return httpapi.Error(ctx, openapi.VERSIONISCURRENT, "Hãy chọn một phiên bản mặc định khác trước khi xoá phiên bản này."), true
	case errors.Is(err, domain.ErrReferenced):
		return httpapi.Error(ctx, openapi.RESOURCEREFERENCED, "Phiên bản đã được bài giao hoặc bài làm sử dụng nên không thể xoá."), true
	default:
		return openapi.ErrorResponse{}, false
	}
}

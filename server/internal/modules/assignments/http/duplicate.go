package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/assignments/application/command"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"time"
)

// DuplicateAssignment copies an assignment as a draft assigned to the classes
// named.
func (h Assignments) DuplicateAssignment(ctx context.Context, request openapi.DuplicateAssignmentRequestObject) (openapi.DuplicateAssignmentResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := assignmentRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	classIDs := make([]string, len(request.Body.ClassIds))
	for i, id := range request.Body.ClassIds {
		classIDs[i] = id.String()
	}

	copied, err := h.app.Commands.Duplicate.Handle(ctx, command.Duplicate{Request: req, ClassIDs: classIDs, Now: time.Now()})
	var invalid *domain.ValidationError
	switch {
	case err == nil:
	case errors.As(err, &invalid):
		return openapi.DuplicateAssignment400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			assignmentValidationError(ctx, invalid))}, nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.DuplicateAssignment404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgAssignmentNotFound(ctx)))}, nil
	case errors.Is(err, domain.ErrTestNotPublished):
		return openapi.DuplicateAssignment409JSONResponse(httpapi.Error(ctx, openapi.TESTNOTPUBLISHED,
			httpx.Text(ctx, "Chỉ có thể giao một phiên bản đề đã xuất bản.", "Only a published version of a test can be assigned."))), nil
	default:
		return nil, err
	}
	return openapi.DuplicateAssignment201JSONResponse(toAPIAssignment(copied)), nil
}

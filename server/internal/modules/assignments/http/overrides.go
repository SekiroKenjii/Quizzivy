package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/assignments/application/command"
	"quizzivy/internal/modules/assignments/application/query"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"strings"
	"time"
)

func msgAssignmentClosed(ctx context.Context) string {
	return httpx.Text(ctx, "Bài giao đã đóng nên không thể gia hạn. Hãy mở lại bài giao.",
		"The assignment has closed, so it cannot be extended. Reopen it instead.")
}

// ExtendAssignment moves the close of an assignment that has not closed later,
// for everyone.
func (h Assignments) ExtendAssignment(ctx context.Context, request openapi.ExtendAssignmentRequestObject) (openapi.ExtendAssignmentResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := assignmentRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	a, err := h.app.Commands.Extend.Handle(ctx, command.Extend{
		Request: req, Minutes: request.Body.Minutes,
		Notify: request.Body.Notify != nil && *request.Body.Notify, Now: time.Now(),
	})
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrNotFound):
		return openapi.ExtendAssignment404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgAssignmentNotFound(ctx)))}, nil
	case errors.Is(err, domain.ErrClosed):
		return openapi.ExtendAssignment409JSONResponse(httpapi.Error(ctx, openapi.ASSIGNMENTCLOSED, msgAssignmentClosed(ctx))), nil
	default:
		return nil, err
	}
	return openapi.ExtendAssignment200JSONResponse(toAPIAssignment(a)), nil
}

// ListStudentOverrides lists the overrides on an assignment, for the students
// the caller reaches.
func (h Assignments) ListStudentOverrides(ctx context.Context, request openapi.ListStudentOverridesRequestObject) (openapi.ListStudentOverridesResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	found, err := h.app.Queries.Overrides.Handle(ctx, query.Overrides{
		AssignmentID: request.Id.String(), Scope: httpapi.ScopeFromContext(ctx),
	})
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.ListStudentOverrides404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgAssignmentNotFound(ctx)))}, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.ListStudentOverrides200JSONResponse{Items: toAPIOverrides(found)}, nil
}

// SetStudentOverrides gives students an override on an assignment, or changes
// the one they have.
func (h Assignments) SetStudentOverrides(ctx context.Context, request openapi.SetStudentOverridesRequestObject) (openapi.SetStudentOverridesResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := assignmentRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	written, err := h.app.Commands.SetOverrides.Handle(ctx, command.SetOverrides{Request: req, Input: toOverrideInput(*request.Body)})
	var invalid *domain.ValidationError
	var notTargeted *domain.NotTargetedError
	switch {
	case err == nil:
	case errors.As(err, &invalid):
		return openapi.SetStudentOverrides400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			assignmentValidationError(ctx, invalid))}, nil
	case errors.As(err, &notTargeted):
		return openapi.SetStudentOverrides422JSONResponse(notTargetedError(ctx, notTargeted)), nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.SetStudentOverrides404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgAssignmentNotFound(ctx)))}, nil
	case errors.Is(err, domain.ErrClosed):
		return openapi.SetStudentOverrides409JSONResponse(httpapi.Error(ctx, openapi.ASSIGNMENTCLOSED,
			httpx.Text(ctx, "Có học viên đã hết thời hạn nên không thể gia hạn thêm. Hãy đặt thời điểm đóng mới.",
				"A student's close has passed, so it cannot be extended. Set a new closing time."))), nil
	default:
		return nil, err
	}
	return openapi.SetStudentOverrides200JSONResponse{Items: toAPIOverrides(written)}, nil
}

// DeleteStudentOverride takes one student's override off an assignment.
func (h Assignments) DeleteStudentOverride(ctx context.Context, request openapi.DeleteStudentOverrideRequestObject) (openapi.DeleteStudentOverrideResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := assignmentRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	_, err := h.app.Commands.DeleteOverride.Handle(ctx, command.DeleteOverride{
		Request: req, StudentID: request.StudentId.String(), Now: time.Now(),
	})
	switch {
	case err == nil:
		return openapi.DeleteStudentOverride204Response{}, nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.DeleteStudentOverride404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy ngoại lệ của học viên này.", "The student's override was not found.")))}, nil
	default:
		return nil, err
	}
}

func toOverrideInput(body openapi.StudentOverrideInput) domain.OverrideInput {
	in := domain.OverrideInput{
		ExtendBy:      body.ExtendBy,
		ClosesAt:      body.ClosesAt,
		DurationMin:   body.DurationMinutes,
		ExtraAttempts: body.ExtraAttempts,
		Reason:        body.Reason,
		Notify:        body.Notify != nil && *body.Notify,
		Now:           time.Now(),
	}
	for _, id := range body.StudentIds {
		in.StudentIDs = append(in.StudentIDs, id.String())
	}
	return in
}

func notTargetedError(ctx context.Context, e *domain.NotTargetedError) openapi.ErrorResponse {
	return httpapi.ErrorWithDetails(ctx, openapi.VALIDATIONFAILED,
		httpx.Text(ctx, "Có học viên không thuộc bài giao này.", "Some students are not on this assignment."),
		map[string]interface{}{"studentIds": httpx.Text(ctx, "Không thuộc bài giao này: ", "Not on this assignment: ") + strings.Join(e.StudentIDs, ", ")})
}

func toAPIOverrides(found []domain.StudentOverride) []openapi.StudentOverride {
	out := make([]openapi.StudentOverride, len(found))
	for i, o := range found {
		out[i] = openapi.StudentOverride{
			StudentId:       httpapi.ParseUUID(o.StudentID),
			StudentName:     o.StudentName,
			ClosesAt:        o.ClosesAt,
			DurationMinutes: o.DurationMin,
			ExtraAttempts:   o.ExtraAttempts,
			Reason:          o.Reason,
			CreatedAt:       o.CreatedAt,
			UpdatedAt:       o.UpdatedAt,
		}
	}
	return out
}

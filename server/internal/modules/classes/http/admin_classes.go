package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/validation"
)

func invalidTeacher(ctx context.Context) openapi.ErrorResponse {
	return httpapi.Error(ctx, openapi.INVALIDRECIPIENT, httpx.Text(ctx,
		"Giáo viên phải là một tài khoản đang hoạt động có quyền vào không gian giáo viên.",
		"The teacher must be an active account with the teacher workspace."))
}

// ListAllClasses implements GET /admin/classes.
func (h Classes) ListAllClasses(ctx context.Context, request openapi.ListAllClassesRequestObject) (openapi.ListAllClassesResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	in := domain.AdminListInput{}
	if request.Params.Tab != nil {
		in.Tab = string(*request.Params.Tab)
	}
	if request.Params.Q != nil {
		in.Query = string(*request.Params.Q)
	}
	in.DisabledTeacher = request.Params.Teacher != nil && *request.Params.Teacher == openapi.ListAllClassesParamsTeacherDisabled
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	result, err := h.app.Queries.ListAll.Handle(ctx, query.ListAll{Input: in})
	if err != nil {
		return nil, err
	}
	items := make([]openapi.AdminClassRow, 0, len(result.Items))
	for _, c := range result.Items {
		items = append(items, toAPIAdminClassRow(c))
	}
	cache := noStore
	var out openapi.ListAllClasses200JSONResponse
	out.Headers.CacheControl = &cache
	out.Body.Items, out.Body.Page, out.Body.PageSize, out.Body.Total = items, result.Page.Number, result.Page.Size, result.Page.Total
	out.Body.Facets = openapi.AdminClassFacets{Active: result.Facets.Active, Archived: result.Facets.Archived}
	out.Body.DisabledTeacherClasses = result.Facets.DisabledTeacherClasses
	return out, nil
}

// CreateClassForTeacher implements POST /admin/classes.
func (h Classes) CreateClassForTeacher(ctx context.Context, request openapi.CreateClassForTeacherRequestObject) (openapi.CreateClassForTeacherResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	who, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	class, err := h.app.Commands.CreateForTeacher.Handle(ctx, command.CreateForTeacher{
		Name: request.Body.Name, TeacherID: request.Body.TeacherId.String(),
		ScheduleLabel: request.Body.ScheduleLabel, Room: request.Body.Room, Actor: who,
	})
	var invalid *validation.Error
	switch {
	case err == nil:
		return openapi.CreateClassForTeacher201JSONResponse(toAPIAdminClass(class)), nil
	case errors.As(err, &invalid):
		return openapi.CreateClassForTeacher400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			httpapi.Invalid(ctx, msgClassInvalid(ctx), invalid))}, nil
	case errors.Is(err, domain.ErrInvalidTeacher):
		return openapi.CreateClassForTeacher422JSONResponse(invalidTeacher(ctx)), nil
	}
	return nil, err
}

// ChangeClassTeacher implements PUT /admin/classes/{id}/teacher.
func (h Classes) ChangeClassTeacher(ctx context.Context, request openapi.ChangeClassTeacherRequestObject) (openapi.ChangeClassTeacherResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	who, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	class, err := h.app.Commands.ChangeTeacher.Handle(ctx, command.ChangeTeacher{
		ClassID: request.Id.String(), TeacherID: request.Body.TeacherId.String(),
		Notify: request.Body.Notify != nil && *request.Body.Notify, Actor: who,
	})
	switch {
	case err == nil:
		return openapi.ChangeClassTeacher200JSONResponse(toAPIAdminClass(class)), nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.ChangeClassTeacher404JSONResponse{
			NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgClassNotFound(ctx))),
		}, nil
	case errors.Is(err, domain.ErrInvalidTeacher):
		return openapi.ChangeClassTeacher422JSONResponse(invalidTeacher(ctx)), nil
	}
	return nil, err
}

func toAPIAdminClassRow(c domain.AdminClass) openapi.AdminClassRow {
	out := openapi.AdminClassRow{
		Id:            httpapi.ParseUUID(c.ID),
		Name:          c.Name,
		ScheduleLabel: c.ScheduleLabel,
		Room:          c.Room,
		StudentCount:  c.StudentCount,
		CreatedAt:     c.CreatedAt,
	}
	if t := c.Teacher; t != nil {
		out.Teacher = &openapi.AdminClassTeacher{
			Id: httpapi.ParseUUID(t.ID), Name: t.Name, AvatarUrl: t.AvatarURL, Disabled: t.Disabled,
		}
	}
	if jc := c.JoinCode; jc != nil && c.Code != "" {
		out.JoinCode = &openapi.AdminClassJoinCode{
			Code:      domain.JoinCodes.Format(c.Code),
			ExpiresAt: jc.ExpiresAt,
			MaxUses:   jc.MaxUses,
			UsesCount: jc.UsesCount,
		}
	}
	return out
}

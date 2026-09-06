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

	openapi_types "github.com/oapi-codegen/runtime/types"
)

const msgClassNotFound = "Không tìm thấy lớp học."

// GetClass implements GET /admin/classes/{id} (§6.4).
func (h Classes) GetClass(ctx context.Context, request openapi.GetClassRequestObject) (openapi.GetClassResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	class, err := h.app.Queries.Get.Handle(ctx, query.Get{ClassID: request.Id.String()})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return openapi.GetClass404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgClassNotFound)),
			}, nil
		}
		return nil, err
	}
	return openapi.GetClass200JSONResponse(toAPIAdminClass(class)), nil
}

// UpdateClass implements PATCH /admin/classes/{id}.
func (h Classes) UpdateClass(ctx context.Context, request openapi.UpdateClassRequestObject) (openapi.UpdateClassResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := domain.UpdateInput{}
	if request.Body.Name != nil {
		in.Name = request.Body.Name
	}
	if request.Body.Description != nil {
		in.Description = request.Body.Description
	}
	if request.Body.SelfJoinEnabled != nil {
		in.SelfJoinEnabled = request.Body.SelfJoinEnabled
	}

	class, err := h.app.Commands.Update.Handle(ctx, command.Update{ClassID: request.Id.String(), Input: in})
	if err == nil && request.Body.Archived != nil {
		class, err = h.app.Commands.Archive.Handle(ctx, command.Archive{ClassID: request.Id.String(), Archived: *request.Body.Archived, ActorID: httpapi.ActorID(ctx), IP: httpx.RequestMetaFromContext(ctx).IP, UserAgent: httpx.RequestMetaFromContext(ctx).UserAgent})
	}
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return openapi.UpdateClass404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgClassNotFound)),
			}, nil
		}
		return nil, err
	}
	return openapi.UpdateClass200JSONResponse(toAPIAdminClass(class)), nil
}

func (h Classes) CreateClass(ctx context.Context, request openapi.CreateClassRequestObject) (openapi.CreateClassResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	selfJoin := true
	if request.Body.SelfJoinEnabled != nil {
		selfJoin = *request.Body.SelfJoinEnabled
	}
	meta := httpx.RequestMetaFromContext(ctx)
	class, err := h.app.Commands.Create.Handle(ctx, command.Create{Name: request.Body.Name, Description: request.Body.Description, SelfJoin: selfJoin, ActorID: httpapi.ActorID(ctx), IP: meta.IP, UserAgent: meta.UserAgent})
	if err != nil {
		return nil, err
	}
	return openapi.CreateClass201JSONResponse(toAPIAdminClass(class)), nil
}

// ListClasses implements GET /admin/classes.
func (h Classes) ListClasses(ctx context.Context, request openapi.ListClassesRequestObject) (openapi.ListClassesResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	in := domain.ListInput{}
	if request.Params.Q != nil {
		in.Query = string(*request.Params.Q)
	}
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}
	if request.Params.Status != nil {
		in.Status = string(*request.Params.Status)
	}

	listResult, err := h.app.Queries.List.Handle(ctx, query.List{Input: in})
	found, page := listResult.Items, listResult.Page
	if err != nil {
		return nil, err
	}
	facets, err := h.app.Queries.Facets.Handle(ctx, query.Facets{Query: in.Query})
	if err != nil {
		return nil, err
	}
	items := make([]openapi.Class, 0, len(found))
	for _, c := range found {
		items = append(items, toAPIAdminClass(c))
	}
	return openapi.ListClasses200JSONResponse{
		Items: items, Page: page.Number, PageSize: page.Size, Total: page.Total,
		Facets: openapi.ClassFacets{
			All: facets.All, Joinable: facets.Joinable, Archived: facets.Archived, Students: facets.Students,
		},
	}, nil
}

// ListClassMembers implements GET /admin/classes/{id}/members (§6.4).
func (h Classes) ListClassMembers(ctx context.Context, request openapi.ListClassMembersRequestObject) (openapi.ListClassMembersResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	in := domain.MembersInput{}
	if request.Params.Q != nil {
		in.Query = string(*request.Params.Q)
	}
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	membersResult, err := h.app.Queries.Members.Handle(ctx, query.Members{ClassID: request.Id.String(), Input: in})
	found, page := membersResult.Items, membersResult.Page
	if err != nil {
		return nil, err
	}
	items := make([]openapi.ClassMember, 0, len(found))
	for _, m := range found {
		items = append(items, toAPIMember(m))
	}
	return openapi.ListClassMembers200JSONResponse{
		Items: items, Page: page.Number, PageSize: page.Size, Total: page.Total,
	}, nil
}

func (h Classes) AddClassMember(ctx context.Context, request openapi.AddClassMemberRequestObject) (openapi.AddClassMemberResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	meta := httpx.RequestMetaFromContext(ctx)

	m, err := h.app.Commands.AddMember.Handle(ctx, command.AddMember{ClassID: request.Id.String(), UserID: request.Body.UserId.String(), ActorID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent})
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrNotFound):
		return openapi.AddClassMember404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgClassNotFound))}, nil
	case errors.Is(err, domain.ErrNotAStudent):
		return openapi.AddClassMember400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			httpapi.Error(ctx, openapi.VALIDATIONFAILED, "Chỉ có thể thêm tài khoản học viên vào lớp."))}, nil
	default:
		return nil, err
	}

	return openapi.AddClassMember201JSONResponse(toAPIMember(m)), nil
}

func toAPIMember(m domain.Member) openapi.ClassMember {
	return openapi.ClassMember{
		UserId:   httpapi.ParseUUID(m.UserID),
		FullName: m.FullName,
		Email:    openapi_types.Email(m.Email),

		JoinedVia:    openapi.ClassMemberJoinedVia(m.JoinedVia),
		JoinedAt:     m.JoinedAt,
		JoinCodeHint: m.JoinCodeHint,
		Stats:        httpapi.StudentStats(m.Stats),
	}
}

// RemoveClassMember implements DELETE /admin/classes/{id}/members/{userId}.
func (h Classes) RemoveClassMember(ctx context.Context, request openapi.RemoveClassMemberRequestObject) (openapi.RemoveClassMemberResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	meta := httpx.RequestMetaFromContext(ctx)

	_, err := h.app.Commands.RemoveMember.Handle(ctx, command.RemoveMember{ClassID: request.Id.String(), UserID: request.UserId.String(), ActorID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return openapi.RemoveClassMember404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgClassNotFound)),
			}, nil
		}
		return nil, err
	}
	return openapi.RemoveClassMember204Response{}, nil
}

func toAPIAdminClass(c domain.Class) openapi.Class {
	out := openapi.Class{
		Id:                  httpapi.ParseUUID(c.ID),
		Name:                c.Name,
		Description:         c.Description,
		StudentCount:        c.StudentCount,
		OpenAssignmentCount: c.OpenAssignmentCount,
		SelfJoinEnabled:     c.SelfJoinEnabled,
		ArchivedAt:          c.ArchivedAt,
		CreatedAt:           c.CreatedAt,
	}
	if jc := c.JoinCode; jc != nil {
		out.JoinCode = &openapi.JoinCodeInfo{
			Hint:      jc.Hint,
			ExpiresAt: jc.ExpiresAt,
			MaxUses:   jc.MaxUses,
			UsesCount: jc.UsesCount,
		}
	}
	return out
}

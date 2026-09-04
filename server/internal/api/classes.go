package api

import (
	"context"
	"errors"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/classes"
	"quizzivy/internal/httpx"
)

const msgClassNotFound = "Không tìm thấy lớp học."

// GetClass implements GET /admin/classes/{id} (§6.4).
//
// Carries the active code's METADATA -- hint, expiry, uses -- and never the
// code. Only a hash is stored (§13.3), so there is nothing here that could
// return it even if a handler wanted to.
func (s *Server) GetClass(ctx context.Context, request openapi.GetClassRequestObject) (openapi.GetClassResponseObject, error) {
	if s.Deps.Classes == nil {
		return nil, httpx.ErrNotImplemented
	}
	class, err := s.Deps.Classes.Get(ctx, request.Id.String())
	if err != nil {
		if errors.Is(err, classes.ErrNotFound) {
			return openapi.GetClass404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(notFound(ctx, msgClassNotFound)),
			}, nil
		}
		return nil, err
	}
	return openapi.GetClass200JSONResponse(toAPIAdminClass(class)), nil
}

// UpdateClass implements PATCH /admin/classes/{id}.
//
// Only the fields actually present in the body are written, so renaming a class
// cannot silently clear its description -- the difference between "absent" and
// "null" is the whole point of a PATCH.
func (s *Server) UpdateClass(ctx context.Context, request openapi.UpdateClassRequestObject) (openapi.UpdateClassResponseObject, error) {
	if s.Deps.Classes == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := classes.UpdateInput{}
	if request.Body.Name != nil {
		in.Name = request.Body.Name
	}
	if request.Body.Description != nil {
		in.Description = request.Body.Description
	}
	if request.Body.SelfJoinEnabled != nil {
		in.SelfJoinEnabled = request.Body.SelfJoinEnabled
	}

	class, err := s.Deps.Classes.Update(ctx, request.Id.String(), in)
	if err == nil && request.Body.Archived != nil {
		class, err = s.Deps.Classes.Archive(ctx, request.Id.String(), *request.Body.Archived,
			actorID(ctx), httpx.RequestMetaFromContext(ctx).IP, httpx.RequestMetaFromContext(ctx).UserAgent)
	}
	if err != nil {
		if errors.Is(err, classes.ErrNotFound) {
			return openapi.UpdateClass404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(notFound(ctx, msgClassNotFound)),
			}, nil
		}
		return nil, err
	}
	return openapi.UpdateClass200JSONResponse(toAPIAdminClass(class)), nil
}

func (s *Server) CreateClass(ctx context.Context, request openapi.CreateClassRequestObject) (openapi.CreateClassResponseObject, error) {
	if s.Deps.Classes == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	selfJoin := true
	if request.Body.SelfJoinEnabled != nil {
		selfJoin = *request.Body.SelfJoinEnabled
	}
	meta := httpx.RequestMetaFromContext(ctx)
	class, err := s.Deps.Classes.Create(ctx, request.Body.Name, request.Body.Description, selfJoin,
		actorID(ctx), meta.IP, meta.UserAgent)
	if err != nil {
		return nil, err
	}
	return openapi.CreateClass201JSONResponse(toAPIAdminClass(class)), nil
}

func actorID(ctx context.Context) string {
	principal, _ := httpx.PrincipalFromContext(ctx)
	return principal.UserID
}

// ListClasses implements GET /admin/classes.
func (s *Server) ListClasses(ctx context.Context, request openapi.ListClassesRequestObject) (openapi.ListClassesResponseObject, error) {
	if s.Deps.Classes == nil {
		return nil, httpx.ErrNotImplemented
	}
	in := classes.ListInput{}
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

	found, page, err := s.Deps.Classes.List(ctx, in)
	if err != nil {
		return nil, err
	}
	facets, err := s.Deps.Classes.Facets(ctx, in.Query)
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
func (s *Server) ListClassMembers(ctx context.Context, request openapi.ListClassMembersRequestObject) (openapi.ListClassMembersResponseObject, error) {
	if s.Deps.Classes == nil {
		return nil, httpx.ErrNotImplemented
	}
	in := classes.MembersInput{}
	if request.Params.Q != nil {
		in.Query = string(*request.Params.Q)
	}
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	found, page, err := s.Deps.Classes.Members(ctx, request.Id.String(), in)
	if err != nil {
		return nil, err
	}
	items := make([]openapi.ClassMember, 0, len(found))
	for _, m := range found {
		items = append(items, openapi.ClassMember{
			UserId:   parseUUID(m.UserID),
			FullName: m.FullName,
			Email:    openapi_types.Email(m.Email),
			// The teacher's own signal for an unexpected enrolment (§6.4).
			JoinedVia:    openapi.ClassMemberJoinedVia(m.JoinedVia),
			JoinedAt:     m.JoinedAt,
			JoinCodeHint: m.JoinCodeHint,
		})
	}
	return openapi.ListClassMembers200JSONResponse{
		Items: items, Page: page.Number, PageSize: page.Size, Total: page.Total,
	}, nil
}

func (s *Server) AddClassMember(ctx context.Context, request openapi.AddClassMemberRequestObject) (openapi.AddClassMemberResponseObject, error) {
	if s.Deps.Classes == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	meta := httpx.RequestMetaFromContext(ctx)

	m, err := s.Deps.Classes.AddMember(ctx, request.Id.String(), request.Body.UserId.String(),
		principal.UserID, meta.IP, meta.UserAgent)
	switch {
	case err == nil:
	case errors.Is(err, classes.ErrNotFound):
		return openapi.AddClassMember404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, msgClassNotFound))}, nil
	case errors.Is(err, classes.ErrNotAStudent):
		return openapi.AddClassMember400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			authError(ctx, openapi.VALIDATIONFAILED, "Chỉ có thể thêm tài khoản học viên vào lớp."))}, nil
	default:
		return nil, err
	}

	return openapi.AddClassMember201JSONResponse{
		UserId:       parseUUID(m.UserID),
		FullName:     m.FullName,
		Email:        openapi_types.Email(m.Email),
		JoinedVia:    openapi.ClassMemberJoinedVia(m.JoinedVia),
		JoinedAt:     m.JoinedAt,
		JoinCodeHint: m.JoinCodeHint,
	}, nil
}

// RemoveClassMember implements DELETE /admin/classes/{id}/members/{userId}.
//
// Revokes access and RETAINS attempts (§6.4). The membership grants access;
// the attempts are the student's work and the teacher's record of it, and
// deleting those because someone left a class would destroy the only evidence
// of what happened.
func (s *Server) RemoveClassMember(ctx context.Context, request openapi.RemoveClassMemberRequestObject) (openapi.RemoveClassMemberResponseObject, error) {
	if s.Deps.Classes == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	meta := httpx.RequestMetaFromContext(ctx)

	err := s.Deps.Classes.RemoveMember(ctx, request.Id.String(), request.UserId.String(),
		principal.UserID, meta.IP, meta.UserAgent)
	if err != nil {
		if errors.Is(err, classes.ErrNotFound) {
			return openapi.RemoveClassMember404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(notFound(ctx, msgClassNotFound)),
			}, nil
		}
		return nil, err
	}
	return openapi.RemoveClassMember204Response{}, nil
}

func toAPIAdminClass(c classes.Class) openapi.Class {
	out := openapi.Class{
		Id:                  parseUUID(c.ID),
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

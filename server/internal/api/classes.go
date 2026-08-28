package api

import (
	"context"
	"errors"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/classes"
	"quizzivy/internal/httpx"
)

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
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(notFound(ctx, "Không tìm thấy lớp học.")),
			}, nil
		}
		return nil, err
	}
	return openapi.GetClass200JSONResponse(toAPIAdminClass(class)), nil
}

// ListClasses implements GET /admin/classes.
func (s *Server) ListClasses(ctx context.Context, _ openapi.ListClassesRequestObject) (openapi.ListClassesResponseObject, error) {
	if s.Deps.Classes == nil {
		return nil, httpx.ErrNotImplemented
	}
	found, err := s.Deps.Classes.List(ctx)
	if err != nil {
		return nil, err
	}
	// Never nil: the contract types `items` as an array, and a null would make
	// every consumer handle a case that only exists because Go's zero slice is
	// nil.
	items := make([]openapi.Class, 0, len(found))
	for _, c := range found {
		items = append(items, toAPIAdminClass(c))
	}
	return openapi.ListClasses200JSONResponse{Items: items}, nil
}

// ListClassMembers implements GET /admin/classes/{id}/members (§6.4).
func (s *Server) ListClassMembers(ctx context.Context, request openapi.ListClassMembersRequestObject) (openapi.ListClassMembersResponseObject, error) {
	if s.Deps.Classes == nil {
		return nil, httpx.ErrNotImplemented
	}
	found, err := s.Deps.Classes.Members(ctx, request.Id.String())
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
	return openapi.ListClassMembers200JSONResponse{Items: items}, nil
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
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(notFound(ctx, "Không tìm thấy lớp học.")),
			}, nil
		}
		return nil, err
	}
	return openapi.RemoveClassMember204Response{}, nil
}

func toAPIAdminClass(c classes.Class) openapi.Class {
	out := openapi.Class{
		Id:              parseUUID(c.ID),
		Name:            c.Name,
		Description:     c.Description,
		StudentCount:    c.StudentCount,
		SelfJoinEnabled: c.SelfJoinEnabled,
		CreatedAt:       c.CreatedAt,
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

package api

import (
	"context"
	"errors"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
	"quizzivy/internal/students"
)

// ListStudents backs §8's students table and the two pickers that add a student
// to a class (G-06) or to an assignment (G-01).
func (s *Server) ListStudents(ctx context.Context, request openapi.ListStudentsRequestObject) (openapi.ListStudentsResponseObject, error) {
	if s.Deps.Students == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := students.ListInput{}
	if request.Params.Q != nil {
		in.Query = string(*request.Params.Q)
	}
	if request.Params.ClassId != nil {
		in.ClassID = request.Params.ClassId.String()
	}
	if request.Params.Cursor != nil {
		in.Cursor = *request.Params.Cursor
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	found, next, err := s.Deps.Students.List(ctx, in)
	if errors.Is(err, students.ErrBadCursor) {
		return openapi.ListStudents400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			authError(ctx, openapi.VALIDATIONFAILED, "Con trỏ phân trang không hợp lệ."))}, nil
	}
	if err != nil {
		return nil, err
	}

	out := openapi.ListStudents200JSONResponse{Items: make([]openapi.User, len(found))}
	for i, st := range found {
		providers := make([]openapi.UserLinkedProviders, 0, len(st.LinkedProviders))
		for _, p := range st.LinkedProviders {
			providers = append(providers, openapi.UserLinkedProviders(p))
		}
		out.Items[i] = openapi.User{
			Id:                 parseUUID(st.ID),
			Email:              openapi_types.Email(st.Email),
			FullName:           st.FullName,
			Role:               openapi.RoleStudent,
			HasPassword:        st.HasPassword,
			LinkedProviders:    providers,
			MustChangePassword: st.MustChangePassword,
			CreatedAt:          st.CreatedAt,
		}
	}
	if next != "" {
		out.NextCursor = &next
	}
	return out, nil
}

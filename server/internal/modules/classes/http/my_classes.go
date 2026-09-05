package http

import (
	"context"
	"quizzivy/gen/openapi"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

// ListMyClasses backs §9's /app/classes in the student's own shape (S-10).
func (h Classes) ListMyClasses(ctx context.Context, _ openapi.ListMyClassesRequestObject) (openapi.ListMyClassesResponseObject, error) {
	if h.classes == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	found, err := h.classes.ListMine(ctx, principal.UserID)
	if err != nil {
		return nil, err
	}
	items := make([]openapi.MyClass, 0, len(found))
	for _, c := range found {
		items = append(items, openapi.MyClass{
			Id:          httpapi.ParseUUID(c.ID),
			Name:        c.Name,
			Description: c.Description,
			TeacherName: c.TeacherName,
			JoinedAt:    c.JoinedAt,
		})
	}
	return openapi.ListMyClasses200JSONResponse{Items: items}, nil
}

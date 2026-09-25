package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"strconv"
)

// PublishTest validates the draft and freezes it as a new version.
func (h Tests) PublishTest(ctx context.Context, request openapi.PublishTestRequestObject) (openapi.PublishTestResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	version, err := h.app.Commands.Publish.Handle(ctx, command.Publish{Request: domain.PublishRequest{
		TestID:    request.Id.String(),
		ActorID:   principal.UserID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	}})
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrDraftNotFound):
		return openapi.PublishTest404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, "Không tìm thấy đề."))}, nil
	case errors.Is(err, domain.ErrNoContent):
		return publishViolations(ctx, []domain.Violation{{
			Rule:    domain.SectionNotEmpty,
			Message: "Đề chưa có phần nào để xuất bản.",
		}}), nil
	default:
		var invalid *domain.PublishValidationError
		if errors.As(err, &invalid) {
			return publishViolations(ctx, invalid.Violations), nil
		}
		return nil, err
	}

	points, err := strconv.ParseFloat(version.TotalPoints, 64)
	if err != nil {
		return nil, err
	}
	return openapi.PublishTest201JSONResponse{
		Id:            httpapi.ParseUUID(version.ID),
		Version:       version.Version,
		TotalPoints:   points,
		QuestionCount: version.QuestionCount,
		PublishedAt:   version.PublishedAt,
		PublishedBy:   version.PublishedBy,
	}, nil
}

func publishViolations(ctx context.Context, violations []domain.Violation) openapi.PublishTest409JSONResponse {
	body := httpapi.Error(ctx, openapi.PUBLISHVALIDATIONFAILED,
		"Đề chưa thể xuất bản. Vui lòng sửa các vấn đề được đánh dấu.")

	out := make([]openapi.PublishValidationError, len(violations))
	for i, v := range violations {
		out[i] = openapi.PublishValidationError{
			Rule:    openapi.PublishValidationErrorRule(v.Rule),
			Message: v.Message,
		}
		if v.SectionID != "" {
			id := httpapi.ParseUUID(v.SectionID)
			out[i].SectionId = &id
		}
		if v.GroupID != "" {
			id := httpapi.ParseUUID(v.GroupID)
			out[i].GroupId = &id
		}
		if v.QuestionID != "" {
			id := httpapi.ParseUUID(v.QuestionID)
			out[i].QuestionId = &id
		}
	}

	var resp openapi.PublishTest409JSONResponse
	resp.Error = body.Error
	resp.Violations = &out
	return resp
}

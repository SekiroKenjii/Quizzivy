package api

import (
	"context"
	"errors"
	"strconv"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
	"quizzivy/internal/tests/publish"
)

// PublishTest validates the draft and freezes it as a new version.
//
// A validation failure returns EVERY problem at once, each anchored to a
// question, so the builder marks them inline rather than surfacing one per
// attempt.
func (s *Server) PublishTest(ctx context.Context, request openapi.PublishTestRequestObject) (openapi.PublishTestResponseObject, error) {
	if s.Deps.Publisher == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	version, err := s.Deps.Publisher.Publish(ctx, publish.Request{
		TestID:    request.Id.String(),
		ActorID:   principal.UserID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	})
	switch {
	case err == nil:
	case errors.Is(err, publish.ErrNotFound):
		return openapi.PublishTest404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, "Không tìm thấy đề."))}, nil
	case errors.Is(err, publish.ErrNoContent):
		return publishViolations(ctx, []publish.Violation{{
			Rule:    publish.SectionNotEmpty,
			Message: "Đề chưa có phần nào để xuất bản.",
		}}), nil
	default:
		var invalid *publish.ValidationError
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
		Id:            parseUUID(version.ID),
		Version:       version.Version,
		TotalPoints:   points,
		QuestionCount: version.QuestionCount,
		PublishedAt:   version.PublishedAt,
		PublishedBy:   version.PublishedBy,
	}, nil
}

func publishViolations(ctx context.Context, violations []publish.Violation) openapi.PublishTest409JSONResponse {
	body := authError(ctx, openapi.PUBLISHVALIDATIONFAILED,
		"Đề chưa thể xuất bản. Vui lòng sửa các vấn đề được đánh dấu.")

	out := make([]openapi.PublishValidationError, len(violations))
	for i, v := range violations {
		out[i] = openapi.PublishValidationError{
			Rule:    openapi.PublishValidationErrorRule(v.Rule),
			Message: v.Message,
		}
		if v.SectionID != "" {
			id := parseUUID(v.SectionID)
			out[i].SectionId = &id
		}
		if v.QuestionID != "" {
			id := parseUUID(v.QuestionID)
			out[i].QuestionId = &id
		}
	}

	var resp openapi.PublishTest409JSONResponse
	resp.Error = body.Error
	resp.Violations = &out
	return resp
}

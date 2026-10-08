package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"strconv"
)

// GetTestVersionDiff lists what differs between a version and another paper
// of the same test.
func (h Tests) GetTestVersionDiff(ctx context.Context, request openapi.GetTestVersionDiffRequestObject) (openapi.GetTestVersionDiffResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	against, err := domain.ParseAgainst(request.Params.Against, request.Version)
	if err != nil {
		return openapi.GetTestVersionDiff400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(againstProblem(ctx, err))}, nil
	}
	result, err := h.app.Queries.Diff.Handle(ctx, query.Diff{
		TestID: request.Id.String(), Version: request.Version, Against: against, Scope: httpapi.ScopeFromContext(ctx),
	})
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrNotFound):
		return openapi.GetTestVersionDiff404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy đề hoặc phiên bản.", "The test or the version was not found.")))}, nil
	case errors.Is(err, domain.ErrVersionUnreadable):
		return openapi.GetTestVersionDiff422JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			httpx.Text(ctx, "Không đọc lại được một nhóm câu hỏi của phiên bản đã xuất bản nên chưa thể so sánh.",
				"A question group of a published version cannot be read back, so it cannot be compared."))), nil
	case errors.Is(err, domain.ErrDraftUnreadable):
		return openapi.GetTestVersionDiff422JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			httpx.Text(ctx, "Nội dung nhóm câu hỏi của bản nháp chưa hợp lệ nên chưa thể so sánh. Hãy kiểm tra các nhóm trong bản nháp.",
				"The draft's question groups are not valid, so it cannot be compared yet. Check the groups in the draft."))), nil
	default:
		return nil, err
	}
	out, err := toAPIDiff(result)
	if err != nil {
		return nil, err
	}
	return openapi.GetTestVersionDiff200JSONResponse(out), nil
}

func againstProblem(ctx context.Context, err error) openapi.ErrorResponse {
	message := httpx.Text(ctx, "Chỉ so sánh với bản nháp, phiên bản trước hoặc một phiên bản khác của đề.",
		"Compare only with the draft, the previous version or another version of the test.")
	if errors.Is(err, domain.ErrSameVersion) {
		message = httpx.Text(ctx, "Không thể so sánh một phiên bản với chính nó.", "A version cannot be compared with itself.")
	}
	problem := httpapi.Error(ctx, openapi.VALIDATIONFAILED, message)
	problem.Error.Details = &map[string]interface{}{"against": message}
	return problem
}

func toAPIDiff(result query.DiffResult) (openapi.TestVersionDiff, error) {
	out := openapi.TestVersionDiff{To: toAPISide(result.To), Changes: make([]openapi.DiffChange, len(result.Changes))}
	if result.From != nil {
		side := toAPISide(*result.From)
		out.From = &side
	}
	for i, change := range result.Changes {
		var err error
		if out.Changes[i], err = toAPIChange(change); err != nil {
			return openapi.TestVersionDiff{}, err
		}
	}
	return out, nil
}

func toAPISide(side domain.DiffSide) openapi.DiffSide {
	if side.Draft {
		return openapi.DiffSide{Kind: openapi.DiffSideKindDraft}
	}
	version, publishedAt := side.Version, side.PublishedAt
	return openapi.DiffSide{Kind: openapi.DiffSideKindVersion, Version: &version, PublishedAt: &publishedAt}
}

func toAPIChange(change domain.Change) (openapi.DiffChange, error) {
	out := openapi.DiffChange{Kind: openapi.DiffChangeKind(change.Kind)}
	if change.Number > 0 {
		number := change.Number
		out.QuestionNumber = &number
	}
	if change.QuestionID != "" {
		id := httpapi.ParseUUID(change.QuestionID)
		out.QuestionId = &id
	}
	if change.Prompt != "" {
		prompt := change.Prompt
		out.Params.Prompt = &prompt
	}
	if len(change.Fields) > 0 {
		fields := make([]openapi.DiffField, len(change.Fields))
		for i, field := range change.Fields {
			fields[i] = openapi.DiffField(field)
		}
		out.Params.Fields = &fields
	}
	if change.AnswerFrom != nil {
		from, to := change.AnswerFrom, change.AnswerTo
		out.Params.AnswerFrom, out.Params.AnswerTo = &from, &to
	}
	if change.Kind == domain.ChangePoints {
		from, err := strconv.ParseFloat(change.PointsFrom, 64)
		if err != nil {
			return openapi.DiffChange{}, err
		}
		to, err := strconv.ParseFloat(change.PointsTo, 64)
		if err != nil {
			return openapi.DiffChange{}, err
		}
		out.Params.PointsFrom, out.Params.PointsTo = &from, &to
	}
	return out, nil
}

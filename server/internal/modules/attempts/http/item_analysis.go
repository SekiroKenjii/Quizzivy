package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

// GetItemAnalysis reads how the students did on each question of an assignment,
// hardest first.
func (h Attempts) GetItemAnalysis(ctx context.Context, request openapi.GetItemAnalysisRequestObject) (openapi.GetItemAnalysisResponseObject, error) {
	if h.app == nil || h.app.Queries.ItemAnalysis == nil {
		return nil, httpx.ErrNotImplemented
	}
	analysis, err := h.app.Queries.ItemAnalysis.Handle(ctx, query.ItemAnalysis{
		AssignmentID: request.Id.String(), Scope: httpapi.ScopeFromContext(ctx),
	})
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.GetItemAnalysis404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy bài giao.", "The assignment was not found.")))}, nil
	}
	if err != nil {
		return nil, err
	}

	out := openapi.GetItemAnalysis200JSONResponse{
		HandedIn: analysis.HandedIn,
		Items:    make([]openapi.ItemAnalysisItem, len(analysis.Items)),
	}
	for i, item := range analysis.Items {
		out.Items[i] = openapi.ItemAnalysisItem{
			QuestionId:    httpapi.ParseUUID(item.QuestionID),
			Number:        item.Number,
			Type:          openapi.QuestionType(item.Type),
			PromptExcerpt: item.PromptExcerpt,
			Answered:      item.Answered,
			CorrectRate:   item.CorrectRate,
		}
	}
	return out, nil
}

package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

// ListGradingQueue reads pending manual answers on reached historical papers.
func (h Attempts) ListGradingQueue(ctx context.Context, request openapi.ListGradingQueueRequestObject) (openapi.ListGradingQueueResponseObject, error) {
	if h.app == nil || h.app.Queries.GradingQueue == nil {
		return nil, httpx.ErrNotImplemented
	}
	q := query.GradingQueue{Scope: httpapi.ScopeFromContext(ctx), Mode: "student"}
	if request.Params.Mode != nil {
		q.Mode = string(*request.Params.Mode)
	}
	if request.Params.AssignmentId != nil {
		value := request.Params.AssignmentId.String()
		q.AssignmentID = &value
	}
	if request.Params.StudentId != nil {
		value := request.Params.StudentId.String()
		q.StudentID = &value
	}
	queue, err := h.app.Queries.GradingQueue.Handle(ctx, q)
	switch {
	case errors.Is(err, domain.ErrPaperNotFound):
		return openapi.ListGradingQueue404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy bài giao hoặc người làm bài.", "The assignment or paper taker was not found.")))}, nil
	case errors.Is(err, domain.ErrGroupContextUnavailable):
		return nil, httpx.ErrNotImplemented
	case err != nil:
		return nil, err
	}
	out := openapi.GradingQueue{Groups: make([]openapi.GradingQueueGroup, len(queue.Groups)), Items: make([]openapi.GradingQueueItem, len(queue.Items)), AnswersRemaining: queue.AnswersRemaining, StudentsWaiting: queue.StudentsWaiting}
	for i, group := range queue.Groups {
		out.Groups[i] = openapi.GradingQueueGroup{Key: group.Key, Kind: openapi.GradingQueueGroupKind(group.Kind), Label: group.Label, Sub: group.Sub, Remaining: group.Remaining}
	}
	for i, item := range queue.Items {
		converted, err := h.toAPIQueueItem(ctx, item)
		if err != nil {
			return nil, err
		}
		out.Items[i] = converted
	}
	return openapi.ListGradingQueue200JSONResponse(out), nil
}

func (h Attempts) toAPIQueueItem(ctx context.Context, item domain.GradingQueueItem) (openapi.GradingQueueItem, error) {
	if item.Question.Media != nil && h.media == nil {
		return openapi.GradingQueueItem{}, httpx.ErrNotImplemented
	}
	question, err := h.toAPIReviewQuestion(ctx, item.Question, item.PublishedAt)
	if err != nil {
		return openapi.GradingQueueItem{}, err
	}
	shared, err := sharedReviewContext(ctx, item.SharedContext, h.adminGroupAsset)
	if err != nil {
		return openapi.GradingQueueItem{}, err
	}
	var answer openapi.Answer
	if err := json.Unmarshal(item.Answer.Payload, &answer); err != nil {
		return openapi.GradingQueueItem{}, fmt.Errorf("grading queue: decode answer %s/%s: %w", item.AttemptID, item.Question.ID, err)
	}
	return openapi.GradingQueueItem{
		AttemptId: httpapi.ParseUUID(item.AttemptID), QuestionId: question.Id,
		AssignmentId: httpapi.ParseUUID(item.AssignmentID), AssignmentTitle: item.AssignmentTitle,
		StudentId: httpapi.ParseUUID(item.StudentID), StudentName: item.StudentName, QuestionNumber: item.QuestionNumber,
		Type: question.Type, Prompt: question.Prompt, PromptContent: question.PromptContent, Answer: answer, Points: question.Points,
		Score: item.Answer.ManualScore, Comment: item.Answer.GraderComment, Media: question.Media, Audio: question.Audio,
		Transcript: question.Transcript, Options: question.Options, Blanks: question.Blanks,
		Explanation: question.Explanation, ExplanationContent: question.ExplanationContent, SampleAnswer: question.SampleAnswer, SharedContext: shared,
	}, nil
}

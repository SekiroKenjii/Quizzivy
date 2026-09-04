package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"quizzivy/gen/openapi"
	"quizzivy/internal/attempts"
	"quizzivy/internal/httpx"
)

// GetAttemptResult is §9's result page. What the policy withheld never left
// the database, so there is nothing here to strip (§13.5).
func (s *Server) GetAttemptResult(ctx context.Context, request openapi.GetAttemptResultRequestObject) (openapi.GetAttemptResultResponseObject, error) {
	if s.Deps.Attempts == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	result, err := s.Deps.Attempts.Result(ctx, request.Id.String(), principal.UserID)
	switch {
	case errors.Is(err, attempts.ErrForbidden), errors.Is(err, attempts.ErrNotFound):
		return openapi.GetAttemptResult403JSONResponse{ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
			authError(ctx, openapi.FORBIDDEN, "Bạn không có quyền xem kết quả này."))}, nil
	case errors.Is(err, attempts.ErrAttemptInProgress):
		return openapi.GetAttemptResult409JSONResponse(authError(ctx, openapi.ATTEMPTINPROGRESS, "Bài chưa được nộp.")), nil
	case errors.Is(err, attempts.ErrAttemptVoided):
		return openapi.GetAttemptResult409JSONResponse(authError(ctx, openapi.ATTEMPTVOIDED, "Lượt làm này đã bị huỷ.")), nil
	case err != nil:
		return nil, err
	}

	questions := make([]openapi.ResultQuestion, len(result.Questions))
	for i, q := range result.Questions {
		converted, err := s.toAPIResultQuestion(ctx, principal.UserID, q)
		if err != nil {
			return nil, err
		}
		questions[i] = converted
	}
	attempt := toAPIAttempt(result.Attempt)
	if result.Score != nil {
		attempt.Score = toAPIScore(*result.Score)
	}
	return openapi.GetAttemptResult200JSONResponse{
		Attempt: attempt,
		Review: openapi.ReviewPolicy{
			ShowScore:          result.Review.ShowScore,
			ShowCorrectAnswers: result.Review.ShowCorrectAnswers,
			ShowExplanations:   result.Review.ShowExplanations,
		},
		TestTitle:   result.TestTitle,
		MaxAttempts: result.MaxAttempts,
		Questions:   questions,
	}, nil
}

func (s *Server) toAPIResultQuestion(ctx context.Context, studentID string, q attempts.ResultQuestion) (openapi.ResultQuestion, error) {
	base, err := s.toAPIStudentQuestion(ctx, studentID, q.Question)
	if err != nil {
		return openapi.ResultQuestion{}, err
	}
	out := openapi.ResultQuestion{
		Id: base.Id, Type: base.Type, Prompt: base.Prompt, Points: base.Points,
		Media: base.Media, Options: base.Options, Blanks: base.Blanks,
		Earned:         q.Earned,
		PendingManual:  ptr(q.PendingManual),
		GraderComment:  q.GraderComment,
		Explanation:    q.Explanation,
		Transcript:     q.Transcript,
		AudioPlaysUsed: q.AudioPlaysUsed,
	}
	if len(q.Answer) > 0 {
		var decoded openapi.Answer
		if err := json.Unmarshal(q.Answer, &decoded); err != nil {
			return openapi.ResultQuestion{}, fmt.Errorf("decode stored answer for %s: %w", q.ID, err)
		}
		out.Answer = &decoded
	}
	if q.CorrectOptions != nil {
		ids := make([]openapi.Uuid, len(q.CorrectOptions))
		for i, id := range q.CorrectOptions {
			ids[i] = parseUUID(id)
		}
		out.CorrectOptionIds = &ids
	}
	if q.CorrectAnswers != nil {
		answers := make([]struct {
			Answer  string       `json:"answer"`
			BlankId openapi.Uuid `json:"blankId"`
		}, len(q.CorrectAnswers))
		for i, ba := range q.CorrectAnswers {
			answers[i].Answer = ba.Answer
			answers[i].BlankId = parseUUID(ba.BlankID)
		}
		out.CorrectAnswers = &answers
	}
	return out, nil
}

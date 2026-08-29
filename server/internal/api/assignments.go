package api

import (
	"context"
	"errors"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/assignments"
	"quizzivy/internal/httpx"
)

// ListAssignments backs §8's assignments list and A-01's "Bài đang mở".
func (s *Server) ListAssignments(ctx context.Context, request openapi.ListAssignmentsRequestObject) (openapi.ListAssignmentsResponseObject, error) {
	if s.Deps.Assignments == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := assignments.ListInput{}
	if request.Params.Status != nil {
		status := assignments.Status(*request.Params.Status)
		in.Status = &status
	}
	if request.Params.Cursor != nil {
		in.Cursor = *request.Params.Cursor
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	found, next, err := s.Deps.Assignments.List(ctx, in)
	if err != nil {
		// The contract gives this operation no 400, so a bad cursor is simply
		// an empty page rather than an invented status.
		if errors.Is(err, assignments.ErrBadCursor) {
			return openapi.ListAssignments200JSONResponse{Items: []openapi.Assignment{}}, nil
		}
		return nil, err
	}

	out := openapi.ListAssignments200JSONResponse{
		Items: make([]openapi.Assignment, len(found)),
	}
	for i, a := range found {
		out.Items[i] = toAPIAssignment(a)
	}
	if next != "" {
		out.NextCursor = &next
	}
	return out, nil
}

func toAPIAssignment(a assignments.Assignment) openapi.Assignment {
	classIDs := make([]openapi.Uuid, len(a.ClassIDs))
	for i, id := range a.ClassIDs {
		classIDs[i] = parseUUID(id)
	}
	studentIDs := make([]openapi.Uuid, len(a.StudentIDs))
	for i, id := range a.StudentIDs {
		studentIDs[i] = parseUUID(id)
	}

	out := openapi.Assignment{
		Id:            parseUUID(a.ID),
		TestId:        parseUUID(a.TestID),
		TestVersionId: parseUUID(a.TestVersionID),
		TestVersion:   a.TestVersion,
		TestTitle:     a.TestTitle,
		Targets: struct {
			ClassIds   []openapi.Uuid `json:"classIds"`
			StudentIds []openapi.Uuid `json:"studentIds"`
		}{ClassIds: classIDs, StudentIds: studentIDs},
		DurationMinutes:  a.DurationMin,
		MaxAttempts:      a.MaxAttempts,
		ShuffleQuestions: a.ShuffleQ,
		ShuffleOptions:   a.ShuffleO,
		Review: openapi.ReviewPolicy{
			ShowScore:          a.Review.ShowScore,
			ShowCorrectAnswers: a.Review.ShowCorrectAnswers,
			ShowExplanations:   a.Review.ShowExplanations,
		},
		Integrity: openapi.IntegrityPolicy{
			RequireFullscreen: a.Integrity.RequireFullscreen,
			BlockCopyPaste:    a.Integrity.BlockCopyPaste,
			MaxFocusLoss:      a.Integrity.MaxFocusLoss,
			OnLimitExceeded:   openapi.IntegrityPolicyOnLimitExceeded(a.Integrity.OnLimitExceeded),
			MinAwayMs:         a.Integrity.MinAwayMs,
		},
		Status:         openapi.AssignmentStatus(assignments.StatusAt(time.Now(), a.OpensAt, a.ClosesAt, a.ClosedAt)),
		SubmittedCount: &a.SubmittedCount,
		TargetCount:    &a.TargetCount,
		FlaggedCount:   &a.FlaggedCount,
	}
	out.Window.OpensAt = a.OpensAt
	out.Window.ClosesAt = a.ClosesAt
	out.Window.ClosedAt = a.ClosedAt
	return out
}

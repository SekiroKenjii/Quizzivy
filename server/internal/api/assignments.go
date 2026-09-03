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
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	found, page, err := s.Deps.Assignments.List(ctx, in)
	if err != nil {
		return nil, err
	}

	out := openapi.ListAssignments200JSONResponse{
		Items:    make([]openapi.Assignment, len(found)),
		Page:     page.Number,
		PageSize: page.Size,
		Total:    page.Total,
	}
	for i, a := range found {
		out.Items[i] = toAPIAssignment(a)
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
		Status: openapi.AssignmentStatus(
			assignments.StatusAt(time.Now(), a.PublishedAt, a.OpensAt, a.ClosesAt, a.ClosedAt),
		),
		PublishedAt:    a.PublishedAt,
		SubmittedCount: &a.SubmittedCount,
		TargetCount:    &a.TargetCount,
		FlaggedCount:   &a.FlaggedCount,
	}
	out.Window.OpensAt = a.OpensAt
	out.Window.ClosesAt = a.ClosesAt
	out.Window.ClosedAt = a.ClosedAt
	return out
}

func (s *Server) GetAssignment(ctx context.Context, request openapi.GetAssignmentRequestObject) (openapi.GetAssignmentResponseObject, error) {
	if s.Deps.Assignments == nil {
		return nil, httpx.ErrNotImplemented
	}
	a, err := s.Deps.Assignments.Get(ctx, request.Id.String())
	if errors.Is(err, assignments.ErrNotFound) {
		return openapi.GetAssignment404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, "Không tìm thấy bài giao."))}, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.GetAssignment200JSONResponse(toAPIAssignment(a)), nil
}

func (s *Server) CreateAssignment(ctx context.Context, request openapi.CreateAssignmentRequestObject) (openapi.CreateAssignmentResponseObject, error) {
	if s.Deps.Assignments == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := assignmentRequest(ctx, "")
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	a, err := s.Deps.Assignments.Create(ctx, req, toWriteInput(*request.Body))
	var invalid *assignments.ValidationError
	switch {
	case err == nil:
	case errors.As(err, &invalid):
		return openapi.CreateAssignment400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			assignmentValidationError(ctx, invalid))}, nil
	case errors.Is(err, assignments.ErrTestNotPublished):
		return openapi.CreateAssignment409JSONResponse(authError(ctx, openapi.TESTNOTPUBLISHED,
			"Chỉ có thể giao một phiên bản đề đã xuất bản.")), nil
	default:
		return nil, err
	}
	return openapi.CreateAssignment201JSONResponse(toAPIAssignment(a)), nil
}

func (s *Server) UpdateAssignment(ctx context.Context, request openapi.UpdateAssignmentRequestObject) (openapi.UpdateAssignmentResponseObject, error) {
	if s.Deps.Assignments == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := assignmentRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	a, err := s.Deps.Assignments.Update(ctx, req, toWriteInput(*request.Body))
	var invalid *assignments.ValidationError
	switch {
	case err == nil:
	case errors.As(err, &invalid):
		return openapi.UpdateAssignment400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			assignmentValidationError(ctx, invalid))}, nil
	case errors.Is(err, assignments.ErrNotFound):
		return openapi.UpdateAssignment404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, "Không tìm thấy bài giao."))}, nil
	case errors.Is(err, assignments.ErrTestNotPublished):
		return openapi.UpdateAssignment409JSONResponse(authError(ctx, openapi.TESTNOTPUBLISHED,
			"Chỉ có thể giao một phiên bản đề đã xuất bản.")), nil
	case errors.Is(err, assignments.ErrVersionLocked):
		return openapi.UpdateAssignment409JSONResponse(authError(ctx, openapi.VERSIONLOCKED,
			"Đã có học viên làm bài, không thể đổi phiên bản đề.")), nil
	default:
		return nil, err
	}
	return openapi.UpdateAssignment200JSONResponse(toAPIAssignment(a)), nil
}

func assignmentRequest(ctx context.Context, id string) (assignments.Request, bool) {
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return assignments.Request{}, false
	}
	meta := httpx.RequestMetaFromContext(ctx)
	return assignments.Request{
		ID: id, ActorID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent,
	}, true
}

func assignmentValidationError(ctx context.Context, invalid *assignments.ValidationError) openapi.ErrorResponse {
	resp := authError(ctx, openapi.VALIDATIONFAILED, "Dữ liệu bài giao không hợp lệ.")
	details := map[string]interface{}{}
	for _, f := range invalid.Fields {
		if _, seen := details[f.Field]; !seen {
			details[f.Field] = f.Message
		}
	}
	resp.Error.Details = &details
	return resp
}

func toWriteInput(body openapi.AssignmentInput) assignments.WriteInput {
	in := assignments.WriteInput{
		TestVersionID: body.TestVersionId.String(),
		OpensAt:       body.Window.OpensAt,
		ClosesAt:      body.Window.ClosesAt,
		DurationMin:   body.DurationMinutes,
		MaxAttempts:   body.MaxAttempts,
		Review: assignments.Review{
			ShowScore:          body.Review.ShowScore,
			ShowCorrectAnswers: body.Review.ShowCorrectAnswers,
			ShowExplanations:   body.Review.ShowExplanations,
		},
		Integrity: assignments.Integrity{
			RequireFullscreen: body.Integrity.RequireFullscreen,
			BlockCopyPaste:    body.Integrity.BlockCopyPaste,
			MaxFocusLoss:      body.Integrity.MaxFocusLoss,
			OnLimitExceeded:   string(body.Integrity.OnLimitExceeded),
			MinAwayMs:         body.Integrity.MinAwayMs,
		},
		Now: time.Now(),
	}
	if body.ShuffleQuestions != nil {
		in.ShuffleQ = *body.ShuffleQuestions
	}
	if body.ShuffleOptions != nil {
		in.ShuffleO = *body.ShuffleOptions
	}
	if body.CloseNow != nil {
		in.CloseNow = *body.CloseNow
	}
	if body.Draft != nil {
		in.Draft = *body.Draft
	}
	for _, id := range body.Targets.ClassIds {
		in.ClassIDs = append(in.ClassIDs, id.String())
	}
	for _, id := range body.Targets.StudentIds {
		in.StudentIDs = append(in.StudentIDs, id.String())
	}
	return in
}

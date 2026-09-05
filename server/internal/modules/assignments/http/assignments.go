package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"time"
)

// ListAssignments backs §8's assignments list and A-01's "Bài đang mở".
func (h Assignments) ListAssignments(ctx context.Context, request openapi.ListAssignmentsRequestObject) (openapi.ListAssignmentsResponseObject, error) {
	if h.assignments == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := domain.ListInput{}
	if request.Params.Status != nil {
		status := domain.Status(*request.Params.Status)
		in.Status = &status
	}
	if request.Params.ClassId != nil {
		classID := request.Params.ClassId.String()
		in.ClassID = &classID
	}
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	found, page, err := h.assignments.List(ctx, in)
	if err != nil {
		return nil, err
	}
	facets, err := h.assignments.Facets(ctx, in)
	if err != nil {
		return nil, err
	}

	out := openapi.ListAssignments200JSONResponse{
		Items:    make([]openapi.Assignment, len(found)),
		Page:     page.Number,
		PageSize: page.Size,
		Total:    page.Total,
		Facets: openapi.AssignmentStatusFacets{
			All: facets.All, Draft: facets.Draft, Scheduled: facets.Scheduled,
			Open: facets.Open, Closed: facets.Closed,
		},
	}
	for i, a := range found {
		out.Items[i] = toAPIAssignment(a)
	}
	return out, nil
}

func toAPIAssignment(a domain.Assignment) openapi.Assignment {
	classes := make([]struct {
		Id           openapi.Uuid `json:"id"`
		Name         string       `json:"name"`
		StudentCount int          `json:"studentCount"`
	}, len(a.Classes))
	for i, c := range a.Classes {
		classes[i].Id = httpapi.ParseUUID(c.ID)
		classes[i].Name = c.Name
		classes[i].StudentCount = c.StudentCount
	}
	students := make([]struct {
		Id   openapi.Uuid `json:"id"`
		Name string       `json:"name"`
	}, len(a.Students))
	for i, st := range a.Students {
		students[i].Id = httpapi.ParseUUID(st.ID)
		students[i].Name = st.Name
	}

	out := openapi.Assignment{
		Id:            httpapi.ParseUUID(a.ID),
		TestId:        httpapi.ParseUUID(a.TestID),
		TestVersionId: httpapi.ParseUUID(a.TestVersionID),
		TestVersion:   a.TestVersion,
		TestTitle:     a.TestTitle,
		Targets: struct {
			Classes []struct {
				Id           openapi.Uuid `json:"id"`
				Name         string       `json:"name"`
				StudentCount int          `json:"studentCount"`
			} `json:"classes"`
			Students []struct {
				Id   openapi.Uuid `json:"id"`
				Name string       `json:"name"`
			} `json:"students"`
		}{Classes: classes, Students: students},
		UpdatedAt:        a.UpdatedAt,
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
			domain.StatusAt(time.Now(), a.PublishedAt, a.OpensAt, a.ClosesAt, a.ClosedAt),
		),
		PublishedAt:         a.PublishedAt,
		SubmittedCount:      &a.SubmittedCount,
		TargetCount:         &a.TargetCount,
		FlaggedCount:        &a.FlaggedCount,
		PendingGradingCount: &a.PendingGradingCount,
	}
	out.Window.OpensAt = a.OpensAt
	out.Window.ClosesAt = a.ClosesAt
	out.Window.ClosedAt = a.ClosedAt
	return out
}

func (h Assignments) GetAssignment(ctx context.Context, request openapi.GetAssignmentRequestObject) (openapi.GetAssignmentResponseObject, error) {
	if h.assignments == nil {
		return nil, httpx.ErrNotImplemented
	}
	a, err := h.assignments.Get(ctx, request.Id.String())
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.GetAssignment404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, "Không tìm thấy bài giao."))}, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.GetAssignment200JSONResponse(toAPIAssignment(a)), nil
}

func (h Assignments) CreateAssignment(ctx context.Context, request openapi.CreateAssignmentRequestObject) (openapi.CreateAssignmentResponseObject, error) {
	if h.assignments == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := assignmentRequest(ctx, "")
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	a, err := h.assignments.Create(ctx, req, toWriteInput(*request.Body))
	var invalid *domain.ValidationError
	switch {
	case err == nil:
	case errors.As(err, &invalid):
		return openapi.CreateAssignment400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			assignmentValidationError(ctx, invalid))}, nil
	case errors.Is(err, domain.ErrTestNotPublished):
		return openapi.CreateAssignment409JSONResponse(httpapi.Error(ctx, openapi.TESTNOTPUBLISHED,
			"Chỉ có thể giao một phiên bản đề đã xuất bản.")), nil
	default:
		return nil, err
	}
	return openapi.CreateAssignment201JSONResponse(toAPIAssignment(a)), nil
}

func (h Assignments) UpdateAssignment(ctx context.Context, request openapi.UpdateAssignmentRequestObject) (openapi.UpdateAssignmentResponseObject, error) {
	if h.assignments == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := assignmentRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	a, err := h.assignments.Update(ctx, req, toWriteInput(*request.Body))
	var invalid *domain.ValidationError
	switch {
	case err == nil:
	case errors.As(err, &invalid):
		return openapi.UpdateAssignment400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			assignmentValidationError(ctx, invalid))}, nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.UpdateAssignment404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, "Không tìm thấy bài giao."))}, nil
	case errors.Is(err, domain.ErrTestNotPublished):
		return openapi.UpdateAssignment409JSONResponse(httpapi.Error(ctx, openapi.TESTNOTPUBLISHED,
			"Chỉ có thể giao một phiên bản đề đã xuất bản.")), nil
	case errors.Is(err, domain.ErrVersionLocked):
		return openapi.UpdateAssignment409JSONResponse(httpapi.Error(ctx, openapi.VERSIONLOCKED,
			"Đã có học viên làm bài, không thể đổi phiên bản đề.")), nil
	default:
		return nil, err
	}
	return openapi.UpdateAssignment200JSONResponse(toAPIAssignment(a)), nil
}

// ReopenAssignment is G-09's "Gia hạn cho tất cả".
func (h Assignments) ReopenAssignment(ctx context.Context, request openapi.ReopenAssignmentRequestObject) (openapi.ReopenAssignmentResponseObject, error) {
	if h.assignments == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := assignmentRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	a, err := h.assignments.Reopen(ctx, req, request.Body.ClosesAt, request.Body.Reason, time.Now())
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrBlankReason):
		return openapi.ReopenAssignment400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			httpapi.FieldError(ctx, "reason", "Hãy ghi lý do mở lại."))}, nil
	case errors.Is(err, domain.ErrClosesInPast):
		return openapi.ReopenAssignment400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			httpapi.FieldError(ctx, "closesAt", "Thời điểm đóng mới phải ở phía trước."))}, nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.ReopenAssignment404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, "Không tìm thấy bài giao."))}, nil
	case errors.Is(err, domain.ErrNotClosed):
		return openapi.ReopenAssignment409JSONResponse(httpapi.Error(ctx, openapi.ASSIGNMENTNOTCLOSED,
			"Bài giao chưa đóng nên không có gì để mở lại.")), nil
	default:
		return nil, err
	}
	return openapi.ReopenAssignment200JSONResponse(toAPIAssignment(a)), nil
}

func assignmentRequest(ctx context.Context, id string) (domain.Request, bool) {
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return domain.Request{}, false
	}
	meta := httpx.RequestMetaFromContext(ctx)
	return domain.Request{
		ID: id, ActorID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent,
	}, true
}

func assignmentValidationError(ctx context.Context, invalid *domain.ValidationError) openapi.ErrorResponse {
	resp := httpapi.Error(ctx, openapi.VALIDATIONFAILED, "Dữ liệu bài giao không hợp lệ.")
	details := map[string]interface{}{}
	for _, f := range invalid.Fields {
		if _, seen := details[f.Field]; !seen {
			details[f.Field] = f.Message
		}
	}
	resp.Error.Details = &details
	return resp
}

func toWriteInput(body openapi.AssignmentInput) domain.WriteInput {
	in := domain.WriteInput{
		TestVersionID: body.TestVersionId.String(),
		OpensAt:       body.Window.OpensAt,
		ClosesAt:      body.Window.ClosesAt,
		DurationMin:   body.DurationMinutes,
		MaxAttempts:   body.MaxAttempts,
		Review: domain.Review{
			ShowScore:          body.Review.ShowScore,
			ShowCorrectAnswers: body.Review.ShowCorrectAnswers,
			ShowExplanations:   body.Review.ShowExplanations,
		},
		Integrity: domain.Integrity{
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

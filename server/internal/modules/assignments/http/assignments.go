package http

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/assignments/application/command"
	"quizzivy/internal/modules/assignments/application/query"
	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"time"
)

func msgAssignmentNotFound(ctx context.Context) string {
	return httpx.Text(ctx, "Không tìm thấy bài giao.", "The assignment was not found.")
}

// ListAssignments backs §8's assignments list and A-01's "Bài đang mở".
func (h Assignments) ListAssignments(ctx context.Context, request openapi.ListAssignmentsRequestObject) (openapi.ListAssignmentsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}

	scope := httpapi.ScopeFromContext(ctx)
	in := domain.ListInput{Scope: scope.Own(), EveryTarget: scope.All}
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

	listResult, err := h.app.Queries.List.Handle(ctx, query.List{Input: in})
	found, page := listResult.Items, listResult.Page
	if err != nil {
		return nil, err
	}
	facets, err := h.app.Queries.Facets.Handle(ctx, query.Facets{Input: in})
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
		Review:           toAPIReview(a.Review),
		Integrity: openapi.IntegrityPolicy{
			RequireFullscreen: a.Integrity.RequireFullscreen,
			BlockCopyPaste:    a.Integrity.BlockCopyPaste,
			MaxFocusLoss:      a.Integrity.MaxFocusLoss,
			OnLimitExceeded:   openapi.IntegrityPolicyOnLimitExceeded(a.Integrity.OnLimitExceeded),
			MinAwayMs:         a.Integrity.MinAwayMs,
		},
		StudentNote: a.StudentNote,
		Status: openapi.AssignmentStatus(
			domain.Schedule.StatusAt(time.Now(), a.PublishedAt, a.OpensAt, a.ClosesAt, a.ClosedAt),
		),
		PublishedAt:         a.PublishedAt,
		SubmittedCount:      &a.SubmittedCount,
		TargetCount:         &a.TargetCount,
		FlaggedCount:        &a.FlaggedCount,
		PendingGradingCount: &a.PendingGradingCount,
		PendingManualCount:  &a.PendingManualCount,
	}
	out.Window.OpensAt = a.OpensAt
	out.Window.ClosesAt = a.ClosesAt
	out.Window.ClosedAt = a.ClosedAt
	return out
}

func toAPIReview(r domain.Review) openapi.ReviewPolicy {
	return openapi.ReviewPolicy{
		ShowScore:          r.ShowScore,
		ShowCorrectAnswers: r.ShowCorrectAnswers,
		ShowExplanations:   r.ShowExplanations,
		Release:            openapi.ReviewRelease(r.Release),
		ShowClassAverage:   r.ShowClassAverage,
	}
}

func (h Assignments) GetAssignment(ctx context.Context, request openapi.GetAssignmentRequestObject) (openapi.GetAssignmentResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	a, err := h.app.Queries.Get.Handle(ctx, query.Get{ID: request.Id.String(), Scope: httpapi.ScopeFromContext(ctx)})
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.GetAssignment404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgAssignmentNotFound(ctx)))}, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.GetAssignment200JSONResponse(toAPIAssignment(a)), nil
}

func (h Assignments) CreateAssignment(ctx context.Context, request openapi.CreateAssignmentRequestObject) (openapi.CreateAssignmentResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := assignmentRequest(ctx, "")
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	var invalid *domain.ValidationError
	input, err := toWriteInput(ctx, *request.Body)
	if errors.As(err, &invalid) {
		return openapi.CreateAssignment400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			assignmentValidationError(ctx, invalid))}, nil
	}
	if err != nil {
		return nil, err
	}
	a, err := h.app.Commands.Create.Handle(ctx, command.Create{Request: req, Input: input})
	switch {
	case err == nil:
	case errors.As(err, &invalid):
		return openapi.CreateAssignment400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			assignmentValidationError(ctx, invalid))}, nil
	case errors.Is(err, domain.ErrTestNotPublished):
		return openapi.CreateAssignment409JSONResponse(httpapi.Error(ctx, openapi.TESTNOTPUBLISHED,
			httpx.Text(ctx, "Chỉ có thể giao một phiên bản đề đã xuất bản.", "Only a published version of a test can be assigned."))), nil
	default:
		return nil, err
	}
	return openapi.CreateAssignment201JSONResponse(toAPIAssignment(a)), nil
}

func (h Assignments) UpdateAssignment(ctx context.Context, request openapi.UpdateAssignmentRequestObject) (openapi.UpdateAssignmentResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := assignmentRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	var invalid *domain.ValidationError
	input, err := toWriteInput(ctx, *request.Body)
	if errors.As(err, &invalid) {
		return openapi.UpdateAssignment400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			assignmentValidationError(ctx, invalid))}, nil
	}
	if err != nil {
		return nil, err
	}
	a, err := h.app.Commands.Update.Handle(ctx, command.Update{Request: req, Input: input})
	switch {
	case err == nil:
	case errors.As(err, &invalid):
		return openapi.UpdateAssignment400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			assignmentValidationError(ctx, invalid))}, nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.UpdateAssignment404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgAssignmentNotFound(ctx)))}, nil
	case errors.Is(err, domain.ErrTestNotPublished):
		return openapi.UpdateAssignment409JSONResponse(httpapi.Error(ctx, openapi.TESTNOTPUBLISHED,
			httpx.Text(ctx, "Chỉ có thể giao một phiên bản đề đã xuất bản.", "Only a published version of a test can be assigned."))), nil
	case errors.Is(err, domain.ErrAssignmentLocked):
		return openapi.UpdateAssignment409JSONResponse(httpapi.Error(ctx, openapi.ASSIGNMENTLOCKED,
			httpx.Text(ctx, "Bài giao đang mở nên không thể đổi đề, thời lượng hoặc số lượt làm.",
				"The assignment is open, so its test, duration and attempts cannot be changed."))), nil
	case errors.Is(err, domain.ErrVersionLocked):
		return openapi.UpdateAssignment409JSONResponse(httpapi.Error(ctx, openapi.VERSIONLOCKED,
			httpx.Text(ctx, "Đã có học viên làm bài, không thể đổi phiên bản đề.",
				"A student has already started, so the test version cannot be changed."))), nil
	default:
		return nil, err
	}
	return openapi.UpdateAssignment200JSONResponse(toAPIAssignment(a)), nil
}

// ReopenAssignment is G-09's "Gia hạn cho tất cả".
func (h Assignments) ReopenAssignment(ctx context.Context, request openapi.ReopenAssignmentRequestObject) (openapi.ReopenAssignmentResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := assignmentRequest(ctx, request.Id.String())
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	a, err := h.app.Commands.Reopen.Handle(ctx, command.Reopen{Request: req, ClosesAt: request.Body.ClosesAt, Reason: request.Body.Reason, Now: time.Now()})
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrBlankReason):
		return openapi.ReopenAssignment400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			httpapi.FieldError(ctx, "reason", httpx.Text(ctx, "Hãy ghi lý do mở lại.", "Give a reason for reopening.")))}, nil
	case errors.Is(err, domain.ErrClosesInPast):
		return openapi.ReopenAssignment400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			httpapi.FieldError(ctx, "closesAt", httpx.Text(ctx, "Thời điểm đóng mới phải ở phía trước.", "The new closing time must be in the future.")))}, nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.ReopenAssignment404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgAssignmentNotFound(ctx)))}, nil
	case errors.Is(err, domain.ErrNotClosed):
		return openapi.ReopenAssignment409JSONResponse(httpapi.Error(ctx, openapi.ASSIGNMENTNOTCLOSED,
			httpx.Text(ctx, "Bài giao chưa đóng nên không có gì để mở lại.",
				"The assignment is not closed, so there is nothing to reopen."))), nil
	default:
		return nil, err
	}
	return openapi.ReopenAssignment200JSONResponse(toAPIAssignment(a)), nil
}

func assignmentRequest(ctx context.Context, id string) (domain.Request, bool) {
	who, ok := httpapi.ActorFromContext(ctx)
	return domain.Request{ID: id, ActorID: who.ID, All: who.Scope.All, IP: who.IP, UserAgent: who.UserAgent}, ok
}

func assignmentValidationError(ctx context.Context, invalid *domain.ValidationError) openapi.ErrorResponse {
	resp := httpapi.Error(ctx, openapi.VALIDATIONFAILED,
		httpx.Text(ctx, "Dữ liệu bài giao không hợp lệ.", "The assignment data is not valid."))
	details := map[string]interface{}{}
	for _, f := range invalid.Fields {
		if _, seen := details[f.Field]; !seen {
			details[f.Field] = f.Message
		}
	}
	resp.Error.Details = &details
	return resp
}

// toWriteInput reads the contract's input. The review options and the student
// note are partial on an update, so it records which of them the body named;
// a note that is not a string or null cannot pass the contract and is refused
// here as well.
func toWriteInput(ctx context.Context, body openapi.AssignmentInput) (domain.WriteInput, error) {
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
			Release:            domain.ReleaseOnSubmit,
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
	if body.Review.Release != nil {
		in.Review.Release, in.ReleaseSet = domain.Release(*body.Review.Release), true
	}
	if body.Review.ShowClassAverage != nil {
		in.Review.ShowClassAverage, in.ClassAverageSet = *body.Review.ShowClassAverage, true
	}
	if len(body.StudentNote) > 0 {
		var note *string
		if err := json.Unmarshal(body.StudentNote, &note); err != nil {
			return domain.WriteInput{}, &domain.ValidationError{Fields: []domain.FieldError{{Field: "studentNote", Message: httpx.Text(ctx, "Ghi chú cho học viên phải là văn bản.", "The note for students must be text.")}}}
		}
		in.StudentNote, in.StudentNoteSet = domain.StudentNoteOf(note), true
	}
	for _, id := range body.Targets.ClassIds {
		in.ClassIDs = append(in.ClassIDs, id.String())
	}
	for _, id := range body.Targets.StudentIds {
		in.StudentIDs = append(in.StudentIDs, id.String())
	}
	return in, nil
}

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/attempts"
	"quizzivy/internal/dashboard"
	"quizzivy/internal/httpx"
	"quizzivy/internal/integrity"
	"quizzivy/internal/review"
)

const msgAttemptNotFound = "Không tìm thấy lượt làm."

// GetAssignmentMonitor backs G-02: one row per targeted student, two queries.
func (s *Server) GetAssignmentMonitor(ctx context.Context, request openapi.GetAssignmentMonitorRequestObject) (openapi.GetAssignmentMonitorResponseObject, error) {
	if s.Deps.Attempts == nil {
		return nil, httpx.ErrNotImplemented
	}
	monitor, err := s.Deps.Attempts.Monitor(ctx, request.Id.String())
	if errors.Is(err, attempts.ErrNotFound) {
		return openapi.GetAssignmentMonitor404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, "Không tìm thấy bài giao."))}, nil
	}
	if err != nil {
		return nil, err
	}

	out := openapi.GetAssignmentMonitor200JSONResponse{
		ServerTime:    monitor.ServerTime,
		QuestionCount: monitor.QuestionCount,
		Rows:          make([]openapi.MonitorRow, len(monitor.Rows)),
	}
	for i, r := range monitor.Rows {
		row := openapi.MonitorRow{
			StudentId:      parseUUID(r.StudentID),
			FullName:       r.FullName,
			State:          openapi.MonitorRowState(r.State),
			AttemptNo:      r.AttemptNo,
			StartedAt:      r.StartedAt,
			DeadlineAt:     r.DeadlineAt,
			SubmittedAt:    r.SubmittedAt,
			AnsweredCount:  r.AnsweredCount,
			FocusLossCount: r.FocusLossCount,
			Flagged:        ptr(r.Flagged),
			AudioOverLimit: ptr(r.AudioOverLimit),
		}
		if r.AttemptID != nil {
			id := rawUUID(*r.AttemptID)
			row.AttemptId = &id
		}
		if r.Score != nil {
			row.Score = toAPIScore(*r.Score)
		}
		out.Rows[i] = row
	}
	return out, nil
}

func toAPIScore(sc attempts.Score) *openapi.AttemptScore {
	return &openapi.AttemptScore{Earned: sc.Earned, Total: sc.Total, PendingManual: sc.PendingManual}
}

// ListAttempts backs the two dashboard queues §15 has no endpoint for.
func (s *Server) ListAttempts(ctx context.Context, request openapi.ListAttemptsRequestObject) (openapi.ListAttemptsResponseObject, error) {
	if s.Deps.Dashboard == nil {
		return nil, httpx.ErrNotImplemented
	}
	in := dashboard.ListInput{Flagged: request.Params.Flagged, PendingGrading: request.Params.PendingGrading}
	if request.Params.Status != nil {
		status := string(*request.Params.Status)
		in.Status = &status
	}
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = *request.Params.Limit
	}
	found, page, err := s.Deps.Dashboard.List(ctx, in)
	if err != nil {
		return nil, err
	}
	out := openapi.ListAttempts200JSONResponse{
		Items: make([]openapi.AttemptListRow, len(found)),
		Page:  page.Number, PageSize: page.Size, Total: page.Total,
	}
	for i, r := range found {
		out.Items[i] = toAPIAttemptListRow(r)
	}
	return out, nil
}

func toAPIAttemptListRow(r dashboard.Recent) openapi.AttemptListRow {
	pending := r.PendingManual
	return openapi.AttemptListRow{
		Id:            parseUUID(r.ID),
		StudentId:     parseUUID(r.StudentID),
		StudentName:   r.StudentName,
		AssignmentId:  parseUUID(r.AssignmentID),
		TestTitle:     r.TestTitle,
		Status:        openapi.AttemptStatus(r.Status),
		SubmittedAt:   r.SubmittedAt,
		Flagged:       r.Flagged,
		PendingManual: &pending,
	}
}

type reviewAnswer = struct {
	Answer         *openapi.Answer `json:"answer"`
	AutoScore      *openapi.Points `json:"autoScore,omitempty"`
	GraderComment  *string         `json:"graderComment,omitempty"`
	ManualScore    *openapi.Points `json:"manualScore,omitempty"`
	RequiresManual *bool           `json:"requiresManual,omitempty"`
}

// GetAttemptForReview backs G-03. This is the one response that carries the
// grading key, and it lives under /admin.
func (s *Server) GetAttemptForReview(ctx context.Context, request openapi.GetAttemptForReviewRequestObject) (openapi.GetAttemptForReviewResponseObject, error) {
	if s.Deps.Review == nil || s.Deps.Auth == nil || s.Deps.Integrity == nil {
		return nil, httpx.ErrNotImplemented
	}
	rv, err := s.Deps.Review.Get(ctx, request.Id.String())
	if errors.Is(err, review.ErrNotFound) {
		return openapi.GetAttemptForReview404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, msgAttemptNotFound))}, nil
	}
	if err != nil {
		return nil, err
	}
	student, err := s.Deps.Auth.CurrentUser(ctx, rv.Attempt.StudentID)
	if err != nil {
		return nil, err
	}
	timeline, err := s.Deps.Integrity.Timeline(ctx, rv.Attempt.ID)
	if err != nil {
		return nil, err
	}

	questions := make([]openapi.AdminQuestion, len(rv.Questions))
	for i, q := range rv.Questions {
		converted, err := s.toAPIReviewQuestion(ctx, q, rv)
		if err != nil {
			return nil, err
		}
		questions[i] = converted
	}
	answers := make(map[string]reviewAnswer, len(rv.Answers))
	for id, a := range rv.Answers {
		entry := reviewAnswer{
			AutoScore: a.AutoScore, ManualScore: a.ManualScore,
			GraderComment: a.GraderComment, RequiresManual: ptr(a.RequiresManual),
		}
		if len(a.Payload) > 0 {
			var decoded openapi.Answer
			if err := json.Unmarshal(a.Payload, &decoded); err != nil {
				return nil, fmt.Errorf("decode stored answer for %s: %w", id, err)
			}
			entry.Answer = &decoded
		}
		answers[id] = entry
	}

	attempt := toAPIAttempt(rv.Attempt)
	if rv.Attempt.Status != attempts.InProgress {
		attempt.Score = toAPIScore(rv.Score)
	}
	return openapi.GetAttemptForReview200JSONResponse{
		Attempt:     attempt,
		Student:     toAPIUser(student),
		TestTitle:   rv.TestTitle,
		MaxAttempts: rv.MaxAttempts,
		Questions:   questions,
		Answers:     answers,
		AudioPlays:  rv.AudioPlays,
		Integrity:   toAPIIntegritySummary(timeline.Summary),
	}, nil
}

func (s *Server) toAPIReviewQuestion(ctx context.Context, q review.Question, rv review.Review) (openapi.AdminQuestion, error) {
	out := openapi.AdminQuestion{
		Id:           parseUUID(q.ID),
		Type:         openapi.QuestionType(q.Type),
		Prompt:       q.Prompt,
		Points:       q.Points,
		Explanation:  q.Explanation,
		SampleAnswer: q.SampleAnswer,
		Transcript:   q.Transcript,
		Tags:         []string{},
		CreatedAt:    rv.PublishedAt,
		UpdatedAt:    rv.PublishedAt,
	}
	if q.Audio != nil {
		out.Audio = &openapi.AudioPolicy{
			MaxPlays: q.Audio.MaxPlays, AllowSeek: q.Audio.AllowSeek,
			ShowTranscriptAfterSubmit: q.Audio.ShowTranscriptAfterSubmit,
		}
	}
	options := make([]openapi.AdminQuestionOption, len(q.Options))
	for i, o := range q.Options {
		options[i] = openapi.AdminQuestionOption{Id: parseUUID(o.ID), Ordinal: o.Ordinal, Text: o.Text, IsCorrect: o.IsCorrect}
	}
	out.Options = &options
	blanks := make([]openapi.AdminQuestionBlank, len(q.Blanks))
	for i, b := range q.Blanks {
		blanks[i] = openapi.AdminQuestionBlank{
			Id: parseUUID(b.ID), Ordinal: b.Ordinal, AcceptedAnswers: b.Accepted, CaseSensitive: b.CaseSensitive,
		}
	}
	out.Blanks = &blanks
	if q.Media != nil && s.Deps.Media != nil {
		asset, err := s.Deps.Media.Get(ctx, q.Media.ID)
		if err != nil {
			return openapi.AdminQuestion{}, fmt.Errorf("resolve review media: %w", err)
		}
		url, err := s.Deps.Media.SignedURL(ctx, asset)
		if err != nil {
			return openapi.AdminQuestion{}, err
		}
		media := toAPIMediaAsset(asset, url)
		out.Media = &media
	}
	return out, nil
}

func toAPIIntegritySummary(sm integrity.Summary) openapi.IntegritySummary {
	return openapi.IntegritySummary{
		TotalAwayMs: sm.TotalAwayMs, AwayEpisodes: sm.AwayEpisodes, PasteCount: sm.PasteCount,
		ResumeCount: sm.ResumeCount, AudioReplays: sm.AudioReplays, OfflineEpisodes: sm.OfflineEpisodes,
	}
}

// GetAttemptEvents backs G-05, the integrity timeline.
func (s *Server) GetAttemptEvents(ctx context.Context, request openapi.GetAttemptEventsRequestObject) (openapi.GetAttemptEventsResponseObject, error) {
	if s.Deps.Integrity == nil {
		return nil, httpx.ErrNotImplemented
	}
	timeline, err := s.Deps.Integrity.Timeline(ctx, request.Id.String())
	if errors.Is(err, integrity.ErrNotFound) {
		return openapi.GetAttemptEvents404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, msgAttemptNotFound))}, nil
	}
	if err != nil {
		return nil, err
	}

	events := make([]openapi.IntegrityEvent, len(timeline.Events))
	for i, e := range timeline.Events {
		event := openapi.IntegrityEvent{
			Id: e.ID, Kind: e.Kind, OccurredAt: e.OccurredAt, OffsetMs: e.OffsetMs,
			DurationMs: e.DurationMs, ClientSeq: -1,
		}
		if e.ClientSeq != nil {
			event.ClientSeq = *e.ClientSeq
		}
		session := parseUUID(e.SessionID)
		event.SessionId = &session
		if e.QuestionID != nil {
			id := rawUUID(*e.QuestionID)
			event.QuestionId = &id
		}
		if len(e.Meta) > 0 {
			var meta map[string]interface{}
			if err := json.Unmarshal(e.Meta, &meta); err == nil {
				event.Meta = &meta
			}
		}
		events[i] = event
	}
	return openapi.GetAttemptEvents200JSONResponse{
		StartedAt: timeline.StartedAt,
		Events:    events,
		Summary:   toAPIIntegritySummary(timeline.Summary),
	}, nil
}

func attemptRequest(ctx context.Context) (attempts.Request, bool) {
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return attempts.Request{}, false
	}
	meta := httpx.RequestMetaFromContext(ctx)
	return attempts.Request{ActorID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent}, true
}

func blankReason(ctx context.Context) openapi.ErrorResponse {
	resp := authError(ctx, openapi.VALIDATIONFAILED, "Cần ghi lý do.")
	details := map[string]interface{}{"reason": "Lý do không được để trống."}
	resp.Error.Details = &details
	return resp
}

// ExtendAttempt is the first of §8's three interventions; each takes a reason.
func (s *Server) ExtendAttempt(ctx context.Context, request openapi.ExtendAttemptRequestObject) (openapi.ExtendAttemptResponseObject, error) {
	if s.Deps.Attempts == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := attemptRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	extended, err := s.Deps.Attempts.Extend(ctx, req, request.Id.String(), request.Body.Minutes, request.Body.Reason)
	switch {
	case errors.Is(err, attempts.ErrBlankReason):
		return openapi.ExtendAttempt400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(blankReason(ctx))}, nil
	case errors.Is(err, attempts.ErrNotFound):
		return openapi.ExtendAttempt404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(notFound(ctx, msgAttemptNotFound))}, nil
	case errors.Is(err, attempts.ErrAttemptVoided):
		return openapi.ExtendAttempt409JSONResponse(authError(ctx, openapi.ATTEMPTVOIDED, "Lượt làm này đã bị huỷ.")), nil
	case errors.Is(err, attempts.ErrAttemptClosed):
		return openapi.ExtendAttempt409JSONResponse(authError(ctx, openapi.ATTEMPTCLOSED, "Lượt làm này đã kết thúc nên không gia hạn được.")), nil
	case err != nil:
		return nil, err
	}
	return openapi.ExtendAttempt200JSONResponse(toAPIAttempt(extended)), nil
}

func (s *Server) ResetAttempt(ctx context.Context, request openapi.ResetAttemptRequestObject) (openapi.ResetAttemptResponseObject, error) {
	if s.Deps.Attempts == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := attemptRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	voided, err := s.Deps.Attempts.Reset(ctx, req, request.Id.String(), request.Body.Reason)
	switch {
	case errors.Is(err, attempts.ErrBlankReason):
		return openapi.ResetAttempt400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(blankReason(ctx))}, nil
	case errors.Is(err, attempts.ErrNotFound):
		return openapi.ResetAttempt404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(notFound(ctx, msgAttemptNotFound))}, nil
	case errors.Is(err, attempts.ErrAttemptVoided):
		return openapi.ResetAttempt409JSONResponse(authError(ctx, openapi.ATTEMPTVOIDED, "Lượt làm này đã bị huỷ.")), nil
	case err != nil:
		return nil, err
	}
	return openapi.ResetAttempt200JSONResponse(toAPIAttempt(voided)), nil
}

func (s *Server) VoidAttempt(ctx context.Context, request openapi.VoidAttemptRequestObject) (openapi.VoidAttemptResponseObject, error) {
	if s.Deps.Attempts == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := attemptRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	voided, err := s.Deps.Attempts.Void(ctx, req, request.Id.String(), request.Body.Reason)
	switch {
	case errors.Is(err, attempts.ErrBlankReason):
		return openapi.VoidAttempt400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(blankReason(ctx))}, nil
	case errors.Is(err, attempts.ErrNotFound):
		return openapi.VoidAttempt404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(notFound(ctx, msgAttemptNotFound))}, nil
	case errors.Is(err, attempts.ErrAttemptVoided):
		return openapi.VoidAttempt409JSONResponse(authError(ctx, openapi.ATTEMPTVOIDED, "Lượt làm này đã bị huỷ.")), nil
	case err != nil:
		return nil, err
	}
	return openapi.VoidAttempt200JSONResponse(toAPIAttempt(voided)), nil
}

// GradeAttempt saves manual marks, per call rather than as one submit.
func (s *Server) GradeAttempt(ctx context.Context, request openapi.GradeAttemptRequestObject) (openapi.GradeAttemptResponseObject, error) {
	if s.Deps.Review == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	items := make([]review.Item, len(request.Body.Items))
	for i, it := range request.Body.Items {
		items[i] = review.Item{QuestionID: it.QuestionId.String(), Points: it.Points, Comment: it.Comment}
	}
	score, err := s.Deps.Review.Grade(ctx, request.Id.String(), principal.UserID, items)
	var invalid *review.ValidationError
	switch {
	case errors.As(err, &invalid):
		return openapi.GradeAttempt400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(gradeValidationError(ctx, invalid))}, nil
	case errors.Is(err, review.ErrNotFound):
		return openapi.GradeAttempt404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(notFound(ctx, msgAttemptNotFound))}, nil
	case errors.Is(err, review.ErrInProgress):
		return openapi.GradeAttempt409JSONResponse(authError(ctx, openapi.ATTEMPTINPROGRESS, "Học viên chưa nộp bài.")), nil
	case errors.Is(err, review.ErrVoided):
		return openapi.GradeAttempt409JSONResponse(authError(ctx, openapi.ATTEMPTVOIDED, "Lượt làm này đã bị huỷ.")), nil
	case err != nil:
		return nil, err
	}
	return openapi.GradeAttempt200JSONResponse(*toAPIScore(score)), nil
}

func gradeValidationError(ctx context.Context, invalid *review.ValidationError) openapi.ErrorResponse {
	resp := authError(ctx, openapi.VALIDATIONFAILED, "Điểm không hợp lệ.")
	details := map[string]interface{}{}
	for _, it := range invalid.Items {
		details[it.QuestionID] = gradeItemMessage(it.Reason)
	}
	resp.Error.Details = &details
	return resp
}

func gradeItemMessage(reason string) string {
	switch reason {
	case "above_ceiling":
		return "Điểm vượt quá điểm tối đa của câu."
	case "unanswered":
		return "Học viên không trả lời câu này."
	default:
		return "Câu này không có trong đề."
	}
}

// FinishGrading recomputes the score from `final_score` and declares the paper graded.
func (s *Server) FinishGrading(ctx context.Context, request openapi.FinishGradingRequestObject) (openapi.FinishGradingResponseObject, error) {
	if s.Deps.Review == nil {
		return nil, httpx.ErrNotImplemented
	}
	graded, err := s.Deps.Review.Finish(ctx, request.Id.String())
	switch {
	case errors.Is(err, review.ErrNotFound):
		return openapi.FinishGrading404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(notFound(ctx, msgAttemptNotFound))}, nil
	case errors.Is(err, review.ErrIncomplete):
		return openapi.FinishGrading409JSONResponse(authError(ctx, openapi.GRADINGINCOMPLETE, "Còn câu tự luận chưa chấm.")), nil
	case errors.Is(err, review.ErrInProgress):
		return openapi.FinishGrading409JSONResponse(authError(ctx, openapi.ATTEMPTINPROGRESS, "Học viên chưa nộp bài.")), nil
	case errors.Is(err, review.ErrVoided):
		return openapi.FinishGrading409JSONResponse(authError(ctx, openapi.ATTEMPTVOIDED, "Lượt làm này đã bị huỷ.")), nil
	case err != nil:
		return nil, err
	}
	return openapi.FinishGrading200JSONResponse(toAPIAttempt(graded)), nil
}

// rawUUID is parseUUID for the fields the generator typed as the runtime's
// UUID rather than the contract's alias.
func rawUUID(s string) openapi_types.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return openapi_types.UUID{}
	}
	return id
}

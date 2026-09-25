package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	identityquery "quizzivy/internal/modules/identity/application/query"
	identitydomain "quizzivy/internal/modules/identity/domain"
	mediahttp "quizzivy/internal/modules/media/http"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

const msgAttemptNotFound = "Không tìm thấy lượt làm."

// GetAssignmentMonitor backs G-02: one row per targeted student, two queries.
func (h Attempts) GetAssignmentMonitor(ctx context.Context, request openapi.GetAssignmentMonitorRequestObject) (openapi.GetAssignmentMonitorResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	if _, err := h.app.Commands.ExpireDue.Handle(ctx, command.ExpireDue{AssignmentID: request.Id.String()}); err != nil {
		return nil, err
	}
	monitor, err := h.app.Queries.Monitor.Handle(ctx, query.Monitor{AssignmentID: request.Id.String()})
	if errors.Is(err, domain.ErrNotFound) {
		return openapi.GetAssignmentMonitor404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, "Không tìm thấy bài giao."))}, nil
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
			StudentId:      httpapi.ParseUUID(r.StudentID),
			FullName:       r.FullName,
			State:          openapi.MonitorRowState(r.State),
			AttemptNo:      r.AttemptNo,
			StartedAt:      r.StartedAt,
			DeadlineAt:     r.DeadlineAt,
			SubmittedAt:    r.SubmittedAt,
			AnsweredCount:  r.AnsweredCount,
			FocusLossCount: r.FocusLossCount,
			Flagged:        httpapi.Ptr(r.Flagged),
			AudioOverLimit: httpapi.Ptr(r.AudioOverLimit),
		}
		if r.AttemptID != nil {
			id := httpapi.RawUUID(*r.AttemptID)
			row.AttemptId = &id
		}
		if r.Score != nil {
			row.Score = toAPIScore(*r.Score)
		}
		out.Rows[i] = row
	}
	return out, nil
}

func toAPIScore(sc domain.Score) *openapi.AttemptScore {
	return &openapi.AttemptScore{Earned: sc.Earned, Total: sc.Total, PendingManual: sc.PendingManual}
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
func (h Attempts) GetAttemptForReview(ctx context.Context, request openapi.GetAttemptForReviewRequestObject) (openapi.GetAttemptForReviewResponseObject, error) {
	if h.app == nil || h.students == nil {
		return nil, httpx.ErrNotImplemented
	}
	rv, err := h.app.Queries.Review.Handle(ctx, query.Review{AttemptID: request.Id.String()})
	if errors.Is(err, domain.ErrGroupContextUnavailable) {
		return nil, httpx.ErrNotImplemented
	}
	if errors.Is(err, domain.ErrPaperNotFound) {
		return openapi.GetAttemptForReview404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgAttemptNotFound))}, nil
	}
	if err != nil {
		return nil, err
	}

	student, err := h.students.Handle(ctx, identityquery.GetStudent{ID: rv.Attempt.StudentID})
	if err != nil {
		return nil, err
	}
	timeline, err := h.app.Queries.Timeline.Handle(ctx, query.Timeline{AttemptID: rv.Attempt.ID})
	if err != nil {
		return nil, err
	}
	questions, err := h.toAPIReviewQuestions(ctx, rv)
	if err != nil {
		return nil, err
	}
	answers, err := toAPIReviewAnswers(rv.Answers)
	if err != nil {
		return nil, err
	}

	attempt := toAPIAttempt(rv.Attempt)
	shared, err := sharedReviewContext(ctx, rv.SharedContext, h.adminGroupAsset)
	if err != nil {
		return nil, err
	}
	if rv.Attempt.Status != domain.InProgress {
		attempt.Score = toAPIScore(rv.Score)
	}
	return openapi.GetAttemptForReview200JSONResponse{
		SharedContext: shared,
		Attempt:       attempt,
		Student:       toAPIUserFromStudent(student),
		TestTitle:     rv.TestTitle,
		MaxAttempts:   rv.MaxAttempts,
		Questions:     questions,
		Answers:       answers,
		AudioPlays:    rv.AudioPlays,
		Integrity:     toAPIIntegritySummary(timeline.Summary),
		TeacherNote:   rv.TeacherNote,
	}, nil
}

func (h Attempts) toAPIReviewQuestions(ctx context.Context, rv domain.Review) ([]openapi.AdminQuestion, error) {
	questions := make([]openapi.AdminQuestion, len(rv.Questions))
	for i, q := range rv.Questions {
		converted, err := h.toAPIReviewQuestion(ctx, q, rv.PublishedAt)
		if err != nil {
			return nil, err
		}
		questions[i] = converted
	}
	return questions, nil
}

func toAPIReviewAnswers(stored map[string]domain.ReviewAnswer) (map[string]reviewAnswer, error) {
	answers := make(map[string]reviewAnswer, len(stored))
	for id, a := range stored {
		entry := reviewAnswer{
			AutoScore: a.AutoScore, ManualScore: a.ManualScore,
			GraderComment: a.GraderComment, RequiresManual: httpapi.Ptr(a.RequiresManual),
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
	return answers, nil
}

// ListAnswersForQuestion is G-04's read: one question, every paper.
func (h Attempts) ListAnswersForQuestion(ctx context.Context, request openapi.ListAnswersForQuestionRequestObject) (openapi.ListAnswersForQuestionResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	byQ, err := h.app.Queries.AnswersForQuestion.Handle(ctx, query.AnswersForQuestion{AssignmentID: request.Id.String(), QuestionID: request.Params.QuestionId.String()})
	switch {
	case errors.Is(err, domain.ErrPaperNotFound):
		return openapi.ListAnswersForQuestion404JSONResponse(httpapi.NotFound(ctx, "Không tìm thấy bài giao.")), nil
	case errors.Is(err, domain.ErrQuestionNotOnPaper):
		return openapi.ListAnswersForQuestion404JSONResponse(httpapi.NotFound(ctx, "Câu hỏi này không có trong đề của bài giao.")), nil
	case errors.Is(err, domain.ErrGroupContextUnavailable):
		return nil, httpx.ErrNotImplemented
	case err != nil:
		return nil, err
	}
	question, err := h.toAPIReviewQuestion(ctx, byQ.Question, byQ.PublishedAt)
	if err != nil {
		return nil, err
	}
	manual := make([]openapi.Uuid, len(byQ.ManualIDs))
	for i, id := range byQ.ManualIDs {
		manual[i] = httpapi.ParseUUID(id)
	}
	items := make([]openapi.QuestionAnswerRow, len(byQ.Items))
	for i, it := range byQ.Items {
		row := openapi.QuestionAnswerRow{
			AttemptId:     httpapi.ParseUUID(it.AttemptID),
			StudentId:     httpapi.ParseUUID(it.StudentID),
			StudentName:   it.StudentName,
			AttemptNo:     it.AttemptNo,
			ManualScore:   it.ManualScore,
			GraderComment: it.GraderComment,
		}
		if len(it.Payload) > 0 {
			var decoded openapi.Answer
			if err := json.Unmarshal(it.Payload, &decoded); err != nil {
				return nil, fmt.Errorf("decode stored answer for %s: %w", it.AttemptID, err)
			}
			row.Answer = &decoded
		}
		items[i] = row
	}
	shared, err := sharedReviewContext(ctx, byQ.SharedContext, h.adminGroupAsset)
	if err != nil {
		return nil, err
	}
	return openapi.ListAnswersForQuestion200JSONResponse{
		SharedContext:     shared,
		Question:          question,
		QuestionNumber:    byQ.Number,
		QuestionCount:     byQ.Count,
		ManualQuestionIds: manual,
		Items:             items,
	}, nil
}

// SetAttemptNote keeps G-05's private note.
func (h Attempts) SetAttemptNote(ctx context.Context, request openapi.SetAttemptNoteRequestObject) (openapi.SetAttemptNoteResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	_, err := h.app.Commands.SetNote.Handle(ctx, command.SetNote{AttemptID: request.Id.String(), Note: request.Body.Note})
	if errors.Is(err, domain.ErrPaperNotFound) {
		return openapi.SetAttemptNote404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgAttemptNotFound))}, nil
	}
	if err != nil {
		return nil, err
	}
	rv, err := h.app.Queries.Review.Handle(ctx, query.Review{AttemptID: request.Id.String()})
	if err != nil {
		return nil, err
	}
	return openapi.SetAttemptNote200JSONResponse{Note: rv.TeacherNote}, nil
}

// FlagAttempt is G-05's mark, set or cleared by hand.
func (h Attempts) FlagAttempt(ctx context.Context, request openapi.FlagAttemptRequestObject) (openapi.FlagAttemptResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := attemptRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	reason := ""
	if request.Body.Reason != nil {
		reason = *request.Body.Reason
	}
	flagged, err := h.app.Commands.Flag.Handle(ctx, command.Flag{Request: req, AttemptID: request.Id.String(), Flagged: request.Body.Flagged, Reason: reason})
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return openapi.FlagAttempt404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgAttemptNotFound))}, nil
	case errors.Is(err, domain.ErrAttemptVoided):
		return openapi.FlagAttempt409JSONResponse(httpapi.Error(ctx, openapi.ATTEMPTVOIDED, "Lượt làm này đã bị huỷ.")), nil
	case err != nil:
		return nil, err
	}
	return openapi.FlagAttempt200JSONResponse(toAPIAttempt(flagged)), nil
}

func (h Attempts) toAPIReviewQuestion(ctx context.Context, q domain.ReviewQuestion, publishedAt time.Time) (openapi.AdminQuestion, error) {
	out := openapi.AdminQuestion{
		Id:                 httpapi.ParseUUID(q.ID),
		Type:               openapi.QuestionType(q.Type),
		Prompt:             q.Prompt,
		PromptContent:      q.PromptContent,
		Points:             q.Points,
		Explanation:        q.Explanation,
		ExplanationContent: q.ExplanationContent,
		SampleAnswer:       q.SampleAnswer,
		Transcript:         q.Transcript,
		Tags:               []string{},
		CreatedAt:          publishedAt,
		UpdatedAt:          publishedAt,
	}
	if q.Audio != nil {
		out.Audio = &openapi.AudioPolicy{
			MaxPlays: q.Audio.MaxPlays, AllowSeek: q.Audio.AllowSeek,
			ShowTranscriptAfterSubmit: q.Audio.ShowTranscriptAfterSubmit,
		}
	}
	options := make([]openapi.AdminQuestionOption, len(q.Options))
	for i, o := range q.Options {
		options[i] = openapi.AdminQuestionOption{Id: httpapi.ParseUUID(o.ID), Ordinal: o.Ordinal, Text: o.Text, Content: o.Content, IsCorrect: o.IsCorrect}
	}
	out.Options = &options
	blanks := make([]openapi.AdminQuestionBlank, len(q.Blanks))
	for i, b := range q.Blanks {
		blanks[i] = openapi.AdminQuestionBlank{
			GapId: b.GapID,
			Id:    httpapi.ParseUUID(b.ID), Ordinal: b.Ordinal, AcceptedAnswers: b.Accepted, CaseSensitive: b.CaseSensitive,
		}
	}
	out.Blanks = &blanks
	if q.Media != nil && h.media != nil {
		asset, err := h.media.Get(ctx, q.Media.ID)
		if err != nil {
			return openapi.AdminQuestion{}, fmt.Errorf("resolve review media: %w", err)
		}
		url, err := h.media.SignedURL(ctx, asset)
		if err != nil {
			return openapi.AdminQuestion{}, err
		}
		media := mediahttp.ToAPIMediaAsset(asset, url)
		out.Media = &media
	}
	return out, nil
}

func toAPIIntegritySummary(sm domain.IntegritySummary) openapi.IntegritySummary {
	return openapi.IntegritySummary{
		TotalAwayMs: sm.TotalAwayMs, AwayEpisodes: sm.AwayEpisodes, PasteCount: sm.PasteCount,
		ResumeCount: sm.ResumeCount, AudioReplays: sm.AudioReplays, OfflineEpisodes: sm.OfflineEpisodes,
	}
}

// GetAttemptEvents backs G-05, the integrity timeline.
func (h Attempts) GetAttemptEvents(ctx context.Context, request openapi.GetAttemptEventsRequestObject) (openapi.GetAttemptEventsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	timeline, err := h.app.Queries.Timeline.Handle(ctx, query.Timeline{AttemptID: request.Id.String()})
	if errors.Is(err, domain.ErrTimelineNotFound) {
		return openapi.GetAttemptEvents404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgAttemptNotFound))}, nil
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
		session := httpapi.ParseUUID(e.SessionID)
		event.SessionId = &session
		if e.QuestionID != nil {
			id := httpapi.RawUUID(*e.QuestionID)
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

func attemptRequest(ctx context.Context) (domain.Request, bool) {
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return domain.Request{}, false
	}
	meta := httpx.RequestMetaFromContext(ctx)
	return domain.Request{ActorID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent}, true
}

// ExtendAttempt is the first of §8's three interventions; each takes a reason.
func (h Attempts) ExtendAttempt(ctx context.Context, request openapi.ExtendAttemptRequestObject) (openapi.ExtendAttemptResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	req, ok := attemptRequest(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	extended, err := h.app.Commands.Extend.Handle(ctx, command.Extend{Request: req, AttemptID: request.Id.String(), Minutes: request.Body.Minutes, Reason: request.Body.Reason})
	switch {
	case errors.Is(err, domain.ErrBlankReason):
		return openapi.ExtendAttempt400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(httpapi.BlankReason(ctx))}, nil
	case errors.Is(err, domain.ErrNotFound):
		return openapi.ExtendAttempt404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgAttemptNotFound))}, nil
	case errors.Is(err, domain.ErrAttemptVoided):
		return openapi.ExtendAttempt409JSONResponse(httpapi.Error(ctx, openapi.ATTEMPTVOIDED, "Lượt làm này đã bị huỷ.")), nil
	case errors.Is(err, domain.ErrAttemptClosed):
		return openapi.ExtendAttempt409JSONResponse(httpapi.Error(ctx, openapi.ATTEMPTCLOSED, "Lượt làm này đã kết thúc nên không gia hạn được.")), nil
	case err != nil:
		return nil, err
	}
	return openapi.ExtendAttempt200JSONResponse(toAPIAttempt(extended)), nil
}

func (h Attempts) ResetAttempt(ctx context.Context, request openapi.ResetAttemptRequestObject) (openapi.ResetAttemptResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	done, refused, err := h.intervene(ctx, request.Id.String(), request.Body.Reason, h.reset)
	switch {
	case err != nil:
		return nil, err
	case refused == refusedBlankReason:
		return openapi.ResetAttempt400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(httpapi.BlankReason(ctx))}, nil
	case refused == refusedNotFound:
		return openapi.ResetAttempt404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgAttemptNotFound))}, nil
	case refused == refusedVoided:
		return openapi.ResetAttempt409JSONResponse(httpapi.Error(ctx, openapi.ATTEMPTVOIDED, msgAttemptVoided)), nil
	}
	return openapi.ResetAttempt200JSONResponse(toAPIAttempt(done)), nil
}

func (h Attempts) VoidAttempt(ctx context.Context, request openapi.VoidAttemptRequestObject) (openapi.VoidAttemptResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	done, refused, err := h.intervene(ctx, request.Id.String(), request.Body.Reason, h.void)
	switch {
	case err != nil:
		return nil, err
	case refused == refusedBlankReason:
		return openapi.VoidAttempt400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(httpapi.BlankReason(ctx))}, nil
	case refused == refusedNotFound:
		return openapi.VoidAttempt404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgAttemptNotFound))}, nil
	case refused == refusedVoided:
		return openapi.VoidAttempt409JSONResponse(httpapi.Error(ctx, openapi.ATTEMPTVOIDED, msgAttemptVoided)), nil
	}
	return openapi.VoidAttempt200JSONResponse(toAPIAttempt(done)), nil
}

const msgAttemptVoided = "Lượt làm này đã bị huỷ."

type interventionRefusal int

const (
	refusedNone interventionRefusal = iota
	refusedBlankReason
	refusedNotFound
	refusedVoided
)

func (h Attempts) reset(ctx context.Context, req domain.Request, id, reason string) (domain.Attempt, error) {
	return h.app.Commands.Reset.Handle(ctx, command.Reset{Request: req, AttemptID: id, Reason: reason})
}

func (h Attempts) void(ctx context.Context, req domain.Request, id, reason string) (domain.Attempt, error) {
	return h.app.Commands.Void.Handle(ctx, command.Void{Request: req, AttemptID: id, Reason: reason})
}

func (h Attempts) intervene(ctx context.Context, id, reason string,
	act func(context.Context, domain.Request, string, string) (domain.Attempt, error)) (domain.Attempt, interventionRefusal, error) {
	req, ok := attemptRequest(ctx)
	if !ok {
		return domain.Attempt{}, refusedNone, httpx.ErrNotImplemented
	}
	done, err := act(ctx, req, id, reason)
	switch {
	case errors.Is(err, domain.ErrBlankReason):
		return domain.Attempt{}, refusedBlankReason, nil
	case errors.Is(err, domain.ErrNotFound):
		return domain.Attempt{}, refusedNotFound, nil
	case errors.Is(err, domain.ErrAttemptVoided):
		return domain.Attempt{}, refusedVoided, nil
	case err != nil:
		return domain.Attempt{}, refusedNone, err
	}
	return done, refusedNone, nil
}

// GradeAttempt saves manual marks, per call rather than as one submit.
func (h Attempts) GradeAttempt(ctx context.Context, request openapi.GradeAttemptRequestObject) (openapi.GradeAttemptResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	items := make([]domain.GradeItem, len(request.Body.Items))
	for i, it := range request.Body.Items {
		items[i] = domain.GradeItem{QuestionID: it.QuestionId.String(), Points: it.Points, Comment: it.Comment}
	}
	score, err := h.app.Commands.Grade.Handle(ctx, command.Grade{AttemptID: request.Id.String(), GraderID: principal.UserID, Items: items})
	var invalid *domain.GradeValidationError
	switch {
	case errors.As(err, &invalid):
		return openapi.GradeAttempt400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(gradeValidationError(ctx, invalid))}, nil
	case errors.Is(err, domain.ErrPaperNotFound):
		return openapi.GradeAttempt404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgAttemptNotFound))}, nil
	case errors.Is(err, domain.ErrPaperInProgress):
		return openapi.GradeAttempt409JSONResponse(httpapi.Error(ctx, openapi.ATTEMPTINPROGRESS, "Học viên chưa nộp bài.")), nil
	case errors.Is(err, domain.ErrPaperVoided):
		return openapi.GradeAttempt409JSONResponse(httpapi.Error(ctx, openapi.ATTEMPTVOIDED, "Lượt làm này đã bị huỷ.")), nil
	case err != nil:
		return nil, err
	}
	return openapi.GradeAttempt200JSONResponse(*toAPIScore(score)), nil
}

func gradeValidationError(ctx context.Context, invalid *domain.GradeValidationError) openapi.ErrorResponse {
	resp := httpapi.Error(ctx, openapi.VALIDATIONFAILED, "Điểm không hợp lệ.")
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
func (h Attempts) FinishGrading(ctx context.Context, request openapi.FinishGradingRequestObject) (openapi.FinishGradingResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	graded, err := h.app.Commands.Finish.Handle(ctx, command.Finish{AttemptID: request.Id.String()})
	switch {
	case errors.Is(err, domain.ErrPaperNotFound):
		return openapi.FinishGrading404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgAttemptNotFound))}, nil
	case errors.Is(err, domain.ErrGradingIncomplete):
		return openapi.FinishGrading409JSONResponse(httpapi.Error(ctx, openapi.GRADINGINCOMPLETE, "Còn câu tự luận chưa chấm.")), nil
	case errors.Is(err, domain.ErrPaperInProgress):
		return openapi.FinishGrading409JSONResponse(httpapi.Error(ctx, openapi.ATTEMPTINPROGRESS, "Học viên chưa nộp bài.")), nil
	case errors.Is(err, domain.ErrPaperVoided):
		return openapi.FinishGrading409JSONResponse(httpapi.Error(ctx, openapi.ATTEMPTVOIDED, "Lượt làm này đã bị huỷ.")), nil
	case err != nil:
		return nil, err
	}
	return openapi.FinishGrading200JSONResponse(toAPIAttempt(graded)), nil
}

func toAPIUserFromStudent(st identitydomain.Student) openapi.User {
	providers := make([]openapi.UserLinkedProviders, 0, len(st.LinkedProviders))
	for _, p := range st.LinkedProviders {
		providers = append(providers, openapi.UserLinkedProviders(p))
	}
	return openapi.User{
		Id:                 httpapi.ParseUUID(st.ID),
		Email:              openapi_types.Email(st.Email),
		FullName:           st.FullName,
		Role:               openapi.RoleStudent,
		HasPassword:        st.HasPassword,
		LinkedProviders:    providers,
		MustChangePassword: st.MustChangePassword,
		CreatedAt:          st.CreatedAt,
	}
}

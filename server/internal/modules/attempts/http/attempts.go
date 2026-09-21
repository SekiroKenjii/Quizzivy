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
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

// StartOrResumeAttempt backs §9's "Bắt đầu": one call whether the student is
// starting, reloading after a crash, or arriving on a second device.
func (h Attempts) StartOrResumeAttempt(ctx context.Context, request openapi.StartOrResumeAttemptRequestObject) (openapi.StartOrResumeAttemptResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	session, err := h.app.Commands.StartOrResume.Handle(ctx, command.StartOrResume{AssignmentID: request.Id.String(), StudentID: principal.UserID})
	switch {
	case errors.Is(err, domain.ErrForbidden), errors.Is(err, domain.ErrNotFound):

		return openapi.StartOrResumeAttempt403JSONResponse{
			ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
				httpapi.Error(ctx, openapi.FORBIDDEN, "Bạn không có quyền làm bài này.")),
		}, nil
	case errors.Is(err, domain.ErrAssignmentClosed):
		return openapi.StartOrResumeAttempt409JSONResponse(httpapi.Error(ctx,
			openapi.ASSIGNMENTNOTOPEN, "Bài thi này hiện không mở.")), nil
	case errors.Is(err, domain.ErrLimitReached):
		return openapi.StartOrResumeAttempt409JSONResponse(httpapi.Error(ctx,
			openapi.ATTEMPTLIMITREACHED, "Bạn đã dùng hết số lượt làm bài.")), nil
	case err != nil:
		return nil, err
	}

	payload, err := h.toAPIAttemptSession(ctx, principal.UserID, session)
	if err != nil {
		return nil, err
	}
	return openapi.StartOrResumeAttempt200JSONResponse(payload), nil
}

// GetAttempt is the ONLY way a student reaches test content (§7). Its response
// is the same shape the start returns, so there is one definition of what a
// student may see rather than two that can drift apart.
func (h Attempts) GetAttempt(ctx context.Context, request openapi.GetAttemptRequestObject) (openapi.GetAttemptResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	session, err := h.app.Queries.Get.Handle(ctx, query.Get{AttemptID: request.Id.String(), StudentID: principal.UserID})
	if errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrNotFound) {
		return openapi.GetAttempt403JSONResponse{
			ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
				httpapi.Error(ctx, openapi.FORBIDDEN, "Bạn không có quyền xem bài làm này.")),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	payload, err := h.toAPIAttemptSession(ctx, principal.UserID, session)
	if err != nil {
		return nil, err
	}
	return openapi.GetAttempt200JSONResponse(payload), nil
}

func (h Attempts) toAPIAttemptSession(ctx context.Context, studentID string, in domain.Session) (openapi.AttemptSession, error) {
	questions := make([]openapi.StudentQuestion, len(in.Questions))
	for i, q := range in.Questions {
		converted, err := h.toAPIStudentQuestion(ctx, studentID, q)
		if err != nil {
			return openapi.AttemptSession{}, err
		}
		questions[i] = converted
	}

	answers, err := toAPIAnswers(in.Answers)
	if err != nil {
		return openapi.AttemptSession{}, err
	}

	sections := make([]openapi.StudentSection, len(in.Sections))
	for i, sec := range in.Sections {
		sections[i] = openapi.StudentSection{
			Id:           httpapi.ParseUUID(sec.ID),
			Title:        sec.Title,
			Instructions: sec.Instructions,
		}
	}

	return openapi.AttemptSession{
		Attempt:     toAPIAttempt(in.Attempt),
		TestTitle:   in.TestTitle,
		Sections:    sections,
		Questions:   questions,
		SessionId:   httpapi.ParseUUID(in.SessionID),
		BeaconToken: in.BeaconToken,
		ServerTime:  in.ServerTime,
		AudioPlays:  in.AudioPlays,
		Answers:     answers,
		Integrity: openapi.IntegrityPolicy{
			RequireFullscreen: in.Integrity.RequireFullscreen,
			BlockCopyPaste:    in.Integrity.BlockCopyPaste,
			MaxFocusLoss:      in.Integrity.MaxFocusLoss,
			OnLimitExceeded:   openapi.IntegrityPolicyOnLimitExceeded(in.Integrity.OnLimitExceeded),
			MinAwayMs:         in.Integrity.MinAwayMs,
		},
	}, nil
}

func toAPIAttempt(a domain.Attempt) openapi.Attempt {
	out := openapi.Attempt{
		Id:            httpapi.ParseUUID(a.ID),
		AssignmentId:  httpapi.ParseUUID(a.AssignmentID),
		StudentId:     httpapi.ParseUUID(a.StudentID),
		TestVersionId: httpapi.ParseUUID(a.TestVersionID),
		AttemptNo:     a.AttemptNo,
		Status:        openapi.AttemptStatus(a.Status),
		StartedAt:     a.StartedAt,
		DeadlineAt:    a.DeadlineAt,
		SubmittedAt:   a.SubmittedAt,
		GradedAt:      a.GradedAt,
	}
	out.Integrity = &struct {
		Flagged        bool `json:"flagged"`
		FocusLossCount int  `json:"focusLossCount"`
	}{Flagged: a.Flagged, FocusLossCount: a.FocusLossCount}
	return out
}

func (h Attempts) toAPIStudentQuestion(ctx context.Context, studentID string, q domain.Question) (openapi.StudentQuestion, error) {
	out := openapi.StudentQuestion{
		Id:        httpapi.ParseUUID(q.ID),
		SectionId: httpapi.ParseUUID(q.SectionID),
		Type:      openapi.QuestionType(q.Type),
		Prompt:    q.Prompt,
		Points:    q.Points,
	}
	if len(q.Options) > 0 {
		options := make([]openapi.StudentOption, len(q.Options))
		for i, o := range q.Options {
			options[i] = openapi.StudentOption{Id: httpapi.ParseUUID(o.ID), Text: o.Text}
		}
		out.Options = &options
	}
	if len(q.Blanks) > 0 {
		blanks := make([]openapi.StudentBlank, len(q.Blanks))
		for i, b := range q.Blanks {
			blanks[i] = openapi.StudentBlank{
				Id:            httpapi.ParseUUID(b.ID),
				Ordinal:       b.Ordinal,
				CaseSensitive: b.CaseSensitive,
			}
		}
		out.Blanks = &blanks
	}
	if q.Audio != nil {
		out.Audio = &openapi.AudioPolicy{
			MaxPlays:                  q.Audio.MaxPlays,
			AllowSeek:                 q.Audio.AllowSeek,
			ShowTranscriptAfterSubmit: q.Audio.ShowTranscriptAfterSubmit,
		}
	}
	if q.Media == nil {
		return out, nil
	}

	signed, err := h.media.MintForStudent(ctx, studentID, q.Media.ID)
	if err != nil {
		return openapi.StudentQuestion{}, fmt.Errorf("sign attempt media: %w", err)
	}
	out.Media = &openapi.MediaAsset{
		Id:               httpapi.ParseUUID(q.Media.ID),
		Kind:             openapi.MediaKind(q.Media.Kind),
		MimeType:         openapi.MediaAssetMimeType(q.Media.MimeType),
		OriginalFilename: q.Media.Filename,
		Bytes:            q.Media.Bytes,
		DurationMs:       q.Media.DurationMs,
		CreatedAt:        q.Media.CreatedAt,
		Url:              signed.URL,
	}
	return out, nil
}

func toAPIAnswers(stored map[string][]byte) (map[string]openapi.Answer, error) {
	out := make(map[string]openapi.Answer, len(stored))
	for questionID, payload := range stored {
		var answer openapi.Answer
		if err := json.Unmarshal(payload, &answer); err != nil {
			return nil, fmt.Errorf("decode stored answer for %s: %w", questionID, err)
		}
		out[questionID] = answer
	}
	return out, nil
}

// SaveAnswers is §9's autosave: answers and the events that accompanied them,
// in one call and one transaction.
func (h Attempts) SaveAnswers(ctx context.Context, request openapi.SaveAnswersRequestObject) (openapi.SaveAnswersResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	in := domain.SaveInput{
		AttemptID: request.Id.String(),
		StudentID: principal.UserID,
		SessionID: request.Body.SessionId.String(),
	}
	var err error
	if in.Answers, err = toDomainAnswers(request.Body.Answers); err != nil {
		return nil, err
	}
	if in.Events, err = toDomainEvents(request.Body.Events); err != nil {
		return nil, err
	}

	saved, err := h.app.Commands.Save.Handle(ctx, command.Save{Input: in})
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return openapi.SaveAnswers403JSONResponse{
			ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
				httpapi.Error(ctx, openapi.FORBIDDEN, "Bạn không có quyền ghi vào bài làm này.")),
		}, nil
	case errors.Is(err, domain.ErrSessionSuperseded):
		return openapi.SaveAnswers409JSONResponse(httpapi.Error(ctx, openapi.SESSIONSUPERSEDED,
			"Bài làm này đã được mở ở nơi khác.")), nil
	case errors.Is(err, domain.ErrDeadlinePassed):
		return openapi.SaveAnswers409JSONResponse(httpapi.Error(ctx, openapi.DEADLINEPASSED,
			"Đã hết giờ làm bài.")), nil
	case errors.Is(err, domain.ErrAttemptClosed):
		return openapi.SaveAnswers409JSONResponse(httpapi.Error(ctx, openapi.ATTEMPTCLOSED,
			"Bài làm này đã kết thúc.")), nil
	case err != nil:
		return nil, err
	}

	if len(saved.Dropped) > 0 {
		h.log().WarnContext(ctx, "autosave dropped answers not on the paper",
			"attempt_id", in.AttemptID,
			"session_id", in.SessionID,
			"student_id", principal.UserID,
			"dropped_question_ids", saved.Dropped,
			"saved", saved.Saved,
			"submitted", len(in.Answers))
	}

	return openapi.SaveAnswers200JSONResponse{
		ServerTime: saved.SavedAt,
		SavedAt:    saved.SavedAt,
	}, nil
}

func toDomainAnswers(in *map[string]openapi.Answer) ([]domain.Answer, error) {
	if in == nil {
		return nil, nil
	}
	out := make([]domain.Answer, 0, len(*in))
	for questionID, answer := range *in {
		payload, err := json.Marshal(answer)
		if err != nil {
			return nil, fmt.Errorf("encode answer for %s: %w", questionID, err)
		}
		out = append(out, domain.Answer{QuestionID: questionID, Payload: payload})
	}
	return out, nil
}

func toDomainEvents(in *[]openapi.IntegrityEventInput) ([]domain.Event, error) {
	if in == nil {
		return nil, nil
	}
	out := make([]domain.Event, len(*in))
	for i, e := range *in {
		out[i] = domain.Event{
			Kind:       e.Kind,
			OccurredAt: e.OccurredAt,
			ClientSeq:  e.ClientSeq,
		}
		if e.QuestionId != nil {
			id := e.QuestionId.String()
			out[i].QuestionID = &id
		}
		if e.Meta != nil {
			meta, err := json.Marshal(e.Meta)
			if err != nil {
				return nil, fmt.Errorf("encode event meta: %w", err)
			}
			out[i].Meta = meta
		}
	}
	return out, nil
}

// RecordAudioPlay increments the server-authoritative counter (§11.4).
func (h Attempts) RecordAudioPlay(ctx context.Context, request openapi.RecordAudioPlayRequestObject) (openapi.RecordAudioPlayResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	plays, err := h.app.Commands.RecordPlay.Handle(ctx, command.RecordPlay{AttemptID: request.Id.String(), StudentID: principal.UserID, QuestionID: request.Body.QuestionId.String()})
	if errors.Is(err, domain.ErrForbidden) {
		return openapi.RecordAudioPlay403JSONResponse{
			ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
				httpapi.Error(ctx, openapi.FORBIDDEN, "Bạn không có quyền nghe câu hỏi này.")),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.RecordAudioPlay200JSONResponse{
		Plays:    plays.Plays,
		MaxPlays: plays.MaxPlays,
	}, nil
}

type beaconFlush struct {
	BeaconToken string                        `json:"beaconToken"`
	SessionID   openapi.Uuid                  `json:"sessionId"`
	Events      []openapi.IntegrityEventInput `json:"events"`
}

// FlushEvents accepts the ordinary authenticated flush and the beacon one.
func (h Attempts) FlushEvents(ctx context.Context, request openapi.FlushEventsRequestObject) (openapi.FlushEventsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := domain.FlushInput{AttemptID: request.Id.String()}
	switch {
	case request.JSONBody != nil:
		principal, ok := httpx.PrincipalFromContext(ctx)
		if !ok {
			return forbiddenFlush(ctx), nil
		}
		in.StudentID = principal.UserID
		in.SessionID = request.JSONBody.SessionId.String()
		events, err := toDomainEvents(&request.JSONBody.Events)
		if err != nil {
			return nil, err
		}
		in.Events = events

	case request.TextBody != nil:
		var body beaconFlush
		if err := json.Unmarshal([]byte(*request.TextBody), &body); err != nil {

			return forbiddenFlush(ctx), nil
		}
		in.BeaconToken = body.BeaconToken
		in.SessionID = body.SessionID.String()
		events, err := toDomainEvents(&body.Events)
		if err != nil {
			return nil, err
		}
		in.Events = events

	default:
		return forbiddenFlush(ctx), nil
	}

	_, err := h.app.Commands.Flush.Handle(ctx, command.Flush{Input: in})
	if errors.Is(err, domain.ErrForbidden) || errors.Is(err, domain.ErrBeaconExpired) {
		return forbiddenFlush(ctx), nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.FlushEvents202Response{}, nil
}

func forbiddenFlush(ctx context.Context) openapi.FlushEvents403JSONResponse {
	return openapi.FlushEvents403JSONResponse{
		ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
			httpapi.Error(ctx, openapi.FORBIDDEN, "Không ghi được nhật ký cho bài làm này.")),
	}
}

// SubmitAttempt closes an attempt and grades everything a machine can.
func (h Attempts) SubmitAttempt(ctx context.Context, request openapi.SubmitAttemptRequestObject) (openapi.SubmitAttemptResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	reason := domain.Manual
	if request.Body != nil && request.Body.Reason != nil {
		reason = domain.Reason(*request.Body.Reason)
	}

	closed, err := h.app.Commands.Submit.Handle(ctx, command.Submit{AttemptID: request.Id.String(), StudentID: principal.UserID, Reason: reason})
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return openapi.SubmitAttempt403JSONResponse{
			ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
				httpapi.Error(ctx, openapi.FORBIDDEN, "Bạn không có quyền nộp bài làm này.")),
		}, nil
	case errors.Is(err, domain.ErrAttemptClosed):
		return openapi.SubmitAttempt409JSONResponse(httpapi.Error(ctx, openapi.ATTEMPTCLOSED,
			"Bài làm này đã được nộp.")), nil
	case err != nil:
		return nil, err
	}
	return openapi.SubmitAttempt200JSONResponse(toAPIAttempt(closed)), nil
}

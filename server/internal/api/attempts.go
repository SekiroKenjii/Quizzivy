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

// StartOrResumeAttempt backs §9's "Bắt đầu": one call whether the student is
// starting, reloading after a crash, or arriving on a second device.
func (s *Server) StartOrResumeAttempt(ctx context.Context, request openapi.StartOrResumeAttemptRequestObject) (openapi.StartOrResumeAttemptResponseObject, error) {
	if s.Deps.Attempts == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	session, err := s.Deps.Attempts.StartOrResume(ctx, request.Id.String(), principal.UserID)
	switch {
	case errors.Is(err, attempts.ErrForbidden), errors.Is(err, attempts.ErrNotFound):
		// One answer for "not yours", "no such assignment" and "still a draft".
		// Which assignments exist is not a student's to enumerate, and a
		// distinct 404 would enumerate them.
		return openapi.StartOrResumeAttempt403JSONResponse{
			ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
				authError(ctx, openapi.FORBIDDEN, "Bạn không có quyền làm bài này.")),
		}, nil
	case errors.Is(err, attempts.ErrAssignmentClosed):
		return openapi.StartOrResumeAttempt409JSONResponse(authError(ctx,
			openapi.ASSIGNMENTNOTOPEN, "Bài thi này hiện không mở.")), nil
	case errors.Is(err, attempts.ErrLimitReached):
		return openapi.StartOrResumeAttempt409JSONResponse(authError(ctx,
			openapi.ATTEMPTLIMITREACHED, "Bạn đã dùng hết số lượt làm bài.")), nil
	case err != nil:
		return nil, err
	}

	payload, err := s.toAPIAttemptSession(ctx, principal.UserID, session)
	if err != nil {
		return nil, err
	}
	return openapi.StartOrResumeAttempt200JSONResponse(payload), nil
}

// GetAttempt is the ONLY way a student reaches test content (§7). Its response
// is the same shape the start returns, so there is one definition of what a
// student may see rather than two that can drift apart.
func (s *Server) GetAttempt(ctx context.Context, request openapi.GetAttemptRequestObject) (openapi.GetAttemptResponseObject, error) {
	if s.Deps.Attempts == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	session, err := s.Deps.Attempts.Get(ctx, request.Id.String(), principal.UserID)
	if errors.Is(err, attempts.ErrForbidden) || errors.Is(err, attempts.ErrNotFound) {
		return openapi.GetAttempt403JSONResponse{
			ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
				authError(ctx, openapi.FORBIDDEN, "Bạn không có quyền xem bài làm này.")),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	payload, err := s.toAPIAttemptSession(ctx, principal.UserID, session)
	if err != nil {
		return nil, err
	}
	return openapi.GetAttempt200JSONResponse(payload), nil
}

func (s *Server) toAPIAttemptSession(ctx context.Context, studentID string, in attempts.Session) (openapi.AttemptSession, error) {
	questions := make([]openapi.StudentQuestion, len(in.Questions))
	for i, q := range in.Questions {
		converted, err := s.toAPIStudentQuestion(ctx, studentID, q)
		if err != nil {
			return openapi.AttemptSession{}, err
		}
		questions[i] = converted
	}

	answers, err := toAPIAnswers(in.Answers)
	if err != nil {
		return openapi.AttemptSession{}, err
	}

	return openapi.AttemptSession{
		Attempt:     toAPIAttempt(in.Attempt),
		Questions:   questions,
		SessionId:   parseUUID(in.SessionID),
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

func toAPIAttempt(a attempts.Attempt) openapi.Attempt {
	out := openapi.Attempt{
		Id:            parseUUID(a.ID),
		AssignmentId:  parseUUID(a.AssignmentID),
		StudentId:     parseUUID(a.StudentID),
		TestVersionId: parseUUID(a.TestVersionID),
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

// toAPIStudentQuestion has no branch that could add a grading key: the domain
// type it reads from has no field for one (§13.5).
func (s *Server) toAPIStudentQuestion(ctx context.Context, studentID string, q attempts.Question) (openapi.StudentQuestion, error) {
	out := openapi.StudentQuestion{
		Id:     parseUUID(q.ID),
		Type:   openapi.QuestionType(q.Type),
		Prompt: q.Prompt,
		Points: q.Points,
	}
	if len(q.Options) > 0 {
		options := make([]openapi.StudentOption, len(q.Options))
		for i, o := range q.Options {
			options[i] = openapi.StudentOption{Id: parseUUID(o.ID), Text: o.Text}
		}
		out.Options = &options
	}
	if len(q.Blanks) > 0 {
		blanks := make([]openapi.StudentBlank, len(q.Blanks))
		for i, b := range q.Blanks {
			blanks[i] = openapi.StudentBlank{Id: parseUUID(b.ID), Ordinal: b.Ordinal}
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

	// Minted per response rather than stored: the URL expires, and one cached
	// alongside the row would hand a student a dead player halfway through a
	// listening question (§11.2).
	signed, err := s.Deps.Media.MintForStudent(ctx, studentID, q.Media.ID)
	if err != nil {
		return openapi.StudentQuestion{}, fmt.Errorf("sign attempt media: %w", err)
	}
	out.Media = &openapi.MediaAsset{
		Id:               parseUUID(q.Media.ID),
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

// toAPIAnswers re-reads the stored jsonb through the generated union so a
// payload that no longer matches the contract fails here, on the way out, and
// not in the student's browser mid-test.
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
func (s *Server) SaveAnswers(ctx context.Context, request openapi.SaveAnswersRequestObject) (openapi.SaveAnswersResponseObject, error) {
	if s.Deps.Attempts == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	in := attempts.SaveInput{
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

	saved, err := s.Deps.Attempts.Save(ctx, in)
	switch {
	case errors.Is(err, attempts.ErrForbidden):
		return openapi.SaveAnswers403JSONResponse{
			ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
				authError(ctx, openapi.FORBIDDEN, "Bạn không có quyền ghi vào bài làm này.")),
		}, nil
	case errors.Is(err, attempts.ErrSessionSuperseded):
		return openapi.SaveAnswers409JSONResponse(authError(ctx, openapi.SESSIONSUPERSEDED,
			"Bài làm này đã được mở ở nơi khác.")), nil
	case errors.Is(err, attempts.ErrDeadlinePassed):
		return openapi.SaveAnswers409JSONResponse(authError(ctx, openapi.DEADLINEPASSED,
			"Đã hết giờ làm bài.")), nil
	case errors.Is(err, attempts.ErrAttemptClosed):
		return openapi.SaveAnswers409JSONResponse(authError(ctx, openapi.ATTEMPTCLOSED,
			"Bài làm này đã kết thúc.")), nil
	case err != nil:
		return nil, err
	}

	// serverTime comes back on every save, not only on the first fetch, so the
	// client's clock offset self-corrects over a long test rather than drifting
	// from whatever it measured once at the start.
	return openapi.SaveAnswers200JSONResponse{
		ServerTime: saved.SavedAt,
		SavedAt:    saved.SavedAt,
	}, nil
}

// toDomainAnswers re-encodes each answer through the generated union, so a
// payload that does not match the contract is refused here rather than stored
// and discovered at grading.
func toDomainAnswers(in *map[string]openapi.Answer) ([]attempts.Answer, error) {
	if in == nil {
		return nil, nil
	}
	out := make([]attempts.Answer, 0, len(*in))
	for questionID, answer := range *in {
		payload, err := json.Marshal(answer)
		if err != nil {
			return nil, fmt.Errorf("encode answer for %s: %w", questionID, err)
		}
		out = append(out, attempts.Answer{QuestionID: questionID, Payload: payload})
	}
	return out, nil
}

func toDomainEvents(in *[]openapi.IntegrityEventInput) ([]attempts.Event, error) {
	if in == nil {
		return nil, nil
	}
	out := make([]attempts.Event, len(*in))
	for i, e := range *in {
		out[i] = attempts.Event{
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
//
// It never refuses a play on count. The client plays first and posts after, so
// a failure here costs a number rather than the audio, and a limit enforced
// over a round trip would punish bad wifi far more often than it would catch
// anyone.
func (s *Server) RecordAudioPlay(ctx context.Context, request openapi.RecordAudioPlayRequestObject) (openapi.RecordAudioPlayResponseObject, error) {
	if s.Deps.Attempts == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	plays, err := s.Deps.Attempts.RecordPlay(ctx,
		request.Id.String(), principal.UserID, request.Body.QuestionId.String())
	if errors.Is(err, attempts.ErrForbidden) {
		return openapi.RecordAudioPlay403JSONResponse{
			ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
				authError(ctx, openapi.FORBIDDEN, "Bạn không có quyền nghe câu hỏi này.")),
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

// beaconFlush is the text/plain body, which the contract types only as a
// string because navigator.sendBeacon has nowhere else to put a credential.
//
// text/plain is CORS-safelisted, so the request skips preflight. That matters
// specifically on unload: a preflight fired from `pagehide` is not reliably
// delivered, and an event log that loses the last thing that happened loses the
// part the teacher most wants (§10.6, D-03).
type beaconFlush struct {
	BeaconToken string                        `json:"beaconToken"`
	SessionID   openapi.Uuid                  `json:"sessionId"`
	Events      []openapi.IntegrityEventInput `json:"events"`
}

// FlushEvents accepts the ordinary authenticated flush and the beacon one.
//
// Always 202, whatever happened, unless the credential is bad. §10.6 makes this
// fire-and-forget: the client cannot act on the outcome, and a flush that
// blocks answering or submitting is worse than a flush that silently did
// nothing.
func (s *Server) FlushEvents(ctx context.Context, request openapi.FlushEventsRequestObject) (openapi.FlushEventsResponseObject, error) {
	if s.Deps.Attempts == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := attempts.FlushInput{AttemptID: request.Id.String()}
	switch {
	case request.JSONBody != nil:
		// The route is open in the contract, so the middleware attaches a
		// principal when a token verifies and leaves it absent otherwise. No
		// principal on this path means no credential at all.
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
			// Unparseable is refused rather than 202'd. 202 means "recorded, do
			// not worry about it", which would be a lie.
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

	err := s.Deps.Attempts.Flush(ctx, in)
	if errors.Is(err, attempts.ErrForbidden) || errors.Is(err, attempts.ErrBeaconExpired) {
		// One answer for a wrong token and a spent one: which it was tells a
		// caller whether the token it holds is real.
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
			authError(ctx, openapi.FORBIDDEN, "Không ghi được nhật ký cho bài làm này.")),
	}
}

// SubmitAttempt closes an attempt and grades everything a machine can.
//
// `sessionId` is accepted and not checked. The contract gives this operation no
// SESSION_SUPERSEDED, and rightly: a student on the tab that lost the race is
// still the student, and refusing their submit would strand finished work
// behind a technicality about which tab it came from.
func (s *Server) SubmitAttempt(ctx context.Context, request openapi.SubmitAttemptRequestObject) (openapi.SubmitAttemptResponseObject, error) {
	if s.Deps.Attempts == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	reason := attempts.Manual
	if request.Body != nil && request.Body.Reason != nil {
		reason = attempts.Reason(*request.Body.Reason)
	}

	closed, err := s.Deps.Attempts.Submit(ctx, request.Id.String(), principal.UserID, reason)
	switch {
	case errors.Is(err, attempts.ErrForbidden):
		return openapi.SubmitAttempt403JSONResponse{
			ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
				authError(ctx, openapi.FORBIDDEN, "Bạn không có quyền nộp bài làm này.")),
		}, nil
	case errors.Is(err, attempts.ErrAttemptClosed):
		return openapi.SubmitAttempt409JSONResponse(authError(ctx, openapi.ATTEMPTCLOSED,
			"Bài làm này đã được nộp.")), nil
	case err != nil:
		return nil, err
	}
	return openapi.SubmitAttempt200JSONResponse(toAPIAttempt(closed)), nil
}

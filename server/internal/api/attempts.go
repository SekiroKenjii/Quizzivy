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

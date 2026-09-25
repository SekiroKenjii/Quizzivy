package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

// RecordGroupAudioPlay accounts for one gesture on a shared frozen recording without gating playback.
func (h Attempts) RecordGroupAudioPlay(ctx context.Context, request openapi.RecordGroupAudioPlayRequestObject) (openapi.RecordGroupAudioPlayResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	plays, err := h.app.Commands.RecordGroupPlay.Handle(ctx, command.RecordGroupPlay{Input: domain.GroupPlayInput{
		AttemptID: request.Id.String(), StudentID: principal.UserID, SessionID: request.Body.SessionId.String(),
		RecordingID: request.Body.RecordingId.String(), PlayID: request.Body.PlayId.String(),
	}})
	switch {
	case errors.Is(err, domain.ErrForbidden):
		return openapi.RecordGroupAudioPlay403JSONResponse{ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(httpapi.Error(ctx, openapi.FORBIDDEN, "Bạn không có quyền nghe bản ghi này."))}, nil
	case errors.Is(err, domain.ErrAttemptClosed):
		return openapi.RecordGroupAudioPlay409JSONResponse(httpapi.Error(ctx, openapi.ATTEMPTCLOSED, "Bài làm này đã kết thúc.")), nil
	case errors.Is(err, domain.ErrSessionSuperseded):
		return openapi.RecordGroupAudioPlay409JSONResponse(httpapi.Error(ctx, openapi.SESSIONSUPERSEDED, "Bài làm này đã được mở ở nơi khác.")), nil
	case errors.Is(err, domain.ErrDeadlinePassed):
		return openapi.RecordGroupAudioPlay409JSONResponse(httpapi.Error(ctx, openapi.DEADLINEPASSED, "Đã hết giờ làm bài.")), nil
	case errors.Is(err, domain.ErrPlayIDConflict):
		return openapi.RecordGroupAudioPlay409JSONResponse(httpapi.Error(ctx, openapi.PLAYIDCONFLICT, "Mã lượt nghe đã được dùng cho bản ghi khác. Vui lòng tải lại bài làm.")), nil
	case err != nil:
		return nil, err
	}
	return openapi.RecordGroupAudioPlay200JSONResponse{PlayId: httpapi.ParseUUID(plays.PlayID), Plays: plays.Plays, MaxPlays: plays.MaxPlays}, nil
}

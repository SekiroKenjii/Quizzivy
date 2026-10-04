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
		return openapi.RecordGroupAudioPlay403JSONResponse{ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(httpapi.Error(ctx, openapi.FORBIDDEN, httpx.Text(ctx, "Bạn không có quyền nghe bản ghi này.", "You do not have permission to listen to this recording.")))}, nil
	case errors.Is(err, domain.ErrAttemptClosed):
		return openapi.RecordGroupAudioPlay409JSONResponse(httpapi.Error(ctx, openapi.ATTEMPTCLOSED, httpx.Text(ctx, "Bài làm này đã kết thúc.", "This attempt has ended."))), nil
	case errors.Is(err, domain.ErrSessionSuperseded):
		return openapi.RecordGroupAudioPlay409JSONResponse(httpapi.Error(ctx, openapi.SESSIONSUPERSEDED, httpx.Text(ctx, "Bài làm này đã được mở ở nơi khác.", "This attempt was opened somewhere else."))), nil
	case errors.Is(err, domain.ErrDeadlinePassed):
		return openapi.RecordGroupAudioPlay409JSONResponse(httpapi.Error(ctx, openapi.DEADLINEPASSED, httpx.Text(ctx, "Đã hết giờ làm bài.", "Time is up."))), nil
	case errors.Is(err, domain.ErrPlayIDConflict):
		return openapi.RecordGroupAudioPlay409JSONResponse(httpapi.Error(ctx, openapi.PLAYIDCONFLICT, httpx.Text(ctx, "Mã lượt nghe đã được dùng cho bản ghi khác. Vui lòng tải lại bài làm.", "This play id was already used for another recording. Please reload the attempt."))), nil
	case err != nil:
		return nil, err
	}
	return openapi.RecordGroupAudioPlay200JSONResponse{PlayId: httpapi.ParseUUID(plays.PlayID), Plays: plays.Plays, MaxPlays: plays.MaxPlays}, nil
}

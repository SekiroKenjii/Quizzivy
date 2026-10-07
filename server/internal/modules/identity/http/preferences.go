package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

// UpdatePreferences applies a self-scoped top-level preference merge.
func (h Identity) UpdatePreferences(ctx context.Context, request openapi.UpdatePreferencesRequestObject) (openapi.UpdatePreferencesResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.UpdatePreferences401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	}
	if request.Body == nil {
		return openapi.UpdatePreferences400JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED, httpx.Text(ctx, "Thiếu tùy chọn.", "Supply preferences."))), nil
	}
	body := request.Body
	patch := domain.Preferences{Theme: (*string)(body.Theme), CompactTables: body.CompactTables, LargerTestText: body.LargerTestText, AssignmentDefaults: (*domain.AssignmentDefaults)(body.AssignmentDefaults)}
	meta := httpx.RequestMetaFromContext(ctx)
	prefs, err := h.app.Commands.UpdatePreferences.Handle(ctx, command.UpdatePreferences{UserID: principal.UserID, Preferences: patch, IP: meta.IP, UserAgent: meta.UserAgent})
	switch {
	case err == nil:
		return openapi.UpdatePreferences200JSONResponse(toAPIPreferences(prefs)), nil
	case errors.Is(err, domain.ErrUserNotFound), errors.Is(err, domain.ErrAccountDisabled):
		return openapi.UpdatePreferences401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	case errors.Is(err, domain.ErrPreferencesInvalid), errors.Is(err, domain.ErrPreferencesTooLarge):
		return openapi.UpdatePreferences400JSONResponse(httpapi.FieldError(ctx, "preferences", httpx.Text(ctx, "Tùy chọn không hợp lệ hoặc vượt quá giới hạn.", "The preferences are invalid or exceed the limit."))), nil
	default:
		return nil, err
	}
}

func toAPIPreferences(p domain.Preferences) openapi.UserPreferences {
	out := openapi.UserPreferences{Theme: (*openapi.UserPreferencesTheme)(p.Theme), CompactTables: p.CompactTables, LargerTestText: p.LargerTestText}
	if d := p.AssignmentDefaults; d != nil {
		out.AssignmentDefaults = &struct {
			BlockCopyPaste    *bool `json:"blockCopyPaste,omitempty"`
			DurationMinutes   *int  `json:"durationMinutes,omitempty"`
			RequireFullscreen *bool `json:"requireFullscreen,omitempty"`
			ShowScore         *bool `json:"showScore,omitempty"`
			ShuffleQuestions  *bool `json:"shuffleQuestions,omitempty"`
		}{d.BlockCopyPaste, d.DurationMinutes, d.RequireFullscreen, d.ShowScore, d.ShuffleQuestions}
	}
	return out
}

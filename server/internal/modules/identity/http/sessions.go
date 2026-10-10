package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/uaparse"
	"quizzivy/internal/shared/opt"
)

// ListSessions implements GET /auth/sessions (§5.2).
func (h Identity) ListSessions(ctx context.Context, _ openapi.ListSessionsRequestObject) (openapi.ListSessionsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.ListSessions401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	}
	sessions, err := h.app.Queries.ListSessions.Handle(ctx, query.ListSessions{
		UserID:       principal.UserID,
		RefreshToken: refreshTokenFromContext(ctx),
	})
	if err != nil {
		return nil, err
	}
	items := make([]openapi.Session, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, toAPISession(session))
	}
	return openapi.ListSessions200JSONResponse{Items: items}, nil
}

// RevokeSession implements DELETE /auth/sessions/{familyId} (§5.2).
func (h Identity) RevokeSession(ctx context.Context, request openapi.RevokeSessionRequestObject) (openapi.RevokeSessionResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.RevokeSession401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	}
	meta := httpx.RequestMetaFromContext(ctx)
	_, err := h.app.Commands.RevokeSession.Handle(ctx, command.RevokeSession{
		UserID:       principal.UserID,
		FamilyID:     request.FamilyId.String(),
		RefreshToken: refreshTokenFromContext(ctx),
		IP:           meta.IP,
		UserAgent:    meta.UserAgent,
	})
	switch {
	case err == nil:
		return openapi.RevokeSession204Response{}, nil
	case errors.Is(err, domain.ErrNoCurrentSession), errors.Is(err, domain.ErrUserNotFound):
		return openapi.RevokeSession401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	case errors.Is(err, domain.ErrSessionNotFound):
		return openapi.RevokeSession404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx,
			httpx.Text(ctx, "Không tìm thấy thiết bị này.", "That device was not found.")))}, nil
	case errors.Is(err, domain.ErrSessionIsCurrent):
		return openapi.RevokeSession409JSONResponse(httpapi.Error(ctx, openapi.SESSIONISCURRENT,
			httpx.Text(ctx, "Đây là thiết bị bạn đang dùng. Hãy đăng xuất thay vì xoá phiên này.",
				"This is the device you are using. Sign out instead."))), nil
	default:
		return nil, err
	}
}

// RevokeOtherSessions implements POST /auth/sessions/revoke-others (§5.2).
func (h Identity) RevokeOtherSessions(ctx context.Context, _ openapi.RevokeOtherSessionsRequestObject) (openapi.RevokeOtherSessionsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.RevokeOtherSessions401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	}
	meta := httpx.RequestMetaFromContext(ctx)
	revoked, err := h.app.Commands.RevokeOtherSessions.Handle(ctx, command.RevokeOtherSessions{
		UserID:       principal.UserID,
		RefreshToken: refreshTokenFromContext(ctx),
		IP:           meta.IP,
		UserAgent:    meta.UserAgent,
	})
	switch {
	case err == nil:
		return openapi.RevokeOtherSessions200JSONResponse{Revoked: revoked}, nil
	case errors.Is(err, domain.ErrNoCurrentSession), errors.Is(err, domain.ErrUserNotFound):
		return openapi.RevokeOtherSessions401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	default:
		return nil, err
	}
}

func toAPISession(s domain.Session) openapi.Session {
	device := uaparse.Parse(opt.Deref(s.UserAgent))
	return openapi.Session{
		FamilyId:   httpapi.ParseUUID(s.FamilyID),
		Device:     opt.String(device.Label()),
		DeviceKind: openapi.SessionDeviceKind(device.Kind),
		Location:   s.GeoLabel,
		LastUsedAt: s.LastUsedAt.UTC(),
		Current:    s.Current,
	}
}

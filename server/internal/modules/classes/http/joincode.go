package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

const msgNoActiveCode = "Lớp này chưa có mã tham gia."

// GetJoinCode implements GET /teacher/classes/{id}/join-code (D5): the active
// code of a class the caller reaches, never cached. Another teacher's class
// answers 404 exactly as a missing one does.
func (h Classes) GetJoinCode(ctx context.Context, request openapi.GetJoinCodeRequestObject) (openapi.GetJoinCodeResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	active, err := h.app.Queries.ActiveCode.Handle(ctx, query.ActiveCode{Scope: httpapi.ScopeFromContext(ctx), ClassID: request.Id.String()})
	switch {
	case errors.Is(err, domain.ErrClassNotFound):
		return openapi.GetJoinCode404JSONResponse{
			NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgClassNotFound)),
		}, nil
	case errors.Is(err, domain.ErrNoActiveCode):
		return openapi.GetJoinCode404JSONResponse{
			NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgNoActiveCode)),
		}, nil
	case err != nil:
		return nil, err
	}
	noStore := "no-store"
	out := openapi.JoinCode{
		Hint:      active.Hint,
		Legacy:    active.Legacy,
		ExpiresAt: active.ExpiresAt,
		MaxUses:   active.MaxUses,
		UsesCount: active.UsesCount,
	}
	if active.Code != "" {
		grouped := domain.JoinCodes.Format(active.Code)
		out.Code = &grouped
	}
	return openapi.GetJoinCode200JSONResponse{Body: out, Headers: openapi.GetJoinCode200ResponseHeaders{CacheControl: &noStore}}, nil
}

// RotateJoinCode implements POST /teacher/classes/{id}/join-code (§6.1).
func (h Classes) RotateJoinCode(ctx context.Context, request openapi.RotateJoinCodeRequestObject) (openapi.RotateJoinCodeResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	who, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	req := domain.RotateRequest{
		ClassID:     request.Id.String(),
		ActorUserID: who.ID,
		All:         who.Scope.All,
		IP:          who.IP,
		UserAgent:   who.UserAgent,
	}
	if request.Body != nil {
		req.ExpiresInDays = request.Body.ExpiresInDays
		req.MaxUses = request.Body.MaxUses
	}

	rotated, err := h.app.Commands.Rotate.Handle(ctx, command.Rotate{Request: req})
	if err != nil {
		if errors.Is(err, domain.ErrClassNotFound) {
			return openapi.RotateJoinCode404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgClassNotFound)),
			}, nil
		}
		return nil, err
	}

	return openapi.RotateJoinCode201JSONResponse{
		Code:      rotated.Code,
		ExpiresAt: rotated.ExpiresAt,
		MaxUses:   rotated.MaxUses,
	}, nil
}

// RevokeJoinCode implements DELETE /teacher/classes/{id}/join-code (§6.4).
func (h Classes) RevokeJoinCode(ctx context.Context, request openapi.RevokeJoinCodeRequestObject) (openapi.RevokeJoinCodeResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	who, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	_, err := h.app.Commands.Revoke.Handle(ctx, command.Revoke{Request: domain.RevokeRequest{
		ClassID:     request.Id.String(),
		ActorUserID: who.ID,
		All:         who.Scope.All,
		IP:          who.IP,
		UserAgent:   who.UserAgent,
	}})
	if err != nil {
		if errors.Is(err, domain.ErrClassNotFound) {
			return openapi.RevokeJoinCode404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgClassNotFound)),
			}, nil
		}
		return nil, err
	}
	return openapi.RevokeJoinCode204Response{}, nil
}

// PreviewJoinCode resolves a join code for an anonymous caller.
func (h Classes) PreviewJoinCode(ctx context.Context, request openapi.PreviewJoinCodeRequestObject) (openapi.PreviewJoinCodeResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}

	result, err := h.app.Queries.Preview.Handle(ctx, query.Preview{Code: request.Body.JoinCode})
	if err != nil {
		return nil, err
	}

	if result.Outcome != domain.PreviewOK {
		return openapi.PreviewJoinCode404JSONResponse(JoinCodeError(ctx, result.Outcome)), nil
	}
	return openapi.PreviewJoinCode200JSONResponse{
		ClassId:     httpapi.ParseUUID(result.ClassID),
		ClassName:   result.ClassName,
		TeacherName: result.TeacherName,
	}, nil
}

// JoinCodeError maps a refusal to its §9 error code and message.
func JoinCodeError(ctx context.Context, outcome domain.PreviewOutcome) openapi.ErrorResponse {
	switch outcome {
	case domain.PreviewRevoked:
		return httpapi.Error(ctx, openapi.JOINCODEREVOKED,
			"Mã lớp này đã bị thu hồi. Vui lòng xin giáo viên mã mới.")
	case domain.PreviewExpired:
		return httpapi.Error(ctx, openapi.JOINCODEEXPIRED,
			"Mã lớp này đã hết hạn. Vui lòng xin giáo viên mã mới.")
	case domain.PreviewExhausted:
		return httpapi.Error(ctx, openapi.JOINCODEEXHAUSTED,
			"Mã lớp này đã hết lượt sử dụng. Vui lòng xin giáo viên mã mới.")
	default:
		return httpapi.Error(ctx, openapi.JOINCODEINVALID,
			"Mã lớp không đúng. Vui lòng kiểm tra lại.")
	}
}

// ToAPIClass renders the §7 Class shape.
func ToAPIClass(c domain.EnrolledClass) openapi.Class {
	return openapi.Class{
		Id:              httpapi.ParseUUID(c.ID),
		Name:            c.Name,
		Description:     c.Description,
		StudentCount:    c.StudentCount,
		SelfJoinEnabled: c.SelfJoinEnabled,
		CreatedAt:       c.CreatedAt,
	}
}

// JoinClass implements POST /app/classes/join (§6.2).
func (h Classes) JoinClass(ctx context.Context, request openapi.JoinClassRequestObject) (openapi.JoinClassResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	result, err := h.app.Commands.EnrolExisting.Handle(ctx, command.EnrolExisting{UserID: principal.UserID, Code: request.Body.JoinCode, Meta: domain.Meta{IP: meta.IP, UserAgent: meta.UserAgent}})
	if err != nil {
		return nil, err
	}
	if result.Outcome != domain.PreviewOK {
		return openapi.JoinClass404JSONResponse(JoinCodeError(ctx, result.Outcome)), nil
	}
	return openapi.JoinClass200JSONResponse(ToAPIClass(result.Class)), nil
}

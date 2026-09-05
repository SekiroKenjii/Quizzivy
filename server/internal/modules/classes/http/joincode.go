package http

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

// RotateJoinCode implements POST /admin/classes/{id}/join-code (§6.1).
//
// This response is the only place the plaintext code ever exists outside the
// browser that receives it. Only a SHA-256 hash is stored (§13.3), so it cannot
// be shown again and there is no endpoint that could -- if the teacher loses
// it, they rotate.
func (h Classes) RotateJoinCode(ctx context.Context, request openapi.RotateJoinCodeRequestObject) (openapi.RotateJoinCodeResponseObject, error) {
	if h.enrolment == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	req := domain.RotateRequest{
		ClassID:     request.Id.String(),
		ActorUserID: principal.UserID,
		IP:          meta.IP,
		UserAgent:   meta.UserAgent,
	}
	if request.Body != nil {
		req.ExpiresInDays = request.Body.ExpiresInDays
		req.MaxUses = request.Body.MaxUses
	}

	rotated, err := h.enrolment.Rotate(ctx, req)
	if err != nil {
		if errors.Is(err, domain.ErrClassNotFound) {
			return openapi.RotateJoinCode404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, "Không tìm thấy lớp học.")),
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

// RevokeJoinCode implements DELETE /admin/classes/{id}/join-code (§6.4).
//
// Revokes without issuing a replacement AND closes self-join. Both, or the
// class is left either advertising a join flow that cannot work or holding a
// live bearer secret the teacher believes they cancelled.
func (h Classes) RevokeJoinCode(ctx context.Context, request openapi.RevokeJoinCodeRequestObject) (openapi.RevokeJoinCodeResponseObject, error) {
	if h.enrolment == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	err := h.enrolment.Revoke(ctx, domain.RevokeRequest{
		ClassID:     request.Id.String(),
		ActorUserID: principal.UserID,
		IP:          meta.IP,
		UserAgent:   meta.UserAgent,
	})
	if err != nil {
		if errors.Is(err, domain.ErrClassNotFound) {
			return openapi.RevokeJoinCode404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, "Không tìm thấy lớp học.")),
			}, nil
		}
		return nil, err
	}
	return openapi.RevokeJoinCode204Response{}, nil
}

// PreviewJoinCode resolves a join code for an anonymous caller.
//
// Public and rate-limited by IP and by normalised code. Every rejection returns
// the same shape so the endpoint cannot enumerate which classes exist.
func (h Classes) PreviewJoinCode(ctx context.Context, request openapi.PreviewJoinCodeRequestObject) (openapi.PreviewJoinCodeResponseObject, error) {
	if h.enrolment == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}

	result, err := h.enrolment.Preview(ctx, request.Body.JoinCode)
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
//
// One mapping, used by /join/preview, /app/classes/join and the joinCode branch
// of /auth/google. Three copies would drift, and the thing they would drift on
// is how much each endpoint gives away about a class (§6.5).
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
	if h.enrolment == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	result, err := h.enrolment.EnrolExisting(ctx, principal.UserID, request.Body.JoinCode,
		domain.Meta{IP: meta.IP, UserAgent: meta.UserAgent})
	if err != nil {
		return nil, err
	}
	if result.Outcome != domain.PreviewOK {
		return openapi.JoinClass404JSONResponse(JoinCodeError(ctx, result.Outcome)), nil
	}
	return openapi.JoinClass200JSONResponse(ToAPIClass(result.Class)), nil
}

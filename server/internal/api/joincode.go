package api

import (
	"context"
	"errors"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
	"quizzivy/internal/join"
)

// RotateJoinCode implements POST /admin/classes/{id}/join-code (§6.1).
//
// This response is the only place the plaintext code ever exists outside the
// browser that receives it. Only a SHA-256 hash is stored (§13.3), so it cannot
// be shown again and there is no endpoint that could -- if the teacher loses
// it, they rotate.
func (s *Server) RotateJoinCode(ctx context.Context, request openapi.RotateJoinCodeRequestObject) (openapi.RotateJoinCodeResponseObject, error) {
	if s.Deps.Join == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	req := join.RotateRequest{
		ClassID:     request.Id.String(),
		ActorUserID: principal.UserID,
		IP:          meta.IP,
		UserAgent:   meta.UserAgent,
	}
	if request.Body != nil {
		req.ExpiresInDays = request.Body.ExpiresInDays
		req.MaxUses = request.Body.MaxUses
	}

	rotated, err := s.Deps.Join.Rotate(ctx, req)
	if err != nil {
		if errors.Is(err, join.ErrClassNotFound) {
			return openapi.RotateJoinCode404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(notFound(ctx, "Không tìm thấy lớp học.")),
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
func (s *Server) RevokeJoinCode(ctx context.Context, request openapi.RevokeJoinCodeRequestObject) (openapi.RevokeJoinCodeResponseObject, error) {
	if s.Deps.Join == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	err := s.Deps.Join.Revoke(ctx, join.RevokeRequest{
		ClassID:     request.Id.String(),
		ActorUserID: principal.UserID,
		IP:          meta.IP,
		UserAgent:   meta.UserAgent,
	})
	if err != nil {
		if errors.Is(err, join.ErrClassNotFound) {
			return openapi.RevokeJoinCode404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(notFound(ctx, "Không tìm thấy lớp học.")),
			}, nil
		}
		return nil, err
	}
	return openapi.RevokeJoinCode204Response{}, nil
}

func notFound(ctx context.Context, message string) openapi.ErrorResponse {
	return authError(ctx, openapi.NOTFOUND, message)
}

// PreviewJoinCode resolves a join code for an anonymous caller.
//
// Public and rate-limited by IP and by normalised code. Every rejection returns
// the same shape so the endpoint cannot enumerate which classes exist.
func (s *Server) PreviewJoinCode(ctx context.Context, request openapi.PreviewJoinCodeRequestObject) (openapi.PreviewJoinCodeResponseObject, error) {
	if s.Deps.Join == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}

	result, err := s.Deps.Join.Preview(ctx, request.Body.JoinCode)
	if err != nil {
		return nil, err
	}

	if result.Outcome != join.PreviewOK {
		return openapi.PreviewJoinCode404JSONResponse(joinCodeError(ctx, result.Outcome)), nil
	}
	return openapi.PreviewJoinCode200JSONResponse{
		ClassId:     parseUUID(result.ClassID),
		ClassName:   result.ClassName,
		TeacherName: result.TeacherName,
	}, nil
}

// joinCodeError maps a refusal to its §9 error code and message.
//
// One mapping, used by /join/preview, /app/classes/join and the joinCode branch
// of /auth/google. Three copies would drift, and the thing they would drift on
// is how much each endpoint gives away about a class (§6.5).
func joinCodeError(ctx context.Context, outcome join.PreviewOutcome) openapi.ErrorResponse {
	switch outcome {
	case join.PreviewRevoked:
		return authError(ctx, openapi.JOINCODEREVOKED,
			"Mã lớp này đã bị thu hồi. Vui lòng xin giáo viên mã mới.")
	case join.PreviewExpired:
		return authError(ctx, openapi.JOINCODEEXPIRED,
			"Mã lớp này đã hết hạn. Vui lòng xin giáo viên mã mới.")
	case join.PreviewExhausted:
		return authError(ctx, openapi.JOINCODEEXHAUSTED,
			"Mã lớp này đã hết lượt sử dụng. Vui lòng xin giáo viên mã mới.")
	default:
		return authError(ctx, openapi.JOINCODEINVALID,
			"Mã lớp không đúng. Vui lòng kiểm tra lại.")
	}
}

// toAPIClass renders the §7 Class shape.
func toAPIClass(c join.EnrolledClass) openapi.Class {
	return openapi.Class{
		Id:              parseUUID(c.ID),
		Name:            c.Name,
		Description:     c.Description,
		StudentCount:    c.StudentCount,
		SelfJoinEnabled: c.SelfJoinEnabled,
		CreatedAt:       c.CreatedAt,
	}
}

// JoinClass implements POST /app/classes/join (§6.2).
func (s *Server) JoinClass(ctx context.Context, request openapi.JoinClassRequestObject) (openapi.JoinClassResponseObject, error) {
	if s.Deps.Join == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	result, err := s.Deps.Join.EnrolExisting(ctx, principal.UserID, request.Body.JoinCode,
		join.Meta{IP: meta.IP, UserAgent: meta.UserAgent})
	if err != nil {
		return nil, err
	}
	if result.Outcome != join.PreviewOK {
		return openapi.JoinClass404JSONResponse(joinCodeError(ctx, result.Outcome)), nil
	}
	return openapi.JoinClass200JSONResponse(toAPIClass(result.Class)), nil
}

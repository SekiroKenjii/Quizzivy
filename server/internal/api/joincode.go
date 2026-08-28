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
	// The whole body is optional: "give me a code" with no options is the
	// common case, and the defaults are §6.1's and O-06's.
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

// PreviewJoinCode implements POST /join/preview (§6.2).
//
// **Leak review.** Public, unauthenticated, and it takes a bearer secret, so
// what it returns and what it refuses both matter.
//
// The success body is exactly three fields, enforced structurally by the
// generated type and by a test that reads the wire bytes rather than the Go
// struct. No student names, no counts, no member list -- §6.5 is explicit, and
// this is the endpoint a stranger reaches with a forwarded code.
//
// The four refusals carry DIFFERENT codes and the SAME information. "Expired"
// versus "wrong code" does confirm that a code once existed, which is accepted:
// the alternative is a student with a stale code being told they typed it wrong
// and giving up, and at 40 bits of entropy behind two rate limits, enumeration
// is not the threat -- forwarding is (R-02). None of the four messages names a
// class, and a class with self-join switched off answers exactly as a
// nonexistent one does.
func (s *Server) PreviewJoinCode(ctx context.Context, request openapi.PreviewJoinCodeRequestObject) (openapi.PreviewJoinCodeResponseObject, error) {
	if s.Deps.Join == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}

	result, err := s.Deps.Join.Preview(ctx, request.Body.JoinCode)
	if err != nil {
		return nil, err
	}

	switch result.Outcome {
	case join.PreviewOK:
		return openapi.PreviewJoinCode200JSONResponse{
			ClassId:     parseUUID(result.ClassID),
			ClassName:   result.ClassName,
			TeacherName: result.TeacherName,
		}, nil
	case join.PreviewRevoked:
		return openapi.PreviewJoinCode404JSONResponse(authError(ctx, openapi.JOINCODEREVOKED,
			"Mã lớp này đã bị thu hồi. Vui lòng xin giáo viên mã mới.")), nil
	case join.PreviewExpired:
		return openapi.PreviewJoinCode404JSONResponse(authError(ctx, openapi.JOINCODEEXPIRED,
			"Mã lớp này đã hết hạn. Vui lòng xin giáo viên mã mới.")), nil
	case join.PreviewExhausted:
		return openapi.PreviewJoinCode404JSONResponse(authError(ctx, openapi.JOINCODEEXHAUSTED,
			"Mã lớp này đã hết lượt sử dụng. Vui lòng xin giáo viên mã mới.")), nil
	default:
		return openapi.PreviewJoinCode404JSONResponse(authError(ctx, openapi.JOINCODEINVALID,
			"Mã lớp không đúng. Vui lòng kiểm tra lại.")), nil
	}
}

package http

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

// SetAvatar implements PUT /me/avatar.
func (h Identity) SetAvatar(ctx context.Context, request openapi.SetAvatarRequestObject) (openapi.SetAvatarResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.SetAvatar401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	}
	if request.Body == nil {
		return noImage(ctx), nil
	}
	part, err := firstFilePart(request.Body)
	if overBodyLimit(err) {
		return avatarTooLarge(ctx), nil
	}
	if err != nil {
		return noImage(ctx), nil
	}
	defer func() { _ = part.Close() }()

	meta := httpx.RequestMetaFromContext(ctx)
	user, err := h.app.Commands.SetAvatar.Handle(ctx, command.SetAvatar{UserID: principal.UserID, Body: part, IP: meta.IP, UserAgent: meta.UserAgent})
	switch {
	case err == nil:
		return openapi.SetAvatar200JSONResponse(h.currentUser(ctx, user, principal.Access.Permissions)), nil
	case errors.Is(err, domain.ErrAvatarsUnavailable):
		return nil, httpx.ErrNotImplemented
	case errors.Is(err, domain.ErrAvatarTooLarge), overBodyLimit(err):
		return avatarTooLarge(ctx), nil
	case errors.Is(err, domain.ErrAvatarUnsupported):
		return openapi.SetAvatar415JSONResponse(httpapi.Error(ctx, openapi.MEDIATYPEUNSUPPORTED,
			httpx.Text(ctx, "Chỉ hỗ trợ ảnh PNG hoặc JPG.", "Only PNG or JPG images are supported."))), nil
	case errors.Is(err, domain.ErrAvatarUnreadable):
		return openapi.SetAvatar415JSONResponse(httpapi.Error(ctx, openapi.MEDIAUNREADABLE,
			httpx.Text(ctx, "Không đọc được ảnh này. Tệp có thể bị lỗi hoặc chưa tải lên hết.",
				"This image cannot be read. It may be damaged or not fully uploaded."))), nil
	case errors.Is(err, domain.ErrAvatarDimensions):
		return openapi.SetAvatar415JSONResponse(httpapi.Error(ctx, openapi.IMAGEDIMENSIONS,
			httpx.Text(ctx, "Ảnh phải có mỗi cạnh từ 200 đến 2048 px và không quá lớn để xử lý.",
				"Each side of the image must be 200 to 2048 pixels, and the image must not be too large to process."))), nil
	case errors.Is(err, domain.ErrAccountDisabled), errors.Is(err, domain.ErrUserNotFound):
		return openapi.SetAvatar401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	default:
		return nil, err
	}
}

// DeleteAvatar implements DELETE /me/avatar.
func (h Identity) DeleteAvatar(ctx context.Context, _ openapi.DeleteAvatarRequestObject) (openapi.DeleteAvatarResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.DeleteAvatar401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	}
	meta := httpx.RequestMetaFromContext(ctx)
	user, err := h.app.Commands.RemoveAvatar.Handle(ctx, command.RemoveAvatar{UserID: principal.UserID, IP: meta.IP, UserAgent: meta.UserAgent})
	switch {
	case err == nil:
		return openapi.DeleteAvatar200JSONResponse(h.currentUser(ctx, user, principal.Access.Permissions)), nil
	case errors.Is(err, domain.ErrAccountDisabled), errors.Is(err, domain.ErrUserNotFound):
		return openapi.DeleteAvatar401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	default:
		return nil, err
	}
}

func (h Identity) currentUser(ctx context.Context, user domain.User, permissions access.Set) openapi.CurrentUser {
	out := toCurrentUser(user, permissions)
	if url := h.avatarURL(ctx, user); url != "" {
		out.AvatarUrl = &url
	}
	return out
}

func (h Identity) avatarURL(ctx context.Context, user domain.User) string {
	if user.AvatarKey == nil {
		return ""
	}
	url, err := h.app.Queries.AvatarURL.Handle(ctx, query.AvatarURL{Key: user.AvatarKey})
	if err != nil {
		return ""
	}
	return url
}

func noImage(ctx context.Context) openapi.SetAvatar400JSONResponse {
	return openapi.SetAvatar400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(httpapi.FieldError(ctx, "file",
		httpx.Text(ctx, "Không tìm thấy ảnh trong yêu cầu tải lên.", "The upload request holds no image.")))}
}

func avatarTooLarge(ctx context.Context) openapi.SetAvatar413JSONResponse {
	return openapi.SetAvatar413JSONResponse(httpapi.Error(ctx, openapi.MEDIATOOLARGE,
		httpx.Text(ctx, "Ảnh vượt quá 2 MB. Vui lòng chọn ảnh nhỏ hơn.", "The image is larger than 2 MB. Please choose a smaller one.")))
}

func overBodyLimit(err error) bool {
	var tooLarge *http.MaxBytesError
	return errors.As(err, &tooLarge)
}

func firstFilePart(reader *multipart.Reader) (*multipart.Part, error) {
	for {
		part, err := reader.NextPart()
		if err != nil {
			return nil, err
		}
		if part.FileName() != "" {
			return part, nil
		}
		_ = part.Close()
	}
}

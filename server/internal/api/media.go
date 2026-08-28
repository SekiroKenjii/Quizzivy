package api

import (
	"context"
	"errors"
	"mime/multipart"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
	"quizzivy/internal/media"
	"quizzivy/internal/media/probe"
)

// UploadMedia implements POST /admin/media (§11.1).
//
// Upload goes THROUGH the backend in v1: the files are small, the volume is
// low, and the backend has to validate them anyway. Presigned direct-to-R2 is
// the P1 optimisation and does not change this call's contract.
//
// The validation order is the contract: size, then magic bytes, then duration.
// A 50 MB upload is refused by the first check and never becomes a parsing
// problem. Neither the extension nor the Content-Type header is consulted at
// any point -- both come from the uploader.
func (s *Server) UploadMedia(ctx context.Context, request openapi.UploadMediaRequestObject) (openapi.UploadMediaResponseObject, error) {
	if s.Deps.Media == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	part, err := nextFilePart(request.Body)
	if err != nil {
		return openapi.UploadMedia415JSONResponse(authError(ctx, openapi.VALIDATIONFAILED,
			"Không tìm thấy tệp trong yêu cầu tải lên.")), nil
	}
	defer func() { _ = part.Close() }()

	meta := httpx.RequestMetaFromContext(ctx)
	asset, err := s.Deps.Media.Upload(ctx, media.UploadInput{
		Filename:   part.FileName(),
		Body:       part,
		UploaderID: principal.UserID,
		IP:         meta.IP,
		UserAgent:  meta.UserAgent,
	})
	switch {
	case err == nil:

	case errors.Is(err, media.ErrTooLarge):
		return openapi.UploadMedia413JSONResponse(authError(ctx, openapi.MEDIATOOLARGE,
			"Tệp vượt quá 10 MB. Vui lòng nén hoặc cắt ngắn tệp.")), nil

	case errors.Is(err, media.ErrTooLong):
		return openapi.UploadMedia415JSONResponse(authError(ctx, openapi.MEDIATOOLONG,
			"Tệp âm thanh dài hơn 5 phút. Vui lòng cắt ngắn.")), nil

	// Sniffed as audio but the duration could not be read. Refused rather than
	// stored with an unknown duration -- media_assets requires one, so storing
	// it would fail at the database and produce a 500 where an explanation is
	// the true answer.
	case errors.Is(err, probe.ErrUnmeasurable):
		return openapi.UploadMedia415JSONResponse(authError(ctx, openapi.MEDIAUNREADABLE,
			"Không đọc được tệp âm thanh này. Tệp có thể bị lỗi hoặc chưa tải lên hết.")), nil

	case errors.Is(err, probe.ErrUnsupportedType):
		return openapi.UploadMedia415JSONResponse(authError(ctx, openapi.MEDIATYPEUNSUPPORTED,
			"Chỉ hỗ trợ mp3, m4a và ảnh png/jpg/webp.")), nil

	default:
		return nil, err
	}

	url, err := s.Deps.Media.SignedURL(ctx, asset)
	if err != nil {
		return nil, err
	}
	return openapi.UploadMedia201JSONResponse(toAPIMediaAsset(asset, url)), nil
}

// nextFilePart walks to the first part that carries a filename.
//
// The contract names the field `file`, but a browser's FormData can put other
// fields alongside it and their order is not guaranteed. Matching on "has a
// filename" rather than on position is what makes that irrelevant.
func nextFilePart(reader *multipart.Reader) (*multipart.Part, error) {
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

func toAPIMediaAsset(a media.Asset, url string) openapi.MediaAsset {
	out := openapi.MediaAsset{
		Id:               parseUUID(a.ID),
		Kind:             openapi.MediaKind(a.Kind),
		Url:              url,
		MimeType:         openapi.MediaAssetMimeType(a.MimeType),
		Bytes:            int(a.Bytes),
		OriginalFilename: a.OriginalFilename,
		CreatedAt:        a.CreatedAt,
	}
	if a.DurationMs != nil {
		out.DurationMs = a.DurationMs
	}
	return out
}

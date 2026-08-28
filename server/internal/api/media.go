package api

import (
	"context"
	"errors"
	"fmt"
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

// ListMedia implements GET /admin/media -- the §8 media library.
//
// Keyset pagination (§13.8): an upload landing mid-pagination shifts every
// OFFSET page by one and shows the reader a duplicate row. A keyset asks for
// "older than this exact row", so a concurrent insert is simply not on the page.
func (s *Server) ListMedia(ctx context.Context, request openapi.ListMediaRequestObject) (openapi.ListMediaResponseObject, error) {
	if s.Deps.Media == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := media.ListInput{}
	if request.Params.Kind != nil {
		kind := media.Kind(*request.Params.Kind)
		in.Kind = &kind
	}
	if request.Params.Cursor != nil {
		in.Cursor = string(*request.Params.Cursor)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	assets, next, err := s.Deps.Media.List(ctx, in)
	if errors.Is(err, media.ErrBadCursor) {
		// The contract says never to construct a cursor, so a bad one is a
		// client bug rather than a server fault.
		return openapi.ListMedia400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(
			authError(ctx, openapi.VALIDATIONFAILED, "Con trỏ phân trang không hợp lệ."))}, nil
	}
	if err != nil {
		return nil, err
	}

	// The item type is anonymous in the generated response, so it is spelled
	// once here and filled by index rather than written out a second time as a
	// composite literal.
	var out openapi.ListMedia200JSONResponse
	out.Items = make([]struct {
		Bytes            int                                               `json:"bytes"`
		CreatedAt        openapi.Timestamp                                 `json:"createdAt"`
		DurationMs       *int                                              `json:"durationMs,omitempty"`
		Id               openapi.Uuid                                      `json:"id"`
		Kind             openapi.MediaKind                                 `json:"kind"`
		MimeType         openapi.ListMedia200JSONResponseBodyItemsMimeType `json:"mimeType"`
		OriginalFilename string                                            `json:"originalFilename"`
		Url              string                                            `json:"url"`
		UsageCount       *int                                              `json:"usageCount,omitempty"`
	}, len(assets))

	for i, a := range assets {
		usage := a.UsageCount
		out.Items[i].Bytes = int(a.Bytes)
		out.Items[i].CreatedAt = a.CreatedAt
		out.Items[i].DurationMs = a.DurationMs
		out.Items[i].Id = parseUUID(a.ID)
		out.Items[i].Kind = openapi.MediaKind(a.Kind)
		out.Items[i].MimeType = openapi.ListMedia200JSONResponseBodyItemsMimeType(a.MimeType)
		out.Items[i].OriginalFilename = a.OriginalFilename
		out.Items[i].Url = a.URL
		out.Items[i].UsageCount = &usage
	}
	if next != "" {
		out.NextCursor = &next
	}
	return out, nil
}

// DeleteMedia implements DELETE /admin/media/{id}.
//
// A referenced asset is 409, not 403: the teacher has every right to it, it is
// simply not deletable while a published version depends on it (§8, §15).
func (s *Server) DeleteMedia(ctx context.Context, request openapi.DeleteMediaRequestObject) (openapi.DeleteMediaResponseObject, error) {
	if s.Deps.Media == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	err := s.Deps.Media.Delete(ctx, media.DeleteInput{
		ID:        request.Id.String(),
		ActorID:   principal.UserID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	})
	switch {
	case err == nil:
		return openapi.DeleteMedia204Response{}, nil

	case errors.Is(err, media.ErrReferenced):
		return openapi.DeleteMedia409JSONResponse(authError(ctx, openapi.MEDIAREFERENCED,
			"Tệp đang được dùng trong một đề đã xuất bản nên không thể xoá.")), nil

	case errors.Is(err, media.ErrNotFound):
		return openapi.DeleteMedia404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			notFound(ctx, "Không tìm thấy tệp."))}, nil

	default:
		return nil, err
	}
}

// GetMediaUrl implements GET /app/media/{assetId}/url -- a student minting a
// signed URL for a listening file (§11.2).
//
// The Cache-Control max-age and the signature TTL are the same constant, so a
// cached response cannot outlive the URL inside it.
func (s *Server) GetMediaUrl(ctx context.Context, request openapi.GetMediaUrlRequestObject) (openapi.GetMediaUrlResponseObject, error) {
	if s.Deps.Media == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	result, err := s.Deps.Media.MintForStudent(ctx, principal.UserID, request.AssetId.String())
	if errors.Is(err, media.ErrForbidden) {
		return openapi.GetMediaUrl403JSONResponse{ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
			authError(ctx, openapi.FORBIDDEN, "Bạn không có quyền truy cập tệp này."))}, nil
	}
	if err != nil {
		return nil, err
	}

	return openapi.GetMediaUrl200JSONResponse{
		Body: struct {
			ExpiresAt openapi.Timestamp `json:"expiresAt"`
			Url       string            `json:"url"`
		}{
			ExpiresAt: result.ExpiresAt,
			Url:       result.URL,
		},
		Headers: openapi.GetMediaUrl200ResponseHeaders{CacheControl: cacheControlForSignedURL},
	}, nil
}

// cacheControlForSignedURL is §11.2's directive, derived from the TTL rather
// than written twice: changing SignedURLTTL without changing this would let a
// cached response outlive the signature it contains.
var cacheControlForSignedURL = fmt.Sprintf("private, max-age=%d", int(media.SignedURLTTL.Seconds()))

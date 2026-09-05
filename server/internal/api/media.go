package api

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
	"quizzivy/internal/media"
	"quizzivy/internal/media/probe"
)

// UploadMedia implements POST /admin/media (§11.1).
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
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	assets, page, err := s.Deps.Media.List(ctx, in)
	if err != nil {
		return nil, err
	}
	var out openapi.ListMedia200JSONResponse
	out.Headers.CacheControl = cacheControlForSignedURLList
	out.Body.Items = make([]openapi.LibraryAsset, len(assets))
	for i, a := range assets {
		usage := a.UsageCount
		usedIn := toAPIReferencingTests(a.UsedIn)
		out.Body.Items[i] = openapi.LibraryAsset{
			Bytes:            int(a.Bytes),
			CreatedAt:        a.CreatedAt,
			DurationMs:       a.DurationMs,
			Id:               parseUUID(a.ID),
			Kind:             openapi.LibraryAssetKind(a.Kind),
			MimeType:         openapi.LibraryAssetMimeType(a.MimeType),
			OriginalFilename: a.OriginalFilename,
			Url:              a.URL,
			UsageCount:       &usage,
			UsedIn:           &usedIn,
		}
	}
	out.Body.Page, out.Body.PageSize, out.Body.Total = page.Number, page.Size, page.Total
	return out, nil
}

func toAPIReferencingTests(refs []media.TestRef) []openapi.ReferencingTest {
	out := make([]openapi.ReferencingTest, len(refs))
	for i, ref := range refs {
		version := ref.Version
		out[i] = openapi.ReferencingTest{Id: parseUUID(ref.ID), Title: ref.Title, Version: &version}
	}
	return out
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
		resp := authError(ctx, openapi.MEDIAREFERENCED,
			"Tệp đang được dùng trong một đề đã xuất bản nên không thể xoá.")
		var blocked *media.ReferencedError
		if errors.As(err, &blocked) {
			refs := toAPIReferencingTests(blocked.Tests)
			resp.Error.Details = &map[string]interface{}{"tests": refs}
		}
		return openapi.DeleteMedia409JSONResponse(resp), nil

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
		Headers: openapi.GetMediaUrl200ResponseHeaders{
			CacheControl: cacheControlForSignedURL(s.Deps.Media.SignedURLTTL()),
		},
	}, nil
}

// cacheControlForSignedURL is §11.2's directive, derived from the TTL the
// service actually signs with rather than written twice. Two independent copies
// of "ten minutes" is precisely how a cache entry comes to outlive the
// signature it holds, which is the failure the directive exists to prevent.
func cacheControlForSignedURL(ttl time.Duration) string {
	return fmt.Sprintf("private, max-age=%d", int(ttl.Seconds()))
}

// cacheControlForSignedURLList is the list endpoint's answer.
const cacheControlForSignedURLList = "private, no-store"

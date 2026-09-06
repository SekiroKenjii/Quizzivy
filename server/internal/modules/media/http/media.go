package http

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/application/query"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"time"
)

// UploadMedia implements POST /admin/media (§11.1).
func (h Media) UploadMedia(ctx context.Context, request openapi.UploadMediaRequestObject) (openapi.UploadMediaResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	part, err := nextFilePart(request.Body)
	if err != nil {
		return openapi.UploadMedia415JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			"Không tìm thấy tệp trong yêu cầu tải lên.")), nil
	}
	defer func() { _ = part.Close() }()

	meta := httpx.RequestMetaFromContext(ctx)
	asset, err := h.app.Commands.Upload.Handle(ctx, command.Upload{Filename: part.FileName(),
		Body:       part,
		UploaderID: principal.UserID,
		IP:         meta.IP,
		UserAgent:  meta.UserAgent,
	})
	switch {
	case err == nil:

	case errors.Is(err, domain.ErrTooLarge):
		return openapi.UploadMedia413JSONResponse(httpapi.Error(ctx, openapi.MEDIATOOLARGE,
			"Tệp vượt quá 10 MB. Vui lòng nén hoặc cắt ngắn tệp.")), nil

	case errors.Is(err, domain.ErrTooLong):
		return openapi.UploadMedia415JSONResponse(httpapi.Error(ctx, openapi.MEDIATOOLONG,
			"Tệp âm thanh dài hơn 5 phút. Vui lòng cắt ngắn.")), nil
	case errors.Is(err, domain.ErrUnmeasurable):
		return openapi.UploadMedia415JSONResponse(httpapi.Error(ctx, openapi.MEDIAUNREADABLE,
			"Không đọc được tệp âm thanh này. Tệp có thể bị lỗi hoặc chưa tải lên hết.")), nil

	case errors.Is(err, domain.ErrUnsupportedType):
		return openapi.UploadMedia415JSONResponse(httpapi.Error(ctx, openapi.MEDIATYPEUNSUPPORTED,
			"Chỉ hỗ trợ mp3, m4a và ảnh png/jpg/webp.")), nil

	default:
		return nil, err
	}

	url, err := h.app.Queries.SignedURL.Handle(ctx, query.SignedURL{Asset: asset})
	if err != nil {
		return nil, err
	}
	return openapi.UploadMedia201JSONResponse(ToAPIMediaAsset(asset, url)), nil
}

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

func ToAPIMediaAsset(a domain.Asset, url string) openapi.MediaAsset {
	out := openapi.MediaAsset{
		Id:               httpapi.ParseUUID(a.ID),
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
func (h Media) ListMedia(ctx context.Context, request openapi.ListMediaRequestObject) (openapi.ListMediaResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := domain.ListInput{}
	if request.Params.Kind != nil {
		kind := domain.Kind(*request.Params.Kind)
		in.Kind = &kind
	}
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	listResult, err := h.app.Queries.List.Handle(ctx, query.List{Input: in})
	assets, page := listResult.Items, listResult.Page
	if err != nil {
		return nil, err
	}
	totalBytes, err := h.app.Queries.TotalBytes.Handle(ctx, query.TotalBytes{Kind: in.Kind})
	if err != nil {
		return nil, err
	}
	var out openapi.ListMedia200JSONResponse
	out.Body.TotalBytes = int(totalBytes)
	out.Headers.CacheControl = cacheControlForSignedURLList
	out.Body.Items = make([]openapi.LibraryAsset, len(assets))
	for i, a := range assets {
		usage := a.UsageCount
		usedIn := ToAPIReferencingTests(a.UsedIn)
		out.Body.Items[i] = openapi.LibraryAsset{
			Bytes:            int(a.Bytes),
			CreatedAt:        a.CreatedAt,
			DurationMs:       a.DurationMs,
			Id:               httpapi.ParseUUID(a.ID),
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

func ToAPIReferencingTests(refs []domain.TestRef) []openapi.ReferencingTest {
	out := make([]openapi.ReferencingTest, len(refs))
	for i, ref := range refs {
		version := ref.Version
		out[i] = openapi.ReferencingTest{Id: httpapi.ParseUUID(ref.ID), Title: ref.Title, Version: &version}
	}
	return out
}

// DeleteMedia implements DELETE /admin/media/{id}.
func (h Media) DeleteMedia(ctx context.Context, request openapi.DeleteMediaRequestObject) (openapi.DeleteMediaResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	meta := httpx.RequestMetaFromContext(ctx)
	_, err := h.app.Commands.Delete.Handle(ctx, command.Delete{Input: domain.DeleteInput{
		ID:        request.Id.String(),
		ActorID:   principal.UserID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
	}})
	switch {
	case err == nil:
		return openapi.DeleteMedia204Response{}, nil

	case errors.Is(err, domain.ErrReferenced):
		resp := httpapi.Error(ctx, openapi.MEDIAREFERENCED,
			"Tệp đang được dùng trong một đề đã xuất bản nên không thể xoá.")
		var blocked *domain.ReferencedError
		if errors.As(err, &blocked) {
			refs := ToAPIReferencingTests(blocked.Tests)
			resp.Error.Details = &map[string]interface{}{"tests": refs}
		}
		return openapi.DeleteMedia409JSONResponse(resp), nil

	case errors.Is(err, domain.ErrNotFound):
		return openapi.DeleteMedia404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, "Không tìm thấy tệp."))}, nil

	default:
		return nil, err
	}
}

// GetMediaUrl implements GET /app/media/{assetId}/url -- a student minting a
// signed URL for a listening file (§11.2).
func (h Media) GetMediaUrl(ctx context.Context, request openapi.GetMediaUrlRequestObject) (openapi.GetMediaUrlResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	result, err := h.app.Queries.MintForStudent.Handle(ctx, query.MintForStudent{StudentID: principal.UserID, AssetID: request.AssetId.String()})
	if errors.Is(err, domain.ErrForbidden) {
		return openapi.GetMediaUrl403JSONResponse{ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
			httpapi.Error(ctx, openapi.FORBIDDEN, "Bạn không có quyền truy cập tệp này."))}, nil
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
			CacheControl: cacheControlForSignedURL(h.app.SignedURLTTL()),
		},
	}, nil
}

func cacheControlForSignedURL(ttl time.Duration) string {
	return fmt.Sprintf("private, max-age=%d", int(ttl.Seconds()))
}

const cacheControlForSignedURLList = "private, no-store"

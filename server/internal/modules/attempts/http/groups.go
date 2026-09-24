package http

import (
	"context"
	"quizzivy/gen/openapi"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

func (h Attempts) groupAsset(ctx context.Context, studentID, id string) (*openapi.MediaAsset, error) {
	if h.media == nil {
		return nil, httpx.ErrNotImplemented
	}
	signed, err := h.media.MintForStudent(ctx, studentID, id)
	if err != nil {
		return nil, err
	}
	asset, err := h.media.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return &openapi.MediaAsset{
		Id: httpapi.ParseUUID(asset.ID), Kind: openapi.MediaKind(asset.Kind),
		MimeType: openapi.MediaAssetMimeType(asset.MimeType), OriginalFilename: asset.OriginalFilename,
		Bytes: int(asset.Bytes), DurationMs: asset.DurationMs, CreatedAt: asset.CreatedAt, Url: signed.URL,
	}, nil
}

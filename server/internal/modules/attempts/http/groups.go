package http

import (
	"context"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/domain"
	mediahttp "quizzivy/internal/modules/media/http"
	testshttp "quizzivy/internal/modules/tests/http"
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

func sharedReviewContext(ctx context.Context, shared *domain.SharedReviewContext, resolve func(context.Context, string) (*openapi.MediaAsset, error)) (*openapi.SharedReviewContext, error) {
	if shared == nil {
		return nil, nil
	}
	groups, err := testshttp.StudentGroups(ctx, shared.Groups, resolve)
	if err != nil {
		return nil, err
	}
	transcripts := shared.Transcripts
	if transcripts == nil {
		transcripts = map[string]string{}
	}
	out := &openapi.SharedReviewContext{Groups: groups, Transcripts: transcripts}
	if shared.AudioPlays != nil {
		out.AudioPlays = &shared.AudioPlays
	}
	return out, nil
}

func (h Attempts) adminGroupAsset(ctx context.Context, id string) (*openapi.MediaAsset, error) {
	if h.media == nil {
		return nil, httpx.ErrNotImplemented
	}
	asset, err := h.media.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	url, err := h.media.SignedURL(ctx, asset)
	if err != nil {
		return nil, err
	}
	out := mediahttp.ToAPIMediaAsset(asset, url)
	return &out, nil
}

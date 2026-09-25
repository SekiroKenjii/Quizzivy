package query

import (
	"context"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/domain"
	"time"
)

type Get struct{ ID string }

type GetHandler struct{ Repo domain.Repository }

func (h GetHandler) Handle(ctx context.Context, in Get) (domain.Import, error) {
	return h.Repo.Get(ctx, in.ID)
}

type List = domain.Filter
type ListHandler struct{ Repo domain.Repository }

func (h ListHandler) Handle(ctx context.Context, in List) (domain.List, error) {
	return h.Repo.List(ctx, in)
}

type Download struct{ ImportID, SourceID string }
type DownloadResult struct {
	URL       string
	ExpiresAt time.Time
}
type DownloadHandler struct {
	Repo  domain.Repository
	Store ports.ObjectStore
}

func (h DownloadHandler) Handle(ctx context.Context, in Download) (DownloadResult, error) {
	parent, err := h.Repo.Get(ctx, in.ImportID)
	if err != nil {
		return DownloadResult{}, err
	}
	if parent.FilesRemovedAt != nil {
		return DownloadResult{}, domain.ErrFilesRemoved
	}
	src, err := h.Repo.Source(ctx, in.ImportID, in.SourceID)
	if err != nil {
		return DownloadResult{}, err
	}
	ttl := time.Minute
	expires := time.Now().Add(ttl)
	url, err := h.Store.SignedDownloadURL(ctx, src.StorageKey, src.Filename, ttl)
	return DownloadResult{URL: url, ExpiresAt: expires}, err
}

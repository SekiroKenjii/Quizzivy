package query

import (
	"context"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/domain"
	"time"
)

type Get struct{ ID string }

// GetHandler reads one import. Reading a queued import also wakes the worker,
// so a lost wake signal heals while a teacher watches the import wait.
type GetHandler struct {
	Repo   domain.Repository
	Worker ports.WorkerSignal
}

func (h GetHandler) Handle(ctx context.Context, in Get) (domain.Import, error) {
	v, err := h.Repo.Get(ctx, in.ID)
	if err == nil && v.Status == "queued" && h.Worker != nil {
		h.Worker.Wake()
	}
	return v, err
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
	src, err := h.Repo.Source(ctx, in.ImportID, in.SourceID)
	if err != nil {
		return DownloadResult{}, err
	}
	ttl := time.Minute
	expires := time.Now().Add(ttl)
	url, err := h.Store.SignedDownloadURL(ctx, src.StorageKey, src.Filename, ttl)
	return DownloadResult{URL: url, ExpiresAt: expires}, err
}

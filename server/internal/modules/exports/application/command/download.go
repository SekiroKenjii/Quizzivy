package command

import (
	"context"
	"time"

	"quizzivy/internal/modules/exports/application/internal/support"
	"quizzivy/internal/modules/exports/domain"
	"quizzivy/internal/shared/actor"
)

// Download hands Actor the address of the file of the export ID, once. It
// fails with domain.ErrNotFound unless Actor requested the export, and with
// domain.ErrUnavailable unless it is ready, unexpired and not fetched yet. The
// address works for domain.URLTTL and names the file
// quizzivy-export-YYYYMMDD.zip, dated by the day the export was requested.
type Download struct {
	Actor actor.Actor
	ID    string
}

// Downloaded is the address a Download returned.
type Downloaded struct {
	URL string
}

type DownloadHandler struct {
	*support.Service
}

func (h DownloadHandler) Handle(ctx context.Context, c Download) (Downloaded, error) {
	url, err := h.Repo.Fetch(ctx, domain.Fetch{ID: c.ID, ActorID: c.Actor.ID, IP: c.Actor.IP, UserAgent: c.Actor.UserAgent},
		func(key string, created time.Time) (string, error) {
			name := "quizzivy-export-" + created.In(h.Zone).Format("20060102") + ".zip"
			return h.Store.SignedDownloadURL(ctx, key, name, domain.URLTTL)
		})
	if err != nil {
		return Downloaded{}, err
	}
	return Downloaded{URL: url}, nil
}

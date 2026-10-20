package command

import (
	"context"
	"strings"

	"quizzivy/internal/modules/audit/application/internal/support"
	"quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/shared/actor"
)

// StartExport begins a file of the log for Actor: the entries of Period, the
// last seven or thirty days or the year so far in the viewer's zone, that match
// Category and Query. It counts them, refuses with domain.ErrExportTooLarge
// when there are more than domain.MaxExportRows, and otherwise audits the
// export as audit.exported before a byte is sent, so every file that leaves is
// on the record. The plan it returns reads exactly the entries it counted.
type StartExport struct {
	Actor    actor.Actor
	Period   domain.Range
	Format   string
	Category domain.Category
	Query    string
}

type StartExportHandler struct {
	*support.Service
}

func (s StartExportHandler) Handle(ctx context.Context, c StartExport) (domain.ExportPlan, error) {
	if c.Category != "" && !c.Category.Valid() {
		return domain.ExportPlan{}, domain.ErrUnknownCategory
	}
	if c.Period != domain.Week && c.Period != domain.Month && c.Period != domain.Year {
		return domain.ExportPlan{}, domain.ErrUnknownRange
	}
	zone, loc, err := s.LocationOf(ctx, c.Actor.ID)
	if err != nil {
		return domain.ExportPlan{}, err
	}
	window, err := domain.ResolveWindow(c.Period, "", "", loc, s.Now())
	if err != nil {
		return domain.ExportPlan{}, err
	}
	search := strings.TrimSpace(c.Query)
	filter := domain.Filter{Category: c.Category, Query: search, Actions: domain.MatchingActions(search), Window: window}

	counted, err := s.Repo.CountExport(ctx, filter)
	if err != nil {
		return domain.ExportPlan{}, err
	}
	if counted.Rows > domain.MaxExportRows {
		return domain.ExportPlan{}, domain.ErrExportTooLarge
	}
	at := s.Now()
	err = s.Repo.RecordExport(ctx, domain.Export{
		ActorID: c.Actor.ID, IP: c.Actor.IP, UserAgent: c.Actor.UserAgent, At: at,
		Period: c.Period, Format: c.Format, Category: c.Category, Query: search, Rows: counted.Rows,
	})
	if err != nil {
		return domain.ExportPlan{}, err
	}
	filter.Through = counted.Through
	return domain.ExportPlan{Filter: filter, Rows: counted.Rows, Zone: zone, At: at}, nil
}

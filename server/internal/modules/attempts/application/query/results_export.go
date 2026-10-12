package query

import (
	"context"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
)

// DefaultZone is the calendar zone times are written in when the caller has
// none that can be resolved.
const DefaultZone = "Asia/Ho_Chi_Minh"

// ResultsExport reads the rows of the assignments named, for the students Scope
// reaches. Every assignment must be one Scope reaches.
type ResultsExport struct {
	AssignmentIDs []string
	Scope         access.Scope
}

// ResultsExportHandler closes the attempts that ran out of time before it
// reads, as the monitor does, and names the zone the caller's times are
// written in: the caller's own, or DefaultZone when it cannot be resolved.
type ResultsExportHandler struct {
	*support.Service
}

func (h ResultsExportHandler) Handle(ctx context.Context, q ResultsExport) (domain.ResultsExport, error) {
	for _, id := range q.AssignmentIDs {
		if err := h.ExpireDue(ctx, q.Scope, id); err != nil {
			return domain.ResultsExport{}, err
		}
	}
	rows, err := h.Store.Results(ctx, q.Scope, q.AssignmentIDs)
	if err != nil {
		return domain.ResultsExport{}, err
	}
	return domain.ResultsExport{Zone: h.zoneOf(ctx, q.Scope.UserID), Rows: rows}, nil
}

func (h ResultsExportHandler) zoneOf(ctx context.Context, userID string) string {
	if h.Zones == nil {
		return DefaultZone
	}
	zone, err := h.Zones.ZoneOf(ctx, userID)
	if err != nil {
		return DefaultZone
	}
	return zone
}

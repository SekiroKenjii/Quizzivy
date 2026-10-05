package query

import (
	"context"
	"quizzivy/internal/modules/dashboard/application/internal/support"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/access"
)

// Summary reads the teacher's home over what Scope reaches; a zero Scope
// reads nothing.
type Summary struct {
	Scope access.Scope
	Range string
}

type SummaryHandler struct {
	*support.Service
}

func (s SummaryHandler) Handle(ctx context.Context, q Summary) (domain.Summary, error) {
	zone := "Asia/Ho_Chi_Minh"
	if s.Zones != nil {
		var err error
		zone, err = s.Zones.ZoneOf(ctx, q.Scope.UserID)
		if err != nil {
			return domain.Summary{}, err
		}
	}
	home, err := s.Repo.Home(ctx, domain.HomeQuery{Scope: q.Scope, Now: s.Now(), Zone: zone, Days: daysOf(q.Range)})
	if err != nil {
		return domain.Summary{}, err
	}
	out, err := s.Repo.Summary(ctx, q.Scope)
	out.Home = home
	return out, err
}

func daysOf(value string) int {
	switch value {
	case "7d":
		return 7
	case "30d":
		return 30
	default:
		return 14
	}
}

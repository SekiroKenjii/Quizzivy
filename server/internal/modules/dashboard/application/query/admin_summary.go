package query

import (
	"context"

	"quizzivy/internal/modules/dashboard/application/internal/support"
	"quizzivy/internal/modules/dashboard/domain"
)

// AdminSummary reads the Admin sidebar's counts for Viewer: the accounts on a temporary password, counted as the Overview's attention row counts them, and only for a viewer who holds people.users.manage.
type AdminSummary struct {
	Viewer domain.AdminViewer
}

type AdminSummaryHandler struct{ *support.Service }

func (s AdminSummaryHandler) Handle(ctx context.Context, q AdminSummary) (domain.AdminSummary, error) {
	if !q.Viewer.Users {
		return domain.AdminSummary{}, nil
	}
	n, err := s.Repo.MustChangePassword(ctx)
	if err != nil {
		return domain.AdminSummary{}, err
	}
	return domain.AdminSummary{MustChangePassword: &n}, nil
}

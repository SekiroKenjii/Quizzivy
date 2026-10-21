package support

import (
	"context"
	"time"

	aiquery "quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/limits"
)

// CreditsView reads the month's AI credits and the requests waiting for a viewer who holds ai.credits.manage. It answers nil, and asks the AI module nothing, for any other viewer and while the assistant is off.
func (s *Service) CreditsView(ctx context.Context, v domain.AdminViewer) (*domain.CreditsView, error) {
	if !v.Credits || s.Credits == nil {
		return nil, nil
	}
	month, err := s.Credits.Handle(ctx, aiquery.AdminCredits{})
	if err != nil {
		return nil, err
	}
	out := domain.CreditsView{
		Credits: domain.Credits{Used: month.Pool.Used, Pool: month.Pool.Total()},
		Waiting: make([]domain.WaitingRequest, len(month.Requests)),
	}
	for i, r := range month.Requests {
		out.Waiting[i] = domain.WaitingRequest{TeacherName: r.TeacherName, Used: r.Used, Allowance: r.Allowance}
	}
	return &out, nil
}

// Usage reads what the centre holds and uses, the day's and month's mail in the centre's zone, and sets it, with the credits already read, against the limits resolved under the deploy's overrides.
func (s *Service) Usage(ctx context.Context, credits *domain.CreditsView) (domain.Usage, error) {
	now := s.Now()
	local := now.In(s.Location)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.Location)
	month := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, s.Location)
	held, err := s.Repo.Held(ctx, domain.EmailWindow{
		Now: now, DayStart: day, NextDay: day.AddDate(0, 0, 1), MonthStart: month, NextMonth: month.AddDate(0, 1, 0),
	})
	if err != nil {
		return domain.Usage{}, err
	}
	stored, err := s.Repo.Limits(ctx)
	if err != nil {
		return domain.Usage{}, err
	}
	var pool *domain.Credits
	if credits != nil {
		pool = &credits.Credits
	}
	return domain.NewUsage(limits.Resolve(stored, s.LimitEnv), held, pool), nil
}

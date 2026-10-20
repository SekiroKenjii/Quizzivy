package support

import (
	"context"

	aiquery "quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/dashboard/domain"
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

// Usage reads what the centre holds and sets it, with the accounts already counted and the credits already read, against the plan.
func (s *Service) Usage(ctx context.Context, accounts domain.Accounts, credits *domain.CreditsView) (domain.Usage, error) {
	held, err := s.Repo.Held(ctx)
	if err != nil {
		return domain.Usage{}, err
	}
	var month *domain.Credits
	if credits != nil {
		month = &credits.Credits
	}
	return domain.NewUsage(s.Plan, accounts, held, month), nil
}

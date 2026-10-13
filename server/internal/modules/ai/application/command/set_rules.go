package command

import (
	"context"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/shared/validation"
)

// SetRules changes the centre's rules; a nil field keeps what is stored. A new default allowance is between domain.MinAllowance and domain.MaxAllowance and rewrites this month's allowances that are not custom, never below what a teacher has used. The warning share is between domain.MinWarnPercent and domain.MaxWarnPercent. The result says how many allowances moved. One audit row names By.
type SetRules struct {
	By               domain.Author
	DefaultAllowance *int64
	WarnPercent      *int
	OnTeacherLimit   *domain.Limit
}

type SetRulesHandler struct {
	*support.Service
}

func (h SetRulesHandler) Handle(ctx context.Context, cmd SetRules) (domain.RulesChange, error) {
	if err := checkAuthor(cmd.By); err != nil {
		return domain.RulesChange{}, err
	}
	var refused validation.Error
	if cmd.DefaultAllowance == nil && cmd.WarnPercent == nil && cmd.OnTeacherLimit == nil {
		refused.Add("defaultAllowance", "change at least one rule")
	}
	if a := cmd.DefaultAllowance; a != nil && (*a < domain.MinAllowance || *a > domain.MaxAllowance) {
		refused.Add("defaultAllowance", "outside the allowance a teacher may have")
	}
	if w := cmd.WarnPercent; w != nil && (*w < domain.MinWarnPercent || *w > domain.MaxWarnPercent) {
		refused.Add("warnPercent", "outside the share a teacher may be warned at")
	}
	if l := cmd.OnTeacherLimit; l != nil && !l.Valid() {
		refused.Add("onTeacherLimit", "must be pause or request")
	}
	if err := refused.OrNil(); err != nil {
		return domain.RulesChange{}, err
	}
	return h.Repo.SetRules(ctx, domain.SetRulesInput{
		By: cmd.By, Month: h.Month(), DefaultAllowance: cmd.DefaultAllowance, WarnPercent: cmd.WarnPercent, OnTeacherLimit: cmd.OnTeacherLimit,
	})
}

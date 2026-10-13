package command

import (
	"context"

	"github.com/google/uuid"

	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/shared/validation"
)

// SetAllowance changes how much one teacher may use this month, or pauses or resumes them; a nil field keeps what is stored. Custom true sets the teacher's own allowance to Credits, between domain.MinAllowance and domain.MaxAllowance, and a change of the centre's default leaves it; Custom false returns the teacher to the default. Approved requests' credits stay on top of either. The change takes effect at once and usage so far still counts, so a figure below it is refused with domain.ErrAllowanceBelowUse. It answers domain.ErrNotATeacher for an id that is not an active, non-student user, and writes one audit row naming By.
type SetAllowance struct {
	By      domain.Author
	UserID  string
	Custom  *bool
	Credits int64
	Paused  *bool
}

type SetAllowanceHandler struct {
	*support.Service
}

func (h SetAllowanceHandler) Handle(ctx context.Context, cmd SetAllowance) (domain.TeacherMonth, error) {
	if err := checkAuthor(cmd.By); err != nil {
		return domain.TeacherMonth{}, err
	}
	if _, err := uuid.Parse(cmd.UserID); err != nil {
		return domain.TeacherMonth{}, domain.ErrNotATeacher
	}
	var refused validation.Error
	if cmd.Custom == nil && cmd.Paused == nil {
		refused.Add("custom", "change the allowance or the pause")
	}
	custom := cmd.Custom != nil && *cmd.Custom
	if custom && (cmd.Credits < domain.MinAllowance || cmd.Credits > domain.MaxAllowance) {
		refused.Add("credits", "outside the allowance a teacher may have")
	}
	if !custom && cmd.Credits != 0 {
		refused.Add("credits", "only a custom allowance has a figure")
	}
	if err := refused.OrNil(); err != nil {
		return domain.TeacherMonth{}, err
	}
	month, err := h.Repo.SetAllowance(ctx, domain.SetAllowanceInput{
		By: cmd.By, Month: h.Month(), UserID: cmd.UserID, Custom: cmd.Custom, Credits: cmd.Credits, Paused: cmd.Paused,
	})
	if err != nil {
		return domain.TeacherMonth{}, err
	}
	rules, err := h.Repo.Rules(ctx)
	if err != nil {
		return domain.TeacherMonth{}, err
	}
	month.Status = domain.StatusOf(month.Used, month.Allowance, rules.WarnPercent, month.Paused)
	return month, nil
}

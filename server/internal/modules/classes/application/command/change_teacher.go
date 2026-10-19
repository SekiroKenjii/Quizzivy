package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/opt"
)

// ChangeTeacher gives a class to TeacherID. With Notify, once the change has
// committed, the new teacher is told and so is the previous one unless their
// account is disabled; naming the current teacher tells nobody. A
// notification that cannot be delivered is logged and does not fail the
// change.
type ChangeTeacher struct {
	ClassID   string
	TeacherID string
	Notify    bool
	Actor     actor.Actor
}

type ChangeTeacherHandler struct {
	*support.Service
	Codes *support.Enrolment
}

func (s ChangeTeacherHandler) Handle(ctx context.Context, cmd ChangeTeacher) (domain.Class, error) {
	change, err := s.Repo.ChangeTeacher(ctx, domain.ChangeTeacherInput{
		ClassID: cmd.ClassID, TeacherID: cmd.TeacherID, ActorUserID: cmd.Actor.ID, Now: s.Codes.Now(),
		IP: opt.String(cmd.Actor.IP), UserAgent: opt.String(cmd.Actor.UserAgent),
	})
	if err != nil {
		return domain.Class{}, err
	}
	if cmd.Notify && change.Changed {
		s.announce(ctx, cmd, change)
	}
	class := change.Class
	return class, s.AttachAverage(ctx, &class)
}

func (s ChangeTeacherHandler) announce(ctx context.Context, cmd ChangeTeacher, change domain.TeacherChange) {
	cqrs.Announce(ctx, s.Codes.Notifier, s.Codes.Logger, "class teacher changed", func(context.Context) ([]notificationscommand.Notify, error) {
		target := &notificationsdomain.Target{Route: notificationsdomain.RouteClasses}
		key := notificationsdomain.TeacherChangedKey(cmd.ClassID)
		out := []notificationscommand.Notify{{
			UserID:    cmd.TeacherID,
			Kind:      notificationsdomain.ClassTeacherChanged,
			Params:    notificationsdomain.TeacherChanged{ClassName: change.Class.Name},
			Target:    target,
			DedupeKey: key,
			Merge:     notificationsdomain.Replace,
		}}
		if change.Previous != nil && !change.PreviousDisabled {
			out = append(out, notificationscommand.Notify{
				UserID:    *change.Previous,
				Kind:      notificationsdomain.ClassTeacherChanged,
				Params:    notificationsdomain.TeacherChanged{ClassName: change.Class.Name, TeacherName: &change.TeacherName},
				Target:    target,
				DedupeKey: key,
				Merge:     notificationsdomain.Replace,
			})
		}
		return out, nil
	})
}

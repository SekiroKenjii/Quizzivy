package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/domain"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

// TransferUserContent hands everything the disabled account ID owns to ToID,
// and tells ToID when Notify is set. The mover moves classes, tests, questions,
// question groups, media and assignments; R7 and R10 extend it for question
// sets, shares, word lists and courses.
type TransferUserContent struct {
	Request domain.WriteRequest
	ID      string
	ToID    string
	Notify  bool
}

type TransferUserContentHandler struct {
	*support.Admin
	Mover    ports.ContentMover
	Notifier ports.Notifier
}

// Handle returns what moved and what stayed with the source. It applies the
// guards in order: domain.ErrUserNotFound, the subset rule (the access
// module's ErrTargetRoleNotAllowed), domain.ErrUserNotDisabled, which the
// caller's own active account always meets, and domain.ErrInvalidRecipient. A move that loses
// to a role change or a deadlock is tried again, up to three times in all.
// The notification is sent after the commit, only when something moved; a
// notification that cannot be delivered is logged and does not fail the move.
func (h TransferUserContentHandler) Handle(ctx context.Context, cmd TransferUserContent) (domain.TransferredContent, error) {
	var result domain.TransferredContent
	err := actOnAccount(ctx, h.Admin, cmd.Request, cmd.ID, false, func(target access.Principal) error {
		var err error
		result, err = h.Mover.Move(ctx, cmd.Request, domain.ContentTransfer{
			FromID: cmd.ID, FromRoleID: target.RoleID, ToID: cmd.ToID, Now: h.Service.Now(),
		})
		return err
	})
	if err != nil {
		return domain.TransferredContent{}, err
	}
	if cmd.Notify && result.Moved.Total() > 0 {
		h.announce(ctx, cmd, result.Moved.Total())
	}
	return result, nil
}

func (h TransferUserContentHandler) announce(ctx context.Context, cmd TransferUserContent, count int) {
	cqrs.Announce(ctx, h.Notifier, h.Students.Logger, "content transferred", func(ctx context.Context) ([]notificationscommand.Notify, error) {
		source, err := h.Repo.Get(ctx, cmd.ID)
		if err != nil {
			return nil, err
		}
		return []notificationscommand.Notify{{
			UserID:    cmd.ToID,
			Kind:      notificationsdomain.ContentTransferred,
			Params:    notificationsdomain.Transferred{TeacherName: source.FullName, Count: count},
			Target:    &notificationsdomain.Target{Route: notificationsdomain.RouteClasses},
			DedupeKey: notificationsdomain.TransferredKey(cmd.ID),
			Merge:     notificationsdomain.Replace,
		}}, nil
	})
}

package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/opt"
)

// RemoveAvatar clears the caller's profile photo.
type RemoveAvatar struct {
	UserID    string
	IP        string
	UserAgent string
}

// RemoveAvatarHandler clears the key in one audited statement and then deletes
// the object it named. A caller with no photo changes nothing and writes no
// audit row. It needs no object store: without one the key is cleared and
// there is no object to delete.
type RemoveAvatarHandler struct{ *support.Service }

func (s RemoveAvatarHandler) Handle(ctx context.Context, cmd RemoveAvatar) (domain.User, error) {
	written, err := s.Users.SetAvatar(ctx, domain.AvatarRecord{UserID: cmd.UserID, Now: s.Now(), IP: opt.String(cmd.IP), UserAgent: opt.String(cmd.UserAgent)})
	if err != nil {
		return domain.User{}, err
	}
	if written.PreviousKey != nil {
		s.DropAvatarObject(ctx, *written.PreviousKey)
	}
	return written.User, nil
}

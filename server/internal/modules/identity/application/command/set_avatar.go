package command

import (
	"bytes"
	"context"
	"io"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/opt"
)

// SetAvatar replaces the caller's profile photo with the image read from Body.
type SetAvatar struct {
	UserID    string
	Body      io.Reader
	IP        string
	UserAgent string
}

// SetAvatarHandler makes the square PNG, keeps it under a new key, points the
// user at it in one audited statement and then deletes the photo it replaced.
// It stores nothing when the image is refused, and removes the new object when
// the write fails. ErrAvatarsUnavailable answers a deployment with no object
// store.
type SetAvatarHandler struct{ *support.Service }

func (s SetAvatarHandler) Handle(ctx context.Context, cmd SetAvatar) (domain.User, error) {
	if s.Avatars == nil || s.Photos == nil {
		return domain.User{}, domain.ErrAvatarsUnavailable
	}
	photo, err := s.Photos.Square(ctx, cmd.Body)
	if err != nil {
		return domain.User{}, err
	}
	key, err := support.AvatarKey(cmd.UserID)
	if err != nil {
		return domain.User{}, err
	}
	if err := s.Avatars.Put(ctx, key, "image/png", bytes.NewReader(photo), int64(len(photo))); err != nil {
		return domain.User{}, err
	}
	written, err := s.Users.SetAvatar(ctx, domain.AvatarRecord{UserID: cmd.UserID, Key: &key, Now: s.Now(), IP: opt.String(cmd.IP), UserAgent: opt.String(cmd.UserAgent)})
	if err != nil {
		s.DropAvatarObject(ctx, key)
		return domain.User{}, err
	}
	if written.PreviousKey != nil {
		s.DropAvatarObject(ctx, *written.PreviousKey)
	}
	return written.User, nil
}

package query

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
)

// AvatarURL asks for the presigned URL of a profile photo, given the key a
// user row holds. A nil key is a user with no photo.
type AvatarURL struct {
	Key *string
}

// AvatarURLHandler signs a GET valid for 24 hours. It answers an empty URL for
// no photo and when no object store is configured, so the photo is left out of
// the payload rather than failing it.
type AvatarURLHandler struct{ *support.Service }

func (s AvatarURLHandler) Handle(ctx context.Context, q AvatarURL) (string, error) {
	if q.Key == nil || s.Avatars == nil {
		return "", nil
	}
	return s.Avatars.SignedURL(ctx, *q.Key, support.AvatarURLTTL)
}

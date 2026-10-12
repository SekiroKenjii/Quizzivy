package support

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// AvatarURLTTL is how long the presigned URL of a profile photo stays valid.
const AvatarURLTTL = 24 * time.Hour

const avatarDeleteBudget = 10 * time.Second

// AvatarKey names a new photo object. The name is a fresh uuid and never the
// upload's file name, so two photos of one user never share a key.
func AvatarKey(userID string) (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("avatar key: %w", err)
	}
	return fmt.Sprintf("avatars/%s/%s.png", userID, id), nil
}

// DropAvatarObject deletes a photo object that nothing references any more,
// on a context that keeps the request's values and not its end, so a client
// that hangs up does not leave the object behind. A failure is logged and
// never returned: the object is unreachable from every response either way.
func (s *Service) DropAvatarObject(ctx context.Context, key string) {
	if s.Avatars == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), avatarDeleteBudget)
	defer cancel()
	if err := s.Avatars.Delete(ctx, key); err != nil {
		s.logger().Warn("avatar object not deleted", "key", key, "err", err)
	}
}

func (s *Service) logger() *slog.Logger {
	if s.Log == nil {
		return slog.Default()
	}
	return s.Log
}

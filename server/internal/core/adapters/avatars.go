package adapters

import (
	"context"

	identityquery "quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/shared/cqrs"
)

// TeacherPhotos is identity's AvatarURL query behind the classes module's
// Avatars port: it signs the photo a teacher's row names, and answers an empty
// URL when the teacher has none or no store is configured.
type TeacherPhotos struct {
	Query cqrs.QueryHandler[identityquery.AvatarURL, string]
}

// AvatarURL signs a GET for the stored photo key.
func (a TeacherPhotos) AvatarURL(ctx context.Context, key string) (string, error) {
	return a.Query.Handle(ctx, identityquery.AvatarURL{Key: &key})
}

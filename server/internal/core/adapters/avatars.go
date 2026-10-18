package adapters

import (
	"context"

	identityquery "quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/platform/storage"
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

// AvatarObjects is the object store behind the identity module's AvatarObjects
// port, which the avatar sweep lists and deletes through.
type AvatarObjects struct {
	Objects *storage.Client
}

// List returns the keys of at most limit stored objects under prefix, in key
// order.
func (s AvatarObjects) List(ctx context.Context, prefix string, limit int) ([]string, error) {
	objects, err := s.Objects.List(ctx, prefix, limit)
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(objects))
	for i, o := range objects {
		keys[i] = o.Key
	}
	return keys, nil
}

// Delete removes the object under key.
func (s AvatarObjects) Delete(ctx context.Context, key string) error {
	return s.Objects.Delete(ctx, key)
}

package command

import (
	"context"

	"quizzivy/internal/modules/access/application/internal/support"
	"quizzivy/internal/modules/access/application/model"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
)

func errMissing(err error) error {
	if err != nil {
		return err
	}
	return domain.ErrRoleNotFound
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func keysOf(keys []access.Key) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = string(k)
	}
	return out
}

func one(ctx context.Context, s *support.Service, by domain.Caller, id string) (model.Role, error) {
	views, err := s.Views(ctx, by, []string{id})
	if err != nil || len(views) == 0 {
		return model.Role{}, errMissing(err)
	}
	return views[0], nil
}

package application

import (
	"context"

	"quizzivy/internal/modules/identity/domain"
)

// NewTemporaryPassword returns a fresh temporary password and its hash.
func temporaryPassword(ctx context.Context) (password, hash string, err error) {
	password, err = domain.TemporaryPassword()
	if err != nil {
		return "", "", err
	}
	hash, err = domain.HashPassword(ctx, password)
	if err != nil {
		return "", "", err
	}
	return password, hash, nil
}

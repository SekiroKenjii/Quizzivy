package application

import (
	"quizzivy/internal/modules/media/domain"

	"github.com/google/uuid"
)

// newAssetID mints the id up front, because the storage key contains it -- the
// object has to be written before the row exists, so the row cannot supply it.
func newAssetID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", domain.ErrNoID
	}
	return id.String(), nil
}

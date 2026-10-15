package support

import (
	"time"

	"quizzivy/internal/modules/exports/application/ports"
	"quizzivy/internal/modules/exports/domain"
)

// Service carries what the handlers share: the repository, the object store,
// the signal that wakes the worker, the clock and the zone the date of a
// download's file name is written in.
type Service struct {
	Repo  domain.Repository
	Store ports.Store
	Wake  ports.Wake
	Now   func() time.Time
	Zone  *time.Location
}

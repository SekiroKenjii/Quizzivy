package support

import (
	"quizzivy/internal/modules/classes/domain"
	"time"
)

// Enrolment carries what the enrolment handlers share: their ports and the helpers they call.
type Enrolment struct {
	Repo domain.Repository
	Now  func() time.Time
}

func NewEnrolment(repo domain.Repository) *Enrolment {
	return &Enrolment{Repo: repo, Now: time.Now}
}

func (s *Enrolment) SetClock(now func() time.Time) { s.Now = now }

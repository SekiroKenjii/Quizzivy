package support

import (
	"quizzivy/internal/modules/classes/domain"
	"time"
)

// Enrolment carries what the enrolment handlers share: their ports and the helpers they call.
type Enrolment struct {
	Repo domain.Repository
	Keys domain.JoinCodeKeys
	Now  func() time.Time
}

func NewEnrolment(repo domain.Repository, keys domain.JoinCodeKeys) *Enrolment {
	return &Enrolment{Repo: repo, Keys: keys, Now: time.Now}
}

// SetClock replaces the time source. Tests only.
func (s *Enrolment) SetClock(now func() time.Time) { s.Now = now }

// Lookup normalises a typed code and returns every hash it may be stored
// under, or false when nothing of it survives normalisation.
func (s *Enrolment) Lookup(typed string) (domain.JoinCodeLookup, bool) {
	normalized := domain.JoinCodes.Normalize(typed)
	if normalized == "" {
		return domain.JoinCodeLookup{}, false
	}
	return s.Keys.LookupHashes(normalized), true
}

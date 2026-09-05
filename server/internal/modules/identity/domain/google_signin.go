package domain

import (
	"errors"
	"fmt"
	classesdomain "quizzivy/internal/modules/classes/domain"
)

var (
	ErrAccountNotProvisioned = errors.New("account not provisioned")
	ErrGoogleUnavailable     = errors.New("google sign-in is not configured")
	ErrSelfEnrolNotAvailable = errors.New("join-code signup is not implemented yet")
)

// JoinCodeRejected is a join code that did not pass. It carries the outcome so
// the HTTP layer can reuse /join/preview's exact mapping: the same four codes,
// the same leak rules, decided in one place rather than two that drift.
type JoinCodeRejected struct {
	Outcome classesdomain.PreviewOutcome
}

func (e JoinCodeRejected) Error() string {
	return fmt.Sprintf("join code rejected (outcome %d)", e.Outcome)
}

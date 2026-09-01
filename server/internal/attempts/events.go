package attempts

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// FlushInput carries the events and exactly one credential.
//
// The two paths exist because navigator.sendBeacon cannot set headers, so the
// pagehide flush has nowhere to put an Authorization header and carries a
// token in its body instead (D-03).
type FlushInput struct {
	AttemptID string
	SessionID string
	Events    []Event

	// StudentID is set when the request arrived with a verified access token.
	StudentID string
	// BeaconToken is set on the sendBeacon path, and grants append-only access
	// to this one attempt's events. Nothing reads with it.
	BeaconToken string
}

// Flush appends a batch of client events.
//
// Deliberately NOT transactional beyond the single insert, and deliberately
// silent about how many rows landed. §10.6 makes this fire-and-forget: a failed
// event flush must never block answering or submitting, so there is nothing for
// a caller to do with a partial result.
//
// It also does not check whether the session is still the attempt's current
// one, unlike an answer write. A superseded tab's events are still true --
// they are attributed to its own session_id, and the timeline is the poorer for
// dropping the last thing a tab did before it lost. The contract agrees: this
// operation has no SESSION_SUPERSEDED.
func (s *Store) Flush(ctx context.Context, in FlushInput, now time.Time) error {
	var (
		studentID  string
		beaconHash []byte
		deadlineAt time.Time
		versionID  string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT student_id, beacon_token_hash, deadline_at, test_version_id
		  FROM app.attempts WHERE id = $1::uuid`, in.AttemptID).
		Scan(&studentID, &beaconHash, &deadlineAt, &versionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrForbidden
	}
	if err != nil {
		return fmt.Errorf("attempts: read attempt for flush: %w", err)
	}

	if err := authorizeFlush(in, studentID, beaconHash, deadlineAt, now); err != nil {
		return err
	}
	return insertEvents(ctx, s.pool, in.AttemptID, in.SessionID, in.Events, versionID)
}

// authorizeFlush accepts either credential and nothing else.
//
// The bearer path ignores the deadline: a `page_hide` arriving a moment after
// time runs out is exactly the event worth having. The beacon path does not,
// because the token was issued "valid until deadlineAt" and a bearer credential
// that outlives its stated life is one an attacker can sit on.
func authorizeFlush(in FlushInput, studentID string, beaconHash []byte, deadlineAt, now time.Time) error {
	switch {
	case in.StudentID != "":
		if in.StudentID != studentID {
			return ErrForbidden
		}
		return nil

	case in.BeaconToken != "":
		presented := sha256.Sum256([]byte(in.BeaconToken))
		// Constant time: this is a bearer secret, and a comparison that returns
		// early leaks how much of a guess was right, one byte at a time.
		if subtle.ConstantTimeCompare(presented[:], beaconHash) != 1 {
			return ErrForbidden
		}
		if now.After(deadlineAt) {
			return ErrBeaconExpired
		}
		return nil
	}
	return ErrForbidden
}

func (s *Service) Flush(ctx context.Context, in FlushInput) error {
	return s.store.Flush(ctx, in, s.now())
}

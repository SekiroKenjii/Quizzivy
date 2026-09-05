package repositories

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"time"

	"github.com/jackc/pgx/v5"
)

// Flush appends a batch of client events.
func (s *Postgres) Flush(ctx context.Context, in domain.FlushInput, now time.Time) error {
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
		return domain.ErrForbidden
	}
	if err != nil {
		return fmt.Errorf("attempts: read attempt for flush: %w", err)
	}

	if err := authorizeFlush(in, studentID, beaconHash, deadlineAt, now); err != nil {
		return err
	}
	return insertEvents(ctx, s.pool, in.AttemptID, in.SessionID, in.Events, versionID)
}

func authorizeFlush(in domain.FlushInput, studentID string, beaconHash []byte, deadlineAt, now time.Time) error {
	switch {
	case in.StudentID != "":
		if in.StudentID != studentID {
			return domain.ErrForbidden
		}
		return nil

	case in.BeaconToken != "":
		presented := sha256.Sum256([]byte(in.BeaconToken))

		if subtle.ConstantTimeCompare(presented[:], beaconHash) != 1 {
			return domain.ErrForbidden
		}
		if now.After(deadlineAt) {
			return domain.ErrBeaconExpired
		}
		return nil
	}
	return domain.ErrForbidden
}

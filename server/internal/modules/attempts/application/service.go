package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	store        domain.Repository
	now          func() time.Time
	newSessionID func() string
	newSeed      func() (int64, error)
	newBeacon    func() (string, []byte, error)
}

func NewService(store domain.Repository) *Service {
	return &Service{
		store:        store,
		now:          time.Now,
		newSessionID: func() string { return uuid.NewString() },
		newSeed:      newSeed,
		newBeacon:    newBeaconToken,
	}
}

// StartOrResume is §9's entry point: one call whether the student is starting
// fresh, reloading, or arriving on a second device.
func (s *Service) StartOrResume(ctx context.Context, assignmentID, studentID string) (domain.Session, error) {
	var err error
	for range 3 {
		var session domain.Session
		session, err = s.startOrResume(ctx, assignmentID, studentID)
		if !errors.Is(err, domain.ErrRaceLost) {
			return session, err
		}
	}
	return domain.Session{}, err
}

func (s *Service) startOrResume(ctx context.Context, assignmentID, studentID string) (domain.Session, error) {
	rules, err := s.store.Rules(ctx, assignmentID, studentID)
	if err != nil {
		return domain.Session{}, err
	}
	if !rules.Targeted {
		return domain.Session{}, domain.ErrForbidden
	}

	session, resumed, err := s.resumeIfLive(ctx, assignmentID, studentID, rules)
	if err != nil || resumed {
		return session, err
	}

	if err := s.canStart(rules); err != nil {
		return domain.Session{}, err
	}
	tally, err := s.store.Tally(ctx, assignmentID, studentID)
	if err != nil {
		return domain.Session{}, err
	}
	if tally.Spent >= rules.MaxAttempts {
		session, resumed, err := s.resumeIfLive(ctx, assignmentID, studentID, rules)
		if err != nil || resumed {
			return session, err
		}
		return domain.Session{}, domain.ErrLimitReached
	}
	return s.create(ctx, assignmentID, studentID, tally.Next, rules)
}

// Get is §7's rule that a student fetches test content through exactly one
// endpoint. It re-reads the paper without disturbing the session: a reload
// takes the attempt over, a refetch does not.
func (s *Service) Get(ctx context.Context, attemptID, studentID string) (domain.Session, error) {
	if err := s.store.ExpireIfDue(ctx, attemptID, s.now()); err != nil {
		return domain.Session{}, err
	}
	attempt, err := s.store.ByID(ctx, attemptID, studentID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Session{}, domain.ErrForbidden
		}
		return domain.Session{}, err
	}
	rules, err := s.store.RulesFor(ctx, attempt.AssignmentID)
	if err != nil {
		return domain.Session{}, err
	}

	beacon, hash, err := s.newBeacon()
	if err != nil {
		return domain.Session{}, err
	}
	if err := s.store.Rebeacon(ctx, attempt.ID, hash); err != nil {
		return domain.Session{}, err
	}
	return s.session(ctx, attempt, beacon, rules)
}

func (s *Service) resumeIfLive(ctx context.Context, assignmentID, studentID string, r domain.Rules) (domain.Session, bool, error) {
	live, err := s.store.Live(ctx, assignmentID, studentID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Session{}, false, nil
	}
	if err != nil {
		return domain.Session{}, false, err
	}

	if s.now().After(live.DeadlineAt) {
		if err := s.store.ExpireIfDue(ctx, live.ID, s.now()); err != nil {
			return domain.Session{}, false, err
		}
		return domain.Session{}, false, nil
	}

	session, err := s.resume(ctx, live, r)
	return session, err == nil, err
}

// canStart is the open-window check, and applies only to a NEW attempt.
func (s *Service) canStart(r domain.Rules) error {
	now := s.now()
	switch {
	case r.PublishedAt == nil:
		return domain.ErrNotFound
	case now.Before(r.OpensAt), !now.Before(r.ClosesAt):
		return domain.ErrAssignmentClosed
	case r.ClosedAt != nil && !now.Before(*r.ClosedAt):
		return domain.ErrAssignmentClosed
	}
	return nil
}

func (s *Service) create(ctx context.Context, assignmentID, studentID string, attemptNo int, r domain.Rules) (domain.Session, error) {
	seed, err := s.newSeed()
	if err != nil {
		return domain.Session{}, err
	}
	beacon, hash, err := s.newBeacon()
	if err != nil {
		return domain.Session{}, err
	}
	now := s.now()

	created, err := s.store.Create(ctx, domain.CreateInput{
		AssignmentID:  assignmentID,
		TestVersionID: r.TestVersionID,
		StudentID:     studentID,
		AttemptNo:     attemptNo,
		SessionID:     s.newSessionID(),
		Seed:          seed,
		BeaconHash:    hash,
		StartedAt:     now,
		DeadlineAt:    r.Deadline(now),
	})
	if err != nil {
		return domain.Session{}, err
	}
	return s.session(ctx, created, beacon, r)
}

func (s *Service) resume(ctx context.Context, live domain.AttemptRecord, r domain.Rules) (domain.Session, error) {
	beacon, hash, err := s.newBeacon()
	if err != nil {
		return domain.Session{}, err
	}
	updated, _, err := s.store.Resume(ctx, domain.ResumeInput{
		AttemptID:  live.ID,
		SessionID:  s.newSessionID(),
		BeaconHash: hash,
		Now:        s.now(),
	})
	if err != nil {
		return domain.Session{}, err
	}
	return s.session(ctx, updated, beacon, r)
}

// session builds the payload both the create and resume paths return, so there
// is exactly one definition of what a student may see.
func (s *Service) session(ctx context.Context, a domain.AttemptRecord, beacon string, r domain.Rules) (domain.Session, error) {
	questions, err := s.store.Questions(ctx, a.TestVersionID)
	if err != nil {
		return domain.Session{}, err
	}
	answers, err := s.store.Answers(ctx, a.ID)
	if err != nil {
		return domain.Session{}, err
	}
	plays, err := s.store.AudioPlays(ctx, a.ID)
	if err != nil {
		return domain.Session{}, err
	}
	return domain.Session{
		Attempt:     a.Attempt,
		Questions:   domain.Deal.Present(a.Seed, r.ShuffleQuestions, r.ShuffleOptions, questions),
		SessionID:   a.SessionID,
		BeaconToken: beacon,
		ServerTime:  s.now(),
		// Server-authoritative (§11.4).
		AudioPlays: plays,
		Answers:    answers,
		Integrity:  r.Integrity,
	}, nil
}

// newSeed draws the Shuffle seed from a CSPRNG rather than the clock. A
// predictable seed is a predictable paper, and the answer order of a paper is
// worth guessing.
func newSeed() (int64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, fmt.Errorf("attempts: generate domain.Shuffle seed: %w", err)
	}
	return int64(binary.BigEndian.Uint64(b[:])), nil
}

// newBeaconToken returns the opaque token and its SHA-256 hash.
//
// [D-03] navigator.sendBeacon cannot set an Authorization header, and the
// 15-minute access token has normally expired by the pagehide of a 60-minute
// test. This is append-only event access, scoped to one attempt and one
// session; it grants no reads.
func newBeaconToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("attempts: generate beacon token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:], nil
}

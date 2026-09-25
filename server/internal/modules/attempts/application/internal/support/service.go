package support

import (
	"context"
	"errors"
	"quizzivy/internal/modules/attempts/application/ports"
	"quizzivy/internal/modules/attempts/domain"
	testsquery "quizzivy/internal/modules/tests/application/query"
	testsdomain "quizzivy/internal/modules/tests/domain"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Service carries what the service handlers share: their ports and the helpers they call.
type Service struct {
	Store        domain.Repository
	Groups       ports.GroupContexts
	Now          func() time.Time
	NewSessionID func() string
	NewSeed      func() (int64, error)
	NewBeacon    func() (string, []byte, error)
}

// ExpireDue closes every attempt on the assignment whose time has run out, so
// the monitor never shows "in progress" beside a deadline in the past.
func (s *Service) ExpireDue(ctx context.Context, assignmentID string) error {
	ids, err := s.Store.DueAttempts(ctx, assignmentID, s.Now())
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.Store.ExpireIfDue(ctx, id, s.Now()); err != nil {
			return err
		}
	}
	return nil
}

func NewService(store domain.Repository) *Service {
	return &Service{
		Store:        store,
		Now:          time.Now,
		NewSessionID: func() string { return uuid.NewString() },
		NewSeed:      NewSeed,
		NewBeacon:    NewBeaconToken,
	}
}

func (s *Service) StartOrResume(ctx context.Context, assignmentID, studentID string) (domain.Session, error) {
	rules, err := s.Store.Rules(ctx, assignmentID, studentID)
	if err != nil {
		return domain.Session{}, err
	}
	if !rules.Targeted {
		return domain.Session{}, domain.ErrForbidden
	}

	session, resumed, err := s.ResumeIfLive(ctx, assignmentID, studentID, rules)
	if err != nil || resumed {
		return session, err
	}

	if err := s.CanStart(rules); err != nil {
		return domain.Session{}, err
	}
	tally, err := s.Store.Tally(ctx, assignmentID, studentID)
	if err != nil {
		return domain.Session{}, err
	}
	if tally.Spent >= rules.MaxAttempts {
		session, resumed, err := s.ResumeIfLive(ctx, assignmentID, studentID, rules)
		if err != nil || resumed {
			return session, err
		}
		return domain.Session{}, domain.ErrLimitReached
	}
	return s.Create(ctx, assignmentID, studentID, tally.Next, rules)
}

func (s *Service) ResumeIfLive(ctx context.Context, assignmentID, studentID string, r domain.Rules) (domain.Session, bool, error) {
	live, err := s.Store.Live(ctx, assignmentID, studentID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Session{}, false, nil
	}
	if err != nil {
		return domain.Session{}, false, err
	}

	if s.Now().After(live.DeadlineAt) {
		if err := s.Store.ExpireIfDue(ctx, live.ID, s.Now()); err != nil {
			return domain.Session{}, false, err
		}
		return domain.Session{}, false, nil
	}

	session, err := s.Resume(ctx, live, r)
	return session, err == nil, err
}

func (s *Service) CanStart(r domain.Rules) error {
	now := s.Now()
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

func (s *Service) Create(ctx context.Context, assignmentID, studentID string, attemptNo int, r domain.Rules) (domain.Session, error) {
	seed, err := s.NewSeed()
	if err != nil {
		return domain.Session{}, err
	}
	beacon, hash, err := s.NewBeacon()
	if err != nil {
		return domain.Session{}, err
	}
	now := s.Now()

	created, err := s.Store.Create(ctx, domain.CreateInput{
		AssignmentID:  assignmentID,
		TestVersionID: r.TestVersionID,
		StudentID:     studentID,
		AttemptNo:     attemptNo,
		SessionID:     s.NewSessionID(),
		Seed:          seed,
		BeaconHash:    hash,
		StartedAt:     now,
		DeadlineAt:    r.Deadline(now),
	})
	if err != nil {
		return domain.Session{}, err
	}
	return s.Session(ctx, created, beacon, r)
}

func (s *Service) Resume(ctx context.Context, live domain.AttemptRecord, r domain.Rules) (domain.Session, error) {
	beacon, hash, err := s.NewBeacon()
	if err != nil {
		return domain.Session{}, err
	}
	updated, _, err := s.Store.Resume(ctx, domain.ResumeInput{
		AttemptID:  live.ID,
		SessionID:  s.NewSessionID(),
		BeaconHash: hash,
		Now:        s.Now(),
	})
	if err != nil {
		return domain.Session{}, err
	}
	return s.Session(ctx, updated, beacon, r)
}

func (s *Service) Session(ctx context.Context, a domain.AttemptRecord, beacon string, r domain.Rules) (domain.Session, error) {
	sections, err := s.Store.Sections(ctx, a.TestVersionID)
	if err != nil {
		return domain.Session{}, err
	}
	questions, err := s.Store.Questions(ctx, a.TestVersionID)
	if err != nil {
		return domain.Session{}, err
	}
	version, err := s.Store.DeliveryVersion(ctx, a.TestVersionID)
	if err != nil {
		return domain.Session{}, err
	}
	questions, err = domain.Deal.PresentVersion(version, a.Seed, r.ShuffleQuestions, r.ShuffleOptions, sections, questions)
	if err != nil {
		return domain.Session{}, err
	}
	groups := []testsdomain.PreviewGroup{}
	if slices.ContainsFunc(questions, func(q domain.Question) bool { return q.GroupID != "" }) {
		if s.Groups == nil {
			return domain.Session{}, domain.ErrGroupContextUnavailable
		}
		groups, err = s.Groups.Handle(ctx, testsquery.GroupContexts{VersionID: a.TestVersionID})
		if err != nil {
			return domain.Session{}, err
		}
	}
	answers, err := s.Store.Answers(ctx, a.ID)
	if err != nil {
		return domain.Session{}, err
	}
	plays, err := s.Store.AudioPlays(ctx, a.ID)
	if err != nil {
		return domain.Session{}, err
	}
	groupPlays := map[string]int{}
	if len(groups) > 0 {
		groupPlays, err = s.Store.GroupAudioPlays(ctx, a.ID)
		if err != nil {
			return domain.Session{}, err
		}
	}
	tally, err := s.Store.Tally(ctx, a.AssignmentID, a.StudentID)
	if err != nil {
		return domain.Session{}, err
	}
	return domain.Session{
		RemainingAttempts: max(0, r.MaxAttempts-tally.Spent),
		Attempt:           a.Attempt,
		TestTitle:         r.TestTitle,
		Sections:          sections,
		Questions:         questions,
		Groups:            groups,
		SessionID:         a.SessionID,
		BeaconToken:       beacon,
		ServerTime:        s.Now(),

		AudioPlays:      plays,
		GroupAudioPlays: groupPlays,
		Answers:         answers,
		Integrity:       r.Integrity,
	}, nil
}

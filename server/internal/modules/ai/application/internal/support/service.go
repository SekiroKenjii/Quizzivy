package support

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/domain"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

// Service carries what the handlers share: the store, the tiers' prices, the calendar zone months are counted in, the clock, the way to tell a user, and the ports to the provider and to what the operations read. A nil Completer is an assistant that is off; a nil read port is one its operation cannot be asked of.
type Service struct {
	Repo      domain.Repository
	Prices    domain.Prices
	Zone      *time.Location
	Now       func() time.Time
	Notifier  ports.Notifier
	Logger    *slog.Logger
	Completer ports.Completer
	Tests     ports.Tests
	Questions ports.Questions
	Grading   ports.Grading
	Analysis  ports.Analysis
	People    ports.People
}

// NewService returns a Service whose clock is the system's and whose logger discards.
func NewService(repo domain.Repository, prices domain.Prices, zone *time.Location) *Service {
	return &Service{Repo: repo, Prices: prices, Zone: zone, Now: time.Now, Logger: slog.New(slog.DiscardHandler)}
}

// Month is the month it is now, as the key of the month's rows.
func (s *Service) Month() time.Time {
	return domain.MonthOf(s.Now(), s.Zone)
}

// TellWarning tells the teacher that they have used the share of their allowance the centre warns at. It runs after the settlement committed and never fails it.
func (s *Service) TellWarning(ctx context.Context, w domain.Warning) {
	cqrs.Announce(ctx, s.Notifier, s.Logger, "ai allowance warning", func(context.Context) ([]notificationscommand.Notify, error) {
		return []notificationscommand.Notify{{
			UserID:    w.UserID,
			Kind:      notificationsdomain.AIAllowanceWarning,
			Params:    notificationsdomain.AllowanceWarning{Used: int(w.Used), Allowance: int(w.Allowance)},
			DedupeKey: notificationsdomain.AllowanceWarningKey(w.Month.Format(time.DateOnly)),
			Merge:     notificationsdomain.Replace,
		}}, nil
	})
}

// TellManagers tells every user who manages credits that a teacher asked for more.
func (s *Service) TellManagers(ctx context.Context, filed domain.Filed) {
	cqrs.Announce(ctx, s.Notifier, s.Logger, "ai credit request", func(context.Context) ([]notificationscommand.Notify, error) {
		out := make([]notificationscommand.Notify, len(filed.Managers))
		for i, id := range filed.Managers {
			out[i] = notificationscommand.Notify{
				UserID:    id,
				Kind:      notificationsdomain.AICreditRequest,
				Params:    notificationsdomain.CreditRequest{TeacherName: filed.TeacherName, Credits: int(filed.Request.Amount)},
				DedupeKey: notificationsdomain.CreditRequestKey(filed.Request.ID),
				Merge:     notificationsdomain.Replace,
			}
		}
		return out, nil
	})
}

// TellDecision tells the teacher how their request was decided.
func (s *Service) TellDecision(ctx context.Context, decided domain.Decided) {
	cqrs.Announce(ctx, s.Notifier, s.Logger, "ai credit decision", func(context.Context) ([]notificationscommand.Notify, error) {
		decision := notificationsdomain.DecisionDeclined
		if decided.Request.Status == domain.RequestApproved {
			decision = notificationsdomain.DecisionApproved
		}
		return []notificationscommand.Notify{{
			UserID:    decided.Request.UserID,
			Kind:      notificationsdomain.AICreditRequestDecided,
			Params:    notificationsdomain.CreditRequestDecided{Credits: int(decided.Request.Amount), Decision: decision},
			DedupeKey: notificationsdomain.CreditDecisionKey(decided.Request.ID),
			Merge:     notificationsdomain.Replace,
		}}, nil
	})
}

// TokensOf converts what the provider reported into what the ledger records.
func TokensOf(u ports.Usage) domain.Tokens {
	return domain.Tokens{Input: u.InputTokens, Output: u.OutputTokens, CacheRead: u.CacheReadTokens, CacheWrite: u.CacheWriteTokens}
}

// MayActOn applies the subset rule to a command that acts on one teacher: the teacher's permissions, except learning.take_tests, must be a subset of the author's (access.CanActOn). A teacher who does not exist is domain.ErrNotATeacher and one the author may not act on is domain.ErrForbidden. System is the maintenance console, which has no permissions to compare and is run with the operator's own database credential, so it is not asked.
func (s *Service) MayActOn(ctx context.Context, by domain.Author, teacherID string) error {
	if by.System() {
		return nil
	}
	if s.People == nil {
		return errors.New("ai: no way to read a teacher's permissions")
	}
	target, err := s.People.Permissions(ctx, teacherID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ErrNotATeacher
	}
	if err != nil {
		return err
	}
	if !access.CanActOn(by.Grants, target) {
		return domain.ErrForbidden
	}
	return nil
}

// LanguageFor is the language to write in: the one the request named, or else the teacher's preference, or Vietnamese when that cannot be read.
func (s *Service) LanguageFor(ctx context.Context, userID string, requested domain.Language) domain.Language {
	if requested.Valid() {
		return requested
	}
	if s.People == nil {
		return domain.Vietnamese
	}
	prefs, err := s.People.Preferences(ctx, userID)
	if err != nil || !prefs.Language.Valid() {
		return domain.Vietnamese
	}
	return prefs.Language
}

// MayReadStudentAnswers is nil when the teacher lets the assistant read student answers and domain.ErrReadOff when they turned it off. A preference that cannot be read refuses, so an answer is never sent on a guess.
func (s *Service) MayReadStudentAnswers(ctx context.Context, userID string) error {
	if s.People == nil {
		return errors.New("ai: no way to read a teacher's preferences")
	}
	prefs, err := s.People.Preferences(ctx, userID)
	if err != nil {
		return err
	}
	if !prefs.ReadStudentAnswers {
		return domain.ErrReadOff
	}
	return nil
}

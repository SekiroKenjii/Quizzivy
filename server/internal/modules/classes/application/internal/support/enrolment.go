package support

import (
	"context"
	"log/slog"
	"quizzivy/internal/modules/classes/application/ports"
	"quizzivy/internal/modules/classes/domain"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/cqrs"
	"time"
)

// Enrolment carries what the enrolment handlers share: their ports and the helpers they call.
type Enrolment struct {
	Repo     domain.Repository
	Keys     domain.JoinCodeKeys
	Now      func() time.Time
	Notifier ports.Notifier
	Logger   *slog.Logger
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

// Joined tells the class's teacher that a student joined it by code, once the
// enrolment has committed. A refused code and a student who was already a
// member tell nobody, and a failure to tell is logged and never returned.
func (s *Enrolment) Joined(ctx context.Context, result domain.EnrolResult) {
	if result.Outcome != domain.PreviewOK || result.AlreadyMember {
		return
	}
	cqrs.Announce(ctx, s.Notifier, s.Logger, "class joined", func(context.Context) ([]notificationscommand.Notify, error) {
		return []notificationscommand.Notify{{
			UserID:    result.TeacherID,
			Kind:      notificationsdomain.ClassJoined,
			Params:    notificationsdomain.Joined{StudentName: result.StudentName, ClassName: result.Class.Name},
			Target:    &notificationsdomain.Target{Route: notificationsdomain.RouteClasses},
			DedupeKey: notificationsdomain.JoinedKey(result.Class.ID, result.UserID),
			Merge:     notificationsdomain.Replace,
		}}, nil
	})
}

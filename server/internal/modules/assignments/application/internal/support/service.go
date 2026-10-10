package support

import (
	"context"
	"log/slog"

	"quizzivy/internal/modules/assignments/application/ports"
	"quizzivy/internal/modules/assignments/domain"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/cqrs"
)

// Service carries what the service handlers share: their ports and the helpers they call.
type Service struct {
	Repo     domain.Repository
	Notifier ports.Notifier
	Logger   *slog.Logger
}

func NewService(repo domain.Repository) *Service {
	return &Service{Repo: repo}
}

// TellMoved tells the students whose close an extension of the assignment
// moved what it now is, after the extension has committed. A failure is
// logged and never returned.
func (s *Service) TellMoved(ctx context.Context, assignmentID string) {
	cqrs.Announce(ctx, s.Notifier, s.Logger, "assignment extended", func(ctx context.Context) ([]notificationscommand.Notify, error) {
		moved, err := s.Repo.ClosesMoved(ctx, assignmentID)
		return extended(assignmentID, moved), err
	})
}

// TellGranted tells those of studentIDs whom an override gave time past the
// assignment's own close what their close now is, after the override has
// committed. A failure is logged and never returned.
func (s *Service) TellGranted(ctx context.Context, assignmentID string, studentIDs []string) {
	cqrs.Announce(ctx, s.Notifier, s.Logger, "assignment override", func(ctx context.Context) ([]notificationscommand.Notify, error) {
		granted, err := s.Repo.ClosesGranted(ctx, assignmentID, studentIDs)
		return extended(assignmentID, granted), err
	})
}

func extended(assignmentID string, e domain.Extension) []notificationscommand.Notify {
	out := make([]notificationscommand.Notify, len(e.Students))
	for i, student := range e.Students {
		out[i] = notificationscommand.Notify{
			UserID:    student.StudentID,
			Kind:      notificationsdomain.AssignmentExtended,
			Params:    notificationsdomain.Extended{Title: e.Title, ClosesAt: student.ClosesAt},
			Target:    &notificationsdomain.Target{Route: notificationsdomain.RouteStudentAssignment, AssignmentID: assignmentID},
			DedupeKey: notificationsdomain.ExtendedKey(assignmentID),
			Merge:     notificationsdomain.Replace,
		}
	}
	return out
}

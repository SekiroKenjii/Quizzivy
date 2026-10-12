package support

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"quizzivy/internal/modules/attempts/application/ports"
	"quizzivy/internal/modules/attempts/domain"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/cqrs"
)

// Announcer tells the people a write to an attempt concerns what it brought
// about, after the write has committed: the teachers who reach the paper when
// it is handed in or flagged by the integrity policy, and the student when a
// teacher declares it graded and the review policy shows the score. A nil
// Announcer, and one without a Notifier, tell nobody.
type Announcer struct {
	Notifier ports.Notifier
	Briefs   domain.Briefings
	Logger   *slog.Logger
	Now      func() time.Time
}

// Announce delivers the notifications m calls for, under the budget
// cqrs.Announce gives it. Nothing it does fails the caller.
func (a *Announcer) Announce(ctx context.Context, m domain.Milestones) {
	if a == nil || !m.Any() {
		return
	}
	cqrs.Announce(ctx, a.Notifier, a.Logger, "attempt", func(ctx context.Context) ([]notificationscommand.Notify, error) {
		now := a.Now()
		brief, err := a.Briefs.Briefing(ctx, m.AttemptID, now)
		if errors.Is(err, domain.ErrNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return notices(m, brief, now), nil
	})
}

func notices(m domain.Milestones, brief domain.Briefing, now time.Time) []notificationscommand.Notify {
	var out []notificationscommand.Notify
	for _, reader := range brief.Readers {
		if m.HandedIn {
			out = append(out, submitted(reader, brief, now))
		}
		if m.Flagged {
			out = append(out, flagged(reader, m, brief))
		}
	}
	if m.Graded && brief.ShowsResult {
		out = append(out, ready(m, brief))
	}
	return out
}

func submitted(reader string, brief domain.Briefing, now time.Time) notificationscommand.Notify {
	toGrade := 0
	if brief.ToGrade {
		toGrade = 1
	}
	return notificationscommand.Notify{
		UserID:    reader,
		Kind:      notificationsdomain.AttemptSubmitted,
		Params:    notificationsdomain.Submitted{Title: brief.Title, Count: 1, ToGrade: toGrade},
		Target:    &notificationsdomain.Target{Route: notificationsdomain.RouteAssignment, AssignmentID: brief.AssignmentID},
		DedupeKey: notificationsdomain.SubmittedKey(brief.AssignmentID, now),
		Merge:     notificationsdomain.Add,
	}
}

func flagged(reader string, m domain.Milestones, brief domain.Briefing) notificationscommand.Notify {
	return notificationscommand.Notify{
		UserID:    reader,
		Kind:      notificationsdomain.AttemptFlagged,
		Params:    notificationsdomain.Flagged{StudentName: brief.StudentName, Title: brief.Title, FocusLost: brief.FocusLost},
		Target:    &notificationsdomain.Target{Route: notificationsdomain.RouteAttempt, AttemptID: m.AttemptID, AssignmentID: brief.AssignmentID},
		DedupeKey: notificationsdomain.FlaggedKey(m.AttemptID),
		Merge:     notificationsdomain.Replace,
	}
}

func ready(m domain.Milestones, brief domain.Briefing) notificationscommand.Notify {
	return notificationscommand.Notify{
		UserID:    brief.StudentID,
		Kind:      notificationsdomain.ResultReady,
		Params:    notificationsdomain.Ready{Title: brief.Title},
		Target:    &notificationsdomain.Target{Route: notificationsdomain.RouteResult, AttemptID: m.AttemptID, AssignmentID: brief.AssignmentID},
		DedupeKey: notificationsdomain.ReadyKey(m.AttemptID),
		Merge:     notificationsdomain.Replace,
	}
}

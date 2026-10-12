package command

import (
	"context"

	"quizzivy/internal/modules/notifications/application/internal/support"
	"quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/cqrs"
)

// Notify tells UserID that something happened. A second Notify with the same
// UserID and DedupeKey merges into the first as Merge says and makes it
// unread again. It is refused, with the domain's error, when a field is
// missing or out of bounds, and it writes nothing when the user switched off
// the event that governs Kind. The write is one statement: the handler never
// reads the stored notification, so concurrent calls lose no increment.
type Notify struct {
	UserID    string
	Kind      domain.Kind
	Params    domain.Params
	Target    *domain.Target
	DedupeKey string
	Merge     domain.Merge
}

type NotifyHandler struct {
	*support.Service
}

func (s NotifyHandler) Handle(ctx context.Context, cmd Notify) (cqrs.Nothing, error) {
	notice := domain.Notice{
		UserID: cmd.UserID, Kind: cmd.Kind, Params: cmd.Params, Target: cmd.Target, DedupeKey: cmd.DedupeKey, Merge: cmd.Merge,
	}
	if err := notice.Validate(); err != nil {
		return cqrs.Nothing{}, err
	}
	_, err := s.Repo.Upsert(ctx, notice)
	return cqrs.Nothing{}, err
}

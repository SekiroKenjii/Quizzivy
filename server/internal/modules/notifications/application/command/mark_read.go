package command

import (
	"context"

	"quizzivy/internal/modules/notifications/application/internal/support"
	"quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/cqrs"
)

// MarkRead marks those of IDs that are UserID's own and unread. An id that is
// another user's, already read or unknown is skipped without a word, so the
// outcome says nothing about an id that is not the user's. No id at all, and
// more than domain.MaxMarkedIDs of them, are refused.
type MarkRead struct {
	UserID string
	IDs    []string
}

type MarkReadHandler struct {
	*support.Service
}

func (s MarkReadHandler) Handle(ctx context.Context, cmd MarkRead) (cqrs.Nothing, error) {
	switch {
	case cmd.UserID == "":
		return cqrs.Nothing{}, domain.ErrNoRecipient
	case len(cmd.IDs) == 0:
		return cqrs.Nothing{}, domain.ErrNoIDs
	case len(cmd.IDs) > domain.MaxMarkedIDs:
		return cqrs.Nothing{}, domain.ErrTooManyIDs
	}
	return cqrs.Nothing{}, s.Repo.MarkRead(ctx, cmd.UserID, cmd.IDs)
}

// MarkAllRead marks every unread notification UserID has.
type MarkAllRead struct {
	UserID string
}

type MarkAllReadHandler struct {
	*support.Service
}

func (s MarkAllReadHandler) Handle(ctx context.Context, cmd MarkAllRead) (cqrs.Nothing, error) {
	if cmd.UserID == "" {
		return cqrs.Nothing{}, domain.ErrNoRecipient
	}
	return cqrs.Nothing{}, s.Repo.MarkAllRead(ctx, cmd.UserID)
}

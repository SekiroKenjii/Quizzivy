package command

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/opt"
)

type Archive struct {
	ClassID   string
	Archived  bool
	ActorID   string
	IP        string
	UserAgent string
}

type ArchiveHandler struct {
	*support.Service
}

func (s ArchiveHandler) Handle(ctx context.Context, cmd Archive) (domain.Class, error) {
	return s.Repo.Archive(ctx, domain.ArchiveInput{
		ClassID: cmd.ClassID, Archived: cmd.Archived, ActorUserID: cmd.ActorID,
		Now: s.Now(), IP: opt.String(cmd.IP), UserAgent: opt.String(cmd.UserAgent),
	})
}

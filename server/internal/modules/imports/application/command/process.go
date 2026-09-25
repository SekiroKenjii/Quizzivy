package command

import (
	"context"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/actor"
)

const maxAttempts = 3

type Process struct {
	ImportID, RequestID string
	ExpectedRevision    int64
	KeyPaper            int
	Actor               actor.Actor
}

type ProcessHandler struct {
	Repo domain.Repository
	Runs ports.Runs
}

func (h ProcessHandler) Handle(ctx context.Context, in Process) (domain.Import, error) {
	current, err := h.Repo.Get(ctx, in.ImportID)
	if err != nil {
		return domain.Import{}, err
	}
	schedule := domain.Schedule{ImportID: in.ImportID, RequestID: in.RequestID, PipelineVersion: worker.PipelineVersion, ExpectedRevision: in.ExpectedRevision, SourceRevision: current.SourceRevision, Actor: in.Actor, MaxAttempts: maxAttempts, Profile: domain.RecognitionProfile{KeyPaper: in.KeyPaper}}
	if _, err := h.Runs.Schedule(ctx, schedule); err != nil {
		return domain.Import{}, err
	}
	return h.Repo.Get(ctx, in.ImportID)
}

type Cancel = domain.Cancel

type CancelHandler struct{ Runs ports.Runs }

func (h CancelHandler) Handle(ctx context.Context, in Cancel) (domain.Import, error) {
	return h.Runs.Cancel(ctx, in)
}

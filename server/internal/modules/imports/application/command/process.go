package command

import (
	"context"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/cqrs"
)

const maxAttempts = 3

type Process struct {
	ImportID, RequestID string
	ExpectedRevision    int64
	KeyPaper            int
	Actor               actor.Actor
}

// ProcessHandler schedules a run of the current source set and wakes the
// worker. Unless Enabled, it refuses with domain.ErrProcessingOff before
// touching the import.
type ProcessHandler struct {
	Repo    domain.Repository
	Runs    ports.Runs
	Worker  ports.WorkerSignal
	Enabled bool
}

func (h ProcessHandler) Handle(ctx context.Context, in Process) (domain.Import, error) {
	if !h.Enabled {
		return domain.Import{}, domain.ErrProcessingOff
	}
	current, err := h.Repo.Get(ctx, in.ImportID)
	if err != nil {
		return domain.Import{}, err
	}
	schedule := domain.Schedule{ImportID: in.ImportID, RequestID: in.RequestID, PipelineVersion: worker.PipelineVersion, ExpectedRevision: in.ExpectedRevision, SourceRevision: current.SourceRevision, Actor: in.Actor, MaxAttempts: maxAttempts, Profile: domain.RecognitionProfile{KeyPaper: in.KeyPaper}}
	if _, err := h.Runs.Schedule(ctx, schedule); err != nil {
		return domain.Import{}, err
	}
	if h.Worker != nil {
		h.Worker.Wake()
	}
	return h.Repo.Get(ctx, in.ImportID)
}

type Nudge struct{}

// NudgeHandler re-sends the worker its wake signal. A screen showing a queued
// import runs it, so a lost signal heals while a teacher watches the import wait.
type NudgeHandler struct{ Worker ports.WorkerSignal }

func (h NudgeHandler) Handle(context.Context, Nudge) (cqrs.Nothing, error) {
	if h.Worker != nil {
		h.Worker.Wake()
	}
	return cqrs.Nothing{}, nil
}

type Cancel = domain.Cancel

type CancelHandler struct{ Runs ports.Runs }

func (h CancelHandler) Handle(ctx context.Context, in Cancel) (domain.Import, error) {
	return h.Runs.Cancel(ctx, in)
}

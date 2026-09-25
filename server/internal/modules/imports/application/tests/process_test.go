package application_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/query"
	"quizzivy/internal/modules/imports/domain"
)

type processRepo struct {
	domain.Repository
	reads  int
	status string
}

func (r *processRepo) Get(context.Context, string) (domain.Import, error) {
	r.reads++
	status := r.status
	if status == "" {
		status = "queued"
	}
	return domain.Import{ID: "import-1", SourceRevision: 3, Status: status}, nil
}

type scheduler struct {
	ports.Runs
	scheduled []domain.Schedule
}

func (s *scheduler) Schedule(_ context.Context, in domain.Schedule) (domain.Run, error) {
	s.scheduled = append(s.scheduled, in)
	return domain.Run{}, nil
}

type signal struct{ wakes int }

func (s *signal) Wake() { s.wakes++ }

func processWith(enabled bool) (*application.Application, *processRepo, *scheduler, *signal) {
	repo, runs, worker := &processRepo{}, &scheduler{}, &signal{}
	app := application.New(application.Dependencies{Repo: repo, Runs: runs, Worker: worker, Processing: enabled})
	return app, repo, runs, worker
}

func TestProcessingIsRefusedWithoutQueueingWhenNoWorkerIsDeployed(t *testing.T) {
	app, repo, runs, worker := processWith(false)
	_, err := app.Commands.Process.Handle(context.Background(), command.Process{ImportID: "import-1", RequestID: "request-1", ExpectedRevision: 2})
	if !errors.Is(err, domain.ErrProcessingOff) {
		t.Fatalf("err = %v, want ErrProcessingOff", err)
	}
	if repo.reads != 0 || len(runs.scheduled) != 0 || worker.wakes != 0 {
		t.Fatalf("a refused process touched the import: %d reads, %d runs, %d wakes", repo.reads, len(runs.scheduled), worker.wakes)
	}
}

func TestProcessingQueuesTheCurrentSourceSetWhenAWorkerIsDeployed(t *testing.T) {
	app, _, runs, worker := processWith(true)
	got, err := app.Commands.Process.Handle(context.Background(), command.Process{ImportID: "import-1", RequestID: "request-1", ExpectedRevision: 2, KeyPaper: 4})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "queued" || len(runs.scheduled) != 1 || worker.wakes != 1 {
		t.Fatalf("status %q, %d runs scheduled, %d wakes", got.Status, len(runs.scheduled), worker.wakes)
	}
	if s := runs.scheduled[0]; s.SourceRevision != 3 || s.ExpectedRevision != 2 || s.Profile.KeyPaper != 4 {
		t.Fatalf("scheduled %+v", s)
	}
}

func TestCapabilitiesReportTheProcessingSwitch(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		app, _, _, _ := processWith(enabled)
		got, err := app.Queries.Capabilities.Handle(context.Background(), query.Capabilities{})
		if err != nil || got.Processing != enabled {
			t.Fatalf("processing %v reported as %+v (%v)", enabled, got, err)
		}
	}
}

func TestANudgeWakesTheWorkerAndReadingDoesNot(t *testing.T) {
	app, _, _, worker := processWith(true)
	if _, err := app.Queries.Get.Handle(context.Background(), query.Get{ID: "import-1"}); err != nil || worker.wakes != 0 {
		t.Fatalf("a query woke the worker: %d wakes, %v", worker.wakes, err)
	}
	if _, err := app.Commands.Nudge.Handle(context.Background(), command.Nudge{}); err != nil || worker.wakes != 1 {
		t.Fatalf("a nudge sent %d wakes: %v", worker.wakes, err)
	}
}

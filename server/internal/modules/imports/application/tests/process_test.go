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
	reads int
}

func (r *processRepo) Get(context.Context, string) (domain.Import, error) {
	r.reads++
	return domain.Import{ID: "import-1", SourceRevision: 3, Status: "queued"}, nil
}

type scheduler struct {
	ports.Runs
	scheduled []domain.Schedule
}

func (s *scheduler) Schedule(_ context.Context, in domain.Schedule) (domain.Run, error) {
	s.scheduled = append(s.scheduled, in)
	return domain.Run{}, nil
}

func processWith(enabled bool) (*application.Application, *processRepo, *scheduler) {
	repo, runs := &processRepo{}, &scheduler{}
	app := application.New(application.Dependencies{Repo: repo, Runs: runs, Processing: enabled})
	return app, repo, runs
}

func TestProcessingIsRefusedWithoutQueueingWhenNoWorkerIsDeployed(t *testing.T) {
	app, repo, runs := processWith(false)
	_, err := app.Commands.Process.Handle(context.Background(), command.Process{ImportID: "import-1", RequestID: "request-1", ExpectedRevision: 2})
	if !errors.Is(err, domain.ErrProcessingOff) {
		t.Fatalf("err = %v, want ErrProcessingOff", err)
	}
	if repo.reads != 0 || len(runs.scheduled) != 0 {
		t.Fatalf("a refused process touched the import: %d reads, %d runs", repo.reads, len(runs.scheduled))
	}
}

func TestProcessingQueuesTheCurrentSourceSetWhenAWorkerIsDeployed(t *testing.T) {
	app, _, runs := processWith(true)
	got, err := app.Commands.Process.Handle(context.Background(), command.Process{ImportID: "import-1", RequestID: "request-1", ExpectedRevision: 2, KeyPaper: 4})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "queued" || len(runs.scheduled) != 1 {
		t.Fatalf("status %q, %d runs scheduled", got.Status, len(runs.scheduled))
	}
	if s := runs.scheduled[0]; s.SourceRevision != 3 || s.ExpectedRevision != 2 || s.Profile.KeyPaper != 4 {
		t.Fatalf("scheduled %+v", s)
	}
}

func TestCapabilitiesReportTheProcessingSwitch(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		app, _, _ := processWith(enabled)
		got, err := app.Queries.Capabilities.Handle(context.Background(), query.Capabilities{})
		if err != nil || got.Processing != enabled {
			t.Fatalf("processing %v reported as %+v (%v)", enabled, got, err)
		}
	}
}

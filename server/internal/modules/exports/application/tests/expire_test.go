package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"quizzivy/internal/modules/exports/application/command"
	"quizzivy/internal/modules/exports/domain"
)

func TestExpiringDeletesEachFileBeforeMarkingItsRow(t *testing.T) {
	app, r, s, _ := build(t)
	r.lapsed = []domain.Lapse{{ID: "a", StorageKey: "exports/a.zip"}, {ID: "b", StorageKey: "exports/b.zip"}}

	out, err := app.Commands.Expire.Handle(context.Background(), command.Expire{Limit: 50})
	if err != nil || out.Marked != 2 {
		t.Fatalf("marked %d (%v), want 2", out.Marked, err)
	}
	if r.lapsedLim != 50 {
		t.Errorf("the repository was asked for %d, want the limit, 50", r.lapsedLim)
	}
	if !slices.Equal(s.deleted, []string{"exports/a.zip", "exports/b.zip"}) || !slices.Equal(r.expired, []string{"a", "b"}) {
		t.Errorf("deleted %v and marked %v, want both files and both rows", s.deleted, r.expired)
	}
}

func TestAFileThatCannotBeDeletedKeepsItsRowReadyForTheNextRun(t *testing.T) {
	app, r, s, _ := build(t)
	boom := errors.New("the bucket is away")
	r.lapsed = []domain.Lapse{{ID: "a", StorageKey: "exports/a.zip"}, {ID: "b", StorageKey: "exports/b.zip"}}
	s.deleteEr = map[string]error{"exports/a.zip": boom}

	out, err := app.Commands.Expire.Handle(context.Background(), command.Expire{Limit: 10})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the bucket's", err)
	}
	if out.Marked != 1 || !slices.Equal(r.expired, []string{"b"}) {
		t.Errorf("marked %d rows %v, want only the export whose file went", out.Marked, r.expired)
	}
}

func TestAFailedMarkIsReportedAndTheRestOfTheBatchStillRuns(t *testing.T) {
	app, r, _, _ := build(t)
	boom := errors.New("the database is away")
	r.lapsed = []domain.Lapse{{ID: "a", StorageKey: "exports/a.zip"}, {ID: "b", StorageKey: "exports/b.zip"}}
	r.expireEr = map[string]error{"a": boom}

	out, err := app.Commands.Expire.Handle(context.Background(), command.Expire{Limit: 10})
	if !errors.Is(err, boom) || out.Marked != 1 {
		t.Errorf("marked %d (%v), want 1 and the database's error", out.Marked, err)
	}
}

func TestARowAnotherMachineExpiredFirstIsNotCounted(t *testing.T) {
	app, r, _, _ := build(t)
	r.lapsed = []domain.Lapse{{ID: "a", StorageKey: "exports/a.zip"}}
	r.gone = map[string]bool{"a": true}

	out, err := app.Commands.Expire.Handle(context.Background(), command.Expire{Limit: 10})
	if err != nil || out.Marked != 0 {
		t.Errorf("marked %d (%v), want 0 for a row that was no longer ready", out.Marked, err)
	}
}

func TestAnUnreadableListIsTheErrorAndDeletesNothing(t *testing.T) {
	app, r, s, _ := build(t)
	boom := errors.New("the database is away")
	r.lapsedEr = boom
	if _, err := app.Commands.Expire.Handle(context.Background(), command.Expire{Limit: 10}); !errors.Is(err, boom) || len(s.deleted) != 0 {
		t.Errorf("err = %v, deleted %v", err, s.deleted)
	}
}

func TestExpiringNeedsAPositiveLimit(t *testing.T) {
	app, r, _, _ := build(t)
	if _, err := app.Commands.Expire.Handle(context.Background(), command.Expire{}); err == nil || r.lapsedLim != 0 {
		t.Errorf("a limit of 0 was accepted (%v)", err)
	}
}

package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
)

var importPruneNow = time.Date(2026, 11, 15, 4, 0, 0, 123456789, time.UTC)

type importRetention struct {
	pruned int
	err    error
	calls  []importPrune
}

type importPrune struct {
	before time.Time
	limit  int
}

func (r *importRetention) Prune(_ context.Context, before time.Time, limit int) (int, error) {
	r.calls = append(r.calls, importPrune{before, limit})
	return r.pruned, r.err
}

func importPruneApp(t *testing.T, store *importRetention) *application.Application {
	t.Helper()
	app := application.New(nil, nil, 0, nil, nil)
	app.SetUserImportRetention(store)
	app.SetClock(func() time.Time { return importPruneNow })
	return app
}

func TestUserImportsAreKeptThirtyDaysAfterTheyFinish(t *testing.T) {
	if got := domain.UserImportsKept; got != 30*24*time.Hour {
		t.Fatalf("UserImportsKept = %v, want 30 days", got)
	}
}

func TestPruningImportsAsksForThoseThatFinishedMoreThanThirtyDaysAgo(t *testing.T) {
	store := &importRetention{pruned: 4}
	app := importPruneApp(t, store)

	got, err := app.Commands.PruneUserImports.Handle(context.Background(), command.PruneUserImports{Limit: 500})

	if err != nil || got != 4 {
		t.Fatalf("pruned = %d, %v, want 4 and no error", got, err)
	}
	if len(store.calls) != 1 {
		t.Fatalf("%d calls to the store, want 1", len(store.calls))
	}
	if want := importPruneNow.Add(-30 * 24 * time.Hour); !store.calls[0].before.Equal(want) || store.calls[0].limit != 500 {
		t.Errorf("store asked to prune %+v, want before %v and limit 500", store.calls[0], want)
	}
}

func TestPruningImportsNeedsAPositiveLimit(t *testing.T) {
	store := &importRetention{}
	app := importPruneApp(t, store)
	for _, limit := range []int{0, -1} {
		if _, err := app.Commands.PruneUserImports.Handle(context.Background(), command.PruneUserImports{Limit: limit}); err == nil {
			t.Errorf("limit %d accepted", limit)
		}
	}
	if len(store.calls) != 0 {
		t.Errorf("the store was asked %d times with no usable limit", len(store.calls))
	}
}

func TestAFailedPruneOfImportsReturnsTheStoresError(t *testing.T) {
	store := &importRetention{err: errors.New("store down")}
	app := importPruneApp(t, store)

	if _, err := app.Commands.PruneUserImports.Handle(context.Background(), command.PruneUserImports{Limit: 10}); !errors.Is(err, store.err) {
		t.Fatalf("err = %v, want the store's", err)
	}
}

func TestWithoutTheRetentionStoreNothingPrunesImports(t *testing.T) {
	bare := application.New(nil, nil, 0, nil, nil)

	if bare.Commands.PruneUserImports != nil {
		t.Error("the prune is available with no retention store")
	}
}

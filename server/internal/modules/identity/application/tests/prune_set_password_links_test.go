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

func TestSetPasswordLinksAreKeptThirtyDaysAfterTheyEnd(t *testing.T) {
	if got := domain.SetPasswordLinksKept; got != 30*24*time.Hour {
		t.Fatalf("SetPasswordLinksKept = %v, want 30 days", got)
	}
}

func TestPruningLinksAsksForThoseThatEndedMoreThanThirtyDaysAgo(t *testing.T) {
	w := newLinkWorld(t)
	w.links.pruned = 7

	got, err := w.app.Commands.PruneSetPasswordLinks.Handle(context.Background(), command.PruneSetPasswordLinks{Limit: 500})

	if err != nil || got != 7 {
		t.Fatalf("pruned = %d, %v, want 7 and no error", got, err)
	}
	if len(w.links.prunes) != 1 {
		t.Fatalf("%d calls to the store, want 1", len(w.links.prunes))
	}
	call := w.links.prunes[0]
	if want := linkNow.Add(-30 * 24 * time.Hour); !call.before.Equal(want) || call.limit != 500 {
		t.Errorf("store asked to prune %+v, want before %v and limit 500", call, want)
	}
}

func TestPruningLinksNeedsAPositiveLimit(t *testing.T) {
	w := newLinkWorld(t)
	for _, limit := range []int{0, -1} {
		if _, err := w.app.Commands.PruneSetPasswordLinks.Handle(context.Background(), command.PruneSetPasswordLinks{Limit: limit}); err == nil {
			t.Errorf("limit %d accepted", limit)
		}
	}
	if len(w.links.prunes) != 0 {
		t.Errorf("the store was asked %d times with no usable limit", len(w.links.prunes))
	}
}

func TestAFailedPruneOfLinksReturnsTheStoresError(t *testing.T) {
	w := newLinkWorld(t)
	w.links.pruneErr = errors.New("store down")

	if _, err := w.app.Commands.PruneSetPasswordLinks.Handle(context.Background(), command.PruneSetPasswordLinks{Limit: 10}); !errors.Is(err, w.links.pruneErr) {
		t.Fatalf("err = %v, want the store's", err)
	}
}

func TestWithoutTheLinksStoreNothingPrunesThem(t *testing.T) {
	bare := application.New(nil, nil, 0, nil, nil)

	if bare.Commands.PruneSetPasswordLinks != nil {
		t.Error("the prune is available with no store of links")
	}
}

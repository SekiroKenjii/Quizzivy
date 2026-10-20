//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"quizzivy/internal/modules/exports/domain"
)

func TestLapsedListsReadyExportsPastTheirTimeOldestFirst(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	older := w.export(owner, seed{status: "ready", expiresIn: -48 * time.Hour})
	newer := w.export(owner, seed{status: "ready", expiresIn: -2 * time.Hour})
	live := w.export(owner, seed{status: "ready", expiresIn: time.Hour})
	failed := w.export(owner, seed{status: "failed", errorCode: domain.CodeFailed})
	gone := w.export(owner, seed{status: "ready", expiresIn: -72 * time.Hour})
	w.exec(`UPDATE app.data_exports SET status = 'expired', storage_key = NULL WHERE id = $1::uuid`, gone)

	lapsed, err := w.repo.Lapsed(context.Background(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	var mine []string
	for _, l := range lapsed {
		if slices.Contains([]string{older, newer, live, failed, gone}, l.ID) {
			mine = append(mine, l.ID)
			if l.StorageKey == "" {
				t.Errorf("the lapsed export %s has no key to delete", l.ID)
			}
		}
	}
	if !slices.Equal(mine, []string{older, newer}) {
		t.Errorf("lapsed = %v, want the two ready exports past their time, oldest first (%s, %s)", mine, older, newer)
	}
}

func TestLapsedTakesAtMostTheLimit(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	for range 3 {
		w.export(owner, seed{status: "ready", expiresIn: -time.Hour})
	}
	lapsed, err := w.repo.Lapsed(context.Background(), 2)
	if err != nil || len(lapsed) != 2 {
		t.Errorf("%d lapsed exports (%v), want the limit, 2", len(lapsed), err)
	}
}

func TestExpireMarksALapsedExportAndForgetsItsFileOnce(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	id := w.export(owner, seed{status: "ready", expiresIn: -time.Hour, downloads: 1})

	marked, err := w.repo.Expire(context.Background(), id)
	if err != nil || !marked {
		t.Fatalf("expire = %v (%v), want true", marked, err)
	}
	got := w.row(id)
	if got.status != "expired" || got.storageKey != nil || got.bytes == nil || got.expiresAt == nil || got.downloads != 1 {
		t.Errorf("row after expiring: %+v, want expired with no key and the rest kept", got)
	}
	if again, err := w.repo.Expire(context.Background(), id); err != nil || again {
		t.Errorf("a second expire = %v (%v), want false", again, err)
	}
	read, err := w.repo.Get(context.Background(), id, owner)
	if err != nil || read.Status != domain.Expired {
		t.Errorf("read status %q (%v), want expired", read.Status, err)
	}
	if _, err := w.repo.Fetch(context.Background(), fetch(id, owner), signer(new([]string))); !errors.Is(err, domain.ErrUnavailable) {
		t.Errorf("fetch of an expired export: %v, want ErrUnavailable", err)
	}
}

func TestExpireLeavesAnExportThatIsStillLiveOrNotReady(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	live := w.export(owner, seed{status: "ready", expiresIn: time.Hour})
	failed := w.export(owner, seed{status: "failed", errorCode: domain.CodeFailed})

	for name, id := range map[string]string{"live": live, "failed": failed} {
		before := w.row(id)
		marked, err := w.repo.Expire(context.Background(), id)
		if err != nil || marked {
			t.Errorf("%s: expire = %v (%v), want false", name, marked, err)
		}
		if after := w.row(id); after.status != before.status || (before.storageKey != nil && (after.storageKey == nil || *after.storageKey != *before.storageKey)) {
			t.Errorf("%s: row changed from %+v to %+v", name, before, after)
		}
	}
}

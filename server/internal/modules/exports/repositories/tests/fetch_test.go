//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/exports/domain"
)

func signer(urls *[]string) domain.Signer {
	return func(key string, created time.Time) (string, error) {
		url := "https://objects.example/" + key + "?created=" + created.UTC().Format("20060102")
		*urls = append(*urls, url)
		return url, nil
	}
}

func fetch(id, actor string) domain.Fetch {
	return domain.Fetch{ID: id, ActorID: actor, IP: "198.51.100.4", UserAgent: "Mozilla/5.0"}
}

func TestAReadyExportIsFetchedOnceAndTheFetchIsAudited(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	id := w.export(owner, seed{status: "ready", expiresIn: time.Hour})
	var urls []string

	got, err := w.repo.Fetch(context.Background(), fetch(id, owner), signer(&urls))
	if err != nil {
		t.Fatal(err)
	}
	stored := w.row(id)
	if len(urls) != 1 || got != urls[0] || stored.storageKey == nil || !strings.HasPrefix(got, "https://objects.example/"+*stored.storageKey+"?created=") {
		t.Errorf("url = %q for key %v", got, stored.storageKey)
	}
	if stored.downloadedAt == nil {
		t.Error("the export is not marked fetched")
	}
	if n := w.auditCount("data_export.downloaded", id); n != 1 {
		t.Errorf("%d audit rows for the fetch, want 1", n)
	}

	if _, err := w.repo.Fetch(context.Background(), fetch(id, owner), signer(&urls)); !errors.Is(err, domain.ErrUnavailable) {
		t.Errorf("the second fetch: %v", err)
	}
	if len(urls) != 1 {
		t.Error("an address was signed for the second fetch")
	}
	if n := w.auditCount("data_export.downloaded", id); n != 1 {
		t.Errorf("the refused fetch was audited: %d rows", n)
	}
}

func TestAnotherAdminsFetchIsNotFoundAndMarksNothing(t *testing.T) {
	w := newWorld(t)
	owner, other := w.admin(), w.admin()
	id := w.export(owner, seed{status: "ready", expiresIn: time.Hour})
	var urls []string
	if _, err := w.repo.Fetch(context.Background(), fetch(id, other), signer(&urls)); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
	if _, err := w.repo.Fetch(context.Background(), fetch("0198f000-0000-7000-8000-00000000dead", owner), signer(&urls)); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a missing id: %v", err)
	}
	if len(urls) != 0 || w.row(id).downloadedAt != nil {
		t.Error("another admin's fetch spent the owner's one fetch")
	}
	if _, err := w.repo.Fetch(context.Background(), fetch(id, owner), signer(&urls)); err != nil {
		t.Errorf("the owner can no longer fetch: %v", err)
	}
}

func TestAnExportThatIsNotReadyUnexpiredAndNewCannotBeFetched(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	cases := map[string]seed{
		"running": {status: "running", attempts: 1, ranFor: time.Second},
		"failed":  {status: "failed", attempts: 1, errorCode: domain.CodeFailed},
		"expired": {status: "ready", expiresIn: -time.Second},
		"fetched": {status: "ready", expiresIn: time.Hour, downloaded: true},
	}
	for name, s := range cases {
		id := w.export(owner, s)
		var urls []string
		if _, err := w.repo.Fetch(context.Background(), fetch(id, owner), signer(&urls)); !errors.Is(err, domain.ErrUnavailable) {
			t.Errorf("%s: error = %v", name, err)
		}
		if len(urls) != 0 {
			t.Errorf("%s: an address was signed", name)
		}
		if name != "fetched" && w.row(id).downloadedAt != nil {
			t.Errorf("%s: the refused fetch marked the export", name)
		}
		w.exec(`DELETE FROM app.data_exports WHERE id = $1::uuid`, id)
	}
	queued := w.export(owner, seed{status: "queued"})
	var urls []string
	if _, err := w.repo.Fetch(context.Background(), fetch(queued, owner), signer(&urls)); !errors.Is(err, domain.ErrUnavailable) {
		t.Errorf("queued: error = %v", err)
	}
}

func TestAFetchWhoseAddressCannotBeSignedSpendsNothing(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	id := w.export(owner, seed{status: "ready", expiresIn: time.Hour})
	unreachable := errors.New("storage unreachable")
	_, err := w.repo.Fetch(context.Background(), fetch(id, owner), func(string, time.Time) (string, error) { return "", unreachable })
	if !errors.Is(err, unreachable) {
		t.Fatalf("error = %v", err)
	}
	if w.row(id).downloadedAt != nil {
		t.Error("the export is marked fetched although the requester received no address")
	}
	if n := w.auditCount("data_export.downloaded", id); n != 0 {
		t.Errorf("%d audit rows for a fetch that gave nothing", n)
	}
	var urls []string
	if _, err := w.repo.Fetch(context.Background(), fetch(id, owner), signer(&urls)); err != nil {
		t.Errorf("the retry: %v", err)
	}
}

func TestFetchesAtOnceHandOutOneAddress(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	id := w.export(owner, seed{status: "ready", expiresIn: time.Hour})
	const attempts = 10
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		granted int
		refused int
		other   []error
	)
	start := make(chan struct{})
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var urls []string
			<-start
			_, err := w.repo.Fetch(context.Background(), fetch(id, owner), signer(&urls))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				granted++
			case errors.Is(err, domain.ErrUnavailable):
				refused++
			default:
				other = append(other, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if len(other) != 0 {
		t.Fatalf("unexpected errors: %v", other)
	}
	if granted != 1 || refused != attempts-1 {
		t.Fatalf("%d fetches granted and %d refused of %d at once", granted, refused, attempts)
	}
	if n := w.auditCount("data_export.downloaded", id); n != 1 {
		t.Errorf("%d audit rows, want 1", n)
	}
}

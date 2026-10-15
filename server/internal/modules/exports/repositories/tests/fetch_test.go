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

func TestAReadyExportIsSignedThreeTimesAndEachSigningIsAudited(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	id := w.export(owner, seed{status: "ready", expiresIn: time.Hour})
	var urls []string
	var first *time.Time

	for n := 1; n <= domain.MaxDownloads; n++ {
		got, err := w.repo.Fetch(context.Background(), fetch(id, owner), signer(&urls))
		if err != nil {
			t.Fatalf("signing %d: %v", n, err)
		}
		stored := w.row(id)
		if len(urls) != n || got != urls[n-1] || stored.storageKey == nil || !strings.HasPrefix(got, "https://objects.example/"+*stored.storageKey+"?created=") {
			t.Errorf("signing %d: url = %q for key %v", n, got, stored.storageKey)
		}
		if stored.downloads != n || stored.downloadedAt == nil {
			t.Errorf("signing %d: downloads = %d, downloadedAt = %v", n, stored.downloads, stored.downloadedAt)
		}
		if first == nil {
			first = stored.downloadedAt
		} else if !stored.downloadedAt.Equal(*first) {
			t.Errorf("signing %d moved downloaded_at from %v to %v; it keeps the first fetch", n, first, stored.downloadedAt)
		}
		if audited := w.auditCount("data_export.downloaded", id); audited != n {
			t.Errorf("signing %d: %d audit rows", n, audited)
		}
	}

	if _, err := w.repo.Fetch(context.Background(), fetch(id, owner), signer(&urls)); !errors.Is(err, domain.ErrUnavailable) {
		t.Errorf("the fourth fetch: %v", err)
	}
	if len(urls) != domain.MaxDownloads {
		t.Errorf("%d addresses were signed, want %d", len(urls), domain.MaxDownloads)
	}
	if n := w.auditCount("data_export.downloaded", id); n != domain.MaxDownloads {
		t.Errorf("the refused fetch was audited: %d rows", n)
	}
	read, err := w.repo.Get(context.Background(), id, owner)
	if err != nil || read.Downloads != domain.MaxDownloads || read.DownloadedAt == nil {
		t.Errorf("Get = %+v, %v", read, err)
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
	if stored := w.row(id); len(urls) != 0 || stored.downloadedAt != nil || stored.downloads != 0 {
		t.Error("another admin's fetch spent one of the owner's fetches")
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
		"spent":   {status: "ready", expiresIn: time.Hour, downloads: domain.MaxDownloads},
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
		if name != "spent" && w.row(id).downloadedAt != nil {
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
	if stored := w.row(id); stored.downloadedAt != nil || stored.downloads != 0 {
		t.Error("the export is counted although the requester received no address")
	}
	if n := w.auditCount("data_export.downloaded", id); n != 0 {
		t.Errorf("%d audit rows for a fetch that gave nothing", n)
	}
	var urls []string
	if _, err := w.repo.Fetch(context.Background(), fetch(id, owner), signer(&urls)); err != nil {
		t.Errorf("the retry: %v", err)
	}
}

func TestTheLastDownloadIsTheCapEvenWhenAskedFor(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	id := w.export(owner, seed{status: "ready", expiresIn: time.Hour, downloads: domain.MaxDownloads - 1})
	var urls []string
	if _, err := w.repo.Fetch(context.Background(), fetch(id, owner), signer(&urls)); err != nil {
		t.Fatalf("the last allowed fetch: %v", err)
	}
	if _, err := w.repo.Fetch(context.Background(), fetch(id, owner), signer(&urls)); !errors.Is(err, domain.ErrUnavailable) {
		t.Errorf("the one after it: %v", err)
	}
	if stored := w.row(id); stored.downloads != domain.MaxDownloads {
		t.Errorf("downloads = %d", stored.downloads)
	}
}

func TestFetchesAtOnceHandOutNoMoreAddressesThanTheCap(t *testing.T) {
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
	if granted != domain.MaxDownloads || refused != attempts-domain.MaxDownloads {
		t.Fatalf("%d fetches granted and %d refused of %d at once", granted, refused, attempts)
	}
	if n := w.auditCount("data_export.downloaded", id); n != domain.MaxDownloads {
		t.Errorf("%d audit rows, want %d", n, domain.MaxDownloads)
	}
}

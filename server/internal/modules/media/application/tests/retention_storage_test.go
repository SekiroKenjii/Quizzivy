//go:build integration

package application_test

import (
	"context"
	"io"
	"net/http"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
	"testing"
	"time"
)

func download(t *testing.T, url string) (int, string) {
	t.Helper()
	response, err := (&http.Client{Timeout: 5 * time.Second}).Get(url)
	if err != nil {
		t.Fatal("download from a signed URL")
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal("read a signed download")
	}
	return response.StatusCode, string(body)
}

func TestTheSweepRemovesTheObjectOfAReplacedFileFromRealStorage(t *testing.T) {
	w := newReplacementWorld(t, nil)
	s := newReplacementRealStore(t, w)
	old := w.asset(t, w.a, domain.KindImage, 20)
	oldKey := replacementOldObject(t, w, s, old)
	app := application.New(w.repo, s, &scriptedAudio{durationMs: 1000}).WithImageProbe(&scriptedImage{width: 1200, height: 800})
	out, err := app.Commands.Replace.Handle(context.Background(), command.Replace{ID: old, Scope: access.Scope{UserID: w.a}, UploaderID: w.a, Filename: "mới.png", Body: imageOf(t, 32)})
	if err != nil {
		t.Fatal(err)
	}
	if out.Previous.DeleteAt == nil || out.Previous.Asset.URL == "" {
		t.Fatalf("the old file is answered as %+v, want its removal time and a URL to download it", out.Previous)
	}
	if status, body := download(t, out.Previous.Asset.URL); status != http.StatusOK || body != "immutable old object" {
		t.Fatalf("downloading the old file from its answer: %d %q", status, body)
	}

	w.exec(t, `UPDATE app.media_assets SET unreferenced_at=$2 WHERE id=$1`, old, epoch)
	sweepAt(t, app, epoch.Add(domain.RetiredFileGrace))
	if status, _ := s.read(t, oldKey); status != http.StatusNotFound {
		t.Errorf("the replaced file's object answers %d after the sweep, want 404", status)
	}
	if !w.purged(t, old) {
		t.Error("the replaced file's row is not marked")
	}
	if status, _ := s.read(t, out.Asset.StorageKey); status != http.StatusOK {
		t.Errorf("the new file's object answers %d, want 200", status)
	}
	if w.purged(t, out.Asset.ID) {
		t.Error("the sweep marked the file that took its place")
	}
}

func TestTheSweepKeepsTheObjectOfAReplacedFileAVersionHolds(t *testing.T) {
	w := newReplacementWorld(t, nil)
	s := newReplacementRealStore(t, w)
	old := w.asset(t, w.a, domain.KindAudio, 20)
	oldKey := replacementOldObject(t, w, s, old)
	if err := publishVersionUsing(context.Background(), w.pool, w.a, old, true); err != nil {
		t.Fatal(err)
	}
	app := application.New(w.repo, s, &scriptedAudio{durationMs: 1000})
	if _, err := replaceAt(t, w, old, domain.KindAudio, 30, 100, epoch); err != nil {
		t.Fatal(err)
	}

	w.exec(t, `UPDATE app.media_assets SET unreferenced_at=$2 WHERE id=$1`, old, epoch)
	sweepAt(t, app, epoch.Add(domain.RetiredFileGrace))
	if status, body := s.read(t, oldKey); status != http.StatusOK || string(body) != "immutable old object" {
		t.Errorf("the object of a file a version holds answers %d after the sweep, want 200", status)
	}
	if w.purged(t, old) || w.dated(t, old) != nil {
		t.Error("the sweep removed or dated a file a version holds")
	}
}

package worker_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/worker"
	"strings"
	"testing"
)

type objectReader struct {
	ports.ArtifactStore
	body string
	size int64
}

func (s objectReader) Open(context.Context, string) (io.ReadCloser, int64, error) {
	return io.NopCloser(strings.NewReader(s.body)), s.size, nil
}

func TestPrivateFetchRejectsTamperingAndRemovesPartialStagingFiles(t *testing.T) {
	digest := sha256.Sum256([]byte("known"))
	for _, store := range []objectReader{{body: "other", size: 5}, {body: "known trailing", size: 5}, {body: "short", size: 20}} {
		dir := t.TempDir()
		if _, err := worker.FetchPrivate(context.Background(), store, dir, "private", 5, digest[:]); !errors.Is(err, worker.ErrPrivateIdentity) {
			t.Fatal("invalid stored bytes accepted")
		}
		files, err := os.ReadDir(dir)
		if err != nil || len(files) != 0 {
			t.Fatal("failed read leaked staging file")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := worker.FetchPrivate(ctx, objectReader{body: "known", size: 5}, t.TempDir(), "private", 5, digest[:]); !errors.Is(err, context.Canceled) {
		t.Fatalf("ignored cancellation: %v", err)
	}
}

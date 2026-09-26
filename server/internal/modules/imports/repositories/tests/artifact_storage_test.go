//go:build integration

package repositories_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"io"
	"os"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/storage"
	"strings"
	"testing"
)

type uncertainStore struct {
	*storage.Client
	fail     bool
	afterPut func()
}

type seekableBody struct{ *strings.Reader }

func (seekableBody) Close() error { return nil }

func (s *uncertainStore) PutImmutable(ctx context.Context, key, kind string, body io.ReadSeeker, size int64, digest []byte) error {
	if err := s.Client.PutImmutable(ctx, key, kind, body, size, digest); err != nil {
		return err
	}
	if s.afterPut != nil {
		s.afterPut()
	}
	if s.fail {
		s.fail = false
		return errors.New("synthetic lost storage response")
	}
	return nil
}
func privateStore(t *testing.T) *storage.Client {
	t.Helper()
	if os.Getenv("S3_ENDPOINT") == "" || os.Getenv("S3_BUCKET") == "" {
		t.Skip("private MinIO required")
	}
	store, err := storage.New(context.Background(), storage.Config{Endpoint: os.Getenv("S3_ENDPOINT"), Region: os.Getenv("S3_REGION"), Bucket: os.Getenv("S3_BUCKET"), AccessKeyID: os.Getenv("S3_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"), ForcePathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestArtifactWriterRecoversLostStorageResponseWithoutDuplicatingOrPublishingPartialSet(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	version := uuid.NewString()
	h.schedule(t, version, 3)
	run := h.claim(t, policy(version))
	p := h.artifactPlan(t, run)
	const payload = `{"blocks":[]}`
	digest := sha256.Sum256([]byte(payload))
	p.Files = p.Files[:1]
	p.Files[0].Bytes = int64(len(payload))
	p.Files[0].SHA256 = digest[:]
	store := &uncertainStore{Client: privateStore(t), fail: true}
	w := worker.ArtifactWriter{Repo: h.repo, Store: store, Quotas: artifactQuotas()}
	open := func(string) (io.ReadSeekCloser, error) { return seekableBody{strings.NewReader(payload)}, nil }
	partial, err := w.Write(ctx, run.Claim(), p, open)
	if err == nil || partial.Ready || len(partial.Files) != 1 {
		t.Fatalf("lost response falsely complete: %v", err)
	}
	t.Cleanup(func() { _ = store.Delete(context.Background(), partial.Files[0].StorageKey) })
	if _, err := h.repo.ArtifactSet(ctx, run.ImportID, partial.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("partial set visible")
	}
	set, err := w.Write(ctx, run.Claim(), p, open)
	if err != nil || !set.Ready || set.ID != partial.ID {
		t.Fatalf("lost response recovery: %v", err)
	}
	file, err := worker.FetchPrivate(ctx, store, t.TempDir(), set.Files[0].StorageKey, set.Files[0].Bytes, set.Files[0].SHA256)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(file)
	if err != nil || string(data) != payload {
		t.Fatal("recovery changed content")
	}
	p.Stage = "normalization"
	store.afterPut = func() { h.expire(t, run.ID) }
	late, err := w.Write(ctx, run.Claim(), p, open)
	t.Cleanup(func() {
		for _, f := range late.Files {
			_ = store.Delete(context.Background(), f.StorageKey)
		}
	})
	if !errors.Is(err, domain.ErrLeaseLost) {
		t.Fatalf("expired upload acknowledged: %v", err)
	}
	if _, err := h.repo.ArtifactSet(ctx, run.ImportID, late.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("late write published")
	}
}

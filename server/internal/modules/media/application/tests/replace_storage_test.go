//go:build integration

package application_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/platform/storage"
	"quizzivy/internal/shared/access"
	"sync"
	"testing"
	"time"
)

type replacementRealStore struct {
	*storage.Client
	mu   sync.Mutex
	keys []string
	t    *testing.T
}

func (s *replacementRealStore) Put(ctx context.Context, key, mime string, body io.Reader, size int64) error {
	s.mu.Lock()
	s.keys = append(s.keys, key)
	s.mu.Unlock()
	return s.Client.Put(ctx, key, mime, body, size)
}
func newReplacementRealStore(t *testing.T, w *replacementWorld) *replacementRealStore {
	t.Helper()
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		t.Fatal("S3_BUCKET is required")
	}
	client, err := storage.New(context.Background(), storage.Config{Endpoint: os.Getenv("S3_ENDPOINT"), Region: os.Getenv("S3_REGION"), Bucket: bucket, AccessKeyID: os.Getenv("S3_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"), ForcePathStyle: true})
	if err != nil {
		t.Fatal("create private storage client")
	}
	store := &replacementRealStore{Client: client, t: t}
	t.Cleanup(func() {
		w.finishPending(t)
		store.mu.Lock()
		defer store.mu.Unlock()
		for _, key := range store.keys {
			if err := store.Delete(context.Background(), key); err != nil {
				t.Error("owned private object cleanup failed")
			}
			if status, _ := store.read(t, key); status != http.StatusNotFound {
				t.Errorf("owned key remains with status %d", status)
			}
		}
	})
	return store
}
func (s *replacementRealStore) read(t *testing.T, key string) (int, []byte) {
	t.Helper()
	url, err := s.SignedURL(context.Background(), key, time.Minute)
	if err != nil {
		t.Fatal("sign private test object")
	}
	client := http.Client{Timeout: 5 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		t.Fatal("read private signed test object")
	}
	data, err := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err = errors.Join(err, closeErr); err != nil {
		t.Fatal("read/close private test object")
	}
	return response.StatusCode, data
}
func replacementOldObject(t *testing.T, w *replacementWorld, s *replacementRealStore, old string) string {
	t.Helper()
	key := w.text(t, `SELECT storage_key FROM app.media_assets WHERE id=$1`, old)
	body := []byte("immutable old object")
	if err := s.Put(context.Background(), key, "image/png", bytes.NewReader(body), int64(len(body))); err != nil {
		t.Fatal("put owned old object")
	}
	return key
}
func TestReplacementRealStorageSuccessKeepsOldBytesAndFreshObject(t *testing.T) {
	w := newReplacementWorld(t, nil)
	s := newReplacementRealStore(t, w)
	old := w.asset(t, w.a, domain.KindImage, 20)
	oldKey := replacementOldObject(t, w, s, old)
	app := application.New(w.repo, s, &scriptedAudio{durationMs: 1000}).WithImageProbe(&scriptedImage{width: 1200, height: 800})
	out, err := app.Commands.Replace.Handle(context.Background(), command.Replace{ID: old, Scope: access.Scope{UserID: w.a}, UploaderID: w.a, Filename: "mới.png", Body: imageOf(t, 32)})
	if err != nil {
		t.Fatal(err)
	}
	status, data := s.read(t, oldKey)
	if status != 200 || string(data) != "immutable old object" {
		t.Fatalf("old bytes/status changed=%d", status)
	}
	status, data = s.read(t, out.Asset.StorageKey)
	if status != 200 || len(data) != 32 || out.Asset.StorageKey == oldKey {
		t.Fatalf("new bytes/status=%d/%d", status, len(data))
	}
	if count := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE id=$1 AND replaced_by=$2`, old, out.Asset.ID); count != 1 {
		t.Fatalf("durable link=%d", count)
	}
}
func TestReplacementRealStorageNoSendCommitFailureDeletesFreshKeyAndKeepsOld(t *testing.T) {
	tr := newReplacementTrace()
	w := newReplacementWorld(t, tr)
	s := newReplacementRealStore(t, w)
	old := w.asset(t, w.a, domain.KindImage, 20)
	oldKey := replacementOldObject(t, w, s, old)
	app := application.New(w.repo, s, &scriptedAudio{durationMs: 1000}).WithImageProbe(&scriptedImage{width: 1200, height: 800})
	tr.cancelCommit.Store(true)
	_, err := app.Commands.Replace.Handle(context.Background(), command.Replace{ID: old, Scope: access.Scope{UserID: w.a}, UploaderID: w.a, Filename: "mới.png", Body: imageOf(t, 32)})
	var failure *domain.ReplacementError
	if !errors.As(err, &failure) || failure.Outcome != domain.ReplacementNotCommitted {
		t.Fatalf("no-send=%v", err)
	}
	if len(s.keys) != 2 {
		t.Fatalf("keys=%d", len(s.keys))
	}
	status, _ := s.read(t, s.keys[1])
	if status != 404 {
		t.Fatalf("failed fresh key remains=%d", status)
	}
	status, data := s.read(t, oldKey)
	if status != 200 || string(data) != "immutable old object" {
		t.Fatalf("old object changed=%d", status)
	}
	if got := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE owner_id=$1`, w.a); got != 1 {
		t.Fatalf("durable rows=%d", got)
	}
}

func TestReplacementRealRollbackFailureStillCompensatesFreshKeyAndDisposes(t *testing.T) {
	tr := newReplacementTrace()
	w := newReplacementWorld(t, tr)
	s := newReplacementRealStore(t, w)
	old := w.asset(t, w.a, domain.KindImage, 20)
	oldKey := replacementOldObject(t, w, s, old)
	app := application.New(w.repo, s, &scriptedAudio{durationMs: 1000}).WithImageProbe(&scriptedImage{width: 1200, height: 800}).WithOwnerQuota(31)
	tr.failRollback.Store(true)
	_, err := app.Commands.Replace.Handle(context.Background(), command.Replace{ID: old, Scope: access.Scope{UserID: w.a}, UploaderID: w.a, Filename: "mới.png", Body: imageOf(t, 32)})
	var failure *domain.ReplacementError
	if !errors.As(err, &failure) || failure.Outcome != domain.ReplacementNotCommitted || failure.RollbackError == nil || !errors.Is(err, domain.ErrQuotaExceeded) || tr.rollbackCloseFailed.Load() {
		t.Fatalf("rollback failure outcome=%v", err)
	}
	if len(s.keys) != 2 {
		t.Fatalf("keys=%d", len(s.keys))
	}
	status, _ := s.read(t, s.keys[1])
	if status != 404 {
		t.Fatalf("fresh key remains=%d", status)
	}
	status, data := s.read(t, oldKey)
	if status != 200 || string(data) != "immutable old object" {
		t.Fatalf("old key changed=%d", status)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 16*time.Second)
	defer cancel()
	for {
		var exists bool
		if err := w.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1)`, tr.rollbackPID.Load()).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("rollback connection not reclaimed")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if got := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE owner_id=$1`, w.a); got != 1 {
		t.Fatalf("rows after rollback failure=%d", got)
	}
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

type replacementDescriptionFailure struct {
	*repositories.Postgres
	cause error
}

func (r replacementDescriptionFailure) ReferencesFor(context.Context, []string) (map[string][]domain.TestRef, error) {
	return nil, r.cause
}
func TestReplacementRealCommittedDescriptionFailureRetainsDurableObject(t *testing.T) {
	w := newReplacementWorld(t, nil)
	s := newReplacementRealStore(t, w)
	old := w.asset(t, w.a, domain.KindImage, 20)
	oldKey := replacementOldObject(t, w, s, old)
	app := application.New(replacementDescriptionFailure{Postgres: w.repo, cause: domain.ErrQuotaExceeded}, s, &scriptedAudio{durationMs: 1000}).WithImageProbe(&scriptedImage{width: 1200, height: 800})
	_, err := app.Commands.Replace.Handle(context.Background(), command.Replace{ID: old, Scope: access.Scope{UserID: w.a}, UploaderID: w.a, Filename: "mới.png", Body: imageOf(t, 32)})
	var failure *domain.ReplacementError
	if !errors.As(err, &failure) || failure.Outcome != domain.ReplacementCommitted || !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("committed response outcome=%v", err)
	}
	if len(s.keys) != 2 {
		t.Fatalf("keys=%d", len(s.keys))
	}
	status, data := s.read(t, s.keys[1])
	if status != 200 || len(data) != 32 {
		t.Fatalf("committed fresh key missing=%d", status)
	}
	status, data = s.read(t, oldKey)
	if status != 200 || string(data) != "immutable old object" {
		t.Fatalf("old key changed=%d", status)
	}
	if got := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE owner_id=$1`, w.a); got != 2 {
		t.Fatalf("durable rows=%d", got)
	}
	if _, err := w.repo.FindReplacementTarget(context.Background(), access.Scope{UserID: w.a}, old); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("old retry target=%v", err)
	}
}
func TestReplacementRealSameOldContendersCleanOnlyLosingFreshObject(t *testing.T) {
	tr := newReplacementTrace()
	w := newReplacementWorld(t, tr)
	s := newReplacementRealStore(t, w)
	old := w.asset(t, w.a, domain.KindImage, 20)
	oldKey := replacementOldObject(t, w, s, old)
	tx := w.blocker(t)
	pid := tx.Conn().PgConn().PID()
	if err := repositories.LockForVersionUse(context.Background(), tx, old); err != nil {
		t.Fatal(err)
	}
	start := func() <-chan replacementAnswer {
		app := application.New(w.repo, s, &scriptedAudio{durationMs: 1000}).WithImageProbe(&scriptedImage{width: 1200, height: 800})
		body := imageOf(t, 32)
		answer := make(chan replacementAnswer, 1)
		w.pending = append(w.pending, answer)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			out, err := app.Commands.Replace.Handle(ctx, command.Replace{ID: old, Scope: access.Scope{UserID: w.a}, UploaderID: w.a, Filename: "mới.png", Body: body})
			answer <- replacementAnswer{result: out, err: err}
			close(answer)
		}()
		return answer
	}
	first, second := start(), start()
	p1 := w.blocked(t, tr, "FOR UPDATE", pid)
	p2 := w.blocked(t, tr, "FOR UPDATE", pid, p1)
	if p1 == p2 {
		t.Fatal("not two independent pool contenders")
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	x, y := replacementWait(t, first), replacementWait(t, second)
	success, refused := x, y
	if x.err != nil {
		success, refused = y, x
	}
	if success.err != nil || !errors.Is(refused.err, domain.ErrNotFound) {
		t.Fatalf("contender outcomes=%v/%v", x.err, y.err)
	}
	if len(s.keys) != 3 {
		t.Fatalf("owned keys=%d", len(s.keys))
	}
	for _, key := range s.keys {
		status, data := s.read(t, key)
		switch key {
		case oldKey:
			if status != 200 || string(data) != "immutable old object" {
				t.Fatalf("old status=%d", status)
			}
		case success.result.Asset.StorageKey:
			if status != 200 || len(data) != 32 {
				t.Fatalf("winner status=%d", status)
			}
		default:
			if status != 404 {
				t.Fatalf("loser fresh key remains=%d", status)
			}
		}
	}
	if got := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE owner_id=$1`, w.a); got != 2 {
		t.Fatalf("durable contender rows=%d", got)
	}
}

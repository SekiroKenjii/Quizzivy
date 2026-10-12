//go:build integration

package application_test

import (
	"context"
	"io"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
	"sync"
	"testing"
	"time"
)

var epoch = time.Date(2020, time.January, 1, 9, 0, 0, 0, time.UTC)

type retentionStore struct {
	mu      sync.Mutex
	deletes map[string]int
	fail    error
	entered chan string
	release chan struct{}
	once    sync.Once
	after   func()
}

func newRetentionStore() *retentionStore { return &retentionStore{deletes: map[string]int{}} }

func (s *retentionStore) gate(t *testing.T) {
	t.Helper()
	s.entered = make(chan string, 8)
	s.release = make(chan struct{})
	t.Cleanup(s.open)
}

func (s *retentionStore) open() { s.once.Do(func() { close(s.release) }) }

func (s *retentionStore) Put(context.Context, string, string, io.Reader, int64) error { return nil }

func (s *retentionStore) SignedURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://example.test/" + key, nil
}

func (s *retentionStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	fail, entered, release := s.fail, s.entered, s.release
	s.fail = nil
	s.mu.Unlock()
	if entered != nil {
		entered <- key
		<-release
	}
	if fail != nil {
		return fail
	}
	s.mu.Lock()
	s.deletes[key]++
	after := s.after
	s.mu.Unlock()
	if after != nil {
		after()
	}
	return nil
}

func (s *retentionStore) deleted(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deletes[key]
}

func sweepAt(t *testing.T, app *application.Application, now time.Time) command.Swept {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	swept, err := app.Commands.Sweep.Handle(ctx, command.Sweep{Now: now})
	if err != nil {
		t.Fatalf("sweep at %v: %v", now, err)
	}
	return swept
}

func (w *replacementWorld) retire(t *testing.T, id string, at time.Time) {
	t.Helper()
	w.exec(t, `UPDATE app.media_assets SET deleted_at=$2,unreferenced_at=$2 WHERE id=$1`, id, at)
}

func (w *replacementWorld) supersede(t *testing.T, id string, at time.Time) {
	t.Helper()
	newer := w.asset(t, w.a, domain.KindAudio, 1)
	w.exec(t, `UPDATE app.media_assets SET replaced_by=$2,unreferenced_at=$3 WHERE id=$1`, id, newer, at)
}

func (w *replacementWorld) key(t *testing.T, id string) string {
	t.Helper()
	return w.text(t, `SELECT storage_key FROM app.media_assets WHERE id=$1`, id)
}

func (w *replacementWorld) purged(t *testing.T, id string) bool {
	t.Helper()
	return w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE id=$1 AND purged_at IS NOT NULL`, id) == 1
}

func (w *replacementWorld) dated(t *testing.T, id string) *time.Time {
	t.Helper()
	var at *time.Time
	if err := w.pool.QueryRow(context.Background(), `SELECT unreferenced_at FROM app.media_assets WHERE id=$1`, id).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func (w *replacementWorld) usage(t *testing.T, owner string) domain.Usage {
	t.Helper()
	usage, err := w.repo.Usage(context.Background(), access.Scope{UserID: owner})
	if err != nil {
		t.Fatal(err)
	}
	return usage
}

func (w *replacementWorld) waitForLockWait(t *testing.T, sqlContains string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if w.scalar(t, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%'||$1||'%'`, sqlContains) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no statement containing %q waited for a lock", sqlContains)
}

package worker_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
)

type retentionLog struct {
	closing              [][]string
	expired              []domain.Cursor
	removed              map[string]bool
	files                map[string][]string
	steps                []string
	idleBefore           time.Time
	committed, cancelled time.Time
	failDelete           string
	pages                int
}

func (l *retentionLog) CloseIdle(_ context.Context, before time.Time, _ int) ([]string, error) {
	l.idleBefore = before
	if len(l.closing) == 0 {
		return nil, nil
	}
	next := l.closing[0]
	l.closing = l.closing[1:]
	return next, nil
}

func (l *retentionLog) ExpiredFiles(_ context.Context, committed, cancelled time.Time, after domain.Cursor, limit int) ([]domain.Cursor, error) {
	l.committed, l.cancelled = committed, cancelled
	l.pages++
	var page []domain.Cursor
	for _, c := range l.expired {
		past := c.UpdatedAt.After(after.UpdatedAt) || (c.UpdatedAt.Equal(after.UpdatedAt) && c.ID > after.ID)
		if past && !l.removed[c.ID] && len(page) < limit {
			page = append(page, c)
		}
	}
	return page, nil
}

func (l *retentionLog) FilesOf(_ context.Context, id string) ([]string, error) {
	return l.files[id], nil
}

func (l *retentionLog) FilesRemoved(_ context.Context, id string) error {
	if l.removed == nil {
		l.removed = map[string]bool{}
	}
	l.removed[id] = true
	l.steps = append(l.steps, "mark "+id)
	return nil
}

func (l *retentionLog) Delete(_ context.Context, key string) error {
	if key == l.failDelete {
		return errors.New("storage unavailable")
	}
	l.steps = append(l.steps, "delete "+key)
	return nil
}

func due(ids ...string) []domain.Cursor {
	at := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	out := make([]domain.Cursor, len(ids))
	for i, id := range ids {
		out[i] = domain.Cursor{UpdatedAt: at.Add(time.Duration(i) * time.Minute), ID: id}
	}
	return out
}

func sweeper(l *retentionLog, batch int) worker.Sweeper {
	return worker.Sweeper{Repo: l, Store: l, Policy: domain.DefaultRetention(), Batch: batch}
}

func TestASweepDeletesTheObjectsBeforeMarkingTheImport(t *testing.T) {
	l := &retentionLog{expired: due("old"), files: map[string][]string{"old": {"originals/old/a", "artifacts/old/s/1"}}}
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	swept, err := sweeper(l, 10).Sweep(context.Background(), now)
	if err != nil || swept != (worker.Swept{Removed: 1}) {
		t.Fatalf("swept %+v: %v", swept, err)
	}
	if want := []string{"delete originals/old/a", "delete artifacts/old/s/1", "mark old"}; !slices.Equal(l.steps, want) {
		t.Fatalf("steps %v, want %v", l.steps, want)
	}
	if !l.idleBefore.Equal(now.AddDate(0, 0, -60)) || !l.committed.Equal(now.AddDate(0, 0, -30)) || !l.cancelled.Equal(now.AddDate(0, 0, -7)) {
		t.Fatalf("cutoffs idle %v committed %v cancelled %v", l.idleBefore, l.committed, l.cancelled)
	}
}

func TestASweepWorksThroughEveryBatchThatIsDue(t *testing.T) {
	l := &retentionLog{closing: [][]string{{"x", "y"}, {"z"}}, expired: due("a", "b", "c", "d", "e")}
	swept, err := sweeper(l, 2).Sweep(context.Background(), time.Now())
	if err != nil || swept != (worker.Swept{Closed: 3, Removed: 5}) {
		t.Fatalf("swept %+v: %v", swept, err)
	}
}

func TestAnImportThatCannotBeRemovedDoesNotHoldBackTheOthers(t *testing.T) {
	l := &retentionLog{expired: due("stuck", "a", "b"), files: map[string][]string{"stuck": {"originals/stuck/a"}}, failDelete: "originals/stuck/a"}
	swept, err := sweeper(l, 1).Sweep(context.Background(), time.Now())
	if err != nil || swept != (worker.Swept{Removed: 2, Failed: 1}) {
		t.Fatalf("swept %+v: %v", swept, err)
	}
	if slices.Contains(l.steps, "mark stuck") || !slices.Contains(l.steps, "mark a") || !slices.Contains(l.steps, "mark b") {
		t.Fatalf("steps %v", l.steps)
	}
	if l.pages > 4 {
		t.Fatalf("the sweep read %d pages for three imports", l.pages)
	}
}

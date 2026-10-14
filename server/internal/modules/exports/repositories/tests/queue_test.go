//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/exports/domain"
	"quizzivy/internal/modules/exports/repositories"
	"quizzivy/internal/platform/db"
)

func near(t *testing.T, what string, got *time.Time, want time.Time, slack time.Duration) {
	t.Helper()
	if got == nil {
		t.Errorf("%s is not set", what)
		return
	}
	if d := got.Sub(want); d < -slack || d > slack {
		t.Errorf("%s = %v, want %v within %v", what, got, want, slack)
	}
}

func TestClaimTakesAQueuedExportAndCountsTheAttempt(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	id := w.export(owner, seed{status: "queued", includeAnswers: true})
	claimed, err := w.repo.Claim(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if claimed.ID != id || claimed.Status != domain.Running || claimed.Attempts != 1 || !claimed.IncludeAnswers || claimed.RequestedBy != owner {
		t.Errorf("claimed = %+v", claimed)
	}
	near(t, "started_at", w.row(id).startedAt, time.Now(), time.Minute)
	if _, err := w.repo.Claim(context.Background()); !errors.Is(err, domain.ErrNoWork) {
		t.Errorf("a second claim while the lease holds: %v", err)
	}
}

func TestClaimFindsNothingToTakeFromFinishedExports(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	w.export(owner, seed{status: "ready", expiresIn: time.Hour})
	w.export(owner, seed{status: "failed", attempts: 1, errorCode: domain.CodeFailed})
	if _, err := w.repo.Claim(context.Background()); !errors.Is(err, domain.ErrNoWork) {
		t.Fatalf("error = %v", err)
	}
}

func TestAnExportRunningPastItsLeaseIsClaimedAgain(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	id := w.export(owner, seed{status: "running", attempts: 1, ranFor: domain.Lease + time.Minute})
	claimed, err := w.repo.Claim(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if claimed.ID != id || claimed.Attempts != 2 {
		t.Errorf("claimed = %+v", claimed)
	}
	near(t, "started_at", w.row(id).startedAt, time.Now(), time.Minute)
}

func TestAnExportRunningInsideItsLeaseIsLeftToItsWorker(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	id := w.export(owner, seed{status: "running", attempts: 1, ranFor: domain.Lease - time.Minute})
	if _, err := w.repo.Claim(context.Background()); !errors.Is(err, domain.ErrNoWork) {
		t.Fatalf("error = %v", err)
	}
	if got := w.row(id); got.status != "running" || got.attempts != 1 {
		t.Errorf("row = %+v", got)
	}
}

func TestAnExportInterruptedThreeTimesIsFailedInsteadOfRetried(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	id := w.export(owner, seed{status: "running", attempts: domain.MaxAttempts, ranFor: domain.Lease + time.Minute})
	if _, err := w.repo.Claim(context.Background()); !errors.Is(err, domain.ErrNoWork) {
		t.Fatalf("error = %v", err)
	}
	got := w.row(id)
	if got.status != "failed" || got.errorCode == nil || *got.errorCode != domain.CodeInterrupted || got.completedAt == nil {
		t.Errorf("row = %+v", got)
	}
	next, err := w.repo.Request(context.Background(), domain.Request{ActorID: owner})
	if err != nil {
		t.Fatalf("the failed export still blocks the next one: %v", err)
	}
	w.track(next.ID)
}

func TestTwoWorkersClaimingAtOnceShareOneExportBetweenThem(t *testing.T) {
	w := newWorld(t)
	w.export(w.admin(), seed{status: "queued"})
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		claimed int
		idle    int
		other   []error
	)
	start := make(chan struct{})
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := w.repo.Claim(context.Background())
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				claimed++
			case errors.Is(err, domain.ErrNoWork):
				idle++
			default:
				other = append(other, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if len(other) != 0 || claimed != 1 || idle != 5 {
		t.Fatalf("claimed %d, idle %d, errors %v", claimed, idle, other)
	}
}

func TestNextDueIsNowForAQueuedExportAndTheLeaseEndForARunningOne(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	if _, ok, err := w.repo.NextDue(context.Background()); err != nil || ok {
		t.Fatalf("nothing waiting: %v, %v", ok, err)
	}
	id := w.export(owner, seed{status: "queued"})
	due, ok, err := w.repo.NextDue(context.Background())
	if err != nil || !ok {
		t.Fatalf("queued: %v, %v", ok, err)
	}
	near(t, "the due time of a queued export", &due, time.Now(), time.Minute)

	w.exec(`DELETE FROM app.data_exports WHERE id = $1::uuid`, id)
	w.export(owner, seed{status: "running", attempts: 1, ranFor: 10 * time.Minute})
	due, ok, err = w.repo.NextDue(context.Background())
	if err != nil || !ok {
		t.Fatalf("running: %v, %v", ok, err)
	}
	near(t, "the due time of a running export", &due, time.Now().Add(domain.Lease-10*time.Minute), time.Minute)
}

func running(t *testing.T, w *world, owner string) domain.Export {
	t.Helper()
	w.export(owner, seed{status: "queued"})
	claimed, err := w.repo.Claim(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return claimed
}

func TestCompleteMakesTheExportReadyForADayAndTellsTheRequesterOnce(t *testing.T) {
	w := newWorld(t)
	owner := w.user("teacher", "Nguyễn Thị Quản Trị")
	claimed := running(t, w, owner)
	done := domain.Completion{ID: claimed.ID, Attempt: claimed.Attempts, StorageKey: domain.StorageKey(claimed.ID), Bytes: 4096}

	if err := w.repo.Complete(context.Background(), done); err != nil {
		t.Fatal(err)
	}
	got := w.row(claimed.ID)
	if got.status != "ready" || got.storageKey == nil || *got.storageKey != done.StorageKey || got.bytes == nil || *got.bytes != 4096 || got.errorCode != nil {
		t.Errorf("row = %+v", got)
	}
	near(t, "completed_at", got.completedAt, time.Now(), time.Minute)
	near(t, "expires_at", got.expiresAt, time.Now().Add(domain.KeepFor), time.Minute)

	mail := w.outboxFor(claimed.ID)
	if len(mail) != 1 {
		t.Fatalf("%d notices queued, want 1", len(mail))
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(mail[0]), &params); err != nil {
		t.Fatal(err)
	}
	if len(params) != 2 || params["fullName"] != "Nguyễn Thị Quản Trị" || params["exportId"] != claimed.ID {
		t.Errorf("the notice carries %v; only the name and the export id belong in it", params)
	}
	var template, address, to string
	err := w.pool.QueryRow(context.Background(), `
		SELECT o.template, o.to_address, o.to_user_id::text FROM app.email_outbox o WHERE o.dedupe_key = $1`,
		"data_export_ready:"+claimed.ID).Scan(&template, &address, &to)
	if err != nil {
		t.Fatal(err)
	}
	var email string
	if err := w.pool.QueryRow(context.Background(), `SELECT email FROM app.users WHERE id = $1::uuid`, owner).Scan(&email); err != nil {
		t.Fatal(err)
	}
	if template != "data_export_ready" || address != email || to != owner {
		t.Errorf("notice = %s to %s (%s)", template, address, to)
	}

	if err := w.repo.Complete(context.Background(), done); !errors.Is(err, domain.ErrLeaseLost) {
		t.Errorf("completing twice: %v", err)
	}
	if n := len(w.outboxFor(claimed.ID)); n != 1 {
		t.Errorf("%d notices after the second completion", n)
	}
}

func TestCompleteWithoutAnOutboxStillMakesTheExportReady(t *testing.T) {
	w := newWorld(t)
	w.repo = repositories.NewPostgres(db.NewContext(w.pool))
	owner := w.admin()
	claimed := running(t, w, owner)
	err := w.repo.Complete(context.Background(), domain.Completion{ID: claimed.ID, Attempt: claimed.Attempts, StorageKey: "exports/x.zip", Bytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.row(claimed.ID); got.status != "ready" {
		t.Errorf("row = %+v", got)
	}
	if n := len(w.outboxFor(claimed.ID)); n != 0 {
		t.Errorf("%d notices from a repository with no outbox", n)
	}
}

func TestAWorkerWhoseLeaseWasTakenCannotSettleTheExport(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	w.export(owner, seed{status: "running", attempts: 1, ranFor: domain.Lease + time.Minute})
	newer, err := w.repo.Claim(context.Background())
	if err != nil || newer.Attempts != 2 {
		t.Fatalf("claim = %+v, %v", newer, err)
	}
	staleCompletion := domain.Completion{ID: newer.ID, Attempt: 1, StorageKey: "exports/stale.zip", Bytes: 1}
	if err := w.repo.Complete(context.Background(), staleCompletion); !errors.Is(err, domain.ErrLeaseLost) {
		t.Errorf("complete: %v", err)
	}
	if err := w.repo.Fail(context.Background(), newer.ID, 1, domain.CodeFailed); !errors.Is(err, domain.ErrLeaseLost) {
		t.Errorf("fail: %v", err)
	}
	if err := w.repo.Release(context.Background(), newer.ID, 1); !errors.Is(err, domain.ErrLeaseLost) {
		t.Errorf("release: %v", err)
	}
	if got := w.row(newer.ID); got.status != "running" || got.attempts != 2 || got.storageKey != nil {
		t.Errorf("the stale worker changed the row: %+v", got)
	}
	if n := len(w.outboxFor(newer.ID)); n != 0 {
		t.Errorf("the stale worker told the requester: %d notices", n)
	}
}

func TestFailRecordsACodeAndSendsNothing(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	claimed := running(t, w, owner)
	if err := w.repo.Fail(context.Background(), claimed.ID, claimed.Attempts, domain.CodeStorage); err != nil {
		t.Fatal(err)
	}
	got := w.row(claimed.ID)
	if got.status != "failed" || got.errorCode == nil || *got.errorCode != domain.CodeStorage || got.storageKey != nil {
		t.Errorf("row = %+v", got)
	}
	near(t, "completed_at", got.completedAt, time.Now(), time.Minute)
	if n := len(w.outboxFor(claimed.ID)); n != 0 {
		t.Errorf("%d notices for a failure", n)
	}
	read, err := w.repo.Get(context.Background(), claimed.ID, owner)
	if err != nil || read.Status != domain.Failed {
		t.Errorf("get = %+v, %v", read, err)
	}
}

func TestAFinishedExportCannotBeFailedAfterwards(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	claimed := running(t, w, owner)
	done := domain.Completion{ID: claimed.ID, Attempt: claimed.Attempts, StorageKey: "exports/ok.zip", Bytes: 7}
	if err := w.repo.Complete(context.Background(), done); err != nil {
		t.Fatal(err)
	}
	if err := w.repo.Fail(context.Background(), claimed.ID, claimed.Attempts, domain.CodeFailed); !errors.Is(err, domain.ErrLeaseLost) {
		t.Errorf("fail after complete: %v", err)
	}
	if got := w.row(claimed.ID); got.status != "ready" {
		t.Errorf("row = %+v", got)
	}
	w.outboxFor(claimed.ID)
}

func TestReleaseGivesTheExportBackWithoutSpendingTheAttempt(t *testing.T) {
	w := newWorld(t)
	owner := w.admin()
	claimed := running(t, w, owner)
	if err := w.repo.Release(context.Background(), claimed.ID, claimed.Attempts); err != nil {
		t.Fatal(err)
	}
	got := w.row(claimed.ID)
	if got.status != "queued" || got.attempts != 0 || got.startedAt != nil {
		t.Errorf("row = %+v", got)
	}
	again, err := w.repo.Claim(context.Background())
	if err != nil || again.ID != claimed.ID || again.Attempts != 1 {
		t.Errorf("claim after release = %+v, %v", again, err)
	}
}

//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"quizzivy/internal/modules/imports/domain"
	"testing"
	"time"
)

func (h harness) schedule(t *testing.T, version string, maxAttempts int) domain.Run {
	t.Helper()
	v := h.create(t)
	receipt := h.finish(t, h.reserve(t, h.upload(v, "exam")))
	run, err := h.repo.Schedule(context.Background(), domain.Schedule{ImportID: v.ID, RequestID: uuid.NewString(), PipelineVersion: version, ExpectedRevision: receipt.Import.Revision, SourceRevision: receipt.Import.SourceRevision, Actor: h.actor, MaxAttempts: maxAttempts})
	if err != nil {
		t.Fatal(err)
	}
	return run
}
func policy(version string) domain.ClaimPolicy {
	return domain.ClaimPolicy{WorkerID: uuid.NewString(), PipelineVersion: version, Lease: 30 * time.Second, GlobalLimit: 2, ActorLimit: 1}
}
func (h harness) claim(t *testing.T, p domain.ClaimPolicy) domain.Run {
	t.Helper()
	run, err := h.repo.Claim(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return run
}
func (h harness) expire(t *testing.T, id string) {
	t.Helper()
	if _, err := h.pool.Exec(context.Background(), `UPDATE app.word_import_runs SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
}
func (h harness) rejectLateWrites(t *testing.T, c domain.Claim) {
	t.Helper()
	ctx := context.Background()
	for _, err := range []error{
		h.repo.Heartbeat(ctx, c, time.Minute),
		h.repo.Progress(ctx, c, "recognition"),
		h.repo.Complete(ctx, c, outcome(json.RawMessage(`{"schemaVersion":1,"candidate":"late"}`))),
		h.repo.Fail(ctx, domain.RunFailure{Claim: c, Code: "LATE_FAILURE", Retryable: true}),
	} {
		if !errors.Is(err, domain.ErrLeaseLost) {
			t.Fatalf("late worker changed state: %v", err)
		}
	}
}

func TestRunIdentityPinsImmutableSourceAndPipeline(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	version := uuid.NewString()
	empty := h.create(t)
	if _, err := h.repo.Schedule(ctx, domain.Schedule{ImportID: empty.ID, ExpectedRevision: 1, SourceRevision: 0, RequestID: uuid.NewString(), PipelineVersion: version, MaxAttempts: 3, Actor: h.actor}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("scheduled without an exam: %v", err)
	}
	run := h.schedule(t, version, 3)
	in := domain.Schedule{ImportID: run.ImportID, RequestID: run.RequestID, SourceRevision: run.SourceRevision, ExpectedRevision: run.ExpectedRevision, PipelineVersion: version, MaxAttempts: 3, Actor: h.actor}
	replay, err := h.repo.Schedule(ctx, in)
	if err != nil || replay.ID != run.ID {
		t.Fatalf("schedule replay duplicated: %v", err)
	}
	in.PipelineVersion = "different"
	if _, err := h.repo.Schedule(ctx, in); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("changed pipeline replay: %v", err)
	}
	parent, err := h.repo.Get(ctx, run.ImportID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.repo.Reserve(ctx, h.upload(parent, "exam"), h.quotas); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("source replaced while queued: %v", err)
	}
	if _, err := h.repo.Run(ctx, empty.ID, run.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-import run: %v", err)
	}
	if _, err := h.repo.Claim(ctx, policy("old-worker-version")); !errors.Is(err, domain.ErrNoWork) {
		t.Fatalf("old pipeline claimed run: %v", err)
	}
}

func TestLeaseExpiryAndTakeoverFenceEveryWorkerWrite(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	version := uuid.NewString()
	h.schedule(t, version, 3)
	first := h.claim(t, policy(version))
	h.expire(t, first.ID)
	h.rejectLateWrites(t, first.Claim())
	second := h.claim(t, policy(version))
	if second.ID != first.ID || second.ClaimToken <= first.ClaimToken || second.AttemptCount != 2 || second.SourceRevision != first.SourceRevision {
		t.Fatalf("takeover lost identity: %+v", second)
	}
	h.rejectLateWrites(t, first.Claim())
	if err := h.repo.Heartbeat(ctx, second.Claim(), time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.Progress(ctx, second.Claim(), "validation"); err != nil {
		t.Fatal(err)
	}
	result := json.RawMessage(`{"schemaVersion":1,"candidate":"synthetic"}`)
	if err := h.repo.Complete(ctx, second.Claim(), outcome(result)); err != nil {
		t.Fatal(err)
	}
	saved, err := h.repo.Run(ctx, second.ImportID, second.ID)
	if err != nil || saved.Status != "succeeded" || len(saved.Result) == 0 {
		t.Fatalf("completion: %+v %v", saved, err)
	}
	parent, err := h.repo.Get(ctx, second.ImportID)
	if err != nil || parent.Status != "needs_review" || len(parent.Sources) != 1 {
		t.Fatalf("review lost source: %+v %v", parent, err)
	}
	if _, err := h.pool.Exec(ctx, `UPDATE app.word_import_runs SET result='{}' WHERE id=$1`, saved.ID); err == nil {
		t.Fatal("terminal machine result was mutable")
	}
	h.rejectLateWrites(t, second.Claim())
}

func TestExhaustedCrashBecomesFailedAndExplicitRetryKeepsOldRun(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	version := uuid.NewString()
	h.schedule(t, version, 1)
	run := h.claim(t, policy(version))
	h.expire(t, run.ID)
	if _, err := h.repo.Claim(ctx, policy(version)); !errors.Is(err, domain.ErrNoWork) {
		t.Fatalf("exhausted claim: %v", err)
	}
	saved, err := h.repo.Run(ctx, run.ImportID, run.ID)
	if err != nil || saved.Status != "failed" || saved.ErrorCode == nil || *saved.ErrorCode != "WORKER_LEASE_EXPIRED" {
		t.Fatalf("expired recovery: %+v %v", saved, err)
	}
	parent, err := h.repo.Get(ctx, run.ImportID)
	if err != nil || parent.Status != "failed" {
		t.Fatal("import stuck processing after final lease")
	}
	retry, err := h.repo.Schedule(ctx, domain.Schedule{ImportID: parent.ID, SourceRevision: parent.SourceRevision, ExpectedRevision: parent.Revision, RequestID: uuid.NewString(), PipelineVersion: version, Actor: h.actor, MaxAttempts: 3})
	if err != nil || retry.ID == run.ID {
		t.Fatalf("explicit retry overwrote history: %v", err)
	}
	old, err := h.repo.Run(ctx, run.ImportID, run.ID)
	if err != nil || old.Status != "failed" {
		t.Fatal("retry changed terminal run")
	}
}

func TestRetryableFailuresStopAtAttemptBudget(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	version := uuid.NewString()
	h.schedule(t, version, 2)
	for attempt := 1; attempt <= 2; attempt++ {
		run := h.claim(t, policy(version))
		if run.AttemptCount != attempt {
			t.Fatalf("attempt counter: %d", run.AttemptCount)
		}
		if err := h.repo.Fail(ctx, domain.RunFailure{Claim: run.Claim(), Code: "STORAGE_UNAVAILABLE", Retryable: true}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.repo.Claim(ctx, policy(version)); !errors.Is(err, domain.ErrNoWork) {
		t.Fatalf("unbounded automatic retry: %v", err)
	}
}

func TestGlobalAndActorLeaseBudgetsSurviveMultipleWorkers(t *testing.T) {
	h := setup(t)
	other := setup(t)
	ctx := context.Background()
	version := uuid.NewString()
	h.schedule(t, version, 3)
	h.schedule(t, version, 3)
	h.claim(t, policy(version))
	if _, err := h.repo.Claim(ctx, policy(version)); !errors.Is(err, domain.ErrNoWork) {
		t.Fatalf("actor lease quota bypassed: %v", err)
	}
	foreign := other.schedule(t, version, 3)
	second := h.claim(t, policy(version))
	if second.ID != foreign.ID {
		t.Fatal("eligible other actor starved")
	}
	p := policy(version)
	p.ActorLimit = 2
	if _, err := h.repo.Claim(ctx, p); !errors.Is(err, domain.ErrNoWork) {
		t.Fatalf("global lease quota bypassed: %v", err)
	}
}

func TestCancellationAndCompletionHaveOnlyOneWinner(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	for range 3 {
		version := uuid.NewString()
		h.schedule(t, version, 3)
		run := h.claim(t, policy(version))
		parent, err := h.repo.Get(ctx, run.ImportID)
		if err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		done := make(chan error, 2)
		go func() {
			<-start
			done <- h.repo.Complete(ctx, run.Claim(), outcome(json.RawMessage(`{"schemaVersion":1}`)))
		}()
		go func() {
			<-start
			_, err := h.repo.Cancel(ctx, domain.Cancel{ImportID: parent.ID, ExpectedRevision: parent.Revision, Actor: h.actor})
			done <- err
		}()
		close(start)
		winners := 0
		for range 2 {
			err := <-done
			if err == nil {
				winners++
			} else if !errors.Is(err, domain.ErrConflict) && !errors.Is(err, domain.ErrLeaseLost) {
				t.Fatal(err)
			}
		}
		if winners != 1 {
			t.Fatalf("cancel/complete both won: %d", winners)
		}
		h.rejectLateWrites(t, run.Claim())
	}
}

func TestRunHistoryKeepsActorWorkerAndFailuresWithoutMutableEvents(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	version := uuid.NewString()
	scheduled := h.schedule(t, version, 3)
	run := h.claim(t, policy(version))
	if err := h.repo.Progress(ctx, run.Claim(), "extraction"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.Progress(ctx, run.Claim(), "extraction"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.Progress(ctx, run.Claim(), "normalization"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stage silently rewound: %v", err)
	}
	if err := h.repo.Fail(ctx, domain.RunFailure{Claim: run.Claim(), Code: "STORAGE_UNAVAILABLE", Retryable: true}); err != nil {
		t.Fatal(err)
	}
	next := h.claim(t, policy(version))
	if err := h.repo.Complete(ctx, next.Claim(), outcome(json.RawMessage(`{"schemaVersion":1}`))); err != nil {
		t.Fatal(err)
	}
	events, err := h.repo.Events(ctx, scheduled.ImportID, scheduled.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 6 || events[0].ActorID == nil || *events[0].ActorID != h.actor.ID || events[1].WorkerID == nil || *events[1].WorkerID != *run.WorkerID || events[3].ErrorCode == nil || *events[3].ErrorCode != "STORAGE_UNAVAILABLE" || events[5].Kind != "completed" {
		t.Fatalf("incomplete history: %+v", events)
	}
	var mutable bool
	if err := h.pool.QueryRow(ctx, `SELECT has_table_privilege('quizzivy_app','app.word_import_run_events','UPDATE,DELETE')`).Scan(&mutable); err != nil || mutable {
		t.Fatalf("run history not append-only: %v", err)
	}
	if _, err := h.repo.Events(ctx, uuid.NewString(), scheduled.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("history escaped parent scope")
	}
}

func outcome(result json.RawMessage) domain.Outcome {
	return domain.Outcome{Result: result, Draft: json.RawMessage(`{"version":"word-draft-v1","title":"","sections":[],"notices":[],"acknowledged":[]}`)}
}

package application_test

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/domain"
	"slices"
	"testing"
	"time"
)

type retentionScript struct {
	scriptedRepo
	observed   domain.Observed
	observeErr error
	observedAt []time.Time
	ids        []string
	dueErr     error
	dues       []dueCall
	outcomes   map[string]domain.PurgeOutcome
	failures   map[string]error
	purged     []domain.PurgeInput
	onPurge    func(id string)
}

type dueCall struct {
	due   time.Time
	after string
	limit int
}

func (r *retentionScript) Observe(_ context.Context, now time.Time) (domain.Observed, error) {
	r.observedAt = append(r.observedAt, now)
	return r.observed, r.observeErr
}

func (r *retentionScript) Due(_ context.Context, due time.Time, after string, limit int) ([]string, error) {
	r.dues = append(r.dues, dueCall{due: due, after: after, limit: limit})
	if r.dueErr != nil {
		return nil, r.dueErr
	}
	var page []string
	for _, id := range r.ids {
		if id > after && len(page) < limit {
			page = append(page, id)
		}
	}
	return page, nil
}

func (r *retentionScript) Purge(_ context.Context, in domain.PurgeInput) (domain.PurgeOutcome, error) {
	r.purged = append(r.purged, in)
	if r.onPurge != nil {
		r.onPurge(in.ID)
	}
	if err := r.failures[in.ID]; err != nil {
		return domain.PurgeSkipped, err
	}
	if outcome, ok := r.outcomes[in.ID]; ok {
		return outcome, nil
	}
	return domain.PurgeRemoved, nil
}

func sweeping(r *retentionScript, store *recordingStore) *application.Application {
	return application.New(r, store, &scriptedAudio{durationMs: 1000})
}

var sweepNow = time.Date(2026, time.October, 12, 9, 0, 0, 0, time.UTC)

func TestTheSweepDatesTheWaitsFirstAndRemovesWhatHasWaitedSevenDays(t *testing.T) {
	r := &retentionScript{observed: domain.Observed{Unreferenced: 3, Named: 2}, ids: []string{"a", "b"}, outcomes: map[string]domain.PurgeOutcome{"b": domain.PurgeNamed}}
	swept, err := sweeping(r, &recordingStore{}).Commands.Sweep.Handle(context.Background(), command.Sweep{Now: sweepNow})
	if err != nil {
		t.Fatal(err)
	}
	if swept.Unreferenced != 3 || swept.Cancelled != 3 || swept.Removed != 1 || swept.Failed != 0 {
		t.Errorf("the sweep counted %+v, want 3 dated, 3 cancelled (2 seen and 1 found named), 1 removed", swept)
	}
	if !slices.Equal(r.observedAt, []time.Time{sweepNow}) {
		t.Errorf("the sweep looked at %v, want once, at %v", r.observedAt, sweepNow)
	}
	wantDue := sweepNow.Add(-domain.RetiredFileGrace)
	if len(r.dues) != 1 || !r.dues[0].due.Equal(wantDue) || r.dues[0].after != "" {
		t.Errorf("the sweep asked for %+v, want the files dated at or before %v", r.dues, wantDue)
	}
	for _, in := range r.purged {
		if !in.Due.Equal(wantDue) || !in.Now.Equal(sweepNow) || in.Remove == nil {
			t.Errorf("a purge was asked as %+v, want the sweep's due time, now and a way to delete the object", in)
		}
	}
}

func TestTheSweepWalksEveryPageInIdOrder(t *testing.T) {
	var ids []string
	for i := range 250 {
		ids = append(ids, fmt.Sprintf("file-%03d", i))
	}
	r := &retentionScript{ids: ids}
	swept, err := sweeping(r, &recordingStore{}).Commands.Sweep.Handle(context.Background(), command.Sweep{Now: sweepNow})
	if err != nil || swept.Removed != 250 {
		t.Fatalf("the sweep removed %d files: %v, want 250", swept.Removed, err)
	}
	var seen []string
	for _, in := range r.purged {
		seen = append(seen, in.ID)
	}
	if !slices.Equal(seen, ids) {
		t.Error("the sweep did not purge every file once, in id order")
	}
	if len(r.dues) != 3 || r.dues[1].after != "file-099" || r.dues[2].after != "file-199" {
		t.Errorf("the sweep paged as %+v, want pages of 100 continuing after the last id", r.dues)
	}
}

func TestAFileThatFailsDoesNotHoldBackTheOthers(t *testing.T) {
	cause := errors.New("storage unreachable")
	r := &retentionScript{ids: []string{"a", "b", "c"}, failures: map[string]error{"b": cause}}
	swept, err := sweeping(r, &recordingStore{}).Commands.Sweep.Handle(context.Background(), command.Sweep{Now: sweepNow})
	if err != nil {
		t.Fatalf("a file's failure ended the sweep: %v", err)
	}
	if swept.Removed != 2 || swept.Failed != 1 || !errors.Is(swept.Cause, cause) {
		t.Errorf("the sweep counted %+v, want 2 removed, 1 failed and its cause", swept)
	}
	if len(r.purged) != 3 {
		t.Errorf("%d files were tried, want all three and none twice", len(r.purged))
	}
}

func TestTheSweepStopsWhenItsTimeIsUp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &retentionScript{ids: []string{"a", "b", "c"}}
	r.onPurge = func(id string) {
		if id == "a" {
			cancel()
		}
	}
	swept, err := sweeping(r, &recordingStore{}).Commands.Sweep.Handle(ctx, command.Sweep{Now: sweepNow})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("the sweep ended with %v, want the cancellation", err)
	}
	if len(r.purged) != 1 || swept.Removed != 1 {
		t.Errorf("%d files were tried and %d removed after the time was up, want only the first", len(r.purged), swept.Removed)
	}
}

func TestTheSweepReportsWhatItCouldNotRead(t *testing.T) {
	cause := errors.New("database unreachable")
	for name, r := range map[string]*retentionScript{
		"looking": {observeErr: cause},
		"listing": {dueErr: cause},
	} {
		swept, err := sweeping(r, &recordingStore{}).Commands.Sweep.Handle(context.Background(), command.Sweep{Now: sweepNow})
		if !errors.Is(err, cause) || swept.Removed != 0 {
			t.Errorf("%s: %+v, %v, want the cause and nothing removed", name, swept, err)
		}
	}
}

type deadlineStore struct {
	recordingStore
	bounded bool
}

func (s *deadlineStore) Delete(ctx context.Context, key string) error {
	deadline, ok := ctx.Deadline()
	s.bounded = ok && time.Until(deadline) > 0
	return s.recordingStore.Delete(ctx, key)
}

func TestTheSweepDeletesAnObjectUnderABoundedContext(t *testing.T) {
	store := &deadlineStore{}
	r := &retentionScript{ids: []string{"a"}}
	app := application.New(r, store, &scriptedAudio{durationMs: 1000})
	if _, err := app.Commands.Sweep.Handle(context.Background(), command.Sweep{Now: sweepNow}); err != nil {
		t.Fatal(err)
	}
	if err := r.purged[0].Remove(context.Background(), "audio/a.mp3"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(store.deletes, []string{"audio/a.mp3"}) || !store.bounded {
		t.Errorf("the object was deleted as %v with a bounded context %v, want the one key, bounded", store.deletes, store.bounded)
	}
}

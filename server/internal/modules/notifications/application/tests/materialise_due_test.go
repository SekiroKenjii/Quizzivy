package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/modules/notifications/domain"
)

var morning = time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)

func dueNotice() domain.Notice {
	return domain.Notice{
		UserID:    reader,
		Kind:      domain.AssignmentOpened,
		Params:    domain.Opened{Title: "Đề giữa kỳ", ClosesAt: morning.Add(48 * time.Hour)},
		Target:    &domain.Target{Route: domain.RouteStudentAssignment, AssignmentID: assignment},
		DedupeKey: domain.OpenedKey(assignment),
		Merge:     domain.Replace,
	}
}

func materialise(t *testing.T, store *fakeStore, user string, at time.Time) (int, error) {
	t.Helper()
	return over(store).Commands.MaterialiseDue.Handle(context.Background(), command.MaterialiseDue{UserID: user, Now: at})
}

func TestMaterialisingReadsTheUsersDueItemsAtTheGivenMomentAndWritesThem(t *testing.T) {
	store := only(t, "Due", "InsertAbsent")
	store.due = []domain.Notice{dueNotice()}
	written, err := materialise(t, store, reader, morning)
	if err != nil || written != 1 {
		t.Fatalf("wrote %d, %v; want 1", written, err)
	}
	if store.userID != reader || !store.dueAt.Equal(morning) {
		t.Errorf("the store was asked for %s at %v, want %s at %v", store.userID, store.dueAt, reader, morning)
	}
	if len(store.inserted) != 1 || store.inserted[0].DedupeKey != domain.OpenedKey(assignment) {
		t.Errorf("the store was handed %+v", store.inserted)
	}
}

func TestMaterialisingRunsAtMostOnceInFiveMinutesPerUser(t *testing.T) {
	cases := []struct {
		name  string
		after time.Duration
		runs  bool
	}{
		{"straight away", 0, false},
		{"four minutes and fifty-nine seconds on", 5*time.Minute - time.Second, false},
		{"exactly five minutes on", 5 * time.Minute, true},
		{"an hour on", time.Hour, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := only(t, "Due", "InsertAbsent")
			store.due = []domain.Notice{dueNotice()}
			app := over(store)
			handle := func(at time.Time) {
				t.Helper()
				if _, err := app.Commands.MaterialiseDue.Handle(context.Background(), command.MaterialiseDue{UserID: reader, Now: at}); err != nil {
					t.Fatal(err)
				}
			}
			handle(morning)
			before := store.calls
			handle(morning.Add(c.after))
			if ran := store.calls > before; ran != c.runs {
				t.Errorf("the second call ran = %v, want %v", ran, c.runs)
			}
		})
	}
}

func TestOneUsersRunDoesNotThrottleAnother(t *testing.T) {
	store := only(t, "Due", "InsertAbsent")
	store.due = []domain.Notice{dueNotice()}
	app := over(store)
	for _, user := range []string{reader, "01935000-0000-7000-8000-0000000000c4"} {
		if _, err := app.Commands.MaterialiseDue.Handle(context.Background(), command.MaterialiseDue{UserID: user, Now: morning}); err != nil {
			t.Fatal(err)
		}
	}
	if store.calls != 4 {
		t.Errorf("the store was called %d times for two users, want a read and a write each", store.calls)
	}
}

func TestAFailedRunDoesNotHoldTheUserOffForFiveMinutes(t *testing.T) {
	store := only(t, "Due", "InsertAbsent")
	store.due = []domain.Notice{dueNotice()}
	app := over(store)
	run := func() error {
		_, err := app.Commands.MaterialiseDue.Handle(context.Background(), command.MaterialiseDue{UserID: reader, Now: morning})
		return err
	}

	store.dueErr = errors.New("the database is away")
	if err := run(); !errors.Is(err, store.dueErr) {
		t.Fatalf("a failed read answered %v", err)
	}
	store.dueErr = nil
	store.insertErr = errors.New("the write was refused")
	if err := run(); !errors.Is(err, store.insertErr) {
		t.Fatalf("a failed write answered %v", err)
	}
	store.insertErr = nil
	before := store.calls
	if err := run(); err != nil || store.calls == before {
		t.Errorf("the run after two failures did not go ahead: %v, %d calls", err, store.calls-before)
	}
}

func TestANoticeTheStoreWouldRefuseIsReportedAndTheOthersStillWritten(t *testing.T) {
	broken := dueNotice()
	broken.Params = domain.Opened{Title: "", ClosesAt: morning}
	broken.DedupeKey = "opened:broken"
	store := only(t, "Due", "InsertAbsent")
	store.due = []domain.Notice{broken, dueNotice()}
	app := over(store)
	written, err := app.Commands.MaterialiseDue.Handle(context.Background(), command.MaterialiseDue{UserID: reader, Now: morning})
	if !errors.Is(err, domain.ErrInvalidParams) {
		t.Errorf("err = %v, want the refusal reported", err)
	}
	if written != 1 || len(store.inserted) != 1 || store.inserted[0].DedupeKey != domain.OpenedKey(assignment) {
		t.Errorf("wrote %d of %+v, want only the valid notice", written, store.inserted)
	}

	before := store.calls
	if _, err := app.Commands.MaterialiseDue.Handle(context.Background(), command.MaterialiseDue{UserID: reader, Now: morning.Add(time.Minute)}); err != nil || store.calls != before {
		t.Errorf("a refusal released the throttle: %v, %d more calls", err, store.calls-before)
	}
}

func TestNothingDueWritesNothing(t *testing.T) {
	store := only(t, "Due")
	written, err := materialise(t, store, reader, morning)
	if err != nil || written != 0 {
		t.Errorf("wrote %d, %v; want nothing", written, err)
	}
	if store.calls != 1 {
		t.Errorf("%d store calls, want only the read", store.calls)
	}
}

func TestMaterialisingNamesAUser(t *testing.T) {
	store := only(t, "")
	if _, err := materialise(t, store, "", morning); !errors.Is(err, domain.ErrNoRecipient) {
		t.Errorf("err = %v, want ErrNoRecipient", err)
	}
	if store.calls != 0 {
		t.Errorf("the store was called %d times for no one", store.calls)
	}
}

func TestSweepingTheThrottleKeepsWhatHasNotExpired(t *testing.T) {
	store := only(t, "Due")
	app := over(store)
	run := func(user int, at time.Time) {
		t.Helper()
		if _, err := app.Commands.MaterialiseDue.Handle(context.Background(), command.MaterialiseDue{UserID: fmt.Sprintf("user-%d", user), Now: at}); err != nil {
			t.Fatal(err)
		}
	}
	for user := range 4100 {
		run(user, morning)
	}

	run(4100, morning.Add(time.Minute))
	before := store.calls
	run(0, morning.Add(time.Minute))
	if store.calls != before {
		t.Error("a sweep forgot a user who had run a minute before, and let them run again")
	}

	run(4101, morning.Add(6*time.Minute))
	before = store.calls
	run(0, morning.Add(6*time.Minute))
	if store.calls != before+1 {
		t.Errorf("a user whose five minutes were over did not run again: %d calls", store.calls-before)
	}
}

package application_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
)

var avatarSweepNow = time.Date(2026, 10, 18, 3, 0, 0, 0, time.UTC)

type sweepLedger struct {
	due     []string
	dueErr  error
	markErr error
	marks   map[string]time.Time
	asked   []int
	order   []string
}

func (l *sweepLedger) Due(_ context.Context, limit int) ([]string, error) {
	l.asked = append(l.asked, limit)
	return l.due, l.dueErr
}

func (l *sweepLedger) Mark(_ context.Context, userID string, at time.Time) error {
	l.order = append(l.order, "mark "+userID)
	if l.markErr != nil {
		return l.markErr
	}
	if l.marks == nil {
		l.marks = map[string]time.Time{}
	}
	l.marks[userID] = at
	return nil
}

type photoBucket struct {
	keys      []string
	listErr   map[string]error
	deleteErr map[string]error
	ledger    *sweepLedger
	lists     int
	sticky    bool
	cancel    func()
}

func (b *photoBucket) List(_ context.Context, prefix string, limit int) ([]string, error) {
	b.lists++
	if err := b.listErr[prefix]; err != nil {
		return nil, err
	}
	var out []string
	for _, k := range b.keys {
		if strings.HasPrefix(k, prefix) && len(out) < limit {
			out = append(out, k)
		}
	}
	return out, nil
}

func (b *photoBucket) Delete(_ context.Context, key string) error {
	if err := b.deleteErr[key]; err != nil {
		return err
	}
	if b.ledger != nil {
		b.ledger.order = append(b.ledger.order, "delete "+key)
	}
	if b.cancel != nil {
		b.cancel()
	}
	if !b.sticky {
		b.keys = slices.DeleteFunc(b.keys, func(k string) bool { return k == key })
	}
	return nil
}

func avatarSweepApp(t *testing.T, ledger *sweepLedger, bucket *photoBucket) *application.Application {
	t.Helper()
	app := application.New(nil, nil, 0, nil, nil)
	if bucket == nil {
		app.SetAvatarSweeps(ledger, nil)
	} else {
		app.SetAvatarSweeps(ledger, bucket)
	}
	app.SetClock(func() time.Time { return avatarSweepNow })
	return app
}

const (
	anonA = "019a0000-0000-7000-8000-00000000000a"
	anonB = "019a0000-0000-7000-8000-00000000000b"
	anonC = "019a0000-0000-7000-8000-00000000000c"
)

func photosOf(id string, n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("avatars/%s/%04d.png", id, i)
	}
	return keys
}

func TestTheSweepEmptiesEachAnonymisedAccountsPrefixAndThenMarksIt(t *testing.T) {
	ledger := &sweepLedger{due: []string{anonA, anonB}}
	neighbour := "avatars/" + anonA + "0/kept.png"
	bucket := &photoBucket{ledger: ledger, keys: append(append(photosOf(anonA, 2), photosOf(anonB, 1)...), neighbour, "branding/logo.png")}
	app := avatarSweepApp(t, ledger, bucket)

	got, err := app.Commands.SweepAvatarObjects.Handle(context.Background(), command.SweepAvatarObjects{Limit: 500})

	if err != nil || got != 2 {
		t.Fatalf("marked = %d, %v, want 2 and no error", got, err)
	}
	if !slices.Equal(ledger.asked, []int{500}) {
		t.Errorf("Due asked for %v, want one call for 500", ledger.asked)
	}
	if !slices.Equal(bucket.keys, []string{neighbour, "branding/logo.png"}) {
		t.Errorf("left in the bucket: %v, want only the objects of no anonymised account", bucket.keys)
	}
	want := []string{
		"delete avatars/" + anonA + "/0000.png", "delete avatars/" + anonA + "/0001.png", "mark " + anonA,
		"delete avatars/" + anonB + "/0000.png", "mark " + anonB,
	}
	if !slices.Equal(ledger.order, want) {
		t.Errorf("the sweep did %v, want each account's objects deleted before it is marked: %v", ledger.order, want)
	}
	for _, id := range []string{anonA, anonB} {
		if at := ledger.marks[id]; !at.Equal(avatarSweepNow) {
			t.Errorf("account %s marked at %v, want the clock's %v", id, at, avatarSweepNow)
		}
	}
}

func TestAnAccountWithNoPhotoIsMarkedWithoutADelete(t *testing.T) {
	ledger := &sweepLedger{due: []string{anonA}}
	bucket := &photoBucket{ledger: ledger}
	app := avatarSweepApp(t, ledger, bucket)

	got, err := app.Commands.SweepAvatarObjects.Handle(context.Background(), command.SweepAvatarObjects{Limit: 10})

	if err != nil || got != 1 || !slices.Equal(ledger.order, []string{"mark " + anonA}) {
		t.Fatalf("marked = %d, %v, did %v; want the account marked and nothing deleted", got, err, ledger.order)
	}
}

func TestTheSweepListsAgainWhileAnAccountsListComesBackFull(t *testing.T) {
	for _, n := range []int{99, 100, 101, 250} {
		ledger := &sweepLedger{due: []string{anonA}}
		bucket := &photoBucket{keys: photosOf(anonA, n)}
		app := avatarSweepApp(t, ledger, bucket)

		got, err := app.Commands.SweepAvatarObjects.Handle(context.Background(), command.SweepAvatarObjects{Limit: 10})

		if err != nil || got != 1 || len(bucket.keys) != 0 {
			t.Errorf("%d photos: marked = %d, %v, %d left; want all gone and the account marked", n, got, err, len(bucket.keys))
		}
		if want := n/100 + 1; bucket.lists != want {
			t.Errorf("%d photos: listed %d times, want %d", n, bucket.lists, want)
		}
	}
}

func TestAPrefixThatDoesNotEmptyIsAFailureAndLeavesTheAccountUnmarked(t *testing.T) {
	ledger := &sweepLedger{due: []string{anonA}}
	bucket := &photoBucket{keys: photosOf(anonA, 100), sticky: true}
	app := avatarSweepApp(t, ledger, bucket)

	got, err := app.Commands.SweepAvatarObjects.Handle(context.Background(), command.SweepAvatarObjects{Limit: 10})

	if err == nil || got != 0 || len(ledger.marks) != 0 {
		t.Fatalf("marked = %d, %v, marks %v; want an error and no mark", got, err, ledger.marks)
	}
	if bucket.lists != 10 {
		t.Errorf("listed %d times, want the bound of 10", bucket.lists)
	}
}

func TestAnAccountWhoseObjectsFailIsLeftUnmarkedAndTheRestAreSwept(t *testing.T) {
	listFailure, deleteFailure := errors.New("list refused"), errors.New("delete refused")
	ledger := &sweepLedger{due: []string{anonA, anonB, anonC}}
	bucket := &photoBucket{
		keys:      append(append(photosOf(anonA, 1), photosOf(anonB, 2)...), photosOf(anonC, 1)...),
		listErr:   map[string]error{"avatars/" + anonA + "/": listFailure},
		deleteErr: map[string]error{"avatars/" + anonB + "/0001.png": deleteFailure},
	}
	app := avatarSweepApp(t, ledger, bucket)

	got, err := app.Commands.SweepAvatarObjects.Handle(context.Background(), command.SweepAvatarObjects{Limit: 10})

	if got != 1 {
		t.Errorf("marked = %d, want only the account whose objects went", got)
	}
	if !errors.Is(err, listFailure) || errors.Is(err, deleteFailure) {
		t.Errorf("error = %v, want the first failure alone", err)
	}
	if _, ok := ledger.marks[anonC]; !ok || len(ledger.marks) != 1 {
		t.Errorf("marks = %v, want only %s", ledger.marks, anonC)
	}
}

func TestAMarkThatFailsStopsTheBatch(t *testing.T) {
	failure := errors.New("database down")
	ledger := &sweepLedger{due: []string{anonA, anonB}, markErr: failure}
	bucket := &photoBucket{ledger: ledger, keys: append(photosOf(anonA, 1), photosOf(anonB, 1)...)}
	app := avatarSweepApp(t, ledger, bucket)

	got, err := app.Commands.SweepAvatarObjects.Handle(context.Background(), command.SweepAvatarObjects{Limit: 10})

	if got != 0 || !errors.Is(err, failure) {
		t.Fatalf("marked = %d, %v, want 0 and the mark's error", got, err)
	}
	if want := []string{"delete avatars/" + anonA + "/0000.png", "mark " + anonA}; !slices.Equal(ledger.order, want) {
		t.Errorf("the sweep did %v, want it to stop at the failed mark: %v", ledger.order, want)
	}
}

func TestASweepWhoseTimeRunsOutStopsBeforeTheNextAccountAndKeepsTheMarksItMade(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ledger := &sweepLedger{due: []string{anonA, anonB}}
	bucket := &photoBucket{ledger: ledger, keys: append(photosOf(anonA, 1), photosOf(anonB, 1)...), cancel: cancel}
	app := avatarSweepApp(t, ledger, bucket)

	got, err := app.Commands.SweepAvatarObjects.Handle(ctx, command.SweepAvatarObjects{Limit: 10})

	if got != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("marked = %d, %v, want the first account marked and the context's error", got, err)
	}
	if _, ok := ledger.marks[anonB]; ok || len(bucket.keys) != 1 {
		t.Errorf("the second account was swept after the time ran out: marks %v, left %v", ledger.marks, bucket.keys)
	}
}

func TestTheSweepNeedsAPositiveLimit(t *testing.T) {
	ledger := &sweepLedger{due: []string{anonA}}
	app := avatarSweepApp(t, ledger, &photoBucket{})
	for _, limit := range []int{0, -1} {
		if _, err := app.Commands.SweepAvatarObjects.Handle(context.Background(), command.SweepAvatarObjects{Limit: limit}); err == nil {
			t.Errorf("limit %d accepted", limit)
		}
	}
	if len(ledger.asked) != 0 {
		t.Errorf("Due was asked %d times with no usable limit", len(ledger.asked))
	}
}

func TestADueThatFailsIsTheSweepsError(t *testing.T) {
	failure := errors.New("database down")
	ledger := &sweepLedger{dueErr: failure}
	app := avatarSweepApp(t, ledger, &photoBucket{})

	if got, err := app.Commands.SweepAvatarObjects.Handle(context.Background(), command.SweepAvatarObjects{Limit: 10}); got != 0 || !errors.Is(err, failure) {
		t.Fatalf("marked = %d, %v, want 0 and the store's error", got, err)
	}
}

func TestWithoutAnObjectStoreNoAccountIsMarked(t *testing.T) {
	ledger := &sweepLedger{due: []string{anonA}}
	app := avatarSweepApp(t, ledger, nil)

	got, err := app.Commands.SweepAvatarObjects.Handle(context.Background(), command.SweepAvatarObjects{Limit: 10})

	if err != nil || got != 0 || len(ledger.marks) != 0 || len(ledger.asked) != 0 {
		t.Fatalf("marked = %d, %v, marks %v, Due asked %v; want nothing read or marked", got, err, ledger.marks, ledger.asked)
	}
}

func TestWithoutTheSweepStoreThereIsNoSweepCommand(t *testing.T) {
	if application.New(nil, nil, 0, nil, nil).Commands.SweepAvatarObjects != nil {
		t.Fatal("SweepAvatarObjects is set before SetAvatarSweeps")
	}
}

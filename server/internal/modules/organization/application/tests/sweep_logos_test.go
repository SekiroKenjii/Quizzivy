package tests

import (
	"context"
	"crypto/sha256"
	"errors"
	"maps"
	"slices"
	"testing"
	"time"

	"quizzivy/internal/modules/organization/application/command"
	"quizzivy/internal/modules/organization/domain"
)

func logoNamed(seed string) (domain.Logo, string) {
	digest := sha256.Sum256([]byte(seed))
	return domain.Logo{SHA256: digest[:], Type: domain.LogoPNG}, "branding/" + hexOf(digest[:]) + ".png"
}

func sweeping(t *testing.T) (*fakeRepo, *memoryStore, func(limit int) (command.SweptLogos, error), time.Time) {
	t.Helper()
	light, _ := logoNamed("light")
	dark, _ := logoNamed("dark")
	current := stored()
	current.Profile.LightLogo, current.Profile.DarkLogo = &light, &dark
	repo := &fakeRepo{settings: current}
	app, clock := build(t, repo)
	store := newStore()
	app.WithLogos(store, fixedChecker{})
	return repo, store, func(limit int) (command.SweptLogos, error) {
		return app.Commands.SweepLogos.Handle(context.Background(), command.SweepLogos{Limit: limit})
	}, clock.Now()
}

func TestTheSweepRemovesOnlyOldLogosNoSettingNames(t *testing.T) {
	_, store, sweep, now := sweeping(t)
	_, lightKey := logoNamed("light")
	_, darkKey := logoNamed("dark")
	_, staleKey := logoNamed("replaced")
	_, freshKey := logoNamed("just uploaded")
	_, edgeKey := logoNamed("a minute over a day")
	_, almostKey := logoNamed("a minute under a day")
	_, exactKey := logoNamed("exactly a day")
	store.put(lightKey, now.Add(-400*24*time.Hour))
	store.put(darkKey, now.Add(-400*24*time.Hour))
	store.put(staleKey, now.Add(-72*time.Hour))
	store.put(freshKey, now.Add(-time.Hour))
	store.put(edgeKey, now.Add(-24*time.Hour-time.Minute))
	store.put(almostKey, now.Add(-24*time.Hour+time.Minute))
	store.put(exactKey, now.Add(-24*time.Hour))
	store.put("media/"+hexOf([]byte("elsewhere"))+".png", now.Add(-400*24*time.Hour))

	swept, err := sweep(100)
	if err != nil {
		t.Fatal(err)
	}
	if swept.Removed != 2 || swept.Seen != 7 {
		t.Errorf("swept = %+v, want 2 removed of the 7 logos seen", swept)
	}
	wantLeft := []string{almostKey, darkKey, exactKey, freshKey, lightKey, "media/" + hexOf([]byte("elsewhere")) + ".png"}
	var left []string
	for key := range store.objects {
		left = append(left, key)
	}
	slices.Sort(left)
	slices.Sort(wantLeft)
	if !slices.Equal(left, wantLeft) {
		t.Errorf("left %v, want %v: both logos in use, anything not yet more than a day old, and what is not a logo", left, wantLeft)
	}

	again, err := sweep(100)
	if err != nil || again.Removed != 0 {
		t.Errorf("a second sweep removed %d (%v), want none", again.Removed, err)
	}
}

func TestTheSweepLooksAtNoMoreThanTheLimit(t *testing.T) {
	_, store, sweep, now := sweeping(t)
	for i := range 5 {
		_, key := logoNamed("stale " + string(rune('a'+i)))
		store.put(key, now.Add(-72*time.Hour))
	}
	swept, err := sweep(3)
	if err != nil || store.listed != 3 || swept.Seen != 3 || swept.Removed != 3 {
		t.Errorf("swept = %+v (%v), listed up to %d, want 3 of 5", swept, err, store.listed)
	}
}

func TestTheSweepNeedsAPositiveLimit(t *testing.T) {
	_, store, sweep, _ := sweeping(t)
	if _, err := sweep(0); err == nil || store.listed != 0 {
		t.Errorf("a limit of 0 was accepted (%v)", err)
	}
}

func TestTheSweepWithoutStorageRemovesNothingAndIsNoError(t *testing.T) {
	app, _ := build(t, &fakeRepo{settings: stored()})
	swept, err := app.Commands.SweepLogos.Handle(context.Background(), command.SweepLogos{Limit: 10})
	if err != nil || swept != (command.SweptLogos{}) {
		t.Errorf("swept = %+v (%v), want nothing", swept, err)
	}
}

func TestTheSweepDeletesNothingWhenItCannotListOrReadTheSettings(t *testing.T) {
	repo, store, sweep, now := sweeping(t)
	_, staleKey := logoNamed("replaced")
	store.put(staleKey, now.Add(-72*time.Hour))

	boom := errors.New("the bucket is away")
	store.listErr = boom
	if _, err := sweep(10); !errors.Is(err, boom) || len(store.deleted) != 0 {
		t.Errorf("a failed list: err = %v, deleted %v", err, store.deleted)
	}
	store.listErr = nil
	repo.readErr = errors.New("the database is away")
	if _, err := sweep(10); !errors.Is(err, repo.readErr) || len(store.deleted) != 0 {
		t.Errorf("a failed read: err = %v, deleted %v", err, store.deleted)
	}
}

func TestOneLogoThatCannotBeDeletedDoesNotStopTheRest(t *testing.T) {
	_, store, sweep, now := sweeping(t)
	_, stuck := logoNamed("stuck")
	_, other := logoNamed("other")
	store.put(stuck, now.Add(-72*time.Hour))
	store.put(other, now.Add(-72*time.Hour))
	boom := errors.New("the bucket is away")
	store.deleteErr[stuck] = boom

	swept, err := sweep(10)
	if !errors.Is(err, boom) || swept.Removed != 1 || !slices.Equal(store.deleted, []string{other}) {
		t.Errorf("swept = %+v (%v), deleted %v, want the other logo gone and the bucket's error", swept, err, store.deleted)
	}
}

func TestTheSweepListsFirstAndReadsTheSettingsAgainBeforeEachRemoval(t *testing.T) {
	repo, store, sweep, now := sweeping(t)
	keyed := map[string]domain.Logo{}
	for _, seed := range []string{"first stale", "second stale"} {
		logo, key := logoNamed(seed)
		keyed[key] = logo
		store.put(key, now.Add(-72*time.Hour))
	}
	keys := slices.Sorted(maps.Keys(keyed))
	var events []string
	store.onList = func() { events = append(events, "list") }
	repo.onRead = func(reads int, settings *domain.Settings) {
		events = append(events, "read")
		if reads == 2 {
			named := keyed[keys[1]]
			settings.Profile.LightLogo = &named
		}
	}

	swept, err := sweep(10)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"list", "read", "read"}; !slices.Equal(events, want) {
		t.Errorf("calls = %v, want %v: the list, then a read of the settings before each removal", events, want)
	}
	if swept.Removed != 1 || !slices.Equal(store.deleted, []string{keys[0]}) {
		t.Errorf("swept = %+v, deleted %v, want only the first: a save named the second after the first was removed", swept, store.deleted)
	}
}

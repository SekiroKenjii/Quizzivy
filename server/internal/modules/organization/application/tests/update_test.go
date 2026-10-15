package tests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"

	"quizzivy/internal/modules/organization/application/command"
	"quizzivy/internal/modules/organization/application/query"
	"quizzivy/internal/modules/organization/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/validation"
)

var admin = actor.Actor{ID: "019535d9-3df7-79fb-b466-fa907fa17f9e", IP: "203.0.113.7", UserAgent: "test"}

func TestASaveIsNormalisedAndTheSnapshotServesItAtOnce(t *testing.T) {
	repo := &fakeRepo{settings: stored()}
	app, _ := build(t, repo)
	if _, err := app.Queries.Current.Handle(context.Background(), query.Current{}); err != nil {
		t.Fatal(err)
	}
	next := stored()
	next.Profile.Name = "  Trung tâm mới  "
	saved, err := app.Commands.UpdateSettings.Handle(context.Background(), command.UpdateSettings{Actor: admin, Settings: next, ExpectedRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Profile.Name != "Trung tâm mới" || saved.Revision != 2 || saved.UpdatedBy != admin.ID {
		t.Fatalf("saved = %+v", saved)
	}
	got, err := app.Queries.Current.Handle(context.Background(), query.Current{})
	if err != nil || got.Profile.Name != "Trung tâm mới" {
		t.Fatalf("the snapshot served %q, %v; want the saved document", got.Profile.Name, err)
	}
	if repo.readCount() != 2 {
		t.Fatalf("reads = %d, want one for the first read and one to compare logos", repo.readCount())
	}
	in := repo.saves[0]
	if in.ActorID != admin.ID || in.IP != admin.IP || in.UserAgent != admin.UserAgent || !in.GoogleConfigured {
		t.Fatalf("save = %+v", in)
	}
}

func TestAnInvalidDocumentNeverReachesTheStore(t *testing.T) {
	repo := &fakeRepo{settings: stored()}
	app, _ := build(t, repo)
	next := stored()
	next.SignIn.Google, next.SignIn.Password = false, false
	_, err := app.Commands.UpdateSettings.Handle(context.Background(), command.UpdateSettings{Actor: admin, Settings: next, ExpectedRevision: 1})
	var invalid *validation.Error
	if !errors.As(err, &invalid) || len(repo.saves) != 0 {
		t.Fatalf("err = %v, saves = %d", err, len(repo.saves))
	}
}

func TestAStaleSaveChangesNothing(t *testing.T) {
	repo := &fakeRepo{settings: stored()}
	app, _ := build(t, repo)
	_, err := app.Commands.UpdateSettings.Handle(context.Background(), command.UpdateSettings{Actor: admin, Settings: stored(), ExpectedRevision: 7})
	if !errors.Is(err, domain.ErrStale) {
		t.Fatalf("err = %v, want ErrStale", err)
	}
	got, _ := app.Queries.Current.Handle(context.Background(), query.Current{})
	if got.Revision != 1 {
		t.Fatalf("revision = %d, want 1", got.Revision)
	}
}

func TestALockoutIsReturnedWithItsCounts(t *testing.T) {
	repo := &fakeRepo{settings: stored(), impact: domain.Impact{AffectedUsers: 3, AffectedAdmins: 2, ActiveAdmins: 2}}
	app, _ := build(t, repo)
	_, err := app.Commands.UpdateSettings.Handle(context.Background(), command.UpdateSettings{Actor: admin, Settings: stored(), ExpectedRevision: 1})
	var locked *domain.LockedOut
	if !errors.As(err, &locked) || locked.Impact.AffectedUsers != 3 || !errors.Is(err, domain.ErrLockout) {
		t.Fatalf("err = %v", err)
	}
}

func TestANewLogoMustBeInTheStoreBeforeASaveNamesIt(t *testing.T) {
	repo := &fakeRepo{settings: stored()}
	app, _ := build(t, repo)
	store := newStore()
	app.WithLogos(store, fixedChecker{})

	body := []byte("\x89PNG pretend")
	digest := sha256.Sum256(body)
	next := stored()
	next.Profile.LightLogo = &domain.Logo{SHA256: digest[:], Type: domain.LogoPNG}

	_, err := app.Commands.UpdateSettings.Handle(context.Background(), command.UpdateSettings{Actor: admin, Settings: next, ExpectedRevision: 1})
	var invalid *validation.Error
	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "organization.logo.light" || invalid.Fields[0].Message != domain.ReasonLogoGone {
		t.Fatalf("err = %v, want the missing logo named", err)
	}

	if err := store.Put(context.Background(), "branding/"+hexOf(digest[:])+".png", domain.LogoPNG, body, digest[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Commands.UpdateSettings.Handle(context.Background(), command.UpdateSettings{Actor: admin, Settings: next, ExpectedRevision: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestALogoAlreadyInUseNeedsNoProbe(t *testing.T) {
	digest := sha256.Sum256([]byte("\x89PNG pretend"))
	logo := &domain.Logo{SHA256: digest[:], Type: domain.LogoPNG}
	current := stored()
	current.Profile.LightLogo = logo
	repo := &fakeRepo{settings: current}
	app, _ := build(t, repo)
	app.WithLogos(newStore(), fixedChecker{})
	next := current
	next.Profile.Name = "Renamed"
	if _, err := app.Commands.UpdateSettings.Handle(context.Background(), command.UpdateSettings{Actor: admin, Settings: next, ExpectedRevision: 1}); err != nil {
		t.Fatalf("a save that keeps the stored logo must not need it in the store again: %v", err)
	}
}

func TestALogoCannotBeNamedOnAServerWithoutStorage(t *testing.T) {
	repo := &fakeRepo{settings: stored()}
	app, _ := build(t, repo)
	next := stored()
	next.Profile.DarkLogo = &domain.Logo{SHA256: make([]byte, 32), Type: domain.LogoPNG}
	_, err := app.Commands.UpdateSettings.Handle(context.Background(), command.UpdateSettings{Actor: admin, Settings: next, ExpectedRevision: 1})
	var invalid *validation.Error
	if !errors.As(err, &invalid) || invalid.Fields[0].Message != domain.ReasonLogoStore {
		t.Fatalf("err = %v", err)
	}
}

func TestAnUploadIsStoredUnderItsHashAndChangesNoSetting(t *testing.T) {
	repo := &fakeRepo{settings: stored()}
	app, _ := build(t, repo)
	store := newStore()
	png := []byte("\x89PNG pretend")
	app.WithLogos(store, fixedChecker{file: domain.LogoFile{Bytes: png, Type: domain.LogoPNG, Width: 512, Height: 512}})

	up, err := app.Commands.UploadLogo.Handle(context.Background(), command.UploadLogo{Body: bytes.NewReader([]byte("raw upload"))})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(png)
	if !bytes.Equal(up.Logo.SHA256, digest[:]) || up.Logo.Type != domain.LogoPNG || up.Width != 512 || up.Height != 512 {
		t.Fatalf("upload = %+v", up)
	}
	if _, ok := store.objects["branding/"+hexOf(digest[:])+".png"]; !ok {
		t.Fatalf("objects = %v", store.objects)
	}
	if len(repo.saves) != 0 {
		t.Fatal("an upload alone must not save the settings")
	}
}

func TestARefusedUploadStoresNothing(t *testing.T) {
	app, _ := build(t, &fakeRepo{settings: stored()})
	store := newStore()
	app.WithLogos(store, fixedChecker{err: domain.ErrLogoUnsupported})
	_, err := app.Commands.UploadLogo.Handle(context.Background(), command.UploadLogo{Body: bytes.NewReader([]byte("x"))})
	if !errors.Is(err, domain.ErrLogoUnsupported) || store.puts != 0 {
		t.Fatalf("err = %v, puts = %d", err, store.puts)
	}
}

func TestAnUploadWithoutStorageSaysSo(t *testing.T) {
	app, _ := build(t, &fakeRepo{settings: stored()})
	_, err := app.Commands.UploadLogo.Handle(context.Background(), command.UploadLogo{Body: bytes.NewReader(nil)})
	if !errors.Is(err, domain.ErrStorageOff) {
		t.Fatalf("err = %v", err)
	}
}

func TestTheLogoQueryReadsTheStoreOncePerLogo(t *testing.T) {
	digest := sha256.Sum256([]byte("\x89PNG pretend"))
	current := stored()
	current.Profile.LightLogo = &domain.Logo{SHA256: digest[:], Type: domain.LogoPNG}
	repo := &fakeRepo{settings: current}
	app, _ := build(t, repo)
	store := newStore()
	app.WithLogos(store, fixedChecker{})
	if err := store.Put(context.Background(), "branding/"+hexOf(digest[:])+".png", domain.LogoPNG, []byte("\x89PNG pretend"), digest[:]); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		got, err := app.Queries.Logo.Handle(context.Background(), query.Logo{Variant: domain.Light})
		if err != nil || string(got.Bytes) != "\x89PNG pretend" || !got.Current(digest[:]) {
			t.Fatalf("logo = %+v, %v", got, err)
		}
	}
	if store.gets != 1 {
		t.Fatalf("store reads = %d, want 1", store.gets)
	}
	if _, err := app.Queries.Logo.Handle(context.Background(), query.Logo{Variant: domain.Dark}); !errors.Is(err, domain.ErrNoLogo) {
		t.Fatalf("dark = %v, want ErrNoLogo", err)
	}
}

func TestSignInImpactNormalisesDomainsAndAsksTheStore(t *testing.T) {
	repo := &fakeRepo{settings: stored(), impact: domain.Impact{AffectedUsers: 4, AffectedAdmins: 0, ActiveAdmins: 2}}
	app, _ := build(t, repo)
	got, err := app.Queries.SignInImpact.Handle(context.Background(), query.SignInImpact{ActorID: admin.ID, Policy: domain.SignIn{Google: true, AllowedGoogleDomains: []string{" Quizzivy.com "}}})
	if err != nil || got.AffectedUsers != 4 || got.Lockout() {
		t.Fatalf("impact = %+v, %v", got, err)
	}
	if !repo.saves[0].GoogleConfigured {
		t.Fatal("the impact must know that Google is configured on this server")
	}
}

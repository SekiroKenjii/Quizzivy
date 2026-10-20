package tests

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/organization/application/query"
)

func TestASecondReadIsServedWithoutAQuery(t *testing.T) {
	repo := &fakeRepo{settings: stored()}
	app, _ := build(t, repo)
	for range 3 {
		if _, err := app.Queries.Current.Handle(context.Background(), query.Current{}); err != nil {
			t.Fatal(err)
		}
	}
	if repo.readCount() != 1 {
		t.Fatalf("reads = %d, want 1", repo.readCount())
	}
}

func TestTheSnapshotRefreshesAfterThirtySeconds(t *testing.T) {
	repo := &fakeRepo{settings: stored()}
	app, clock := build(t, repo)
	read := func() string {
		got, err := app.Queries.Current.Handle(context.Background(), query.Current{})
		if err != nil {
			t.Fatal(err)
		}
		return got.Profile.Name
	}
	read()
	repo.mu.Lock()
	repo.settings.Profile.Name = "Saved on another server"
	repo.mu.Unlock()

	clock.advance(29 * time.Second)
	if got := read(); got != "Quizzivy" || repo.readCount() != 1 {
		t.Fatalf("after 29s: name %q, reads %d; want the remembered document", got, repo.readCount())
	}
	clock.advance(2 * time.Second)
	if got := read(); got != "Saved on another server" || repo.readCount() != 2 {
		t.Fatalf("after 31s: name %q, reads %d; want a fresh read", got, repo.readCount())
	}
}

func TestAnIdleServerNeverQueriesTheDatabase(t *testing.T) {
	repo := &fakeRepo{settings: stored()}
	_, clock := build(t, repo)
	clock.advance(time.Hour)
	if repo.readCount() != 0 {
		t.Fatalf("reads = %d, want none until someone asks", repo.readCount())
	}
}

func TestConcurrentReadersShareOneRefresh(t *testing.T) {
	repo := &fakeRepo{settings: stored(), block: make(chan struct{})}
	app, _ := build(t, repo)
	var done sync.WaitGroup
	for range 20 {
		done.Add(1)
		go func() {
			defer done.Done()
			if _, err := app.Queries.Current.Handle(context.Background(), query.Current{}); err != nil {
				t.Error(err)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(repo.block)
	done.Wait()
	if repo.readCount() != 1 {
		t.Fatalf("reads = %d, want 1", repo.readCount())
	}
}

func TestAFailedRefreshKeepsTheLastDocument(t *testing.T) {
	repo := &fakeRepo{settings: stored()}
	app, clock := build(t, repo)
	if _, err := app.Queries.Current.Handle(context.Background(), query.Current{}); err != nil {
		t.Fatal(err)
	}
	repo.mu.Lock()
	repo.readErr = errors.New("neon is waking up")
	repo.mu.Unlock()
	clock.advance(time.Minute)
	got, err := app.Queries.Current.Handle(context.Background(), query.Current{})
	if err != nil || got.Profile.Name != "Quizzivy" {
		t.Fatalf("got %q, %v; want the last document and no error", got.Profile.Name, err)
	}
	if _, err := app.Queries.Current.Handle(context.Background(), query.Current{}); err != nil || repo.readCount() != 2 {
		t.Fatalf("a failed refresh must not be retried on every request: reads = %d, err = %v", repo.readCount(), err)
	}
}

func TestAFirstReadThatFailsIsReturned(t *testing.T) {
	repo := &fakeRepo{settings: stored(), readErr: errors.New("down")}
	app, _ := build(t, repo)
	if _, err := app.Queries.Current.Handle(context.Background(), query.Current{}); err == nil {
		t.Fatal("a server with nothing remembered must not invent a document")
	}
}

func TestTheSignInPolicyAndBrandingComeFromTheSnapshot(t *testing.T) {
	s := stored()
	s.SignIn.Password = false
	s.SignIn.AllowedGoogleDomains = []string{"quizzivy.com"}
	s.Profile.ContactEmail = "hello@quizzivy.com"
	repo := &fakeRepo{settings: s}
	app, _ := build(t, repo)
	policy, err := app.Queries.SignInPolicy.Handle(context.Background(), query.SignInPolicy{})
	if err != nil || policy.Password || !policy.Google || !policy.AllowsGoogleEmail("A@Quizzivy.com") || policy.AllowsGoogleEmail("a@gmail.com") {
		t.Fatalf("policy = %+v, %v", policy, err)
	}
	branding, err := app.Queries.Branding.Handle(context.Background(), query.Branding{})
	if err != nil || branding.Name != "Quizzivy" || branding.ContactEmail != "hello@quizzivy.com" {
		t.Fatalf("branding = %+v, %v", branding, err)
	}
	if repo.readCount() != 1 {
		t.Fatalf("reads = %d, want one shared by both", repo.readCount())
	}
}

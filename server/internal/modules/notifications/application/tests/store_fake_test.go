package application_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"quizzivy/internal/modules/notifications/application"
	"quizzivy/internal/modules/notifications/domain"
)

type fakeStore struct {
	t       *testing.T
	allowed []string
	calls   int

	notice  domain.Notice
	query   domain.ListQuery
	userID  string
	ids     []string
	saved   []domain.Preference
	cutoff  time.Time
	stored  []domain.Preference
	page    domain.Page
	unread  int
	deleted int64

	due       []domain.Notice
	dueAt     time.Time
	dueErr    error
	inserted  []domain.Notice
	insertErr error
}

func only(t *testing.T, methods ...string) *fakeStore {
	t.Helper()
	return &fakeStore{t: t, allowed: methods}
}

func (f *fakeStore) call(method string) {
	f.t.Helper()
	f.calls++
	if !slices.Contains(f.allowed, method) {
		f.t.Errorf("the handler called %s; it may only call %q", method, f.allowed)
	}
}

func (f *fakeStore) Upsert(_ context.Context, n domain.Notice) (bool, error) {
	f.call("Upsert")
	f.notice = n
	return true, nil
}

func (f *fakeStore) List(_ context.Context, q domain.ListQuery) (domain.Page, error) {
	f.call("List")
	f.query = q
	return f.page, nil
}

func (f *fakeStore) MarkRead(_ context.Context, userID string, ids []string) error {
	f.call("MarkRead")
	f.userID, f.ids = userID, ids
	return nil
}

func (f *fakeStore) MarkAllRead(_ context.Context, userID string) error {
	f.call("MarkAllRead")
	f.userID = userID
	return nil
}

func (f *fakeStore) Unread(_ context.Context, userID string) (int, error) {
	f.call("Unread")
	f.userID = userID
	return f.unread, nil
}

func (f *fakeStore) Preferences(_ context.Context, userID string) ([]domain.Preference, error) {
	f.call("Preferences")
	f.userID = userID
	return f.stored, nil
}

func (f *fakeStore) SavePreferences(_ context.Context, userID string, prefs []domain.Preference) ([]domain.Preference, error) {
	f.call("SavePreferences")
	f.userID, f.saved = userID, prefs
	answered := slices.Clone(prefs)
	slices.Reverse(answered)
	return answered, nil
}

func (f *fakeStore) DeleteBefore(_ context.Context, cutoff time.Time) (int64, error) {
	f.call("DeleteBefore")
	f.cutoff = cutoff
	return f.deleted, nil
}

func (f *fakeStore) Due(_ context.Context, userID string, now time.Time) ([]domain.Notice, error) {
	f.call("Due")
	f.userID, f.dueAt = userID, now
	return f.due, f.dueErr
}

func (f *fakeStore) InsertAbsent(_ context.Context, userID string, notices []domain.Notice) (int, error) {
	f.call("InsertAbsent")
	f.userID, f.inserted = userID, notices
	return len(notices), f.insertErr
}

func over(store *fakeStore) *application.Application {
	return application.New(store)
}

const (
	reader     = "01935000-0000-7000-8000-0000000000c3"
	assignment = "01935000-0000-7000-8000-00000000a551"
)

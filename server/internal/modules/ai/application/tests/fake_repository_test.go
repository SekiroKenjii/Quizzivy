package application_test

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/ai/application"
	"quizzivy/internal/modules/ai/domain"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

var testPrices = domain.Prices{
	Fast:    domain.Price{Input: 100_000, Output: 500_000, CacheRead: 10_000},
	Capable: domain.Price{Input: 1_000_000_000, Output: 2_000_000_000, CacheRead: 100_000_000},
}

var saigon = time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)

type fakeRepository struct {
	mu sync.Mutex

	reserved   []domain.ReserveInput
	settled    []domain.Settlement
	released   []domain.Reservation
	staleSince []time.Time
	allowances []domain.SetAllowanceInput
	additions  []domain.AddCreditsInput
	rules      []domain.SetRulesInput
	filed      []domain.CreateRequestInput
	decisions  []domain.DecideInput

	reserveErr  error
	settleOut   domain.Settled
	settleErr   error
	releaseErr  error
	filedOut    domain.Filed
	filedErr    error
	decidedOut  domain.Decided
	decidedErr  error
	adminOut    domain.AdminCredits
	currentRule domain.Rules
	usageOut    domain.MonthUsage
	owners      map[string]string
}

func (f *fakeRepository) Reserve(_ context.Context, in domain.ReserveInput) (domain.Reservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reserved = append(f.reserved, in)
	if f.reserveErr != nil {
		return domain.Reservation{}, f.reserveErr
	}
	return domain.Reservation{ID: "r-1", Month: in.Month, UserID: in.UserID, Feature: in.Feature, Credits: in.Credits}, nil
}

func (f *fakeRepository) Settle(_ context.Context, s domain.Settlement) (domain.Settled, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.settled = append(f.settled, s)
	if f.settleErr != nil {
		return domain.Settled{}, f.settleErr
	}
	out := f.settleOut
	if out == (domain.Settled{}) {
		out = domain.Settled{Charged: s.Metered, Metered: s.Metered}
	}
	return out, nil
}

func (f *fakeRepository) Release(_ context.Context, r domain.Reservation) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = append(f.released, r)
	return f.releaseErr == nil, f.releaseErr
}

func (f *fakeRepository) ReleaseStale(_ context.Context, cutoff time.Time) (domain.Released, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.staleSince = append(f.staleSince, cutoff)
	return domain.Released{Reservations: 2, Credits: 300}, nil
}

func (f *fakeRepository) Usage(_ context.Context, month time.Time, _ string) (domain.MonthUsage, error) {
	out := f.usageOut
	out.Month = month
	return out, nil
}

func (f *fakeRepository) RequestOwner(_ context.Context, id string) (string, error) {
	if owner, ok := f.owners[id]; ok {
		return owner, nil
	}
	return teacherID, nil
}

func (f *fakeRepository) Admin(_ context.Context, month time.Time) (domain.AdminCredits, error) {
	out := f.adminOut
	out.Month = month
	return out, nil
}

func (f *fakeRepository) Rules(context.Context) (domain.Rules, error) { return f.currentRule, nil }

func (f *fakeRepository) SetAllowance(_ context.Context, in domain.SetAllowanceInput) (domain.TeacherMonth, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.allowances = append(f.allowances, in)
	return domain.TeacherMonth{UserID: in.UserID, Allowance: 20_000, Used: 16_000}, nil
}

func (f *fakeRepository) AddCredits(_ context.Context, in domain.AddCreditsInput) (domain.Pool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.additions = append(f.additions, in)
	return domain.Pool{Plan: 200_000, Added: in.Credits}, nil
}

func (f *fakeRepository) SetRules(_ context.Context, in domain.SetRulesInput) (domain.RulesChange, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rules = append(f.rules, in)
	return domain.RulesChange{}, nil
}

func (f *fakeRepository) CreateRequest(_ context.Context, in domain.CreateRequestInput) (domain.Filed, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.filed = append(f.filed, in)
	return f.filedOut, f.filedErr
}

func (f *fakeRepository) DecideRequest(_ context.Context, in domain.DecideInput) (domain.Decided, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.decisions = append(f.decisions, in)
	return f.decidedOut, f.decidedErr
}

var _ domain.Repository = (*fakeRepository)(nil)

type recordingNotifier struct {
	mu   sync.Mutex
	sent []notificationscommand.Notify
}

func (n *recordingNotifier) Handle(_ context.Context, cmd notificationscommand.Notify) (cqrs.Nothing, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, cmd)
	return cqrs.Nothing{}, nil
}

type fixture struct {
	repo     *fakeRepository
	app      *application.Application
	notifier *recordingNotifier
	logs     *bytes.Buffer
}

var september = time.Date(2090, 9, 24, 5, 0, 0, 0, saigon)

func newFixture(t *testing.T) fixture {
	t.Helper()
	repo := &fakeRepository{currentRule: domain.Rules{PoolCredits: 200_000, DefaultAllowance: 20_000, WarnPercent: 80, OnTeacherLimit: domain.LimitRequest}}
	notifier := &recordingNotifier{}
	logs := &bytes.Buffer{}
	app := application.New(repo, application.Options{
		Prices: testPrices, Zone: saigon, Notifier: notifier, Logger: slog.New(slog.NewTextHandler(logs, nil)),
		People: everyoneAlike{},
	})
	app.SetClock(func() time.Time { return september })
	return fixture{repo: repo, app: app, notifier: notifier, logs: logs}
}

type everyoneAlike struct{}

func (everyoneAlike) Preferences(context.Context, string) (domain.Preferences, error) {
	return domain.Preferences{Language: domain.English, ReadStudentAnswers: true}, nil
}

func (everyoneAlike) Permissions(context.Context, string) (access.Set, error) {
	return access.Set{}, nil
}

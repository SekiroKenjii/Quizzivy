package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/dashboard/application"
	"quizzivy/internal/modules/dashboard/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	notificationsquery "quizzivy/internal/modules/notifications/application/query"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/paging"
	"testing"
	"time"
)

type store struct {
	t        *testing.T
	home     domain.HomeQuery
	scope    access.Scope
	now      time.Time
	canGrade bool
	counts   int
}

func (s *store) Summary(_ context.Context, scope access.Scope) (domain.Summary, error) {
	s.scope = scope
	return domain.Summary{}, nil
}
func (s *store) Home(_ context.Context, q domain.HomeQuery) (domain.Home, error) {
	s.home = q
	return domain.Home{}, nil
}
func (*store) List(context.Context, domain.ListQuery) ([]domain.Recent, paging.Page, error) {
	return nil, paging.Page{}, nil
}
func (s *store) LiveAssignments(_ context.Context, scope access.Scope, now time.Time) (int, error) {
	s.scope = scope
	s.now = now
	return 3, nil
}
func (s *store) AnswersToGrade(_ context.Context, scope access.Scope) (int, error) {
	if !s.canGrade {
		s.t.Fatal("grading count called without permission")
	}
	s.scope = scope
	s.counts++
	return 0, nil
}

type zones struct {
	got *string
	err error
}

func (z zones) ZoneOf(_ context.Context, id string) (string, error) {
	*z.got = id
	return "Asia/Tokyo", z.err
}

func TestCalendarInputsReachTheStore(t *testing.T) {
	for _, test := range []struct {
		value string
		days  int
	}{{"", 14}, {"7d", 7}, {"14d", 14}, {"30d", 30}} {
		t.Run(test.value, func(t *testing.T) {
			repo := &store{t: t}
			app := application.New(repo)
			now := time.Date(2026, 10, 5, 1, 2, 3, 0, time.UTC)
			app.SetClock(func() time.Time { return now })
			scope := access.Scope{UserID: "teacher", All: true}
			if _, err := app.Queries.Summary.Handle(context.Background(), query.Summary{Scope: scope, Range: test.value}); err != nil {
				t.Fatal(err)
			}
			if repo.home.Now != now || repo.home.Zone != "Asia/Ho_Chi_Minh" || repo.home.Days != test.days || repo.home.Scope != scope || repo.scope != scope {
				t.Fatalf("calendar input = %+v, legacy scope=%+v", repo.home, repo.scope)
			}
			var got string
			app.WithZones(zones{got: &got})
			if _, err := app.Queries.Summary.Handle(context.Background(), query.Summary{Scope: scope}); err != nil {
				t.Fatal(err)
			}
			if got != "teacher" || repo.home.Zone != "Asia/Tokyo" {
				t.Fatalf("zone caller=%q, input=%+v", got, repo.home)
			}
		})
	}
}

func TestZoneErrorsAreNotHidden(t *testing.T) {
	var got string
	expected := errors.New("zone unavailable")
	app := application.New(&store{t: t}).WithZones(zones{got: &got, err: expected})
	if _, err := app.Queries.Summary.Handle(context.Background(), query.Summary{}); !errors.Is(err, expected) {
		t.Fatalf("zone error=%v", err)
	}
}

func TestNavSkipsForbiddenWorkAndUsesTheCallersNotifications(t *testing.T) {
	for _, canGrade := range []bool{false, true} {
		t.Run(map[bool]string{false: "without grading", true: "with grading"}[canGrade], func(t *testing.T) {
			repo := &store{t: t, canGrade: canGrade}
			now := time.Date(2026, 10, 5, 1, 0, 0, 0, time.UTC)
			var recipient string
			app := application.New(repo).WithNotifications(cqrs.HandlerFunc[notificationsquery.Summary, notificationsdomain.Summary](func(_ context.Context, q notificationsquery.Summary) (notificationsdomain.Summary, error) {
				recipient = q.UserID
				return notificationsdomain.Summary{UnreadNotifications: 7}, nil
			}))
			app.SetClock(func() time.Time { return now })
			scope := access.Scope{UserID: "teacher"}
			out, err := app.Queries.Nav.Handle(context.Background(), query.Nav{Scope: scope, CanGrade: canGrade})
			if err != nil {
				t.Fatal(err)
			}
			if out.LiveAssignments == nil || *out.LiveAssignments != 3 || out.UnreadNotifications != 7 || recipient != "teacher" || repo.now != now || repo.scope != scope {
				t.Fatalf("nav=%+v recipient=%q", out, recipient)
			}
			if canGrade {
				if out.AnswersToGrade == nil || *out.AnswersToGrade != 0 || repo.counts != 1 {
					t.Fatalf("permitted zero=%+v counts=%d", out, repo.counts)
				}
			} else if out.AnswersToGrade != nil || repo.counts != 0 {
				t.Fatalf("forbidden count=%+v", out)
			}
		})
	}
}

func TestNavNeedsItsNotificationsPort(t *testing.T) {
	app := application.New(&store{t: t})
	if _, err := app.Queries.Nav.Handle(context.Background(), query.Nav{}); !errors.Is(err, domain.ErrNotificationsUnavailable) {
		t.Fatalf("missing port=%v", err)
	}
}

func TestNotificationErrorsAreNotHidden(t *testing.T) {
	expected := errors.New("notifications unavailable")
	app := application.New(&store{t: t}).WithNotifications(cqrs.HandlerFunc[notificationsquery.Summary, notificationsdomain.Summary](func(context.Context, notificationsquery.Summary) (notificationsdomain.Summary, error) {
		return notificationsdomain.Summary{}, expected
	}))
	if _, err := app.Queries.Nav.Handle(context.Background(), query.Nav{}); !errors.Is(err, expected) {
		t.Fatalf("port error=%v", err)
	}
}

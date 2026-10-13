package application_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"quizzivy/internal/modules/ai/application"
	"quizzivy/internal/modules/ai/application/command"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/shared/access"
)

const (
	adminID  = "0198f5f0-0000-7000-8000-0000000000a1"
	otherID  = "0198f5f0-0000-7000-8000-0000000000a2"
	ghostID  = "0198f5f0-0000-7000-8000-0000000000a3"
	requests = "0198f5f0-0000-7000-8000-0000000000b1"
)

type permissionsOf map[string]access.Set

func (p permissionsOf) Preferences(context.Context, string) (domain.Preferences, error) {
	return domain.Preferences{Language: domain.English}, nil
}

func (p permissionsOf) Permissions(_ context.Context, id string) (access.Set, error) {
	set, ok := p[id]
	if !ok {
		return access.Set{}, domain.ErrNotFound
	}
	return set, nil
}

func subsetFixture(t *testing.T) (fixture, permissionsOf) {
	t.Helper()
	people := permissionsOf{
		teacherID: access.NewSet(access.AIUse, access.ContentTestsWrite, access.LearningTakeTests),
		adminID:   access.NewSet(access.AIUse, access.ContentTestsWrite, access.AICreditsManage, access.PeopleUsersManage),
		otherID:   access.NewSet(access.AIUse),
	}
	f := newFixture(t)
	repo := f.repo
	repo.owners = map[string]string{requests: teacherID}
	f.app = application.New(repo, application.Options{
		Prices: testPrices, Zone: saigon, Notifier: f.notifier, Logger: slog.New(slog.DiscardHandler), People: people,
	})
	f.app.SetClock(func() time.Time { return september })
	return f, people
}

func TestAnAuthorMayActOnATeacherWhosePermissionsAreASubsetOfTheirs(t *testing.T) {
	f, _ := subsetFixture(t)
	manager := domain.Author{UserID: adminID, Grants: access.NewSet(access.AIUse, access.ContentTestsWrite, access.AICreditsManage, access.PeopleUsersManage)}
	paused := true

	if _, err := f.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: manager, UserID: teacherID, Paused: &paused}); err != nil {
		t.Fatalf("pausing a teacher whose keys the author holds: %v", err)
	}
	if len(f.repo.allowances) != 1 {
		t.Errorf("the store was asked %d times, want once", len(f.repo.allowances))
	}
}

func TestAnAuthorMayNotChangeATeacherHoldingWhatTheyDoNot(t *testing.T) {
	f, _ := subsetFixture(t)
	narrow := domain.Author{UserID: otherID, Grants: access.NewSet(access.AICreditsManage)}
	yes, paused := true, true

	for name, run := range map[string]func() error{
		"pause": func() error {
			_, err := f.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: narrow, UserID: adminID, Paused: &paused})
			return err
		},
		"resume": func() error {
			resumed := false
			_, err := f.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: narrow, UserID: adminID, Paused: &resumed})
			return err
		},
		"allowance": func() error {
			_, err := f.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: narrow, UserID: adminID, Custom: &yes, Credits: 30_000})
			return err
		},
		"approval": func() error {
			f.repo.owners[requests] = adminID
			_, err := f.app.Commands.DecideRequest.Handle(context.Background(), command.DecideRequest{By: narrow, ID: requests, Approve: true})
			return err
		},
		"decline": func() error {
			f.repo.owners[requests] = adminID
			_, err := f.app.Commands.DecideRequest.Handle(context.Background(), command.DecideRequest{By: narrow, ID: requests})
			return err
		},
	} {
		if err := run(); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("%s: err = %v, want ErrForbidden", name, err)
		}
	}
	if len(f.repo.allowances) != 0 || len(f.repo.decisions) != 0 {
		t.Errorf("a refused change reached the store: %d allowances, %d decisions", len(f.repo.allowances), len(f.repo.decisions))
	}
	if len(f.notifier.sent) != 0 {
		t.Errorf("a refused decision told %d people", len(f.notifier.sent))
	}
}

func TestTakingTestsGivesNoPowerOverAnyoneSoItIsLeftOutOfTheComparison(t *testing.T) {
	f, _ := subsetFixture(t)
	narrow := domain.Author{UserID: otherID, Grants: access.NewSet(access.AIUse, access.ContentTestsWrite, access.AICreditsManage)}
	paused := true

	if _, err := f.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: narrow, UserID: teacherID, Paused: &paused}); err != nil {
		t.Fatalf("a teacher who also holds learning.take_tests: %v", err)
	}
}

func TestADecisionAsksWhoseRequestItIsBeforeTheSubsetRule(t *testing.T) {
	f, _ := subsetFixture(t)
	manager := domain.Author{UserID: adminID, Grants: access.NewSet(access.AIUse, access.ContentTestsWrite, access.AICreditsManage, access.PeopleUsersManage)}

	if _, err := f.app.Commands.DecideRequest.Handle(context.Background(), command.DecideRequest{By: manager, ID: requests, Approve: true}); err != nil {
		t.Fatal(err)
	}
	if len(f.repo.decisions) != 1 || !f.repo.decisions[0].Approve {
		t.Errorf("decisions %+v", f.repo.decisions)
	}
}

func TestATeacherWhoDoesNotExistIsNotATeacherAndNotAForbiddenOne(t *testing.T) {
	f, _ := subsetFixture(t)
	manager := domain.Author{UserID: adminID, Grants: access.NewSet(access.AICreditsManage)}
	paused := true

	if _, err := f.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: manager, UserID: ghostID, Paused: &paused}); !errors.Is(err, domain.ErrNotATeacher) {
		t.Errorf("err = %v, want ErrNotATeacher", err)
	}
}

func TestTheConsoleActingAsSystemIsNotAskedTheSubsetRule(t *testing.T) {
	f, _ := subsetFixture(t)
	paused := true

	if _, err := f.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: domain.Author{Operator: "ops@example.test"}, UserID: adminID, Paused: &paused}); err != nil {
		t.Fatalf("System pausing an Admin: %v", err)
	}
}

func TestWithNoWayToReadAPermissionTheSubsetRuleRefusesAndDoesNotGuess(t *testing.T) {
	f := newFixture(t)
	app := application.New(f.repo, application.Options{Prices: testPrices, Zone: saigon})
	paused := true

	_, err := app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: domain.Author{UserID: adminID, Grants: access.NewSet(access.AICreditsManage)}, UserID: teacherID, Paused: &paused})
	if err == nil || errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("err = %v, want a failure that is not an answer about the teacher", err)
	}
	if len(f.repo.allowances) != 0 {
		t.Error("the change reached the store")
	}
}

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/dashboard/application"
	"quizzivy/internal/modules/dashboard/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/shared/access"
	"reflect"
	"testing"
	"time"
)

type destinationStore struct {
	*store
	seen   domain.SummaryQuery
	result domain.Summary
	err    error
}

func (s *destinationStore) Summary(_ context.Context, q domain.SummaryQuery) (domain.Summary, error) {
	s.seen = q
	return s.result, s.err
}

func TestDestinationPermissionAndCalendarInputsReachThePublicRepository(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		t.Run(map[bool]string{false: "denied", true: "allowed"}[allowed], func(t *testing.T) {
			pair := &domain.FlaggedAttempt{AssignmentID: "assignment", AttemptID: "attempt"}
			repo := &destinationStore{store: &store{t: t}, result: domain.Summary{FlaggedAttempts: 4, NewestFlaggedAttempt: pair}}
			app := application.New(repo)
			now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
			app.SetClock(func() time.Time { return now })
			scope := access.Scope{UserID: "teacher", All: true}
			out, err := app.Queries.Summary.Handle(context.Background(), query.Summary{Scope: scope, Range: "30d", CanReviewFlagged: allowed})
			if err != nil {
				t.Fatal(err)
			}
			if repo.seen != (domain.SummaryQuery{Scope: scope, CanReviewFlagged: allowed}) || repo.home.Scope != scope || repo.home.Now != now || repo.home.Days != 30 || repo.home.Zone != "Asia/Ho_Chi_Minh" {
				t.Fatalf("summary=%+v home=%+v", repo.seen, repo.home)
			}
			if out.FlaggedAttempts != 4 || !reflect.DeepEqual(out.NewestFlaggedAttempt, pair) {
				t.Fatalf("result=%+v", out)
			}
		})
	}
}
func TestDestinationRepositoryFailureReachesThePublicCaller(t *testing.T) {
	expected := errors.New("destination query failed")
	app := application.New(&destinationStore{store: &store{t: t}, err: expected})
	if _, err := app.Queries.Summary.Handle(context.Background(), query.Summary{CanReviewFlagged: true}); !errors.Is(err, expected) {
		t.Fatalf("error=%v", err)
	}
}

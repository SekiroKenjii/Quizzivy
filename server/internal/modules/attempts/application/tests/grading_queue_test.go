package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	testsquery "quizzivy/internal/modules/tests/application/query"
	testsdomain "quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"reflect"
	"testing"
)

type queueRepo struct {
	domain.ReviewRepository
	result      domain.GradingQueue
	err         error
	seen        domain.GradingQueueQuery
	transcripts int
}

func (r *queueRepo) GradingQueue(_ context.Context, q domain.GradingQueueQuery) (domain.GradingQueue, error) {
	r.seen = q
	return r.result, r.err
}
func (r *queueRepo) GroupTranscripts(context.Context, string) (map[string]string, error) {
	r.transcripts++
	return map[string]string{"recording": "Teacher transcript", "other": "Private unrelated"}, nil
}

func TestGradingQueueQueryPreservesFiltersScopeAndErrors(t *testing.T) {
	for _, mode := range []string{"", "student", "question"} {
		t.Run("mode="+mode, func(t *testing.T) {
			assignment, student := "assignment", "student"
			repo := &queueRepo{result: domain.GradingQueue{AnswersRemaining: 201, StudentsWaiting: 2}}
			app := application.New(nil, repo, nil)
			want := query.GradingQueue{Mode: mode, Scope: access.Scope{UserID: "teacher", All: true}, AssignmentID: &assignment, StudentID: &student}
			got, err := app.Queries.GradingQueue.Handle(context.Background(), want)
			if err != nil {
				t.Fatal(err)
			}
			if want.Mode == "" {
				want.Mode = "student"
			}
			if !reflect.DeepEqual(repo.seen, want) || got.AnswersRemaining != 201 || got.StudentsWaiting != 2 {
				t.Fatalf("query/result changed: %+v %+v", repo.seen, got)
			}
			repo.err = domain.ErrPaperNotFound
			if _, err := app.Queries.GradingQueue.Handle(context.Background(), want); !errors.Is(err, domain.ErrPaperNotFound) {
				t.Fatalf("repository error: %v", err)
			}
		})
	}
}

func TestGradingQueueQueryCachesOnlyRelevantFrozenContext(t *testing.T) {
	repo := &queueRepo{result: domain.GradingQueue{Items: []domain.GradingQueueItem{
		{VersionID: "version", Question: domain.ReviewQuestion{GroupID: "group"}},
		{VersionID: "version", Question: domain.ReviewQuestion{GroupID: "group"}},
		{VersionID: "version", Question: domain.ReviewQuestion{}},
	}}}
	calls := 0
	app := application.New(nil, repo, nil).WithGroupContexts(cqrs.HandlerFunc[testsquery.GroupContexts, []testsdomain.PreviewGroup](func(_ context.Context, q testsquery.GroupContexts) ([]testsdomain.PreviewGroup, error) {
		calls++
		if q.VersionID != "version" {
			t.Fatalf("wrong version: %+v", q)
		}
		return []testsdomain.PreviewGroup{{ID: "group", Recordings: []testsdomain.PreviewRecording{{ID: "recording"}}}, {ID: "unrelated", Recordings: []testsdomain.PreviewRecording{{ID: "other"}}}}, nil
	}))
	out, err := app.Queries.GradingQueue.Handle(context.Background(), query.GradingQueue{})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || repo.transcripts != 1 || out.Items[0].SharedContext != out.Items[1].SharedContext || out.Items[2].SharedContext != nil {
		t.Fatalf("context cache/plain-item boundary: calls=%d reads=%d %+v", calls, repo.transcripts, out)
	}
	shared := out.Items[0].SharedContext
	if len(shared.Groups) != 1 || shared.Groups[0].ID != "group" || len(shared.Transcripts) != 1 || shared.Transcripts["recording"] != "Teacher transcript" || shared.AudioPlays != nil {
		t.Fatalf("unrelated/cross-attempt context: %+v", shared)
	}
}

func TestGradingQueueMissingSharedServiceFailsClosed(t *testing.T) {
	repo := &queueRepo{result: domain.GradingQueue{Items: []domain.GradingQueueItem{{Question: domain.ReviewQuestion{GroupID: "group"}}}}}
	app := application.New(nil, repo, nil)
	if _, err := app.Queries.GradingQueue.Handle(context.Background(), query.GradingQueue{}); !errors.Is(err, domain.ErrGroupContextUnavailable) {
		t.Fatalf("nil port=%v", err)
	}
	app.WithGroupContexts(cqrs.HandlerFunc[testsquery.GroupContexts, []testsdomain.PreviewGroup](func(context.Context, testsquery.GroupContexts) ([]testsdomain.PreviewGroup, error) {
		return []testsdomain.PreviewGroup{{ID: "unrelated"}}, nil
	}))
	if _, err := app.Queries.GradingQueue.Handle(context.Background(), query.GradingQueue{}); !errors.Is(err, domain.ErrGroupContextUnavailable) {
		t.Fatalf("missing represented group=%v", err)
	}
}

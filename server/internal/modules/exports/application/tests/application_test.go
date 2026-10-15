package application_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"quizzivy/internal/modules/exports/application"
	"quizzivy/internal/modules/exports/application/command"
	"quizzivy/internal/modules/exports/application/query"
	"quizzivy/internal/modules/exports/domain"
	"quizzivy/internal/shared/actor"
)

type repo struct {
	requested []domain.Request
	fetched   []domain.Fetch
	got       []string
	requestEr error
	fetchEr   error
	created   time.Time
}

func (r *repo) Request(_ context.Context, in domain.Request) (domain.Export, error) {
	r.requested = append(r.requested, in)
	if r.requestEr != nil {
		return domain.Export{}, r.requestEr
	}
	return domain.Export{ID: "export-1", RequestedBy: in.ActorID, IncludeAnswers: in.IncludeAnswers, Status: domain.Queued}, nil
}

func (r *repo) Get(_ context.Context, id, requester string) (domain.Export, error) {
	r.got = append(r.got, id+"/"+requester)
	return domain.Export{ID: id, RequestedBy: requester, Status: domain.Ready}, nil
}

func (r *repo) Fetch(_ context.Context, in domain.Fetch, sign domain.Signer) (string, error) {
	r.fetched = append(r.fetched, in)
	if r.fetchEr != nil {
		return "", r.fetchEr
	}
	return sign("exports/"+in.ID+".zip", r.created)
}

type store struct {
	key, name string
	ttl       time.Duration
	signErr   error
}

func (s *store) Put(context.Context, string, string, io.Reader, int64) error { return nil }

func (s *store) SignedDownloadURL(_ context.Context, key, filename string, ttl time.Duration) (string, error) {
	s.key, s.name, s.ttl = key, filename, ttl
	if s.signErr != nil {
		return "", s.signErr
	}
	return "https://objects.example/" + key + "?sig=1", nil
}

type wake struct{ calls int }

func (w *wake) Wake() { w.calls++ }

var admin = actor.Actor{ID: "admin-1", IP: "203.0.113.9", UserAgent: "Mozilla/5.0"}

func build(t *testing.T) (*application.Application, *repo, *store, *wake) {
	t.Helper()
	zone, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	r := &repo{created: time.Date(2026, time.October, 14, 18, 0, 0, 0, time.UTC)}
	s, w := &store{}, &wake{}
	return application.New(r, s, w, zone), r, s, w
}

func TestRequestingAnExportQueuesItForTheActorAndWakesTheWorker(t *testing.T) {
	app, r, _, w := build(t)
	got, err := app.Commands.Request.Handle(context.Background(), command.Request{Actor: admin, IncludeAnswers: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.Queued || !got.IncludeAnswers || got.RequestedBy != "admin-1" {
		t.Errorf("export = %+v", got)
	}
	want := domain.Request{ActorID: "admin-1", IP: "203.0.113.9", UserAgent: "Mozilla/5.0", IncludeAnswers: true}
	if len(r.requested) != 1 || r.requested[0] != want {
		t.Errorf("repository asked %+v, want %+v", r.requested, want)
	}
	if w.calls != 1 {
		t.Errorf("the worker was woken %d times, want 1", w.calls)
	}
}

func TestARequestWhileAnotherIsActiveDoesNotWakeTheWorker(t *testing.T) {
	app, r, _, w := build(t)
	r.requestEr = domain.ErrInProgress
	_, err := app.Commands.Request.Handle(context.Background(), command.Request{Actor: admin})
	if !errors.Is(err, domain.ErrInProgress) {
		t.Fatalf("error = %v", err)
	}
	if w.calls != 0 {
		t.Error("the worker was woken for a request that queued nothing")
	}
}

func TestARequestWithinTheHourDoesNotWakeTheWorkerAndKeepsItsWait(t *testing.T) {
	app, r, _, w := build(t)
	r.requestEr = &domain.TooSoonError{RetryAfter: 20 * time.Minute}
	_, err := app.Commands.Request.Handle(context.Background(), command.Request{Actor: admin})
	var soon *domain.TooSoonError
	if !errors.As(err, &soon) || soon.RetryAfter != 20*time.Minute {
		t.Fatalf("error = %v", err)
	}
	if w.calls != 0 {
		t.Error("the worker was woken for a request that queued nothing")
	}
}

func TestADownloadSignsTheFileForOneMinuteUnderADatedName(t *testing.T) {
	app, r, s, _ := build(t)
	got, err := app.Commands.Download.Handle(context.Background(), command.Download{Actor: admin, ID: "export-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://objects.example/exports/export-1.zip?sig=1" {
		t.Errorf("url = %q", got.URL)
	}
	if s.key != "exports/export-1.zip" || s.ttl != time.Minute {
		t.Errorf("signed %q for %v", s.key, s.ttl)
	}
	if s.name != "quizzivy-export-20261015.zip" {
		t.Errorf("the file is named %q; the date is the day the export was requested in the centre's zone", s.name)
	}
	if want := (domain.Fetch{ID: "export-1", ActorID: "admin-1", IP: "203.0.113.9", UserAgent: "Mozilla/5.0"}); len(r.fetched) != 1 || r.fetched[0] != want {
		t.Errorf("repository fetched %+v, want %+v", r.fetched, want)
	}
}

func TestADownloadThatCannotBeSignedFails(t *testing.T) {
	app, _, s, _ := build(t)
	s.signErr = errors.New("storage unreachable")
	_, err := app.Commands.Download.Handle(context.Background(), command.Download{Actor: admin, ID: "export-1"})
	if !errors.Is(err, s.signErr) {
		t.Fatalf("error = %v", err)
	}
}

func TestADownloadPassesTheRepositorysRefusalsOn(t *testing.T) {
	for _, refusal := range []error{domain.ErrNotFound, domain.ErrUnavailable} {
		app, r, s, _ := build(t)
		r.fetchEr = refusal
		_, err := app.Commands.Download.Handle(context.Background(), command.Download{Actor: admin, ID: "export-1"})
		if !errors.Is(err, refusal) {
			t.Errorf("error = %v, want %v", err, refusal)
		}
		if s.key != "" {
			t.Error("an address was signed for a refused fetch")
		}
	}
}

func TestReadingAnExportAsksForTheActorsOwn(t *testing.T) {
	app, r, _, _ := build(t)
	got, err := app.Queries.Get.Handle(context.Background(), query.Get{Actor: admin, ID: "export-1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "export-1" || len(r.got) != 1 || r.got[0] != "export-1/admin-1" {
		t.Errorf("got %+v after asking for %v", got, r.got)
	}
}

func TestANilZoneDatesTheFileInUTC(t *testing.T) {
	r := &repo{created: time.Date(2026, time.October, 14, 18, 0, 0, 0, time.UTC)}
	s := &store{}
	app := application.New(r, s, &wake{}, nil)
	if _, err := app.Commands.Download.Handle(context.Background(), command.Download{Actor: admin, ID: "export-1"}); err != nil {
		t.Fatal(err)
	}
	if s.name != "quizzivy-export-20261014.zip" {
		t.Errorf("name = %q", s.name)
	}
}

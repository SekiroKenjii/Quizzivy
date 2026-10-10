package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/validation"

	"golang.org/x/text/unicode/norm"
)

type titledRepository struct {
	repository
	created domain.Create
	writes  int
}

func (r *titledRepository) Create(_ context.Context, in domain.Create, _ domain.Quotas) (domain.Import, error) {
	r.writes++
	r.created = in
	return domain.Import{ID: "import"}, nil
}

type savedDrafts struct {
	domain.Drafts
	saved  domain.SaveDraft
	writes int
}

func (d *savedDrafts) Draft(context.Context, access.Scope, string) (domain.StoredDraft, error) {
	return domain.StoredDraft{}, nil
}

func (d *savedDrafts) SaveDraft(_ context.Context, in domain.SaveDraft) (domain.StoredDraft, error) {
	d.writes++
	d.saved = in
	return domain.StoredDraft{Draft: in.Draft}, nil
}

func TestACreatedImportIsStoredWithItsTitleComposed(t *testing.T) {
	repo := &titledRepository{}
	app := application.New(application.Dependencies{Repo: repo, Quotas: domain.DefaultQuotas()})
	if _, err := app.Commands.Create.Handle(context.Background(), command.Create{RequestID: "request", Title: norm.NFD.String("Đề thi thử")}); err != nil {
		t.Fatal(err)
	}
	if repo.created.Title != "Đề thi thử" || repo.created.RequestID != "request" {
		t.Fatalf("created=%+v", repo.created)
	}
}

func TestAReviewIsStoredWithItsTitleComposed(t *testing.T) {
	drafts := &savedDrafts{}
	app := application.New(application.Dependencies{Drafts: drafts})
	if _, err := app.Commands.SaveReview.Handle(context.Background(), command.SaveReview{ImportID: "import", ExpectedRevision: 1, Title: norm.NFD.String("Đề thi thử")}); err != nil {
		t.Fatal(err)
	}
	if drafts.saved.Draft.Title != "Đề thi thử" || drafts.saved.ExpectedRevision != 1 {
		t.Fatalf("saved=%+v", drafts.saved)
	}
}

func TestATitleComposingLeavesOverItsLimitIsRefusedBeforeAnyWrite(t *testing.T) {
	title := strings.Repeat("क़", 200)
	repo, drafts := &titledRepository{}, &savedDrafts{}
	app := application.New(application.Dependencies{Repo: repo, Drafts: drafts, Quotas: domain.DefaultQuotas()})
	for label, run := range map[string]func() error{
		"create": func() error {
			_, err := app.Commands.Create.Handle(context.Background(), command.Create{Title: title})
			return err
		},
		"review": func() error {
			_, err := app.Commands.SaveReview.Handle(context.Background(), command.SaveReview{ImportID: "import", Title: title})
			return err
		},
	} {
		t.Run(label, func(t *testing.T) {
			var invalid *validation.Error
			if err := run(); !errors.As(err, &invalid) || invalid.Fields[0].Field != "title" {
				t.Fatalf("err=%v", err)
			}
		})
	}
	if repo.writes != 0 || drafts.writes != 0 {
		t.Fatalf("a refused title was written: %d %d", repo.writes, drafts.writes)
	}
}

func TestAnUploadedSourceKeepsItsFilenameComposed(t *testing.T) {
	repo := &repository{}
	app := intake(repo, &objects{repo: repo}, inspector{}, t.TempDir())
	in := upload()
	in.Filename = norm.NFD.String("Đề cương ôn tập.docx")
	if _, err := app.Commands.Upload.Handle(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if repo.source.Filename != "Đề cương ôn tập.docx" || repo.source.Format != "docx" {
		t.Fatalf("source=%+v", repo.source)
	}
}

func TestAFilenameComposingWouldTakeOverTheLimitIsKeptAsItCame(t *testing.T) {
	repo := &repository{}
	app := intake(repo, &objects{repo: repo}, inspector{}, t.TempDir())
	in := upload()
	in.Filename = strings.Repeat("क़", 128) + ".docx"
	if _, err := app.Commands.Upload.Handle(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if repo.source.Filename != in.Filename {
		t.Fatalf("filename was rewritten past the limit: %q", repo.source.Filename)
	}
}

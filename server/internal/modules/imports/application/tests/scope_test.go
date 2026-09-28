package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/query"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

type ownedShelf struct {
	domain.Repository
	domain.Drafts
	ports.Runs
	removed    bool
	scopes     []access.Scope
	calls      []string
	committed  *domain.Commit
	owner      string
	by         actor.Actor
	materialed bool
}

func (s *ownedShelf) reaches(scope access.Scope) bool {
	s.scopes = append(s.scopes, scope)
	return scope.All || scope.UserID == "A"
}

func (s *ownedShelf) Get(_ context.Context, scope access.Scope, id string) (domain.Import, error) {
	if !s.reaches(scope) {
		return domain.Import{}, domain.ErrNotFound
	}
	v := domain.Import{ID: id, CreatedBy: "A", Status: "needs_review", Revision: 2, SourceRevision: 1, Sources: []domain.Source{{ID: "source-1", Role: "exam", Filename: "de.docx"}}}
	if s.removed {
		now := time.Now()
		v.FilesRemovedAt = &now
	}
	return v, nil
}

func (s *ownedShelf) List(_ context.Context, in domain.Filter) (domain.List, error) {
	s.reaches(in.Scope)
	return domain.List{}, nil
}

func (s *ownedShelf) Source(_ context.Context, scope access.Scope, importID, id string) (domain.Source, error) {
	if !s.reaches(scope) {
		return domain.Source{}, domain.ErrNotFound
	}
	return domain.Source{ID: id, ImportID: importID, StorageKey: "originals/x", Filename: "de.docx"}, nil
}

func (s *ownedShelf) Reserve(context.Context, domain.Reserve, domain.Quotas) (domain.Source, error) {
	s.calls = append(s.calls, "reserve")
	return domain.Source{}, errors.New("reserve reached")
}

func (s *ownedShelf) Draft(_ context.Context, scope access.Scope, id string) (domain.StoredDraft, error) {
	if !s.reaches(scope) {
		return domain.StoredDraft{}, domain.ErrNotFound
	}
	return domain.StoredDraft{ImportID: id, Title: "Đề", Status: "needs_review", CreatedBy: "A", Revision: 1, Draft: plannable()}, nil
}

func (s *ownedShelf) Commit(_ context.Context, scope access.Scope, _ string) (domain.Commit, error) {
	if !s.reaches(scope) || s.committed == nil {
		return domain.Commit{}, domain.ErrNotFound
	}
	return *s.committed, nil
}

func (s *ownedShelf) SaveDraft(context.Context, domain.SaveDraft) (domain.StoredDraft, error) {
	s.calls = append(s.calls, "save")
	return domain.StoredDraft{}, nil
}

func (s *ownedShelf) Schedule(context.Context, domain.Schedule) (domain.Run, error) {
	s.calls = append(s.calls, "schedule")
	return domain.Run{}, nil
}

func (s *ownedShelf) DraftRun(context.Context, string) (domain.Run, error) {
	s.calls = append(s.calls, "draft run")
	return domain.Run{}, errors.New("no run")
}

func (s *ownedShelf) RecordCommit(_ context.Context, in domain.CommitRecord) error {
	s.committed = &in.Commit
	return nil
}

func (s *ownedShelf) Materialize(ctx context.Context, _ domain.CommitPlan, owner string, by actor.Actor, record func(context.Context, domain.CommitStore, string) error) (string, error) {
	s.materialed, s.owner, s.by = true, owner, by
	return "test-1", record(ctx, s, "test-1")
}

func prose(text string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"format": "semantic_v1", "blocks": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text, "marks": []string{}}}}}})
	return raw
}

func plannable() domain.Draft {
	q := domain.DraftQuestion{ID: "q1", Label: "1", Type: "single_choice", Prompt: prose("Chọn"), Points: "1", Blanks: []domain.DraftBlank{}, Source: []domain.SourceRef{},
		Origins: domain.Origins{Type: domain.InferredStructure, Prompt: domain.SourceExplicit, Options: domain.SourceExplicit, Answer: domain.SourceExplicit, Points: domain.TeacherEntered},
		Options: []domain.DraftOption{{ID: "q1A", Label: "A", Content: prose("A")}, {ID: "q1B", Label: "B", Content: prose("B")}},
		Answer:  domain.DraftAnswer{State: domain.AnswerKnown, OptionIDs: []string{"q1A"}, Evidence: []domain.SourceRef{}}}
	return domain.Draft{Version: domain.DraftVersion, Title: "Đề", Notices: []domain.Finding{}, Acknowledged: []string{},
		Sections: []domain.DraftSection{{ID: "s1", Title: "Phần 1", Origin: domain.SourceExplicit, Source: []domain.SourceRef{}, Items: []domain.DraftItem{{Question: &q}}}}}
}

func shelfApp(s *ownedShelf, dir string) *application.Application {
	return application.New(application.Dependencies{Repo: s, Drafts: s, Runs: s, Materializer: s, Store: &objects{}, WorkDir: dir, Processing: true, Quotas: domain.DefaultQuotas()})
}

func TestEveryReadCarriesItsScopeAndAnotherCreatorsImportIsMissing(t *testing.T) {
	for name, c := range map[string]struct {
		scope   access.Scope
		reaches bool
	}{"B": {access.Scope{UserID: "B"}, false}, "A": {access.Scope{UserID: "A"}, true}, "scope.all": {access.Scope{UserID: "X", All: true}, true}} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			s := &ownedShelf{}
			app := shelfApp(s, t.TempDir())
			errs := map[string]error{}
			_, errs["get"] = app.Queries.Get.Handle(ctx, query.Get{ID: "import-1", Scope: c.scope})
			_, errs["download"] = app.Queries.Download.Handle(ctx, query.Download{ImportID: "import-1", SourceID: "source-1", Scope: c.scope})
			_, errs["review"] = app.Queries.Review.Handle(ctx, query.Review{ImportID: "import-1", Scope: c.scope})
			if _, err := app.Queries.List.Handle(ctx, query.List{Scope: c.scope}); err != nil {
				t.Fatal(err)
			}
			for op, err := range errs {
				if c.reaches != (err == nil) {
					t.Errorf("%s: %v", op, err)
				}
				if !c.reaches && !errors.Is(err, domain.ErrNotFound) {
					t.Errorf("%s answered %v, want ErrNotFound", op, err)
				}
			}
			for _, seen := range s.scopes {
				if seen != c.scope {
					t.Errorf("a read ran in %+v, want %+v", seen, c.scope)
				}
			}
			if want := map[bool]int{true: 6, false: 4}[c.reaches]; len(s.scopes) != want {
				t.Errorf("%d scoped reads, want %d", len(s.scopes), want)
			}
		})
	}
}

func TestAnotherCreatorsRemovedFilesAnswerAsMissing(t *testing.T) {
	ctx := context.Background()
	s := &ownedShelf{removed: true}
	app := shelfApp(s, t.TempDir())
	b := access.Scope{UserID: "B"}
	for op, err := range map[string]error{
		"download": second(app.Queries.Download.Handle(ctx, query.Download{ImportID: "import-1", SourceID: "source-1", Scope: b})),
		"review":   second(app.Queries.Review.Handle(ctx, query.Review{ImportID: "import-1", Scope: b})),
		"source":   second(app.Queries.SourceView.Handle(ctx, query.SourceView{ImportID: "import-1", Role: "exam", Scope: b})),
	} {
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B's %s of A's swept import: %v, want ErrNotFound", op, err)
		}
	}
	if _, err := app.Queries.SourceView.Handle(ctx, query.SourceView{ImportID: "import-1", Role: "exam", Scope: access.Scope{UserID: "A"}}); !errors.Is(err, domain.ErrFilesRemoved) {
		t.Errorf("A's source view of a swept import: %v, want ErrFilesRemoved", err)
	}
	s.removed = false
	if _, err := app.Queries.SourceView.Handle(ctx, query.SourceView{ImportID: "import-1", Role: "exam", Scope: b}); !errors.Is(err, domain.ErrNotFound) || len(s.calls) != 0 {
		t.Errorf("B's source view: %v after %v, want ErrNotFound before the draft's run is read", err, s.calls)
	}
}

func TestAnotherCreatorsWritesStopBeforeAnythingHappens(t *testing.T) {
	ctx := context.Background()
	s := &ownedShelf{}
	dir := t.TempDir()
	app := shelfApp(s, dir)
	b := actor.Actor{ID: "B"}
	for op, err := range map[string]error{
		"upload": second(app.Commands.Upload.Handle(ctx, command.Upload{ImportID: "import-1", UploadID: "u", Role: "exam", Filename: "de.docx", Actor: b,
			Body: strings.NewReader("PK"), End: func() error { return nil }})),
		"process": second(app.Commands.Process.Handle(ctx, command.Process{ImportID: "import-1", RequestID: "r", ExpectedRevision: 2, Actor: b})),
		"save":    second(app.Commands.SaveReview.Handle(ctx, command.SaveReview{ImportID: "import-1", ExpectedRevision: 1, Title: "B", Actor: b})),
		"commit":  second(app.Commands.Commit.Handle(ctx, command.Commit{ImportID: "import-1", RequestID: "r", DraftRevision: 1, Actor: b})),
	} {
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B's %s: %v, want ErrNotFound", op, err)
		}
	}
	if len(s.calls) != 0 || s.materialed {
		t.Errorf("B's refused writes reached %v (materialized %v)", s.calls, s.materialed)
	}
	for _, seen := range s.scopes {
		if seen != (access.Scope{UserID: "B"}) {
			t.Errorf("a write read in %+v, want B's own scope", seen)
		}
	}
	if staged, err := os.ReadDir(dir); err != nil || len(staged) != 0 {
		t.Errorf("a refused upload staged %d files (%v)", len(staged), err)
	}
}

func TestACommitBelongsToTheImportsCreatorWhoeverCommitsIt(t *testing.T) {
	for name, by := range map[string]actor.Actor{
		"the creator": {ID: "A"},
		"scope.all":   {ID: "X", Scope: access.Scope{UserID: "X", All: true}},
	} {
		t.Run(name, func(t *testing.T) {
			s := &ownedShelf{}
			result, err := shelfApp(s, t.TempDir()).Commands.Commit.Handle(context.Background(), command.Commit{ImportID: "import-1", RequestID: "r", DraftRevision: 1, Actor: by})
			if err != nil {
				t.Fatal(err)
			}
			if !s.materialed || s.owner != "A" || s.by.ID != by.ID || result.TestID != "test-1" {
				t.Errorf("materialized for %q by %q (test %q), want A's content made by %s", s.owner, s.by.ID, result.TestID, by.ID)
			}
		})
	}
}

func second[T any](_ T, err error) error { return err }

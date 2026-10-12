package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/application/query"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"strings"
	"testing"
	"unicode/utf8"
)

type pasteRepository struct {
	repository
	scope  access.Scope
	refuse error
}

func (r *pasteRepository) Get(_ context.Context, scope access.Scope, _ string) (domain.Import, error) {
	r.scope = scope
	return domain.Import{ID: "import"}, r.refuse
}

func TestPasteStoresNFCCharactersAndPreservesOriginalLineBreaks(t *testing.T) {
	repo := &repository{}
	store := &objects{repo: repo}
	app := intake(repo, store, inspector{err: errors.New("text must not be inspected as Word")}, t.TempDir())
	text := "\ufeffCâu 1. tiếng 😀\r\n\r\nA. một"
	receipt, err := app.Commands.Paste.Handle(context.Background(), command.Paste{ImportID: "import", UploadID: "paste", ExpectedRevision: 1, Text: text})
	if err != nil {
		t.Fatal(err)
	}
	want := "\ufeffCâu 1. tiếng 😀\r\n\r\nA. một"
	if string(store.data) != want || receipt.Source.Format != "text" || receipt.Source.Role != "exam" || receipt.Source.Filename != "pasted-text.txt" || receipt.Source.Bytes != int64(len(want)) || receipt.Source.Characters == nil || *receipt.Source.Characters != utf8.RuneCountInString(want) {
		t.Fatalf("stored text metadata: %+v %q", receipt.Source, store.data)
	}
	limits, err := app.Queries.Limits.Handle(context.Background(), query.Limits{})
	if err != nil || limits.PasteMaxCharacters != 100000 || len(limits.Formats) != 2 || limits.Formats[0] != "docx" || limits.Formats[1] != "pdf" {
		t.Fatalf("limits: %+v %v", limits, err)
	}
	if _, err := app.Commands.Upload.Handle(context.Background(), command.Upload{Filename: "pasted-text.txt", Body: strings.NewReader(want)}); !errors.Is(err, domain.ErrUnsupported) {
		t.Fatalf("multipart txt admitted: %v", err)
	}
}

func TestPasteCharacterAndNonblankLineBoundaries(t *testing.T) {
	for _, test := range []struct {
		name, text string
		want       error
	}{
		{"maximum scalars", strings.Repeat("😀", 100000), nil},
		{"one extra scalar", strings.Repeat("😀", 100001), domain.ErrTooLarge},
		{"maximum NFC", strings.Repeat("e\u0301", 100000), nil},
		{"maximum nonblank lines", strings.Repeat("x\r\n", 20000), nil},
		{"extra nonblank line", strings.Repeat("x\r", 20001), domain.ErrTooLarge},
		{"blank lines uncounted", strings.Repeat("\n", 25000) + "x", nil},
		{"blank", " \t\n\u00a0", domain.ErrInvalid},
		{"BOM only", "\ufeff", domain.ErrInvalid},
		{"NUL", "question\x00", domain.ErrInvalid},
		{"invalid UTF8", string([]byte{0xff}), domain.ErrInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &repository{}
			store := &objects{repo: repo}
			app := intake(repo, store, inspector{}, t.TempDir())
			_, err := app.Commands.Paste.Handle(context.Background(), command.Paste{Text: test.text})
			if !errors.Is(err, test.want) {
				t.Fatalf("paste: %v want %v", err, test.want)
			}
			if test.want != nil && (repo.reserved != 0 || store.puts != 0) {
				t.Fatal("invalid paste reserved or wrote bytes")
			}
		})
	}
}

func TestPasteSharesBusyIntakeWithUploadAndRecoversStorageFailure(t *testing.T) {
	repo := &repository{}
	store := &objects{repo: repo}
	started, release := make(chan struct{}), make(chan struct{})
	app := intake(repo, store, inspector{started: started, release: release}, t.TempDir())
	result := make(chan error, 1)
	go func() { _, err := app.Commands.Upload.Handle(context.Background(), upload()); result <- err }()
	<-started
	_, busy := app.Commands.Paste.Handle(context.Background(), command.Paste{Text: "plain"})
	close(release)
	err := <-result
	if err != nil || !errors.Is(busy, domain.ErrBusy) {
		t.Fatalf("shared intake: %v %v", err, busy)
	}
	repo = &repository{}
	store = &objects{repo: repo, fail: true}
	app = intake(repo, store, inspector{}, t.TempDir())
	in := command.Paste{Text: "plain", ImportID: "import", UploadID: "paste", ExpectedRevision: 1}
	if _, err := app.Commands.Paste.Handle(context.Background(), in); err == nil {
		t.Fatal("storage failure disappeared")
	}
	firstID := repo.source.ID
	if _, err := app.Commands.Paste.Handle(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Commands.Paste.Handle(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if repo.source.ID != firstID || repo.finished != 1 || store.puts != 2 {
		t.Fatal("recovery or completed replay duplicated source")
	}
}

func TestPasteChecksScopedImportBeforeReservation(t *testing.T) {
	for _, all := range []bool{false, true} {
		repo := &pasteRepository{refuse: domain.ErrNotFound}
		store := &objects{repo: &repo.repository}
		handler := command.PasteHandler{Upload: command.UploadHandler{Repo: repo, Store: store, Slots: make(chan struct{}, 1), WorkDir: t.TempDir()}}
		by := actor.Actor{ID: "caller", Scope: access.Scope{UserID: "caller", All: all}}
		_, err := handler.Handle(context.Background(), command.Paste{Text: "plain", Actor: by})
		if !errors.Is(err, domain.ErrNotFound) || repo.scope != by.Scope || repo.reserved != 0 || store.puts != 0 {
			t.Fatalf("scope: %+v %v", repo.scope, err)
		}
	}
}

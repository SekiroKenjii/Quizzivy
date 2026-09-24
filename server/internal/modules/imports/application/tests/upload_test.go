package application_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/domain"
	"testing"
	"time"
)

type repository struct {
	domain.Repository
	source             domain.Source
	reserved, finished int
	failFinish         bool
}

func (r *repository) Get(context.Context, string) (domain.Import, error) {
	return domain.Import{ID: "import"}, nil
}
func (r *repository) Reserve(_ context.Context, in domain.Reserve, _ domain.Quotas) (domain.Source, error) {
	r.reserved++
	if r.source.ID == "" {
		r.source = in.Source
		r.source.ID = "source"
		r.source.StorageKey = "originals/import/source"
	}
	return r.source, nil
}
func (r *repository) Finish(context.Context, domain.Finish) (domain.Receipt, error) {
	if r.failFinish {
		r.failFinish = false
		return domain.Receipt{}, errors.New("lost database response")
	}
	if !r.source.Ready {
		r.finished++
		r.source.Ready = true
		r.source.SourceRevision = 1
	}
	return domain.Receipt{Source: r.source, Import: domain.Import{ID: "import", Revision: 2, SourceRevision: 1}}, nil
}

type objects struct {
	repo *repository
	fail bool
	puts int
	data []byte
}

func (o *objects) Put(_ context.Context, key, _ string, r io.Reader, _ int64) error {
	if o.repo.reserved == 0 || o.repo.source.StorageKey != key {
		return errors.New("untracked object write")
	}
	o.puts++
	if o.fail {
		o.fail = false
		return errors.New("storage unavailable")
	}
	var err error
	o.data, err = io.ReadAll(r)
	return err
}
func (*objects) SignedDownloadURL(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

type inspector struct {
	err     error
	started chan struct{}
	release chan struct{}
}

func (i inspector) Inspect(context.Context, io.ReaderAt, int64) error {
	if i.started != nil {
		close(i.started)
		<-i.release
	}
	return i.err
}
func upload() command.Upload {
	return command.Upload{ImportID: "import", UploadID: "upload", Role: "exam", ExpectedRevision: 1, Filename: "Đề.docx", Body: bytes.NewReader([]byte("synthetic Word bytes"))}
}

func TestFailedStorageAndLostCompletionAreRetryableWithoutNewIdentity(t *testing.T) {
	for _, failure := range []string{"storage", "completion"} {
		t.Run(failure, func(t *testing.T) {
			repo := &repository{failFinish: failure == "completion"}
			store := &objects{repo: repo, fail: failure == "storage"}
			dir := t.TempDir()
			app := application.New(repo, store, inspector{}, dir, domain.DefaultQuotas())
			if _, err := app.Commands.Upload.Handle(context.Background(), upload()); err == nil {
				t.Fatal("injected failure disappeared")
			}
			if repo.source.ID == "" || repo.finished != 0 {
				t.Fatal("failed upload was not durably reserved")
			}
			recovered, err := app.Commands.Upload.Handle(context.Background(), upload())
			if err != nil {
				t.Fatal(err)
			}
			if recovered.Source.ID != repo.source.ID || repo.finished != 1 {
				t.Fatal("retry duplicated source")
			}
			if _, err := app.Commands.Upload.Handle(context.Background(), upload()); err != nil {
				t.Fatal(err)
			}
			if store.puts != 2 || string(store.data) != "synthetic Word bytes" {
				t.Fatalf("replay rewrote completed bytes: %d", store.puts)
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 0 {
				t.Fatalf("staging not cleaned: %v", err)
			}
		})
	}
}

func TestInvalidOrExcessiveUploadNeverReachesStorage(t *testing.T) {
	for _, test := range []struct {
		name                  string
		input                 command.Upload
		inspection, errorWant error
	}{
		{name: "active", input: upload(), inspection: domain.ErrUnsupported, errorWant: domain.ErrUnsupported},
		{name: "broken", input: upload(), inspection: domain.ErrInvalid, errorWant: domain.ErrInvalid},
		{name: "oversize", input: command.Upload{ImportID: "import", Filename: "Large.docx", Body: io.LimitReader(zeros{}, domain.MaxSourceBytes+1)}, errorWant: domain.ErrTooLarge},
		{name: "empty", input: command.Upload{ImportID: "import", Filename: "Empty.docx", Body: bytes.NewReader(nil)}, errorWant: domain.ErrInvalid},
		{name: "traversal", input: command.Upload{Filename: "../outside.docx"}, errorWant: domain.ErrUnsupported},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &repository{}
			store := &objects{repo: repo}
			dir := t.TempDir()
			app := application.New(repo, store, inspector{err: test.inspection}, dir, domain.DefaultQuotas())
			_, err := app.Commands.Upload.Handle(context.Background(), test.input)
			if !errors.Is(err, test.errorWant) {
				t.Fatalf("wanted %v, got %v", test.errorWant, err)
			}
			if repo.reserved != 0 || store.puts != 0 {
				t.Fatal("rejected source reached persistence")
			}
			files, err := filepath.Glob(filepath.Join(dir, "*"))
			if err != nil || len(files) != 0 {
				t.Fatal("rejected upload left staging")
			}
		})
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestOnlyOneIntakeCanHoldExpandedDocumentMemory(t *testing.T) {
	repo := &repository{}
	store := &objects{repo: repo}
	started, release := make(chan struct{}), make(chan struct{})
	app := application.New(repo, store, inspector{started: started, release: release}, t.TempDir(), domain.DefaultQuotas())
	done := make(chan error, 1)
	go func() { _, err := app.Commands.Upload.Handle(context.Background(), upload()); done <- err }()
	<-started
	_, err := app.Commands.Upload.Handle(context.Background(), upload())
	close(release)
	if !errors.Is(err, domain.ErrBusy) {
		t.Fatalf("second intake not bounded: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

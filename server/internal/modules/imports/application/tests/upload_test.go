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
	"quizzivy/internal/modules/imports/application/query"
	"quizzivy/internal/modules/imports/domain"
	"slices"
	"testing"
	"time"
)

type repository struct {
	domain.Repository
	source             domain.Source
	reserved, finished int
	failFinish         bool
	closed             bool
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
	if r.closed {
		return domain.Receipt{}, domain.ErrConflict
	}
	if !r.source.Ready {
		r.finished++
		r.source.Ready = true
		r.source.SourceRevision = 1
	}
	return domain.Receipt{Source: r.source, Import: domain.Import{ID: "import", Revision: 2, SourceRevision: 1}}, nil
}

type objects struct {
	repo    *repository
	fail    bool
	puts    int
	deletes int
	data    []byte
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
func (o *objects) Delete(context.Context, string) error {
	o.deletes++
	return nil
}
func (*objects) PutImmutable(context.Context, string, string, io.ReadSeeker, int64, []byte) error {
	return errors.New("not an artifact store")
}
func (*objects) Open(context.Context, string) (io.ReadCloser, int64, error) {
	return nil, 0, errors.New("not an artifact store")
}

func intake(repo *repository, store *objects, check inspector, dir string) *application.Application {
	return application.New(application.Dependencies{Repo: repo, Store: store, Inspector: check, WorkDir: dir, Quotas: domain.DefaultQuotas()})
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
			app := intake(repo, store, inspector{}, dir)
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
			if store.deletes != 0 {
				t.Fatal("a retryable failure deleted the stored bytes")
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 0 {
				t.Fatalf("staging not cleaned: %v", err)
			}
		})
	}
}

func TestAnUploadThatLandsAfterTheImportClosedLeavesNothingBehind(t *testing.T) {
	repo := &repository{closed: true}
	store := &objects{repo: repo}
	app := intake(repo, store, inspector{}, t.TempDir())
	if _, err := app.Commands.Upload.Handle(context.Background(), upload()); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("upload into a closed import: %v", err)
	}
	if store.puts != 1 || store.deletes != 1 {
		t.Fatalf("puts %d, deletes %d: the orphaned object was not removed", store.puts, store.deletes)
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
			app := intake(repo, store, inspector{err: test.inspection}, dir)
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

func TestAPDFIsAdmittedByItsSignatureAlone(t *testing.T) {
	for _, test := range []struct {
		name, filename, body string
		want                 error
	}{
		{name: "pdf", filename: "Đề.pdf", body: "%PDF-1.7\nsynthetic"},
		{name: "upper case", filename: "ĐÁP ÁN.PDF", body: "%PDF-1.4\nsynthetic"},
		{name: "leading bytes", filename: "Đề.pdf", body: "\xef\xbb\xbf%PDF-1.4\nsynthetic"},
		{name: "not a pdf", filename: "Đề.pdf", body: "synthetic Word bytes", want: domain.ErrInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &repository{}
			store := &objects{repo: repo}
			app := intake(repo, store, inspector{err: domain.ErrUnsupported}, t.TempDir())
			in := upload()
			in.Filename, in.Body = test.filename, bytes.NewReader([]byte(test.body))
			_, err := app.Commands.Upload.Handle(context.Background(), in)
			if !errors.Is(err, test.want) {
				t.Fatalf("wanted %v, got %v", test.want, err)
			}
			if test.want == nil && (repo.source.Format != "pdf" || store.puts != 1) {
				t.Fatalf("stored %q with %d writes", repo.source.Format, store.puts)
			}
			if test.want != nil && (repo.reserved != 0 || store.puts != 0) {
				t.Fatal("a rejected PDF reached persistence")
			}
		})
	}
}

func TestLimitsNameThePDFBesideWord(t *testing.T) {
	app := intake(&repository{}, &objects{}, inspector{}, t.TempDir())
	limits, err := app.Queries.Limits.Handle(context.Background(), query.Limits{})
	if err != nil || !slices.Equal(limits.Formats, []string{"docx", "pdf"}) {
		t.Fatalf("formats = %v, %v", limits.Formats, err)
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestOnlyOneIntakeCanHoldExpandedDocumentMemory(t *testing.T) {
	repo := &repository{}
	store := &objects{repo: repo}
	started, release := make(chan struct{}), make(chan struct{})
	app := intake(repo, store, inspector{started: started, release: release}, t.TempDir())
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

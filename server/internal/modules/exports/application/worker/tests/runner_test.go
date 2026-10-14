package worker_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/exports/application/ports"
	"quizzivy/internal/modules/exports/application/worker"
	"quizzivy/internal/modules/exports/domain"
)

type queue struct {
	mu          sync.Mutex
	claim       *domain.Export
	claimErr    error
	completeErr error
	failErr     error
	releaseErr  error
	completed   []domain.Completion
	failed      []failure
	released    []int
	settleErr   error
	due         time.Time
	dueOK       bool
}

type failure struct {
	id      string
	attempt int
	code    string
}

func (q *queue) Claim(context.Context) (domain.Export, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.claimErr != nil {
		return domain.Export{}, q.claimErr
	}
	if q.claim == nil {
		return domain.Export{}, domain.ErrNoWork
	}
	out := *q.claim
	q.claim = nil
	return out, nil
}

func (q *queue) NextDue(context.Context) (time.Time, bool, error) { return q.due, q.dueOK, nil }

func (q *queue) Complete(ctx context.Context, done domain.Completion) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.settleErr = ctx.Err()
	q.completed = append(q.completed, done)
	return q.completeErr
}

func (q *queue) Fail(ctx context.Context, id string, attempt int, code string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.settleErr = ctx.Err()
	q.failed = append(q.failed, failure{id, attempt, code})
	return q.failErr
}

func (q *queue) Release(ctx context.Context, _ string, attempt int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.settleErr = ctx.Err()
	q.released = append(q.released, attempt)
	return q.releaseErr
}

type source struct {
	rows    map[string][][]string
	asked   []string
	failOn  string
	blockOn string
}

func (s *source) Read(ctx context.Context, fn func(domain.Snapshot) error) error {
	return fn(snapshot{s, ctx})
}

type snapshot struct {
	s   *source
	ctx context.Context
}

var errQuery = errors.New("pq: the secret detail of the query that failed")

func (n snapshot) Rows(ctx context.Context, file domain.File, _ bool, emit func([]string) error) error {
	n.s.asked = append(n.s.asked, file.Name)
	if file.Name == n.s.failOn {
		return errQuery
	}
	if file.Name == n.s.blockOn {
		<-ctx.Done()
		return ctx.Err()
	}
	for _, row := range n.s.rows[file.Name] {
		if err := emit(row); err != nil {
			return err
		}
	}
	return nil
}

type store struct {
	putErr      error
	key         string
	contentType string
	size        int64
	body        []byte
}

func (s *store) Put(_ context.Context, key, contentType string, body io.Reader, size int64) error {
	if s.putErr != nil {
		return s.putErr
	}
	raw, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.key, s.contentType, s.size, s.body = key, contentType, size, raw
	return nil
}

func (s *store) SignedDownloadURL(context.Context, string, string, time.Duration) (string, error) {
	return "", errors.New("the worker never signs")
}

var _ ports.Store = (*store)(nil)

type plain struct {
	w io.Writer
}

func (p plain) Row(cells ...string) error {
	_, err := io.WriteString(p.w, strings.Join(cells, ",")+"\r\n")
	return err
}

func (plain) Flush() error { return nil }

func table(w io.Writer) (ports.Table, error) { return plain{w}, nil }

type fixture struct {
	q   *queue
	src *source
	st  *store
	dir string
	r   worker.Runner
}

func newFixture(t *testing.T, includeAnswers bool) *fixture {
	t.Helper()
	f := &fixture{
		q: &queue{claim: &domain.Export{ID: "0198f000-0000-7000-8000-000000000001", Attempts: 2, IncludeAnswers: includeAnswers, Status: domain.Running}},
		src: &source{rows: map[string][][]string{
			domain.UsersFile: {{"u1", "an@example.com"}, {"u2", "binh@example.com"}},
			domain.TestsFile: {{"t1", "Kiểm tra"}},
		}},
		st:  &store{},
		dir: t.TempDir(),
	}
	f.r = worker.Runner{
		Queue: f.q, Source: f.src, Store: f.st, Table: table, WorkDir: f.dir,
		Now: func() time.Time { return time.Date(2026, time.October, 14, 9, 30, 0, 0, time.UTC) },
	}
	return f
}

func (f *fixture) leftovers(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func unzip(t *testing.T, raw []byte) (names []string, files map[string]string) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("the stored object is not a ZIP: %v", err)
	}
	files = map[string]string{}
	for _, entry := range reader.File {
		in, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(in)
		_ = in.Close()
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, entry.Name)
		files[entry.Name] = string(body)
	}
	return names, files
}

func TestAnExportIsZippedStoredAndReported(t *testing.T) {
	f := newFixture(t, false)
	worked, err := f.r.RunOne(context.Background())
	if err != nil || !worked {
		t.Fatalf("RunOne = %v, %v", worked, err)
	}
	if f.st.key != "exports/0198f000-0000-7000-8000-000000000001.zip" || f.st.contentType != "application/zip" {
		t.Errorf("stored %q as %q", f.st.key, f.st.contentType)
	}
	if int64(len(f.st.body)) != f.st.size {
		t.Errorf("the object is %d bytes but the store was told %d", len(f.st.body), f.st.size)
	}
	if len(f.q.completed) != 1 || len(f.q.failed) != 0 || len(f.q.released) != 0 {
		t.Fatalf("settled completed=%v failed=%v released=%v", f.q.completed, f.q.failed, f.q.released)
	}
	done := f.q.completed[0]
	want := domain.Completion{ID: "0198f000-0000-7000-8000-000000000001", Attempt: 2, StorageKey: f.st.key, Bytes: f.st.size}
	if done != want {
		t.Errorf("completion = %+v, want %+v", done, want)
	}

	names, files := unzip(t, f.st.body)
	wantNames := []string{domain.ReadmeFile}
	for _, file := range domain.Files(false) {
		wantNames = append(wantNames, file.Name)
	}
	if !slices.Equal(names, wantNames) {
		t.Errorf("the ZIP holds %v, want %v", names, wantNames)
	}
	if !strings.HasPrefix(files[domain.ReadmeFile], "\xEF\xBB\xBFBẢN XUẤT DỮ LIỆU QUIZZIVY") {
		t.Errorf("the README does not open with a byte-order mark and the Vietnamese title: %.40q", files[domain.ReadmeFile])
	}
	users := domain.Files(false)[0]
	wantUsers := strings.Join(users.Header(), ",") + "\r\nu1,an@example.com\r\nu2,binh@example.com\r\n"
	if files[domain.UsersFile] != wantUsers {
		t.Errorf("users.csv = %q, want %q", files[domain.UsersFile], wantUsers)
	}
	roles := files[domain.RolesFile]
	if strings.Count(roles, "\r\n") != 1 {
		t.Errorf("an empty table still has its header and nothing else: %q", roles)
	}
	if got := f.leftovers(t); len(got) != 0 {
		t.Errorf("the work directory still holds %v", got)
	}
}

func TestAnswersAreInTheZipOnlyWhenAsked(t *testing.T) {
	for _, asked := range []bool{false, true} {
		f := newFixture(t, asked)
		f.src.rows[domain.AnswersFile] = [][]string{{"a1", "q1"}}
		if _, err := f.r.RunOne(context.Background()); err != nil {
			t.Fatal(err)
		}
		names, files := unzip(t, f.st.body)
		has := slices.Contains(names, domain.AnswersFile)
		if has != asked {
			t.Errorf("includeAnswers=%v: answers.csv present=%v in %v", asked, has, names)
		}
		if asked && names[len(names)-1] != domain.AnswersFile {
			t.Errorf("answers.csv is not the last file: %v", names)
		}
		if asked && !strings.Contains(files[domain.AnswersFile], "a1,q1") {
			t.Errorf("answers.csv lost its row: %q", files[domain.AnswersFile])
		}
		if !asked && slices.Contains(f.src.asked, domain.AnswersFile) {
			t.Error("the snapshot was asked for answers an admin did not choose")
		}
		if !asked && strings.Contains(files[domain.ReadmeFile], domain.AnswersFile) {
			t.Error("the README describes a file the ZIP does not hold")
		}
		if asked && !strings.Contains(files[domain.ReadmeFile], domain.AnswersFile) {
			t.Error("the README leaves out the answers file")
		}
	}
}

func TestNothingWaitingIsNotAnError(t *testing.T) {
	f := newFixture(t, false)
	f.q.claim = nil
	worked, err := f.r.RunOne(context.Background())
	if err != nil || worked {
		t.Fatalf("RunOne = %v, %v", worked, err)
	}
	if f.st.key != "" || len(f.src.asked) != 0 {
		t.Error("an empty queue still built something")
	}
}

func TestAClaimThatFailsIsReported(t *testing.T) {
	f := newFixture(t, false)
	f.q.claimErr = errors.New("database down")
	worked, err := f.r.RunOne(context.Background())
	if err == nil || worked {
		t.Fatalf("RunOne = %v, %v", worked, err)
	}
}

func TestAQueryThatFailsFailsTheExportWithACodeAndNoDetail(t *testing.T) {
	f := newFixture(t, false)
	f.src.failOn = domain.ClassesFile
	worked, err := f.r.RunOne(context.Background())
	if err != nil || !worked {
		t.Fatalf("a failed build is the export's outcome, not RunOne's error: %v, %v", worked, err)
	}
	if len(f.q.completed) != 0 || len(f.q.released) != 0 || len(f.q.failed) != 1 {
		t.Fatalf("settled completed=%v failed=%v released=%v", f.q.completed, f.q.failed, f.q.released)
	}
	got := f.q.failed[0]
	if got.code != domain.CodeFailed || got.attempt != 2 || got.id != "0198f000-0000-7000-8000-000000000001" {
		t.Errorf("failure = %+v", got)
	}
	if strings.Contains(got.code, "secret") {
		t.Error("the error text reached the table")
	}
	if f.st.key != "" {
		t.Error("a half-built ZIP was uploaded")
	}
	if got := f.leftovers(t); len(got) != 0 {
		t.Errorf("the work directory still holds %v", got)
	}
}

func TestAStoreThatFailsFailsTheExportAsStorage(t *testing.T) {
	f := newFixture(t, false)
	f.st.putErr = errors.New("bucket unreachable")
	if _, err := f.r.RunOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.q.failed) != 1 || f.q.failed[0].code != domain.CodeStorage || len(f.q.completed) != 0 {
		t.Fatalf("failed=%v completed=%v", f.q.failed, f.q.completed)
	}
	if got := f.leftovers(t); len(got) != 0 {
		t.Errorf("the work directory still holds %v", got)
	}
}

func TestABuildPastItsTimeoutFailsAsATimeout(t *testing.T) {
	f := newFixture(t, false)
	f.src.blockOn = domain.TestsFile
	f.r.Timeout = 20 * time.Millisecond
	started := time.Now()
	if _, err := f.r.RunOne(context.Background()); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatal("the timeout did not cut the build off")
	}
	if len(f.q.failed) != 1 || f.q.failed[0].code != domain.CodeTimeout {
		t.Fatalf("failed=%v released=%v", f.q.failed, f.q.released)
	}
	if f.q.settleErr != nil {
		t.Error("the failure was written with a spent context")
	}
}

func TestAShutdownGivesTheExportBackWithoutSpendingTheAttempt(t *testing.T) {
	f := newFixture(t, false)
	f.src.blockOn = domain.TestsFile
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	worked, err := f.r.RunOne(ctx)
	if err != nil || !worked {
		t.Fatalf("RunOne = %v, %v", worked, err)
	}
	if len(f.q.released) != 1 || f.q.released[0] != 2 || len(f.q.failed) != 0 || len(f.q.completed) != 0 {
		t.Fatalf("released=%v failed=%v completed=%v", f.q.released, f.q.failed, f.q.completed)
	}
	if f.q.settleErr != nil {
		t.Error("the release was written with the cancelled context, so it would never reach the database")
	}
	if got := f.leftovers(t); len(got) != 0 {
		t.Errorf("the work directory still holds %v", got)
	}
}

func TestALostLeaseIsNotAnErrorAndLeavesTheNewOwnersRowAlone(t *testing.T) {
	for name, set := range map[string]func(*queue){
		"complete": func(q *queue) { q.completeErr = domain.ErrLeaseLost },
		"fail":     func(q *queue) { q.failErr = domain.ErrLeaseLost },
	} {
		f := newFixture(t, false)
		set(f.q)
		if name == "fail" {
			f.src.failOn = domain.UsersFile
		}
		worked, err := f.r.RunOne(context.Background())
		if err != nil || !worked {
			t.Errorf("%s: RunOne = %v, %v", name, worked, err)
		}
	}
}

func TestAnErrorWritingTheOutcomeIsReported(t *testing.T) {
	f := newFixture(t, false)
	f.q.completeErr = errors.New("database down")
	worked, err := f.r.RunOne(context.Background())
	if err == nil || !worked {
		t.Fatalf("RunOne = %v, %v", worked, err)
	}
}

func TestNextDueIsTheQueuesAnswer(t *testing.T) {
	f := newFixture(t, false)
	f.q.due, f.q.dueOK = time.Date(2026, time.October, 14, 10, 0, 0, 0, time.UTC), true
	due, ok, err := f.r.NextDue(context.Background())
	if err != nil || !ok || !due.Equal(f.q.due) {
		t.Fatalf("NextDue = %v, %v, %v", due, ok, err)
	}
}

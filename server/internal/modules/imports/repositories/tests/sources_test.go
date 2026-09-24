//go:build integration

package repositories_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/actor"
	"sync"
	"testing"
)

type harness struct {
	pool   *pgxpool.Pool
	repo   *repositories.Postgres
	actor  actor.Actor
	quotas domain.Quotas
}

func setup(t *testing.T) harness {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var id string
	if err := pool.QueryRow(ctx, `INSERT INTO app.users(email,full_name,role) VALUES($1,'Import teacher','admin') RETURNING id::text`, uuid.NewString()+"@example.test").Scan(&id); err != nil {
		t.Fatal(err)
	}
	connection := pool
	if dsn := os.Getenv("TEST_APP_DATABASE_URL"); dsn != "" {
		connection, err = pgxpool.New(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(connection.Close)
	}
	t.Cleanup(func() {
		for _, sql := range []string{
			`UPDATE app.word_imports SET source_revision=NULL WHERE created_by=$1`,
			`DELETE FROM app.word_import_run_events WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
			`DELETE FROM app.word_import_runs WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
			`DELETE FROM app.word_import_source_set_items WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
			`DELETE FROM app.word_import_sources WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
			`DELETE FROM app.word_import_source_sets WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
			`DELETE FROM app.word_imports WHERE created_by=$1`,
		} {
			if _, err := pool.Exec(ctx, sql, id); err != nil {
				t.Errorf("fixture cleanup: %v", err)
			}
		}
	})
	quotas := domain.DefaultQuotas()
	quotas.GlobalImports = 100000
	return harness{pool: pool, repo: repositories.NewPostgres(db.NewContext(connection)), actor: actor.Actor{ID: id}, quotas: quotas}
}
func (h harness) create(t *testing.T) domain.Import {
	t.Helper()
	v, err := h.repo.Create(context.Background(), domain.Create{RequestID: uuid.NewString(), Title: "Đề tiếng Anh", Actor: h.actor}, h.quotas)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func (h harness) upload(v domain.Import, role string) domain.Reserve {
	hash := sha256.Sum256([]byte(uuid.NewString()))
	return domain.Reserve{Actor: h.actor, Source: domain.Source{ImportID: v.ID, UploadID: uuid.NewString(), ExpectedRevision: v.Revision, Role: role, Filename: "Đề.docx", Format: "docx", Bytes: 100, SHA256: hash[:]}}
}
func (h harness) reserve(t *testing.T, in domain.Reserve) domain.Source {
	t.Helper()
	v, err := h.repo.Reserve(context.Background(), in, h.quotas)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func (h harness) finish(t *testing.T, s domain.Source) domain.Receipt {
	t.Helper()
	v, err := h.repo.Finish(context.Background(), domain.Finish{ImportID: s.ImportID, SourceID: s.ID, Actor: h.actor})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestCreateReplayAndConcurrentQuota(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	in := domain.Create{RequestID: uuid.NewString(), Title: "Đề tiếng Anh", Actor: h.actor}
	first, err := h.repo.Create(ctx, in, h.quotas)
	if err != nil {
		t.Fatal(err)
	}
	again, err := h.repo.Create(ctx, in, h.quotas)
	if err != nil || again.ID != first.ID {
		t.Fatalf("lost-response replay: %+v %v", again, err)
	}
	in.Title = "changed"
	if _, err := h.repo.Create(ctx, in, h.quotas); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("changed replay: %v", err)
	}
	q := h.quotas
	q.ActorImports = 2
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := h.repo.Create(ctx, domain.Create{RequestID: uuid.NewString(), Title: "Concurrent", Actor: h.actor}, q)
			results <- err
		}()
	}
	close(start)
	passed, limited := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			passed++
		} else if errors.Is(err, domain.ErrQuota) {
			limited++
		} else {
			t.Fatal(err)
		}
	}
	if passed != 1 || limited != 1 {
		t.Fatalf("quota race: %d accepted %d limited", passed, limited)
	}
}

func TestSourcesAreImmutableRevisionedAndScoped(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	v := h.create(t)
	examIn := h.upload(v, "exam")
	exam := h.reserve(t, examIn)
	pending, err := h.repo.Get(ctx, v.ID)
	if err != nil || pending.PendingUploads != 1 || len(pending.Sources) != 0 {
		t.Fatalf("pending receipt: %+v %v", pending, err)
	}
	if _, err := h.repo.Source(ctx, v.ID, exam.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("pending source downloadable: %v", err)
	}
	first := h.finish(t, exam)
	if first.Import.Revision != 2 || first.Source.SourceRevision != 1 || len(first.Import.Sources) != 1 {
		t.Fatalf("first set: %+v", first)
	}
	key := h.reserve(t, h.upload(first.Import, "answer_key"))
	second := h.finish(t, key)
	if second.Import.SourceRevision != 2 || len(second.Import.Sources) != 2 {
		t.Fatalf("key dropped exam: %+v", second)
	}
	old, err := h.repo.Reserve(ctx, examIn, h.quotas)
	if err != nil || old.ID != exam.ID {
		t.Fatalf("replay after another source: %v", err)
	}
	replay := h.finish(t, old)
	if replay.Import.Revision != 3 || replay.Source.SourceRevision != 1 {
		t.Fatalf("replay advanced head: %+v", replay)
	}
	altered := examIn
	altered.Source.Filename = "Different.docx"
	if _, err := h.repo.Reserve(ctx, altered, h.quotas); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("changed identity: %v", err)
	}
	replacement := h.reserve(t, h.upload(second.Import, "exam"))
	third := h.finish(t, replacement)
	if len(third.Import.Sources) != 2 || third.Import.SourceRevision != 3 {
		t.Fatalf("replace lost key: %+v", third)
	}
	if _, err := h.repo.Source(ctx, v.ID, exam.ID); err != nil {
		t.Fatalf("historical original disappeared: %v", err)
	}
	other := h.create(t)
	if _, err := h.repo.Source(ctx, other.ID, exam.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-import download accepted: %v", err)
	}
	var source string
	if err := h.pool.QueryRow(ctx, `SELECT source_id::text FROM app.word_import_source_set_items WHERE import_id=$1 AND revision=1 AND role='exam'`, v.ID).Scan(&source); err != nil || source != exam.ID {
		t.Fatalf("history overwritten: %v", err)
	}
	filtered, err := h.repo.List(ctx, domain.Filter{Search: "de tieng anh", Page: 1, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range filtered.Items {
		if item.ID == v.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("Vietnamese title search did not fold accents")
	}
	escaped, err := h.repo.List(ctx, domain.Filter{Search: "%_"})
	if err != nil || escaped.Page.Total != 0 {
		t.Fatalf("wildcards were interpreted: %+v %v", escaped, err)
	}
}

func TestConcurrentFinishesRejectStaleSourcesAndRetainReservation(t *testing.T) {
	h := setup(t)
	v := h.create(t)
	ctx := context.Background()
	a := h.reserve(t, h.upload(v, "exam"))
	b := h.reserve(t, h.upload(v, "answer_key"))
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, s := range []domain.Source{a, b} {
		go func() {
			<-start
			_, err := h.repo.Finish(ctx, domain.Finish{ImportID: v.ID, SourceID: s.ID, Actor: h.actor})
			results <- err
		}()
	}
	close(start)
	ok, conflicts := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			ok++
		} else if errors.Is(err, domain.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatalf("concurrent finish: %d successes %d conflicts", ok, conflicts)
	}
	saved, err := h.repo.Get(ctx, v.ID)
	if err != nil || saved.PendingUploads != 1 || saved.SourceRevision != 1 {
		t.Fatalf("pending conflict lost: %+v %v", saved, err)
	}
}

func TestSimultaneousReplayProducesOneSourceSet(t *testing.T) {
	h := setup(t)
	v := h.create(t)
	in := h.upload(v, "exam")
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan domain.Receipt, 2)
	failures := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			s, err := h.repo.Reserve(ctx, in, h.quotas)
			if err != nil {
				failures <- err
				return
			}
			r, err := h.repo.Finish(ctx, domain.Finish{ImportID: v.ID, SourceID: s.ID, Actor: h.actor})
			if err != nil {
				failures <- err
				return
			}
			results <- r
		})
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	a, b := <-results, <-results
	if a.Source.ID != b.Source.ID || a.Import.SourceRevision != 1 || b.Import.SourceRevision != 1 {
		t.Fatal("concurrent replay duplicated the source")
	}
}

func TestPendingBytesCountTowardsQuota(t *testing.T) {
	h := setup(t)
	v := h.create(t)
	ctx := context.Background()
	q := h.quotas
	q.ActorBytes = 150
	if _, err := h.repo.Reserve(ctx, h.upload(v, "exam"), q); err != nil {
		t.Fatal(err)
	}
	if _, err := h.repo.Reserve(ctx, h.upload(v, "answer_key"), q); !errors.Is(err, domain.ErrQuota) {
		t.Fatalf("interrupted upload evaded quota: %v", err)
	}
}

func TestApplicationCannotRewriteOriginalsOrSourceHistory(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	for _, table := range []string{"word_import_sources", "word_import_source_sets", "word_import_source_set_items"} {
		var canUpdate, canDelete bool
		if err := h.pool.QueryRow(ctx, `SELECT has_table_privilege('quizzivy_app',$1,'UPDATE'),has_table_privilege('quizzivy_app',$1,'DELETE')`, "app."+table).Scan(&canUpdate, &canDelete); err != nil {
			t.Fatal(err)
		}
		if canUpdate || canDelete {
			t.Fatalf("mutable original/history: %s", table)
		}
	}
	v := h.create(t)
	pending := h.reserve(t, h.upload(v, "exam"))
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `INSERT INTO app.word_import_source_sets(import_id,revision,created_by) VALUES($1,1,$2)`, v.ID, h.actor.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.word_import_source_set_items(import_id,revision,role,source_id) VALUES($1,1,'exam',$2)`, v.ID, pending.ID); err == nil {
		t.Fatal("database allowed unfinished bytes into a source set")
	}
}

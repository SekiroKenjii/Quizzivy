//go:build integration

package repositories_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net/http"
	"os"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/application/query"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/storage"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"slices"
	"strings"
	"testing"
	"time"
)

type textPurpose struct {
	pool  *pgxpool.Pool
	repo  *repositories.Postgres
	app   *application.Application
	store *storage.Client
	actor actor.Actor
	ids   []string
}

func newTextPurpose(t *testing.T, storageRequired bool) *textPurpose {
	t.Helper()
	name := os.Getenv("TEST_TEXT_IMPORT_DATABASE_NAME")
	dsn := os.Getenv("TEST_TEXT_IMPORT_DATABASE_URL")
	if name == "" || dsn == "" {
		t.Fatal("an invocation-owned TEST_TEXT_IMPORT_DATABASE_NAME and TEST_TEXT_IMPORT_DATABASE_URL are required")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var actual, role string
	var version, migration int
	var super bool
	err = pool.QueryRow(context.Background(), `SELECT current_database(),current_user,current_setting('server_version_num')::int,(SELECT rolsuper FROM pg_roles WHERE rolname=current_user)`).Scan(&actual, &role, &version, &super)
	if err != nil || actual != name || version < 180000 || super || (role != "quizzivy_app" && role != "quizzivy_migrate") {
		t.Fatalf("purpose identity database=%s role=%s version=%d migration=%d super=%v error=%v", actual, role, version, migration, super, err)
	}
	migrateDSN := os.Getenv("TEST_TEXT_IMPORT_MIGRATE_DATABASE_URL")
	if migrateDSN == "" {
		t.Fatal("the exact purpose migration observer is required")
	}
	observer, err := pgxpool.New(context.Background(), migrateDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(observer.Close)
	var observerDB, observerRole string
	var observerSuper bool
	if err := observer.QueryRow(context.Background(), `SELECT current_database(),current_user,(SELECT rolsuper FROM pg_roles WHERE rolname=current_user),(SELECT max(version_id) FROM public.goose_db_version WHERE is_applied)`).Scan(&observerDB, &observerRole, &observerSuper, &migration); err != nil || observerDB != name || observerRole != "quizzivy_migrate" || observerSuper || migration < 91 {
		t.Fatalf("migration observer database=%s role=%s migration=%d super=%v error=%v", observerDB, observerRole, migration, observerSuper, err)
	}
	h := &textPurpose{pool: pool, repo: repositories.NewPostgres(db.NewContext(pool)), actor: actor.Actor{ID: uuid.NewString()}, ids: []string{}}
	h.actor.Scope = access.Scope{UserID: h.actor.ID}
	if storageRequired {
		bucket := os.Getenv("TEST_TEXT_IMPORT_BUCKET")
		if bucket == "" || os.Getenv("S3_ENDPOINT") == "" {
			t.Fatal("an exact invocation-owned TEST_TEXT_IMPORT_BUCKET and S3_ENDPOINT are required")
		}
		h.store, err = storage.New(context.Background(), storage.Config{Endpoint: os.Getenv("S3_ENDPOINT"), Region: os.Getenv("S3_REGION"), Bucket: bucket, AccessKeyID: os.Getenv("S3_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"), ForcePathStyle: true})
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var users, audits int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.users WHERE id=$1`, h.actor.ID).Scan(&users); err != nil {
			t.Error(err)
		}
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE actor_user_id=$1`, h.actor.ID).Scan(&audits); err != nil {
			t.Error(err)
		}
		for _, id := range h.ids {
			keys, err := h.repo.FilesOf(ctx, id)
			if err != nil {
				t.Error(err)
				continue
			}
			for _, key := range keys {
				if h.store != nil {
					if err := h.store.Delete(ctx, key); err != nil {
						t.Errorf("owned object cleanup: %v", err)
						continue
					}
					body, _, err := h.store.Open(ctx, key)
					if err == nil {
						if body != nil {
							if closeErr := body.Close(); closeErr != nil {
								t.Error(closeErr)
							}
						}
						t.Errorf("owned object remains: %s", key)
					} else if !textObjectMissing(err) {
						t.Errorf("owned object absence query: %v", err)
					}
				}
			}
		}
		t.Logf("purpose=%s role=%s fixtureTeacher=%s imports=%v retainedUsers=%d retainedAppendOnlyAudits=%d; whole-purpose disposal belongs to invocation driver", actual, role, h.actor.ID, h.ids, users, audits)
	})
	if _, err := pool.Exec(context.Background(), `INSERT INTO app.users(id,email,full_name,role_id) VALUES($1,$2,'Text import fixture',(SELECT id FROM app.roles WHERE builtin_key='teacher'))`, h.actor.ID, uuid.NewString()+"@example.test"); err != nil {
		t.Fatal(err)
	}
	h.app = application.New(application.Dependencies{Repo: h.repo, Drafts: h.repo, Runs: h.repo, Artifacts: h.repo, Store: h.store, Inspector: adapters.ImportInspector{}, WorkDir: t.TempDir(), Quotas: domain.DefaultQuotas(), Processing: true})
	return h
}

func (h *textPurpose) create(t *testing.T, title string) domain.Import {
	t.Helper()
	id := uuid.NewString()
	h.ids = append(h.ids, id)
	if _, err := h.pool.Exec(context.Background(), `INSERT INTO app.word_imports(id,title,created_by,request_id) VALUES($1,$2,$3,$4)`, id, title, h.actor.ID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	out, err := h.repo.Get(context.Background(), h.actor.Scope, id)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestTextSourceCharactersReplayAndScopedOriginals(t *testing.T) {
	h := newTextPurpose(t, true)
	ctx := context.Background()
	parent := h.create(t, "Private pasted exam")
	in := command.Paste{ImportID: parent.ID, UploadID: uuid.NewString(), ExpectedRevision: parent.Revision, Actor: h.actor, Text: "tiếng\r\nplain"}
	first, err := h.app.Commands.Paste.Handle(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	in.Text = "tiếng\r\nplain"
	replay, err := h.app.Commands.Paste.Handle(ctx, in)
	if err != nil || replay.Source.ID != first.Source.ID || replay.Source.SourceRevision != first.Source.SourceRevision || replay.Source.Characters == nil || *replay.Source.Characters != 12 {
		t.Fatalf("canonical replay: %+v %v", replay, err)
	}
	in.Text = "changed"
	if _, err := h.app.Commands.Paste.Handle(ctx, in); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("changed replay: %v", err)
	}
	in.Text = "tiếng\r\nplain"
	in.Actor = actor.Actor{ID: uuid.NewString()}
	if _, err := h.app.Commands.Paste.Handle(ctx, in); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign paste: %v", err)
	}
	for _, scope := range []access.Scope{{UserID: uuid.NewString()}, {}} {
		if _, err := h.repo.Source(ctx, scope, parent.ID, first.Source.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("foreign original: %v", err)
		}
	}
	src, err := h.repo.Source(ctx, h.actor.Scope, parent.ID, first.Source.ID)
	if err != nil || src.Characters == nil || *src.Characters != 12 || src.Format != "text" {
		t.Fatalf("original metadata: %+v %v", src, err)
	}
	sources, err := h.repo.Sources(ctx, parent.ID, first.Source.SourceRevision)
	if err != nil || len(sources) != 1 || sources[0].Characters == nil || *sources[0].Characters != 12 {
		t.Fatalf("source set: %+v %v", sources, err)
	}
	original, _, err := h.store.Open(ctx, src.StorageKey)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(original)
	closeErr := original.Close()
	if err != nil || closeErr != nil || string(data) != "tiếng\r\nplain" {
		t.Fatalf("original bytes: %q %v %v", data, err, closeErr)
	}
}

func TestTextSourceCharactersConstraintsAndLegacyColumnInsert(t *testing.T) {
	h := newTextPurpose(t, false)
	ctx := context.Background()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	parent := uuid.NewString()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := tx.Rollback(ctx); err != nil {
			t.Error(err)
		}
		var n int
		if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM app.word_imports WHERE id=$1`, parent).Scan(&n); err != nil {
			t.Error(err)
		} else if n != 0 {
			t.Error("rollback import remains")
		}
	})
	if _, err := tx.Exec(ctx, `INSERT INTO app.word_imports(id,title,created_by,request_id) VALUES($1,'Rollback source constraints',$2,$3)`, parent, h.actor.ID, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	repo := repositories.NewPostgres(db.NewContext(tx))
	hash := sha256.Sum256([]byte("x"))
	n := 1
	over := 100001
	zero := 0
	for _, test := range []struct {
		name, format string
		characters   *int
		valid        bool
	}{{"text present", "text", &n, true}, {"text missing", "text", nil, false}, {"text zero", "text", &zero, false}, {"text overlimit", "text", &over, false}, {"Word absent", "docx", nil, true}, {"Word present", "docx", &n, false}} {
		t.Run(test.name, func(t *testing.T) {
			_, err := repo.Reserve(ctx, domain.Reserve{Actor: h.actor, Source: domain.Source{ImportID: parent, UploadID: uuid.NewString(), ExpectedRevision: 1, Role: "exam", Filename: "fixture", Format: test.format, Characters: test.characters, Bytes: 1, SHA256: hash[:]}}, domain.DefaultQuotas())
			if (err == nil) != test.valid {
				t.Fatalf("constraint valid=%v error=%v", test.valid, err)
			}
		})
	}
	var characters *int
	err = tx.QueryRow(ctx, `INSERT INTO app.word_import_sources(import_id,upload_id,expected_revision,role,filename,format,bytes,checksum_sha256,storage_key,uploaded_by) VALUES($1,$2,1,'exam','old.docx','docx',1,$3,$4,$5) RETURNING characters`, parent, uuid.NewString(), hash[:], "legacy/"+uuid.NewString(), h.actor.ID).Scan(&characters)
	if err != nil || characters != nil {
		t.Fatalf("old column insert: %v %v", characters, err)
	}
}

func TestTextHistorySearchExcludesOnlyPastedFilenameAndKeepsFacets(t *testing.T) {
	h := newTextPurpose(t, true)
	ctx := context.Background()
	parent := h.create(t, "Real title")
	pasted, err := h.app.Commands.Paste.Handle(ctx, command.Paste{ImportID: parent.ID, UploadID: uuid.NewString(), ExpectedRevision: parent.Revision, Actor: h.actor, Text: "plain"})
	if err != nil {
		t.Fatal(err)
	}
	list, err := h.repo.List(ctx, domain.Filter{Scope: h.actor.Scope, Search: "pasted-text"})
	if err != nil || list.Page.Total != 0 || list.Facets.All != 0 {
		t.Fatalf("synthetic filename matched: %+v %v", list, err)
	}
	list, err = h.repo.List(ctx, domain.Filter{Scope: h.actor.Scope, Search: "real title"})
	if err != nil || list.Page.Total != 1 || list.Facets.All != 1 || list.Items[0].Sources[0].Characters == nil {
		t.Fatalf("title population: %+v %v", list, err)
	}
	key, err := h.app.Commands.Upload.Handle(ctx, command.Upload{ImportID: parent.ID, UploadID: uuid.NewString(), ExpectedRevision: pasted.Import.Revision, Role: "answer_key", Filename: "real-key.pdf", Actor: h.actor, Body: strings.NewReader("%PDF-1.4\nprivate key")})
	if err != nil {
		t.Fatal(err)
	}
	list, err = h.repo.List(ctx, domain.Filter{Scope: h.actor.Scope, Search: "real-key"})
	if err != nil || list.Page.Total != 1 || list.Facets.All != 1 || len(list.Items[0].Sources) != 2 {
		t.Fatalf("real companion filename population: %+v %v", list, err)
	}
	if key.Source.Characters != nil {
		t.Fatal("file source invented characters")
	}
	list, err = h.repo.List(ctx, domain.Filter{Scope: access.Scope{UserID: uuid.NewString()}, Search: "real-key"})
	if err != nil || list.Page.Total != 0 || list.Facets.All != 0 {
		t.Fatalf("foreign history: %+v %v", list, err)
	}
}

func TestTextPipelineCompanionKeySourceViewDownloadAndRetention(t *testing.T) {
	h := newTextPurpose(t, true)
	ctx := context.Background()
	parent := h.create(t, "Text pipeline")
	original := "Part I: Synthetic\nQuestion 1 Choose A. first B. second"
	paste, err := h.app.Commands.Paste.Handle(ctx, command.Paste{ImportID: parent.ID, UploadID: uuid.NewString(), ExpectedRevision: parent.Revision, Actor: h.actor, Text: original})
	if err != nil {
		t.Fatal(err)
	}
	key, err := h.app.Commands.Upload.Handle(ctx, command.Upload{ImportID: parent.ID, UploadID: uuid.NewString(), ExpectedRevision: paste.Import.Revision, Role: "answer_key", Filename: "key.docx", Actor: h.actor, Body: bytes.NewReader(nativePaper(t, []string{"Part I: Synthetic", "Question 1. B"}))})
	if err != nil {
		t.Fatal(err)
	}
	scheduled, err := h.repo.Schedule(ctx, domain.Schedule{ImportID: parent.ID, RequestID: uuid.NewString(), ExpectedRevision: key.Import.Revision, SourceRevision: key.Import.SourceRevision, Actor: h.actor, PipelineVersion: worker.PipelineVersion, MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.repo.Claim(ctx, domain.ClaimPolicy{WorkerID: uuid.NewString(), PipelineVersion: "word-pipeline-v2", Lease: time.Minute, GlobalLimit: 2, ActorLimit: 1}); !errors.Is(err, domain.ErrNoWork) {
		t.Fatalf("v2 claimed v3: %v", err)
	}
	run, err := h.repo.Claim(ctx, domain.ClaimPolicy{WorkerID: uuid.NewString(), PipelineVersion: worker.PipelineVersion, Lease: time.Minute, GlobalLimit: 2, ActorLimit: 1})
	if err != nil || run.ID != scheduled.ID {
		t.Fatalf("claim: %+v %v", run, err)
	}
	pipeline := worker.Pipeline{Sources: h.repo, Artifacts: h.repo, Store: h.store, Engine: adapters.ImportProcessing{WorkDir: t.TempDir()}, WorkDir: t.TempDir(), Quotas: domain.ArtifactQuotas{ActorBytes: 1 << 30, GlobalBytes: 2 << 30, SetsPerImport: 100}}
	out, err := pipeline.Process(ctx, run, func(stage string) error { return h.repo.Progress(ctx, run.Claim(), stage) })
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.Complete(ctx, run.Claim(), out); err != nil {
		t.Fatal(err)
	}
	var draft domain.Draft
	if err := json.Unmarshal(out.Draft, &draft); err != nil {
		t.Fatal(err)
	}
	questions := draft.Questions()
	if len(questions) != 1 || questions[0].Answer.State != domain.AnswerKnown || !slices.Equal(questions[0].Answer.OptionIDs, []string{questions[0].Options[1].ID}) {
		t.Fatal("text/key answer association changed")
	}
	view, err := h.app.Queries.SourceView.Handle(ctx, query.SourceView{ImportID: parent.ID, Role: "exam", Scope: h.actor.Scope})
	if err != nil || view.Filename != "pasted-text.txt" || view.Evidence.Version != "text-lines-v1" || len(view.Evidence.Blocks) != 2 || view.Evidence.Blocks[1].ID != "l2" {
		t.Fatalf("private source view: %+v %v", view, err)
	}
	if _, err := h.app.Queries.SourceView.Handle(ctx, query.SourceView{ImportID: parent.ID, Role: "exam", Scope: access.Scope{UserID: uuid.NewString()}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign source view: %v", err)
	}
	download, err := h.app.Queries.Download.Handle(ctx, query.Download{ImportID: parent.ID, SourceID: paste.Source.ID, Scope: h.actor.Scope})
	if err != nil || download.URL == "" {
		t.Fatalf("private download available=%v error=%v", download.URL != "", err)
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Get(download.URL)
	if err != nil {
		t.Fatal("private presigned exchange failed")
	}
	downloaded, readErr := io.ReadAll(io.LimitReader(response.Body, int64(len(original))+1))
	closeErr := response.Body.Close()
	if response.StatusCode != http.StatusOK || readErr != nil || closeErr != nil || string(downloaded) != original {
		t.Fatalf("private presigned exchange status=%d exactBytes=%v readError=%v closeError=%v", response.StatusCode, string(downloaded) == original, readErr, closeErr)
	}
	if _, err := h.app.Queries.Download.Handle(ctx, query.Download{ImportID: parent.ID, SourceID: paste.Source.ID, Scope: access.Scope{UserID: uuid.NewString()}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign download: %v", err)
	}
	current, err := h.repo.Get(ctx, h.actor.Scope, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.repo.Cancel(ctx, domain.Cancel{ImportID: parent.ID, ExpectedRevision: current.Revision, Actor: h.actor}); err != nil {
		t.Fatal(err)
	}
	keys, err := h.repo.FilesOf(ctx, parent.ID)
	if err != nil || len(keys) < 3 {
		t.Fatalf("retention ownership: %v %v", keys, err)
	}
	swept, err := (worker.Sweeper{Repo: h.repo, Store: h.store, Policy: domain.DefaultRetention(), Batch: 20}).Sweep(ctx, time.Now().Add(8*24*time.Hour))
	if err != nil || swept.Removed != 1 || swept.Failed != 0 {
		t.Fatalf("text retention: %+v %v", swept, err)
	}
	current, err = h.repo.Get(ctx, h.actor.Scope, parent.ID)
	if err != nil || current.FilesRemovedAt == nil || !textCharacterSource(current.Sources, paste.Source.ID) {
		t.Fatalf("retained history: %+v %v", current, err)
	}
	for _, key := range keys {
		body, _, err := h.store.Open(ctx, key)
		if err == nil {
			if body != nil {
				if closeErr := body.Close(); closeErr != nil {
					t.Error(closeErr)
				}
			}
			t.Fatalf("removed object exists: %s", key)
		}
		if !textObjectMissing(err) {
			t.Fatal(err)
		}
	}
	if _, err := h.app.Queries.Download.Handle(ctx, query.Download{ImportID: parent.ID, SourceID: paste.Source.ID, Scope: h.actor.Scope}); !errors.Is(err, domain.ErrFilesRemoved) {
		t.Fatalf("removed download: %v", err)
	}
}

func textObjectMissing(err error) bool {
	var api smithy.APIError
	return errors.As(err, &api) && (api.ErrorCode() == "NoSuchKey" || api.ErrorCode() == "NotFound")
}
func textCharacterSource(sources []domain.Source, id string) bool {
	for _, source := range sources {
		if source.ID == id {
			return source.Characters != nil
		}
	}
	return false
}

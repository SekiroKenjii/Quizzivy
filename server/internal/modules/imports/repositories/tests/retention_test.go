//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/repositories"
	"quizzivy/internal/platform/db"
)

func clock(t *testing.T, h harness) time.Time {
	t.Helper()
	var now time.Time
	if err := h.pool.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	return now
}

func rollback(t *testing.T, h harness) (*repositories.Postgres, pgx.Tx) {
	t.Helper()
	tx, err := h.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return repositories.NewPostgres(db.NewContext(tx)), tx
}

func withDraft(t *testing.T, h harness, status string) string {
	t.Helper()
	run := h.schedule(t, uuid.NewString(), 3)
	ctx := context.Background()
	if _, err := h.pool.Exec(ctx, `UPDATE app.word_import_runs SET status='cancelled',claim_token=claim_token+1,completed_at=clock_timestamp() WHERE id=$1`, run.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `UPDATE app.word_imports SET status=$2 WHERE id=$1`, run.ImportID, status); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `INSERT INTO app.word_import_drafts(import_id,run_id,body) VALUES($1,$2,'{}')`, run.ImportID, run.ID); err != nil {
		t.Fatal(err)
	}
	return run.ImportID
}

func audited(t *testing.T, tx pgx.Tx, id string) []string {
	t.Helper()
	rows, err := tx.Query(context.Background(), `SELECT action FROM app.audit_log WHERE entity='word_import' AND entity_id=$1 ORDER BY id`, id)
	if err != nil {
		t.Fatal(err)
	}
	actions, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return actions
}

func TestAnIdleImportIsClosedOnlyOnceNothingHasTouchedItSinceTheCutoff(t *testing.T) {
	h := setup(t)
	before := clock(t, h)
	waiting := h.create(t).ID
	reviewed := withDraft(t, h, "needs_review")
	after := clock(t, h)
	repo, tx := rollback(t, h)
	ctx := context.Background()

	closed, err := repo.CloseIdle(ctx, before, 1000)
	if err != nil || slices.Contains(closed, waiting) || slices.Contains(closed, reviewed) {
		t.Fatalf("imports touched after the cutoff were closed: %v %v", closed, err)
	}
	closed, err = repo.CloseIdle(ctx, after, 1000)
	if err != nil || !slices.Contains(closed, waiting) || !slices.Contains(closed, reviewed) {
		t.Fatalf("idle imports were not closed: %v %v", closed, err)
	}
	var status string
	var idle bool
	if err := tx.QueryRow(ctx, `SELECT status,closed_idle FROM app.word_imports WHERE id=$1`, waiting).Scan(&status, &idle); err != nil || status != "cancelled" || !idle {
		t.Fatalf("closed import status %q idle %v: %v", status, idle, err)
	}
	if actions := audited(t, tx, waiting); !slices.Contains(actions, "import.closed_idle") {
		t.Fatalf("closure not audited: %v", actions)
	}
}

func TestADraftSaveOrAnUploadKeepsAnImportAlive(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	reviewed := withDraft(t, h, "needs_review")
	waiting := h.create(t)
	cutoff := clock(t, h)
	if _, err := h.repo.SaveDraft(ctx, domain.SaveDraft{ImportID: reviewed, ExpectedRevision: 1, Actor: h.actor}); err != nil {
		t.Fatal(err)
	}
	h.reserve(t, h.upload(waiting, "exam"))
	repo, _ := rollback(t, h)
	closed, err := repo.CloseIdle(ctx, cutoff, 1000)
	if err != nil || slices.Contains(closed, reviewed) || slices.Contains(closed, waiting.ID) {
		t.Fatalf("an import touched after the cutoff was closed: %v %v", closed, err)
	}
}

func TestExpiredFilesFollowTheCommitAndCancelClocks(t *testing.T) {
	h := setup(t)
	before := clock(t, h)
	old := withDraft(t, h, "committed")
	recent := withDraft(t, h, "committed")
	cancelled := withDraft(t, h, "cancelled")
	after := clock(t, h)
	repo, tx := rollback(t, h)
	ctx := context.Background()
	for id, at := range map[string]time.Time{old: after.AddDate(0, 0, -31), recent: after.AddDate(0, 0, -29)} {
		if _, err := tx.Exec(ctx, `INSERT INTO app.word_import_commits(import_id,request_id,draft_revision,digest,committed_by,committed_at) VALUES($1,$2,1,$3,$4,$5)`,
			id, uuid.NewString(), make([]byte, 32), h.actor.ID, at); err != nil {
			t.Fatal(err)
		}
	}
	expired, err := ids(repo.ExpiredFiles(ctx, after.AddDate(0, 0, -30), before, domain.Cursor{}, 1000))
	if err != nil || !slices.Contains(expired, old) || slices.Contains(expired, recent) || slices.Contains(expired, cancelled) {
		t.Fatalf("expired at the cutoffs: %v %v", expired, err)
	}
	expired, err = ids(repo.ExpiredFiles(ctx, after.AddDate(0, 0, -30), after, domain.Cursor{}, 1000))
	if err != nil || !slices.Contains(expired, cancelled) {
		t.Fatalf("a cancellation past its cutoff is not expired: %v %v", expired, err)
	}
}

func ids(page []domain.Cursor, err error) ([]string, error) {
	out := make([]string, len(page))
	for i, c := range page {
		out[i] = c.ID
	}
	return out, err
}

func TestAnIdleClosureIsDueAtOnceAndTheCursorMovesPastIt(t *testing.T) {
	h := setup(t)
	before := clock(t, h)
	id := h.create(t).ID
	after := clock(t, h)
	repo, _ := rollback(t, h)
	ctx := context.Background()
	if closed, err := repo.CloseIdle(ctx, after, 1000); err != nil || !slices.Contains(closed, id) {
		t.Fatalf("closing: %v %v", closed, err)
	}
	page, err := repo.ExpiredFiles(ctx, before, before, domain.Cursor{}, 1000)
	var mine domain.Cursor
	for _, c := range page {
		if c.ID == id {
			mine = c
		}
	}
	if err != nil || mine.ID == "" {
		t.Fatalf("an import closed as idle is not due at once: %v %v", page, err)
	}
	rest, err := ids(repo.ExpiredFiles(ctx, before, before, mine, 1000))
	if err != nil || slices.Contains(rest, id) {
		t.Fatalf("the cursor did not move past the import: %v %v", rest, err)
	}
}

func TestRemovingFilesDeletesTheDraftMarksTheImportAndIsIdempotent(t *testing.T) {
	h := setup(t)
	id := withDraft(t, h, "cancelled")
	repo := h.repo
	ctx := context.Background()
	keys, err := repo.FilesOf(ctx, id)
	if err != nil || len(keys) != 1 || !strings.HasPrefix(keys[0], "originals/"+id+"/") {
		t.Fatalf("files of an import: %v %v", keys, err)
	}
	for range 2 {
		if err := repo.FilesRemoved(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	var drafts int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM app.word_import_drafts WHERE import_id=$1`, id).Scan(&drafts); err != nil || drafts != 0 {
		t.Fatalf("draft rows left: %d %v", drafts, err)
	}
	got, err := repo.Get(ctx, id)
	if err != nil || got.FilesRemovedAt == nil {
		t.Fatalf("the import does not report its files removed: %v %v", got.FilesRemovedAt, err)
	}
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	actions := audited(t, tx, id)
	if n := len(slices.DeleteFunc(slices.Clone(actions), func(a string) bool { return a != "import.files_removed" })); n != 1 {
		t.Fatalf("removal audited %d times: %v", n, actions)
	}
	later := time.Now().Add(time.Hour)
	if expired, err := ids(repo.ExpiredFiles(ctx, later, later, domain.Cursor{}, 1000)); err != nil || slices.Contains(expired, id) {
		t.Fatalf("a removed import is swept again: %v %v", expired, err)
	}
}

func TestFilesAreRemovedOnlyFromImportsThatCanNeverRunAgain(t *testing.T) {
	h := setup(t)
	id := withDraft(t, h, "needs_review")
	repo, tx := rollback(t, h)
	ctx := context.Background()
	if err := repo.FilesRemoved(ctx, id); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("removing a reviewable import's files: %v", err)
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT guard`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE app.word_imports SET files_removed_at=now() WHERE id=$1`, id); err == nil {
		t.Fatal("the schema accepted removed files on an import still under review")
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT guard`); err != nil {
		t.Fatal(err)
	}
}

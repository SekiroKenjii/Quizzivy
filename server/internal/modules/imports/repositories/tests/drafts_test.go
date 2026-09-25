//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/imports/domain"
	"testing"

	"github.com/google/uuid"
)

func machineDraft(title string) json.RawMessage {
	raw, _ := json.Marshal(domain.Draft{Version: domain.DraftVersion, Title: title, Sections: []domain.DraftSection{}, Notices: []domain.Finding{}, Acknowledged: []string{}})
	return raw
}

func (h harness) processed(t *testing.T, title string) domain.Run {
	t.Helper()
	version := uuid.NewString()
	run := h.schedule(t, version, 3)
	claimed := h.claim(t, policy(version))
	if claimed.ID != run.ID {
		t.Fatalf("claimed %s, scheduled %s", claimed.ID, run.ID)
	}
	if err := h.repo.Complete(context.Background(), claimed.Claim(), domain.Outcome{Result: json.RawMessage(`{"schemaVersion":1}`), Draft: machineDraft(title)}); err != nil {
		t.Fatal(err)
	}
	return claimed
}

func (h harness) reprocess(t *testing.T, importID, title string) {
	t.Helper()
	ctx := context.Background()
	current, err := h.repo.Get(ctx, importID)
	if err != nil {
		t.Fatal(err)
	}
	version := uuid.NewString()
	if _, err := h.repo.Schedule(ctx, domain.Schedule{ImportID: importID, RequestID: uuid.NewString(), PipelineVersion: version, ExpectedRevision: current.Revision, SourceRevision: current.SourceRevision, Actor: h.actor, MaxAttempts: 3}); err != nil {
		t.Fatal(err)
	}
	claimed := h.claim(t, policy(version))
	if err := h.repo.Complete(ctx, claimed.Claim(), domain.Outcome{Result: json.RawMessage(`{"schemaVersion":1}`), Draft: machineDraft(title)}); err != nil {
		t.Fatal(err)
	}
}

func TestACompletedRunLeavesItsDraftUnderReview(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	run := h.processed(t, "TEST 1")
	stored, err := h.repo.Draft(ctx, run.ImportID)
	if err != nil || stored.Revision != 1 || stored.Draft.Title != "TEST 1" || stored.Status != "needs_review" || stored.Reprocessed {
		t.Fatalf("draft %+v err %v", stored, err)
	}
	current, err := h.repo.Get(ctx, run.ImportID)
	if err != nil || current.DraftRevision != 1 || current.Run == nil || current.Run.Status != "succeeded" || current.Run.Stage != "ready" {
		t.Fatalf("import %+v err %v", current, err)
	}
	listed, err := h.repo.List(ctx, domain.Filter{Search: current.Title})
	if err != nil || len(listed.Items) == 0 || listed.Items[0].Run == nil {
		t.Fatalf("history without progress: %+v %v", listed.Items, err)
	}
	if _, err := h.repo.Draft(ctx, h.create(t).ID); !errors.Is(err, domain.ErrNoDraft) {
		t.Fatalf("unprocessed import: %v", err)
	}
	if _, err := h.repo.Draft(ctx, uuid.NewString()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing import: %v", err)
	}
}

func TestReprocessingReplacesAnUntouchedDraftButNeverTeacherEdits(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	run := h.processed(t, "first")
	h.reprocess(t, run.ImportID, "second")
	untouched, err := h.repo.Draft(ctx, run.ImportID)
	if err != nil || untouched.Draft.Title != "second" || untouched.Revision != 2 || untouched.Reprocessed {
		t.Fatalf("untouched draft %+v err %v", untouched, err)
	}
	edited := untouched.Draft
	edited.Title = "teacher title"
	saved, err := h.repo.SaveDraft(ctx, domain.SaveDraft{ImportID: run.ImportID, ExpectedRevision: untouched.Revision, Draft: edited, Actor: h.actor})
	if err != nil || saved.Revision != 3 {
		t.Fatalf("save %+v err %v", saved, err)
	}
	h.reprocess(t, run.ImportID, "third")
	kept, err := h.repo.Draft(ctx, run.ImportID)
	if err != nil || kept.Draft.Title != "teacher title" || kept.Revision != 3 || !kept.Reprocessed {
		t.Fatalf("teacher edit overwritten: %+v err %v", kept, err)
	}
	if _, err := h.repo.AdoptCandidate(ctx, domain.AdoptCandidate{ImportID: run.ImportID, ExpectedRevision: 2, Actor: h.actor}); !errors.Is(err, domain.ErrStale) {
		t.Fatalf("stale adopt: %v", err)
	}
	adopted, err := h.repo.AdoptCandidate(ctx, domain.AdoptCandidate{ImportID: run.ImportID, ExpectedRevision: 3, Actor: h.actor})
	if err != nil || adopted.Draft.Title != "third" || adopted.Revision != 4 || adopted.Reprocessed {
		t.Fatalf("adopted %+v err %v", adopted, err)
	}
	if _, err := h.repo.AdoptCandidate(ctx, domain.AdoptCandidate{ImportID: run.ImportID, ExpectedRevision: 4, Actor: h.actor}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("adopt without a candidate: %v", err)
	}
}

func TestStaleSavesAndClosedImportsAreRejected(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	run := h.processed(t, "TEST 2")
	stored, err := h.repo.Draft(ctx, run.ImportID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.repo.SaveDraft(ctx, domain.SaveDraft{ImportID: run.ImportID, ExpectedRevision: stored.Revision + 1, Draft: stored.Draft, Actor: h.actor}); !errors.Is(err, domain.ErrStale) {
		t.Fatalf("stale save: %v", err)
	}
	current, err := h.repo.Get(ctx, run.ImportID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.repo.Cancel(ctx, domain.Cancel{ImportID: run.ImportID, ExpectedRevision: current.Revision, Actor: h.actor}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.repo.SaveDraft(ctx, domain.SaveDraft{ImportID: run.ImportID, ExpectedRevision: stored.Revision, Draft: stored.Draft, Actor: h.actor}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("save after cancel: %v", err)
	}
}

func TestACommitIsRecordedOnceAndClosesTheImport(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	run := h.processed(t, "TEST 3")
	record := domain.CommitRecord{Commit: domain.Commit{ImportID: run.ImportID, RequestID: uuid.NewString(), DraftRevision: 1, Digest: make([]byte, 32)}, Actor: h.actor}
	stale := record
	stale.DraftRevision = 2
	if err := h.repo.RecordCommit(ctx, stale); !errors.Is(err, domain.ErrStale) {
		t.Fatalf("stale commit: %v", err)
	}
	if err := h.repo.RecordCommit(ctx, record); err != nil {
		t.Fatal(err)
	}
	stored, err := h.repo.Commit(ctx, run.ImportID)
	if err != nil || stored.RequestID != record.RequestID || stored.TestID != nil {
		t.Fatalf("commit %+v err %v", stored, err)
	}
	if err := h.repo.RecordCommit(ctx, record); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("second commit: %v", err)
	}
	current, err := h.repo.Get(ctx, run.ImportID)
	if err != nil || current.Status != "committed" {
		t.Fatalf("import %+v err %v", current, err)
	}
	if _, err := h.repo.Cancel(ctx, domain.Cancel{ImportID: run.ImportID, ExpectedRevision: current.Revision, Actor: h.actor}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("cancel after commit: %v", err)
	}
}

func TestTheApplicationCannotRewriteCommitHistory(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	for table, want := range map[string][2]bool{"word_import_commits": {false, false}, "word_import_drafts": {true, false}} {
		var canUpdate, canDelete bool
		if err := h.pool.QueryRow(ctx, `SELECT has_table_privilege('quizzivy_app',$1,'UPDATE'),has_table_privilege('quizzivy_app',$1,'DELETE')`, "app."+table).Scan(&canUpdate, &canDelete); err != nil {
			t.Fatal(err)
		}
		if canUpdate != want[0] || canDelete != want[1] {
			t.Fatalf("%s: update=%v delete=%v", table, canUpdate, canDelete)
		}
	}
}

func TestTheLatestRunCarriesItsStartAndTheTeachersKeyPaper(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	v := h.create(t)
	receipt := h.finish(t, h.reserve(t, h.upload(v, "exam")))
	if _, err := h.repo.Schedule(ctx, domain.Schedule{ImportID: v.ID, RequestID: uuid.NewString(), PipelineVersion: uuid.NewString(), ExpectedRevision: receipt.Import.Revision, SourceRevision: receipt.Import.SourceRevision, Actor: h.actor, MaxAttempts: 3, Profile: domain.RecognitionProfile{KeyPaper: 2}}); err != nil {
		t.Fatal(err)
	}
	current, err := h.repo.Get(ctx, v.ID)
	if err != nil || current.Run == nil || current.Run.Profile.KeyPaper != 2 || current.Run.CreatedAt.IsZero() || current.Run.CreatedAt.After(current.Run.UpdatedAt) {
		t.Fatalf("run %+v err %v", current.Run, err)
	}
}

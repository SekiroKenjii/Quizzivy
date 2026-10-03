//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

type shelf struct {
	awaiting, review, committed, cancelled string
	ready, pending                         domain.Source
	reviewRun                              domain.Run
}

func (h harness) shelf(t *testing.T) shelf {
	t.Helper()
	ctx := context.Background()
	var s shelf
	v := h.create(t)
	receipt := h.finish(t, h.reserve(t, h.upload(v, "exam")))
	s.awaiting, s.ready = v.ID, receipt.Source
	s.pending = h.reserve(t, h.upload(receipt.Import, "answer_key"))
	s.reviewRun = h.processed(t, "Đề đang duyệt")
	s.review = s.reviewRun.ImportID
	committed := h.processed(t, "Đề đã tạo")
	s.committed = committed.ImportID
	if err := h.repo.RecordCommit(ctx, domain.CommitRecord{Commit: domain.Commit{ImportID: s.committed, RequestID: uuid.NewString(), DraftRevision: 1, Digest: make([]byte, 32)}, Actor: h.actor}); err != nil {
		t.Fatal(err)
	}
	cancelled := h.create(t)
	if _, err := h.repo.Cancel(ctx, domain.Cancel{ImportID: cancelled.ID, ExpectedRevision: cancelled.Revision, Actor: h.actor}); err != nil {
		t.Fatal(err)
	}
	s.cancelled = cancelled.ID
	return s
}

func (h harness) state(t *testing.T, id string) string {
	t.Helper()
	var out string
	if err := h.pool.QueryRow(context.Background(), `SELECT concat_ws('|', i.revision, i.status, i.updated_at, coalesce(i.source_revision, 0),
		(SELECT count(*) FROM app.word_import_sources s WHERE s.import_id = i.id),
		(SELECT count(*) FROM app.word_import_sources s WHERE s.import_id = i.id AND s.ready),
		(SELECT count(*) FROM app.word_import_runs r WHERE r.import_id = i.id),
		(SELECT coalesce(max(d.revision), 0) FROM app.word_import_drafts d WHERE d.import_id = i.id),
		(SELECT count(*) FROM app.word_import_commits c WHERE c.import_id = i.id))
		  FROM app.word_imports i WHERE i.id = $1`, id).Scan(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func emptyDraft(title string) domain.Draft {
	return domain.Draft{Version: domain.DraftVersion, Title: title, Sections: []domain.DraftSection{}, Notices: []domain.Finding{}, Acknowledged: []string{}}
}

func TestAnotherCreatorsImportAnswersAsAMissingOne(t *testing.T) {
	a, b := setup(t), setup(t)
	ctx := context.Background()
	s := a.shelf(t)
	by := b.actor
	scope := access.Scope{UserID: by.ID}
	before := map[string]string{}
	for _, id := range []string{s.awaiting, s.review, s.committed, s.cancelled} {
		before[id] = a.state(t, id)
	}
	targets := map[string]string{"awaiting sources": s.awaiting, "under review": s.review, "committed": s.committed, "cancelled": s.cancelled, "missing": uuid.NewString()}
	for label, id := range targets {
		for op, err := range map[string]error{
			"get":    second(b.repo.Get(ctx, scope, id)),
			"draft":  second(b.repo.Draft(ctx, scope, id)),
			"commit": second(b.repo.Commit(ctx, scope, id)),
			"source": second(b.repo.Source(ctx, scope, id, s.ready.ID)),
			"reserve": second(b.repo.Reserve(ctx, domain.Reserve{Actor: by, Source: domain.Source{ImportID: id, UploadID: s.pending.UploadID, ExpectedRevision: 1,
				Role: "exam", Filename: "B.docx", Format: "docx", Bytes: 100, SHA256: make([]byte, 32)}}, b.quotas)),
			"finish":   second(b.repo.Finish(ctx, domain.Finish{ImportID: id, SourceID: s.pending.ID, Actor: by})),
			"schedule": second(b.repo.Schedule(ctx, domain.Schedule{ImportID: id, RequestID: s.reviewRun.RequestID, PipelineVersion: s.reviewRun.PipelineVersion, ExpectedRevision: 999, SourceRevision: 1, Actor: by, MaxAttempts: 3})),
			"cancel":   second(b.repo.Cancel(ctx, domain.Cancel{ImportID: id, ExpectedRevision: 999, Actor: by})),
			"save":     second(b.repo.SaveDraft(ctx, domain.SaveDraft{ImportID: id, ExpectedRevision: 999, Draft: emptyDraft("B"), Actor: by})),
			"adopt":    second(b.repo.AdoptCandidate(ctx, domain.AdoptCandidate{ImportID: id, ExpectedRevision: 999, Actor: by})),
			"record":   b.repo.RecordCommit(ctx, domain.CommitRecord{Commit: domain.Commit{ImportID: id, RequestID: uuid.NewString(), DraftRevision: 1, Digest: make([]byte, 32)}, Actor: by}),
		} {
			if !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("B's %s on a %s import: %v, want ErrNotFound", op, label, err)
			}
		}
	}
	for id, was := range before {
		if now := a.state(t, id); now != was {
			t.Errorf("B's refused calls changed A's import %s from %s to %s", id, was, now)
		}
	}
	if _, err := b.repo.Source(ctx, scope, s.awaiting, s.ready.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("B reading A's source: %v", err)
	}
	own := b.create(t)
	if _, err := b.repo.Source(ctx, scope, own.ID, s.ready.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("B reading A's source through B's own import: %v", err)
	}
	if _, err := b.repo.Draft(ctx, scope, own.ID); !errors.Is(err, domain.ErrNoDraft) {
		t.Errorf("B's own import without a draft: %v, want ErrNoDraft", err)
	}
}

func TestEachCreatorListsAndSearchesOnlyTheirOwnImports(t *testing.T) {
	a, b := setup(t), setup(t)
	ctx := context.Background()
	mark, request := uuid.NewString()[:8], uuid.NewString()
	titled, err := a.repo.Create(ctx, domain.Create{RequestID: request, Title: "Riêng " + mark, Actor: a.actor}, a.quotas)
	if err != nil {
		t.Fatal(err)
	}
	named := a.create(t)
	upload := a.upload(named, "exam")
	upload.Source.Filename = "rieng-" + mark + ".docx"
	a.finish(t, a.reserve(t, upload))
	mine := b.create(t)
	for name, c := range map[string]struct {
		scope access.Scope
		want  []string
	}{
		"A":              {access.Scope{UserID: a.actor.ID}, []string{titled.ID, named.ID}},
		"B":              {access.Scope{UserID: b.actor.ID}, nil},
		"scope.all":      {everyone, []string{titled.ID, named.ID}},
		"the zero scope": {access.Scope{}, nil},
	} {
		listed, err := b.repo.List(ctx, domain.Filter{Search: mark, Scope: c.scope, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		if got := importIDs(listed.Items); !sameSet(got, c.want) || listed.Page.Total != len(c.want) {
			t.Errorf("%s searching A's mark lists %v (total %d), want %v", name, got, listed.Page.Total, c.want)
		}
	}
	for name, c := range map[string]struct {
		scope access.Scope
		finds bool
	}{"B": {access.Scope{UserID: b.actor.ID}, true}, "A": {access.Scope{UserID: a.actor.ID}, false}, "the zero scope": {access.Scope{}, false}} {
		listed, err := b.repo.List(ctx, domain.Filter{Scope: c.scope, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		if found := contains(importIDs(listed.Items), mine.ID); found != c.finds {
			t.Errorf("%s's history shows B's import: %v, want %v", name, found, c.finds)
		}
		if name == "the zero scope" && listed.Page.Total != 0 {
			t.Errorf("the zero scope's history totals %d", listed.Page.Total)
		}
	}
	replayed, err := b.repo.Create(ctx, domain.Create{RequestID: request, Title: "Riêng " + mark, Actor: b.actor}, b.quotas)
	if err != nil || replayed.ID == titled.ID || replayed.CreatedBy != b.actor.ID {
		t.Errorf("B reusing an id A used made %+v (%v), want B's own import", replayed, err)
	}
}

func TestScopeAllActsOnAnotherCreatorsImportAsItsActor(t *testing.T) {
	a, admin := setup(t), setup(t)
	ctx := context.Background()
	s := a.shelf(t)
	all := access.Scope{UserID: admin.actor.ID, All: true}
	by := actor.Actor{ID: admin.actor.ID, Scope: all}
	for label, id := range map[string]string{"awaiting sources": s.awaiting, "under review": s.review, "committed": s.committed, "cancelled": s.cancelled} {
		if _, err := admin.repo.Get(ctx, all, id); err != nil {
			t.Errorf("scope.all reading a %s import: %v", label, err)
		}
	}
	if _, err := admin.repo.Source(ctx, all, s.awaiting, s.ready.ID); err != nil {
		t.Errorf("scope.all reading a source: %v", err)
	}
	if _, err := admin.repo.Draft(ctx, all, s.awaiting); !errors.Is(err, domain.ErrNoDraft) {
		t.Errorf("scope.all reading an import without a draft: %v, want ErrNoDraft", err)
	}
	if stored, err := admin.repo.Draft(ctx, all, s.review); err != nil || stored.CreatedBy != a.actor.ID {
		t.Errorf("scope.all reading a draft: %+v (%v), want A as its creator", stored.CreatedBy, err)
	}
	if _, err := admin.repo.Commit(ctx, all, s.committed); err != nil {
		t.Errorf("scope.all reading a commit: %v", err)
	}

	current, err := admin.repo.Get(ctx, all, s.awaiting)
	if err != nil {
		t.Fatal(err)
	}
	finished, err := admin.repo.Finish(ctx, domain.Finish{ImportID: s.awaiting, SourceID: s.pending.ID, Actor: by})
	if err != nil {
		t.Fatalf("scope.all finishing A's upload: %v", err)
	}
	upload := a.upload(finished.Import, "answer_key")
	upload.Actor = by
	reserved, err := admin.repo.Reserve(ctx, upload, admin.quotas)
	if err != nil || reserved.UploadedBy != admin.actor.ID {
		t.Fatalf("scope.all uploading into A's import: %+v (%v)", reserved, err)
	}
	if finished.Import.CreatedBy != a.actor.ID || current.CreatedBy != a.actor.ID {
		t.Errorf("the import changed hands: %s", finished.Import.CreatedBy)
	}
	run, err := admin.repo.Schedule(ctx, domain.Schedule{ImportID: s.awaiting, RequestID: uuid.NewString(), PipelineVersion: uuid.NewString(),
		ExpectedRevision: finished.Import.Revision, SourceRevision: finished.Import.SourceRevision, Actor: by, MaxAttempts: 3})
	if err != nil || run.RequestedBy != admin.actor.ID {
		t.Fatalf("scope.all processing A's import: %+v (%v)", run, err)
	}
	if _, err := admin.repo.Cancel(ctx, domain.Cancel{ImportID: s.awaiting, ExpectedRevision: finished.Import.Revision + 1, Actor: by}); err != nil {
		t.Errorf("scope.all cancelling A's import: %v", err)
	}

	stored, err := admin.repo.Draft(ctx, all, s.review)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := admin.repo.SaveDraft(ctx, domain.SaveDraft{ImportID: s.review, ExpectedRevision: stored.Revision, Draft: emptyDraft("Sửa"), Actor: by})
	if err != nil {
		t.Fatalf("scope.all editing A's draft: %v", err)
	}
	a.reprocess(t, s.review, "Bản mới")
	if _, err := admin.repo.AdoptCandidate(ctx, domain.AdoptCandidate{ImportID: s.review, ExpectedRevision: saved.Revision, Actor: by}); err != nil {
		t.Fatalf("scope.all adopting A's reprocessed draft: %v", err)
	}
	adopted, err := admin.repo.Draft(ctx, all, s.review)
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.repo.RecordCommit(ctx, domain.CommitRecord{Commit: domain.Commit{ImportID: s.review, RequestID: uuid.NewString(), DraftRevision: adopted.Revision, Digest: make([]byte, 32)}, Actor: by}); err != nil {
		t.Fatalf("scope.all committing A's import: %v", err)
	}
	var committedBy, creator string
	if err := a.pool.QueryRow(ctx, `SELECT c.committed_by::text, i.created_by::text FROM app.word_import_commits c JOIN app.word_imports i ON i.id = c.import_id WHERE c.import_id = $1`, s.review).Scan(&committedBy, &creator); err != nil {
		t.Fatal(err)
	}
	if committedBy != admin.actor.ID || creator != a.actor.ID {
		t.Errorf("committed by %s for creator %s, want the Admin for A", committedBy, creator)
	}

	nobody := actor.Actor{}
	if _, err := admin.repo.Cancel(ctx, domain.Cancel{ImportID: s.cancelled, ExpectedRevision: 0, Actor: nobody}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a zero actor cancelling: %v", err)
	}
}

func second[T any](_ T, err error) error { return err }

func importIDs(items []domain.Import) []string {
	out := []string{}
	for _, v := range items {
		out = append(out, v.ID)
	}
	return out
}

func contains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, v := range b {
		if !contains(a, v) {
			return false
		}
	}
	return true
}

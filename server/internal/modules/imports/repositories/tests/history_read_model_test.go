//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/access"
)

func TestHistoryFacetsFollowSearchAndIgnoreRepeatedStatusSelection(t *testing.T) {
	h, foreign := setup(t), setup(t)
	ctx := context.Background()
	mark := uuid.NewString()
	statuses := []string{"awaiting_sources", "queued", "processing", "committing", "needs_review", "failed", "committed", "cancelled"}
	for _, status := range statuses {
		v := h.create(t)
		if _, err := h.pool.Exec(ctx, `UPDATE app.word_imports SET title=$2,status=$3 WHERE id=$1`, v.ID, "Đề "+mark, status); err != nil {
			t.Fatal(err)
		}
	}
	other := foreign.create(t)
	if _, err := h.pool.Exec(ctx, `UPDATE app.word_imports SET title=$2 WHERE id=$1`, other.ID, "Đề "+mark); err != nil {
		t.Fatal(err)
	}
	h.create(t)
	want := domain.StatusFacets{All: 8, Processing: 4, NeedsReview: 1, Failed: 1, Committed: 1, Cancelled: 1}
	for _, selected := range [][]string{nil, {"failed"}, {"failed", "needs_review", "failed"}} {
		got, err := h.repo.List(ctx, domain.Filter{Search: mark, Status: selected, Scope: access.Scope{UserID: h.actor.ID}, Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		if got.Facets != want {
			t.Fatalf("statuses %v facets %+v, want %+v", selected, got.Facets, want)
		}
		expected := len(statuses)
		if len(selected) == 1 {
			expected = 1
		}
		if len(selected) > 1 {
			expected = 2
		}
		if got.Page.Total != expected || len(got.Items) != expected {
			t.Fatalf("statuses %v total %d items %d", selected, got.Page.Total, len(got.Items))
		}
		for _, item := range got.Items {
			if item.CreatedBy != h.actor.ID {
				t.Fatalf("foreign history item %+v", item)
			}
		}
	}
	selected := []string{"failed", "needs_review"}
	first, err := h.repo.List(ctx, domain.Filter{Search: mark, Status: selected, Scope: access.Scope{UserID: h.actor.ID}, Limit: 1, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.repo.List(ctx, domain.Filter{Search: mark, Status: selected, Scope: access.Scope{UserID: h.actor.ID}, Limit: 1, Page: 2})
	if err != nil {
		t.Fatal(err)
	}
	if first.Page.Total != 2 || second.Page.Total != 2 || len(first.Items) != 1 || len(second.Items) != 1 || first.Items[0].ID == second.Items[0].ID || first.Facets != second.Facets {
		t.Fatalf("pages %+v %+v", first, second)
	}
	empty, err := h.repo.List(ctx, domain.Filter{Search: mark, Scope: access.Scope{}, Limit: 100})
	if err != nil || empty.Facets != (domain.StatusFacets{}) || empty.Page.Total != 0 || len(empty.Items) != 0 {
		t.Fatalf("zero scope %+v %v", empty, err)
	}
	named := h.create(t)
	upload := h.upload(named, "exam")
	upload.Source.Filename = "Nghe-" + mark + "-100%_.docx"
	h.finish(t, h.reserve(t, upload))
	filename, err := h.repo.List(ctx, domain.Filter{Search: "nghe-" + mark + "-100%_", Status: []string{"failed"}, Scope: access.Scope{UserID: h.actor.ID}})
	if err != nil || filename.Facets.All != 1 || filename.Facets.Processing != 1 || filename.Page.Total != 0 {
		t.Fatalf("literal filename search %+v %v", filename, err)
	}
}

func historyCounts(t *testing.T, h harness, id string) *domain.ReviewCounts {
	t.Helper()
	got, err := h.repo.List(context.Background(), domain.Filter{Scope: access.Scope{UserID: h.actor.ID}, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range got.Items {
		if item.ID == id {
			return item.ReviewCounts
		}
	}
	t.Fatalf("history missing %s", id)
	return nil
}

func requireCurrentCounts(t *testing.T, h harness, id string, draft domain.Draft) {
	t.Helper()
	summary := domain.Assess(draft).Summary
	want := &domain.ReviewCounts{NeedsAction: summary.Blocking, ToConfirm: summary.NeedsDecision}
	if got := historyCounts(t, h, id); !reflect.DeepEqual(got, want) {
		t.Fatalf("current counts %+v want %+v", got, want)
	}
}

func TestHistoryCountsTrackMachineWritesSavesAndAdoptionButNotPendingCandidates(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	run := h.processed(t, "first")
	stored, err := h.repo.Draft(ctx, everyone, run.ImportID)
	if err != nil {
		t.Fatal(err)
	}
	requireCurrentCounts(t, h, run.ImportID, stored.Draft)
	if historyCounts(t, h, h.create(t).ID) != nil {
		t.Fatal("an import without a draft invented counts")
	}
	edited := stored.Draft
	edited.Notices = []domain.Finding{{ID: "one", Code: domain.CodeSourceObject, Severity: domain.ReviewRequired}, {ID: "two", Code: domain.CodeSourceObject, Severity: domain.ReviewRequired}}
	saved, err := h.repo.SaveDraft(ctx, domain.SaveDraft{ImportID: run.ImportID, ExpectedRevision: stored.Revision, Draft: edited, Actor: h.actor})
	if err != nil {
		t.Fatal(err)
	}
	requireCurrentCounts(t, h, run.ImportID, edited)
	edited.Acknowledged = []string{"one"}
	saved, err = h.repo.SaveDraft(ctx, domain.SaveDraft{ImportID: run.ImportID, ExpectedRevision: saved.Revision, Draft: edited, Actor: h.actor})
	if err != nil {
		t.Fatal(err)
	}
	requireCurrentCounts(t, h, run.ImportID, edited)
	h.reprocess(t, run.ImportID, "candidate")
	requireCurrentCounts(t, h, run.ImportID, edited)
	if _, err := h.repo.SaveDraft(ctx, domain.SaveDraft{ImportID: run.ImportID, ExpectedRevision: saved.Revision - 1, Draft: emptyDraft("stale"), Actor: h.actor}); err == nil {
		t.Fatal("stale save accepted")
	}
	requireCurrentCounts(t, h, run.ImportID, edited)
	if _, err := h.pool.Exec(ctx, `UPDATE app.word_import_drafts SET open_action_count=NULL,open_confirm_count=NULL WHERE import_id=$1`, run.ImportID); err != nil {
		t.Fatal(err)
	}
	if historyCounts(t, h, run.ImportID) != nil {
		t.Fatal("untouched legacy draft invented counts")
	}
	h.reprocess(t, run.ImportID, "newer candidate")
	if historyCounts(t, h, run.ImportID) != nil {
		t.Fatal("candidate backfilled the legacy current body")
	}
	adopted, err := h.repo.AdoptCandidate(ctx, domain.AdoptCandidate{ImportID: run.ImportID, ExpectedRevision: saved.Revision, Actor: h.actor})
	if err != nil {
		t.Fatal(err)
	}
	requireCurrentCounts(t, h, run.ImportID, adopted.Draft)
	h.reprocess(t, run.ImportID, "untouched replacement")
	replaced, err := h.repo.Draft(ctx, everyone, run.ImportID)
	if err != nil {
		t.Fatal(err)
	}
	requireCurrentCounts(t, h, run.ImportID, replaced.Draft)
}

func TestCurrentDraftCountsAboveSmallintRangeRemainExact(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	run := h.processed(t, "large")
	stored, err := h.repo.Draft(ctx, everyone, run.ImportID)
	if err != nil {
		t.Fatal(err)
	}
	draft := stored.Draft
	for i := 0; i < 32768; i++ {
		draft.Notices = append(draft.Notices, domain.Finding{ID: strconv.Itoa(i), Code: domain.CodeSourceObject, Severity: domain.ReviewRequired})
	}
	raw, err := json.Marshal(draft)
	if err != nil || len(raw) >= 8<<20 {
		t.Fatalf("bounded draft bytes %d err %v", len(raw), err)
	}
	if err := domain.ValidateEdit(draft); err != nil {
		t.Fatal(err)
	}
	saved, err := h.repo.SaveDraft(ctx, domain.SaveDraft{ImportID: run.ImportID, ExpectedRevision: stored.Revision, Draft: draft, Actor: h.actor})
	if err != nil {
		t.Fatal(err)
	}
	requireCurrentCounts(t, h, run.ImportID, saved.Draft)
	if got := historyCounts(t, h, run.ImportID); got == nil || got.ToConfirm != 32768 {
		t.Fatalf("large counts %+v", got)
	}
}

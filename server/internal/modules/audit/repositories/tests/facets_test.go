//go:build integration

package repositories_test

import (
	"testing"

	"quizzivy/internal/modules/audit/application/query"
	"quizzivy/internal/modules/audit/domain"
)

func (w *world) manyBy(actor, entity string, n int, day int) {
	w.t.Helper()
	for i := range n {
		w.write(entry{actor: actor, entity: entity, action: "x.y", at: march(day, 9, i)})
	}
}

func TestFacetsCountEachCategoryAndTheSameWhicheverCategoryIsSelected(t *testing.T) {
	w := newWorld(t)
	actor := w.user("teacher", "Người làm "+w.nonce, "")
	w.manyBy(actor, "user", 2, 10)
	w.manyBy(actor, "student", 1, 10)
	w.manyBy(actor, "class", 1, 11)
	w.manyBy(actor, "class_member", 1, 11)
	w.manyBy(actor, "attempt", 3, 12)
	w.manyBy(actor, "test", 1, 13)
	w.manyBy(actor, "question", 2, 13)
	w.manyBy(actor, "media_asset", 2, 14)
	w.manyBy(actor, "word_import", 1, 14)
	w.manyBy(actor, "maintenance_window", 1, 15)
	w.manyBy(actor, "ai_settings", 2, 15)
	want := domain.Facets{All: 17, Accounts: 3, Classes: 2, Attempts: 3, Tests: 3, Media: 2, Imports: 1}

	for _, category := range append([]domain.Category{""}, domain.Categories()...) {
		q := inMarch()
		q.Query, q.Category = w.nonce, category
		got := w.list(q)

		if got.Facets != want {
			t.Errorf("category %q: facets %+v, want %+v", category, got.Facets, want)
		}
		if got.Page.Total != want.Of(category) || len(got.Entries) != want.Of(category) {
			t.Errorf("category %q: total %d with %d entries, want %d of each", category, got.Page.Total, len(got.Entries), want.Of(category))
		}
	}
	if sum := want.Accounts + want.Classes + want.Attempts + want.Tests + want.Media + want.Imports; sum != want.All-3 {
		t.Fatalf("the fixture is wrong: the six add up to %d", sum)
	}
}

func TestAnAllOnlyEntryIsCountedInAllAndInNoFamily(t *testing.T) {
	w := newWorld(t)
	actor := w.user("teacher", "Người làm "+w.nonce, "")
	w.manyBy(actor, "maintenance_window", 2, 12)
	w.manyBy(actor, "attempt_events", 1, 12)
	w.manyBy(actor, "audit_log", 1, 12)

	q := inMarch()
	q.Query = w.nonce
	got := w.list(q)

	if got.Facets != (domain.Facets{All: 4}) {
		t.Errorf("facets %+v, want all 4 and every family 0", got.Facets)
	}
	if len(got.Entries) != 4 {
		t.Errorf("the unfiltered list holds %d, want the four", len(got.Entries))
	}
	for _, e := range got.Entries {
		if e.Category != nil {
			t.Errorf("entry about %s has category %s, want none", e.EntityType, *e.Category)
		}
	}
	for _, category := range domain.Categories() {
		q.Category = category
		if filtered := w.list(q); len(filtered.Entries) != 0 || filtered.Page.Total != 0 {
			t.Errorf("category %s holds %d entries", category, len(filtered.Entries))
		}
	}
}

func TestFacetsFollowTheRangeAndTheSearch(t *testing.T) {
	w := newWorld(t)
	mine := w.user("teacher", "Người làm "+w.nonce, "")
	other := w.user("teacher", "Người khác "+nonce(t), "")
	w.manyBy(mine, "class", 2, 12)
	w.manyBy(mine, "test", 1, 12)
	w.manyBy(other, "class", 5, 12)
	for i := range 3 {
		w.write(entry{actor: mine, entity: "class", action: "x.y", at: feb(20, 9, i)})
	}

	inRange := w.list(query.List{Range: domain.Custom, From: "2001-03-01", To: "2001-03-31", Query: w.nonce})
	if want := (domain.Facets{All: 3, Classes: 2, Tests: 1}); inRange.Facets != want {
		t.Errorf("under the search and the month: %+v, want %+v", inRange.Facets, want)
	}
	wide := w.list(query.List{Range: domain.Custom, From: "2001-02-01", To: "2001-03-31", Query: w.nonce})
	if want := (domain.Facets{All: 6, Classes: 5, Tests: 1}); wide.Facets != want {
		t.Errorf("under the search and two months: %+v, want %+v", wide.Facets, want)
	}
	nobody := w.list(query.List{Range: domain.Custom, From: "2001-03-01", To: "2001-03-31", Query: "không ai tên " + w.nonce})
	if nobody.Facets != (domain.Facets{}) || nobody.Page.Total != 0 || len(nobody.Entries) != 0 {
		t.Errorf("a search nobody matches: %+v", nobody)
	}
}

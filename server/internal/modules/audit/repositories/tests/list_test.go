//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"quizzivy/internal/modules/audit/application/query"
	"quizzivy/internal/modules/audit/domain"
)

func (w *world) list(q query.List) query.ListResult {
	w.t.Helper()
	result, err := w.app.Queries.List.Handle(context.Background(), q)
	if err != nil {
		w.t.Fatalf("list %+v: %v", q, err)
	}
	return result
}

func ids(entries []domain.Entry) []int64 {
	out := make([]int64, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}

func inMarch() query.List {
	return query.List{Range: domain.Custom, From: "2001-03-01", To: "2001-03-31", Limit: 100}
}

func TestTheListIsNewestFirstAndAnIdBreaksATie(t *testing.T) {
	w := newWorld(t)
	oldest := w.write(entry{at: march(10, 9, 0)})
	middle := w.write(entry{at: march(11, 9, 0)})
	first := w.write(entry{at: march(12, 9, 0)})
	second := w.write(entry{at: march(12, 9, 0)})

	got := w.list(inMarch())

	if want := []int64{second, first, middle, oldest}; !slices.Equal(ids(got.Entries), want) {
		t.Errorf("order %v, want %v: newest first, and of two at one instant the later id first", ids(got.Entries), want)
	}
}

func TestAnEntryCarriesWhatWasStored(t *testing.T) {
	w := newWorld(t)
	teacher := w.user("teacher", "Giáo viên "+w.nonce, "")
	class := w.class("Lớp 9A", teacher)
	w.write(entry{
		actor: teacher, action: "class.created", entity: "class", entityID: class, at: march(12, 9, 30),
		ip: "203.0.113.7", userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		diff: `{"name": {"old": null, "new": "Lớp 9A"}, "reason": "import"}`,
	})

	q := inMarch()
	q.Query = w.nonce
	got := w.list(q)

	if len(got.Entries) != 1 {
		t.Fatalf("%d entries, want 1", len(got.Entries))
	}
	e := got.Entries[0]
	if e.Actor == nil || e.Actor.ID != teacher || e.Actor.Name != "Giáo viên "+w.nonce {
		t.Errorf("actor %+v", e.Actor)
	}
	if e.EntityType != "class" || e.EntityID == nil || *e.EntityID != class {
		t.Errorf("entity %s %v", e.EntityType, e.EntityID)
	}
	if e.Label == nil || *e.Label != "Lớp 9A" {
		t.Errorf("label %v, want the class's name", e.Label)
	}
	if e.Category == nil || *e.Category != domain.Classes {
		t.Errorf("category %v", e.Category)
	}
	if e.IP == nil || *e.IP != "203.0.113.7" {
		t.Errorf("ip %v, want the bare address", e.IP)
	}
	if e.UserAgent == nil {
		t.Error("the user agent was not read")
	}
	if len(e.Changes) != 1 || e.Changes[0].Field != "name" || e.Changes[0].To != "Lớp 9A" || e.Details["reason"] != "import" {
		t.Errorf("changes %+v, details %+v", e.Changes, e.Details)
	}
	if !e.OccurredAt.Equal(march(12, 9, 30)) {
		t.Errorf("occurred at %v", e.OccurredAt)
	}
}

func TestADisplayNameIsTheActorsNameWhenThereIsOne(t *testing.T) {
	w := newWorld(t)
	actor := w.user("teacher", "Tên đầy đủ "+w.nonce, "Cô Lan")
	w.write(entry{actor: actor, at: march(12, 9, 0)})

	q := inMarch()
	q.Query = w.nonce
	got := w.list(q)

	if len(got.Entries) != 1 || got.Entries[0].Actor.Name != "Cô Lan" {
		t.Errorf("entries %+v, want the display name", got.Entries)
	}
}

func TestTheSystemHasNoActor(t *testing.T) {
	w := newWorld(t)
	w.write(entry{action: "maintenance.scheduled", entity: "maintenance_window", at: march(12, 9, 0)})
	w.write(entry{action: "student.anonymized", entity: "users", at: march(12, 10, 0)})

	got := w.list(inMarch())

	if len(got.Entries) != 2 {
		t.Fatalf("%d entries", len(got.Entries))
	}
	for _, e := range got.Entries {
		if e.Actor != nil {
			t.Errorf("%s has actor %+v, want the system", e.Action, e.Actor)
		}
	}
}

func TestAnActorSinceRemovedIsTheSystemToo(t *testing.T) {
	w := newWorld(t)
	actor := w.user("teacher", "Người đã xoá "+w.nonce, "")
	w.write(entry{actor: actor, at: march(12, 9, 0)})
	w.exec(`DELETE FROM app.users WHERE id = $1::uuid`, actor)

	got := w.list(inMarch())

	if len(got.Entries) != 1 || got.Entries[0].Actor != nil {
		t.Errorf("entries %+v, want one with no actor", got.Entries)
	}
}

func TestPagingNamesThePageTheSizeAndTheTotal(t *testing.T) {
	w := newWorld(t)
	var all []int64
	for i := range 7 {
		all = append(all, w.write(entry{at: march(10+i, 9, 0)}))
	}
	slices.Reverse(all)

	q := inMarch()
	q.Limit = 3
	second := func(page int) query.ListResult {
		q.Page = page
		return w.list(q)
	}

	if got := second(1); !slices.Equal(ids(got.Entries), all[0:3]) || got.Page.Number != 1 || got.Page.Size != 3 || got.Page.Total != 7 {
		t.Errorf("page 1: %v %+v", ids(got.Entries), got.Page)
	}
	if got := second(2); !slices.Equal(ids(got.Entries), all[3:6]) || got.Page.Number != 2 {
		t.Errorf("page 2: %v %+v", ids(got.Entries), got.Page)
	}
	if got := second(3); !slices.Equal(ids(got.Entries), all[6:7]) {
		t.Errorf("page 3: %v", ids(got.Entries))
	}
	past := second(9)
	if past.Entries == nil || len(past.Entries) != 0 || past.Page.Total != 7 || past.Page.Number != 9 {
		t.Errorf("a page past the end: %+v, want an empty list with the same total", past)
	}
}

func TestTheDefaultPageIsTenAndTheLimitIsClamped(t *testing.T) {
	w := newWorld(t)
	for i := range 12 {
		w.write(entry{at: march(1+i, 9, 0)})
	}
	q := inMarch()
	q.Limit = 0
	if got := w.list(q); len(got.Entries) != 10 || got.Page.Size != 10 || got.Page.Total != 12 {
		t.Errorf("default: %d entries, %+v", len(got.Entries), got.Page)
	}
	q.Limit = 500
	if got := w.list(q); got.Page.Size != 100 || len(got.Entries) != 12 {
		t.Errorf("clamped: %d entries, %+v", len(got.Entries), got.Page)
	}
	q.Limit, q.Page = 5, -3
	if got := w.list(q); got.Page.Number != 1 {
		t.Errorf("negative page: %+v", got.Page)
	}
}

func TestACategoryKeepsOnlyItsEntities(t *testing.T) {
	w := newWorld(t)
	class := w.write(entry{entity: "class", at: march(10, 9, 0)})
	member := w.write(entry{entity: "class_member", at: march(11, 9, 0)})
	w.write(entry{entity: "test", action: "test.created", at: march(12, 9, 0)})
	w.write(entry{entity: "maintenance_window", action: "maintenance.ended", at: march(13, 9, 0)})

	q := inMarch()
	q.Category = domain.Classes
	got := w.list(q)

	if want := []int64{member, class}; !slices.Equal(ids(got.Entries), want) {
		t.Errorf("classes: %v, want %v", ids(got.Entries), want)
	}
	if got.Page.Total != 2 {
		t.Errorf("total %d, want the classes facet, 2", got.Page.Total)
	}
}

func TestAnUnknownCategoryAndAnUnknownRangeAreRefused(t *testing.T) {
	w := newWorld(t)
	if _, err := w.app.Queries.List.Handle(context.Background(), query.List{Category: "organization"}); !errors.Is(err, domain.ErrUnknownCategory) {
		t.Errorf("category: %v", err)
	}
	if _, err := w.app.Queries.List.Handle(context.Background(), query.List{Range: "decade"}); !errors.Is(err, domain.ErrUnknownRange) {
		t.Errorf("range: %v", err)
	}
}

func TestEachRangeReachesBackToTheViewersMidnight(t *testing.T) {
	w := newWorld(t)
	actor := w.user("teacher", "Người xem "+w.nonce, "")
	rows := map[string]int64{
		"today midnight":   w.write(entry{actor: actor, at: march(15, 0, 0)}),
		"just before":      w.write(entry{actor: actor, at: march(14, 23, 59)}),
		"week start":       w.write(entry{actor: actor, at: march(9, 0, 0)}),
		"before the week":  w.write(entry{actor: actor, at: march(8, 23, 59)}),
		"month start":      w.write(entry{actor: actor, at: feb(14, 0, 0)}),
		"before the month": w.write(entry{actor: actor, at: feb(13, 23, 0)}),
	}
	cases := []struct {
		kind domain.Range
		want []string
	}{
		{domain.Today, []string{"today midnight"}},
		{domain.Week, []string{"today midnight", "just before", "week start"}},
		{domain.Month, []string{"today midnight", "just before", "week start", "before the week", "month start"}},
		{domain.Forever, []string{"today midnight", "just before", "week start", "before the week", "month start", "before the month"}},
		{"", []string{"today midnight", "just before", "week start"}},
	}
	for _, c := range cases {
		got := w.list(query.List{Range: c.kind, Query: w.nonce, Limit: 100})
		want := make([]int64, len(c.want))
		for i, name := range c.want {
			want[i] = rows[name]
		}
		gotIDs := ids(got.Entries)
		if !slices.Equal(gotIDs, want) {
			t.Errorf("%q reads %v, want %v (%v)", c.kind, gotIDs, want, c.want)
		}
		if got.Facets.All != len(c.want) {
			t.Errorf("%q counts %d, want %d", c.kind, got.Facets.All, len(c.want))
		}
	}
}

func TestACustomRangeIsBothDaysWholeInTheViewersZone(t *testing.T) {
	w := newWorld(t)
	actor := w.user("teacher", "Người xem "+w.nonce, "")
	before := w.write(entry{actor: actor, at: march(8, 23, 59)})
	first := w.write(entry{actor: actor, at: march(9, 0, 0)})
	last := w.write(entry{actor: actor, at: march(14, 23, 59)})
	after := w.write(entry{actor: actor, at: march(15, 0, 0)})

	got := w.list(query.List{Range: domain.Custom, From: "2001-03-09", To: "2001-03-14", Query: w.nonce})

	if want := []int64{last, first}; !slices.Equal(ids(got.Entries), want) {
		t.Errorf("reads %v, want %v, leaving out %d and %d", ids(got.Entries), want, before, after)
	}
	one := w.list(query.List{Range: domain.Custom, From: "2001-03-15", To: "2001-03-15", Query: w.nonce})
	if !slices.Equal(ids(one.Entries), []int64{after}) {
		t.Errorf("one day reads %v, want %v", ids(one.Entries), after)
	}
}

func TestTheRefusalsOfARangeReachTheCaller(t *testing.T) {
	w := newWorld(t)
	for _, q := range []query.List{
		{Range: domain.Week, From: "2001-03-01"},
		{Range: domain.Custom, From: "2001-03-01"},
		{Range: domain.Custom, From: "2001-03-02", To: "2001-03-01"},
	} {
		var refused *domain.RangeError
		if _, err := w.app.Queries.List.Handle(context.Background(), q); !errors.As(err, &refused) {
			t.Errorf("%+v: %v, want a range error", q, err)
		}
	}
}

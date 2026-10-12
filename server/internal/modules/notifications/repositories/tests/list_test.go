//go:build integration

package repositories_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestTheListPagesThroughEveryRowOnceNewestFirst(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	userID := newUser(t, pool, "teacher")
	fill(t, app, userID, 45)
	all := idsOf(rowsOf(t, pool, userID))
	if len(all) != 45 {
		t.Fatalf("the fixture holds %d rows, want 45", len(all))
	}
	slices.Reverse(all)

	first := page(t, app, userID, "", 20)
	second := page(t, app, userID, first.NextBefore, 20)
	third := page(t, app, userID, second.NextBefore, 20)
	if len(first.Items) != 20 || len(second.Items) != 20 || len(third.Items) != 5 {
		t.Fatalf("the pages hold %d, %d and %d rows, want 20, 20 and 5", len(first.Items), len(second.Items), len(third.Items))
	}
	if first.NextBefore != first.Items[19].ID || second.NextBefore != second.Items[19].ID {
		t.Errorf("the cursors are %q and %q, want the last id of each page", first.NextBefore, second.NextBefore)
	}
	if third.NextBefore != "" {
		t.Errorf("the last page names a next one: %q", third.NextBefore)
	}
	seen := slices.Concat(listed(first), listed(second), listed(third))
	if !slices.Equal(seen, all) {
		t.Errorf("the three pages hold\n  %v\nwant every row once, newest first\n  %v", seen, all)
	}
	if byDefault := page(t, app, userID, "", 0); len(byDefault.Items) != 20 {
		t.Errorf("a page without a size holds %d rows, want 20", len(byDefault.Items))
	}
	if largest := page(t, app, userID, "", 500); len(largest.Items) != 45 || largest.NextBefore != "" {
		t.Errorf("an oversized page holds %d rows (next %q), want the 45 within the cap of 50", len(largest.Items), largest.NextBefore)
	}
}

func TestAPageThatEndsTheListNamesNoNextPage(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	userID := newUser(t, pool, "student")
	fill(t, app, userID, 40)
	first := page(t, app, userID, "", 20)
	second := page(t, app, userID, first.NextBefore, 20)
	if len(second.Items) != 20 || second.NextBefore != "" {
		t.Fatalf("the second of two full pages holds %d rows and names %q, want 20 and no next page", len(second.Items), second.NextBefore)
	}
	if first.NextBefore == "" {
		t.Error("the first of two full pages names no next page")
	}
	oldest := second.Items[19].ID
	if beyond := page(t, app, userID, oldest, 20); len(beyond.Items) != 0 || beyond.NextBefore != "" {
		t.Errorf("a page before the oldest row holds %d rows (next %q), want none", len(beyond.Items), beyond.NextBefore)
	}
	if empty := page(t, app, newUser(t, pool, "student"), "", 20); len(empty.Items) != 0 || empty.NextBefore != "" {
		t.Errorf("a user with no notification is shown %d", len(empty.Items))
	}
}

func TestACursorIsAPositionInTheCallersOwnListAndNothingElse(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	a, b := newUser(t, pool, "teacher"), newUser(t, pool, "teacher")
	for range 3 {
		fill(t, app, a, 1)
		fill(t, app, b, 1)
	}
	mine, theirs := idsOf(rowsOf(t, pool, a)), idsOf(rowsOf(t, pool, b))
	if len(mine) != 3 || len(theirs) != 3 {
		t.Fatalf("the fixture holds %d and %d rows, want three each", len(mine), len(theirs))
	}

	for _, cursor := range append(slices.Clone(theirs), uuid.NewString(), "ffffffff-ffff-7fff-bfff-ffffffffffff", "00000000-0000-7000-8000-000000000000") {
		var want []string
		for _, id := range mine {
			if id < cursor {
				want = append(want, id)
			}
		}
		slices.Reverse(want)
		got := listed(page(t, app, a, cursor, 20))
		if !slices.Equal(got, want) {
			t.Errorf("before %s the caller is shown %v, want only their own older rows %v", cursor, got, want)
		}
		for _, id := range got {
			if slices.Contains(theirs, id) {
				t.Errorf("before %s the caller is shown another user's row %s", cursor, id)
			}
		}
	}
	if got := listed(page(t, app, a, "", 20)); len(got) != 3 || slices.ContainsFunc(got, func(id string) bool { return slices.Contains(theirs, id) }) {
		t.Errorf("the caller's list is %v, want their own three rows", got)
	}
}

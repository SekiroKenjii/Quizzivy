//go:build integration

package application_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
)

type seededLink struct {
	created time.Time
	expires time.Time
	used    *time.Time
	revoked *time.Time
}

func at(t time.Time) *time.Time { return &t }

func (w *dbLinks) seed(t *testing.T, userID string, l seededLink) string {
	t.Helper()
	var id string
	if err := w.admin.QueryRow(context.Background(), `
		INSERT INTO app.set_password_tokens (user_id, token_hash, purpose, created_at, expires_at, used_at, revoked_at)
		VALUES ($1::uuid, sha256(gen_random_uuid()::text::bytea), 'reset', $2, $3, $4, $5)
		RETURNING id::text`, userID, l.created, l.expires, l.used, l.revoked).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *dbLinks) linkIDs(t *testing.T, userID string) []string {
	t.Helper()
	found, err := w.admin.Query(context.Background(), `SELECT id::text FROM app.set_password_tokens WHERE user_id = $1::uuid`, userID)
	if err != nil {
		t.Fatal(err)
	}
	defer found.Close()
	var ids []string
	for found.Next() {
		var id string
		if err := found.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := found.Err(); err != nil {
		t.Fatal(err)
	}
	slices.Sort(ids)
	return ids
}

func (w *dbLinks) prune(t *testing.T, limit int) int {
	t.Helper()
	n, err := w.app.Commands.PruneSetPasswordLinks.Handle(context.Background(), command.PruneSetPasswordLinks{Limit: limit})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

const day = 24 * time.Hour

func TestALinkIsPrunedThirtyDaysAfterItWasUsedRevokedOrExpired(t *testing.T) {
	w := newDBLinks(t)
	student := w.account(t, "hs", true)
	now := time.Now().Truncate(time.Microsecond)
	issuedAt := func(age time.Duration) seededLink {
		created := now.Add(-age)
		return seededLink{created: created, expires: created.Add(domain.SetPasswordLifetime)}
	}
	usedAt := func(age, after time.Duration) seededLink {
		l := issuedAt(age)
		l.used = at(l.created.Add(after))
		return l
	}
	revokedAt := func(age, after time.Duration) seededLink {
		l := issuedAt(age)
		l.revoked = at(l.created.Add(after))
		return l
	}
	lateRevoke := issuedAt(31*day + time.Hour)
	lateRevoke.revoked = at(now.Add(-day))

	cases := []struct {
		name string
		link seededLink
		gone bool
	}{
		{"used, and expired, long ago", usedAt(40*day, time.Hour), true},
		{"used just over 30 days ago, expiring just inside", usedAt(30*day+2*time.Hour, time.Hour), true},
		{"revoked just over 30 days ago, expiring just inside", revokedAt(30*day+2*time.Hour, time.Hour), true},
		{"expired over 30 days ago and never touched", issuedAt(31*day + time.Hour), true},
		{"expired over 30 days ago, revoked yesterday", lateRevoke, true},
		{"used just under 30 days ago", usedAt(29*day+12*time.Hour, time.Hour), false},
		{"revoked just under 30 days ago", revokedAt(29*day+12*time.Hour, time.Hour), false},
		{"expired just under 30 days ago", issuedAt(29*day + 12*time.Hour), false},
		{"expired yesterday and unused", issuedAt(2 * day), false},
		{"issued a minute ago", issuedAt(time.Minute), false},
	}
	ids := make([]string, len(cases))
	gone := 0
	for i, c := range cases {
		ids[i] = w.seed(t, student, c.link)
		if c.gone {
			gone++
		}
	}
	if _, err := w.issue(t, student, domain.LinkInvite, "", false); err != nil {
		t.Fatal(err)
	}
	live := w.lastMail(t, student).token
	before := len(w.linkIDs(t, student))

	removed := w.prune(t, 10000)

	kept := w.linkIDs(t, student)
	for i, c := range cases {
		if stays := slices.Contains(kept, ids[i]); stays == c.gone {
			t.Errorf("%s: kept = %v, want %v", c.name, stays, !c.gone)
		}
	}
	if got := before - len(kept); got != gone || removed < gone {
		t.Errorf("%d of the account's links went and the prune reported %d, want %d of them", got, removed, gone)
	}
	if !w.usable(live) {
		t.Error("a link that can still be used was pruned")
	}

	w.prune(t, 10000)
	if again := w.linkIDs(t, student); !slices.Equal(again, kept) {
		t.Errorf("a second run changed the links: %v, want %v", again, kept)
	}
}

func TestLinksArePrunedTheEarliestFirstAndAtMostLimitAtATime(t *testing.T) {
	w := newDBLinks(t)
	student := w.account(t, "hs", true)
	now := time.Now().Truncate(time.Microsecond)

	var ids []string
	for i := range 5 {
		created := now.Add(-3650*day - time.Duration(5-i)*day)
		ids = append(ids, w.seed(t, student, seededLink{created: created, expires: created.Add(domain.SetPasswordLifetime)}))
	}

	if n := w.prune(t, 2); n != 2 {
		t.Fatalf("pruned %d, want 2", n)
	}
	if got, want := w.linkIDs(t, student), sorted(ids[2:]); !slices.Equal(got, want) {
		t.Errorf("after one batch the links are %v, want the three newest %v", got, want)
	}
	if n := w.prune(t, 2); n != 2 {
		t.Fatalf("pruned %d, want 2", n)
	}
	if n := w.prune(t, 2); n != 1 {
		t.Fatalf("pruned %d, want the last 1", n)
	}
	if got := w.linkIDs(t, student); len(got) != 0 {
		t.Errorf("links left: %v", got)
	}
}

func sorted(ids []string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

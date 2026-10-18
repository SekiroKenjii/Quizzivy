//go:build integration

package application_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/repositories"
)

type sweepWorld struct {
	*usersWorld
	sweeps *repositories.AvatarSweeps
	bucket *photoBucket
	users  []string
}

func newSweepWorld(t *testing.T) *sweepWorld {
	t.Helper()
	w := &sweepWorld{usersWorld: newUsersWorld(t, false), bucket: &photoBucket{}}
	w.sweeps = repositories.NewAvatarSweeps(w.dbx)
	w.app.SetAvatarSweeps(w.sweeps, w.bucket)
	t.Cleanup(func() {
		if _, err := w.admin.Exec(context.Background(), `DELETE FROM app.users WHERE id::text = ANY($1::text[])`, w.users); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return w
}

type accountShape struct {
	anonymous bool
	disabled  bool
	photo     bool
}

func (w *sweepWorld) account(t *testing.T, s accountShape) string {
	t.Helper()
	id := w.insertUser(t, "photo", "student")
	w.users = append(w.users, id)
	var key *string
	if s.photo {
		k := "avatars/" + id + "/current.png"
		key = &k
	}
	if _, err := w.admin.Exec(context.Background(), `
		UPDATE app.users
		   SET email = CASE WHEN $2 THEN 'anonymous-' || id::text || '@anonymous.invalid' ELSE email END,
		       disabled_at = CASE WHEN $3 THEN now() END,
		       avatar_key = $4
		 WHERE id = $1::uuid`, id, s.anonymous, s.disabled, key); err != nil {
		t.Fatal(err)
	}
	w.bucket.keys = append(w.bucket.keys, "avatars/"+id+"/old.png", "avatars/"+id+"/current.png")
	return id
}

func (w *sweepWorld) sweep(t *testing.T) int {
	t.Helper()
	n, err := w.app.Commands.SweepAvatarObjects.Handle(context.Background(), command.SweepAvatarObjects{Limit: 10000})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func (w *sweepWorld) photos(id string) int {
	n := 0
	for _, k := range w.bucket.keys {
		if strings.HasPrefix(k, "avatars/"+id+"/") {
			n++
		}
	}
	return n
}

func (w *sweepWorld) mark(t *testing.T, id string) *time.Time {
	t.Helper()
	var at *time.Time
	if err := w.admin.QueryRow(context.Background(), `SELECT (SELECT swept_at FROM app.avatar_sweeps WHERE user_id = $1::uuid)`, id).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func TestOnlyAnAnonymisedAccountsPhotosAreSweptAndItIsMarkedOnce(t *testing.T) {
	w := newSweepWorld(t)
	anonymised := w.account(t, accountShape{anonymous: true, disabled: true})
	live := w.account(t, accountShape{photo: true})
	disabled := w.account(t, accountShape{disabled: true, photo: true})
	disabledWithoutPhoto := w.account(t, accountShape{disabled: true})
	enabledWithReservedAddress := w.account(t, accountShape{anonymous: true})
	keyStillSet := w.account(t, accountShape{anonymous: true, disabled: true, photo: true})

	if got := w.sweep(t); got < 1 {
		t.Fatalf("marked %d accounts, want at least the anonymised one", got)
	}

	if n := w.photos(anonymised); n != 0 {
		t.Errorf("%d photo object(s) of the anonymised account are left", n)
	}
	first := w.mark(t, anonymised)
	if first == nil {
		t.Fatal("the anonymised account is not marked")
	}
	for name, id := range map[string]string{
		"a live account": live, "a disabled account that is not anonymised": disabled, "a disabled account with no photo key": disabledWithoutPhoto,
		"an enabled account with the reserved address": enabledWithReservedAddress, "an anonymous account whose photo key is set": keyStillSet,
	} {
		if n := w.photos(id); n != 2 {
			t.Errorf("%s has %d photo object(s) left, want both", name, n)
		}
		if w.mark(t, id) != nil {
			t.Errorf("%s is marked", name)
		}
	}

	w.bucket.keys = append(w.bucket.keys, "avatars/"+anonymised+"/late.png")
	w.sweep(t)
	if n := w.photos(anonymised); n != 1 {
		t.Errorf("a marked account was swept again: %d object(s) left, want the one written after its mark", n)
	}
	if again := w.mark(t, anonymised); again == nil || !again.Equal(*first) {
		t.Errorf("the mark moved from %v to %v", first, again)
	}
}

func TestAMarkedAccountIsNoLongerDueAndASecondMarkKeepsTheFirst(t *testing.T) {
	w := newSweepWorld(t)
	id := w.account(t, accountShape{anonymous: true, disabled: true})
	ctx := context.Background()

	due, err := w.sweeps.Due(ctx, 10000)
	if err != nil || !slices.Contains(due, id) {
		t.Fatalf("due = %v, %v; want it to hold the anonymised account", len(due), err)
	}
	if !slices.IsSorted(due) {
		t.Error("the due accounts are not in id order")
	}
	first := time.Date(2026, 10, 18, 3, 0, 0, 0, time.UTC)
	if err := w.sweeps.Mark(ctx, id, first); err != nil {
		t.Fatal(err)
	}
	if err := w.sweeps.Mark(ctx, id, first.Add(time.Hour)); err != nil {
		t.Fatalf("a second mark failed: %v", err)
	}
	if at := w.mark(t, id); at == nil || !at.Equal(first) {
		t.Errorf("marked at %v, want the first mark %v", at, first)
	}
	due, err = w.sweeps.Due(ctx, 10000)
	if err != nil || slices.Contains(due, id) {
		t.Errorf("a marked account is still due (%v)", err)
	}
}

func TestTheDueAccountsAreTakenInIdOrderAtMostLimitAtATime(t *testing.T) {
	w := newSweepWorld(t)
	var mine []string
	for range 3 {
		mine = append(mine, w.account(t, accountShape{anonymous: true, disabled: true}))
	}
	slices.Sort(mine)
	ctx := context.Background()
	due, err := w.sweeps.Due(ctx, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.IsSorted(due) {
		t.Error("the due accounts are not in id order")
	}
	at := slices.Index(due, mine[0])
	if at < 0 {
		t.Fatal("the first of the accounts is not due")
	}
	limited, err := w.sweeps.Due(ctx, at+1)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(limited, due[:at+1]) {
		t.Errorf("Due(%d) = %d accounts, want the first %d of every due account in id order", at+1, len(limited), at+1)
	}
}

func TestTheAppRoleCannotChangeOrRemoveAMark(t *testing.T) {
	w := newSweepWorld(t)
	id := w.account(t, accountShape{anonymous: true, disabled: true})
	ctx := context.Background()
	if err := w.sweeps.Mark(ctx, id, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`UPDATE app.avatar_sweeps SET swept_at = now() WHERE user_id = $1::uuid`,
		`DELETE FROM app.avatar_sweeps WHERE user_id = $1::uuid`,
	} {
		_, err := w.dbx.Exec(ctx, stmt, id)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || pg.Code != pgerrcode.InsufficientPrivilege {
			t.Errorf("%s as the app role: %v, want permission denied", stmt, err)
		}
	}
}

func TestAMarkGoesWithItsAccount(t *testing.T) {
	w := newSweepWorld(t)
	id := w.account(t, accountShape{anonymous: true, disabled: true})
	ctx := context.Background()
	if err := w.sweeps.Mark(ctx, id, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := w.admin.Exec(ctx, `DELETE FROM app.users WHERE id = $1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	if n := w.count(t, `SELECT count(*) FROM app.avatar_sweeps WHERE user_id = $1::uuid`, id); n != 0 {
		t.Errorf("%d mark(s) outlived the account", n)
	}
}

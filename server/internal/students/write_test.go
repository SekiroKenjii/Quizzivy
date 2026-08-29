package students_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/students"
)

func TestCreatingAStudentEnrolsThemAndForcesAChange(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	w := seedWorld(t, pool, "10.00")
	ctx := context.Background()
	email := "new-" + nonce(t) + "@example.com"

	created, err := store.Create(ctx, students.Request{ActorID: w.admin}, students.CreateInput{
		Email:    email,
		FullName: "Vũ Minh Khôi",
		ClassIDs: []string{w.class},
		Hash:     "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
		Now:      time.Now(),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE user_id = $1::uuid`, created.ID)
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE entity_id = $1::uuid`, created.ID)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id = $1::uuid`, created.ID)
	})

	if !created.MustChangePassword {
		t.Error("an admin-created account did not force a password change")
	}
	if !created.HasPassword {
		t.Error("no password was set")
	}
	if len(created.Classes) != 1 || created.Classes[0].ID != w.class {
		t.Errorf("classes = %+v, want the one it was created in", created.Classes)
	}
	// The pairing §6.4 lets a teacher read: this is not a code join.
	if created.Classes[0].JoinedVia != "admin" {
		t.Errorf("joinedVia = %q, want admin", created.Classes[0].JoinedVia)
	}
}

// users_email_lower_key is an expression index, not a constraint on the column,
// so the collision has to be recognised from the error rather than avoided with
// ON CONFLICT.
func TestEmailUniquenessIgnoresCase(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	w := seedWorld(t, pool, "10.00")
	ctx := context.Background()
	email := "Mixed-" + nonce(t) + "@Example.com"

	input := students.CreateInput{
		Email:    email,
		FullName: "Người Đầu",
		Hash:     "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
		Now:      time.Now(),
	}
	created, err := store.Create(ctx, students.Request{ActorID: w.admin}, input)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE entity_id = $1::uuid`, created.ID)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id = $1::uuid`, created.ID)
	})

	lowered := input
	lowered.Email = strings.ToLower(email)
	lowered.FullName = "Người Sau"
	if _, err := store.Create(ctx, students.Request{ActorID: w.admin}, lowered); !errors.Is(err, students.ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken for the same address in another case, got %v", err)
	}
}

func TestDisablingHidesAStudentWithoutDeletingTheirWork(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	w := seedWorld(t, pool, "10.00")
	ctx := context.Background()

	a := w.assignment(t, pool)
	w.attempt(t, pool, attempt{assignment: a, no: 1, status: "graded", earned: p("6.00"), total: p("10.00")})

	yes := true
	// The contract declares 200 with a StudentRow for this exact request. An
	// earlier version read the row back through the live-students query and so
	// answered 404 for a write that had already committed, which tells an
	// operator the revocation failed when it did not.
	disabled, err := store.Update(ctx, students.Request{ActorID: w.admin}, students.UpdateInput{
		ID: w.student, Disabled: &yes, Now: time.Now(),
	})
	if err != nil {
		t.Fatalf("disabling returned an error for a write that lands: %v", err)
	}
	if disabled.ID != w.student {
		t.Errorf("returned id = %s, want %s", disabled.ID, w.student)
	}

	var off bool
	if err := pool.QueryRow(ctx,
		`SELECT disabled_at IS NOT NULL FROM app.users WHERE id = $1::uuid`, w.student).
		Scan(&off); err != nil {
		t.Fatal(err)
	}
	if !off {
		t.Fatal("the account was not disabled")
	}

	// It does leave the listings, which is the one-way door worth naming: Get
	// and List both hide disabled rows, so nothing in the API can find them
	// again to set disabled:false.
	if _, err := store.Get(ctx, w.student); !errors.Is(err, students.ErrNotFound) {
		t.Errorf("Get on a disabled student = %v, want ErrNotFound", err)
	}

	// §6.4: revoking access must never touch the attempts.
	var attempts int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.attempts WHERE student_id = $1::uuid`, w.student).
		Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1: disabling is not deleting", attempts)
	}
}

func TestResettingAPasswordRevokesEverySession(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	w := seedWorld(t, pool, "10.00")
	ctx := context.Background()

	// Two live families, as two signed-in devices.
	for i := range 2 {
		if _, err := pool.Exec(ctx, `
			INSERT INTO app.refresh_tokens (user_id, family_id, token_hash, expires_at)
			VALUES ($1::uuid, gen_random_uuid(), sha256(($2 || $1)::bytea), now() + interval '30 days')`,
			w.student, string(rune('a'+i))); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.ResetPassword(ctx, students.Request{ActorID: w.admin},
		w.student, "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA", time.Now()); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	var live int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.refresh_tokens
		  WHERE user_id = $1::uuid AND revoked_at IS NULL`, w.student).Scan(&live); err != nil {
		t.Fatal(err)
	}
	// All of them: the admin is acting, so there is no caller session to keep,
	// and a reset that leaves the attacker's family alive achieves nothing.
	if live != 0 {
		t.Errorf("%d refresh families survived the reset", live)
	}

	var mustChange bool
	if err := pool.QueryRow(ctx,
		`SELECT must_change_password FROM app.users WHERE id = $1::uuid`, w.student).
		Scan(&mustChange); err != nil {
		t.Fatal(err)
	}
	if !mustChange {
		t.Error("the reset did not force a change at next sign-in")
	}
}

// An abandoned attempt is the last thing the student did, and the header counts
// it as activity -- so the row must not read as "never started anything".
func TestAnAbandonedAttemptStillCountsAsActivity(t *testing.T) {
	pool := newPool(t)
	store := students.NewStore(pool)
	w := seedWorld(t, pool, "10.00")
	a := w.assignment(t, pool)

	// Started, never submitted, deadline long gone. Nothing ever flips it.
	w.attempt(t, pool, attempt{assignment: a, no: 1, status: "in_progress", live: false})

	stats := statsOf(t, store, w.student)
	if stats.LiveAttempt {
		t.Error("an expired attempt is not live")
	}
	if stats.LastAttemptAt == nil {
		t.Fatal("the row reports no activity at all, which the schema defines " +
			"as never having started anything -- while Facets counts them active")
	}
	if time.Since(*stats.LastAttemptAt) > 4*time.Hour {
		t.Errorf("lastAttemptAt = %v, want the start of the abandoned attempt", *stats.LastAttemptAt)
	}
}

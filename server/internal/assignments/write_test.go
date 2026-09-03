package assignments_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/assignments"
)

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func nonce(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// world is one admin, one class holding one student, and one published test
// with a version -- the least that makes an assignment legal.
type world struct {
	admin     string
	class     string
	student   string
	testID    string
	versionID string
}

func seedWorld(t *testing.T, pool *pgxpool.Pool, status string) world {
	t.Helper()
	ctx := context.Background()
	id := nonce(t)
	var w world

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	must(pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'Giáo viên','admin')
		 RETURNING id::text`, "asg-a-"+id+"@example.com").Scan(&w.admin))
	must(pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'Học viên','student')
		 RETURNING id::text`, "asg-s-"+id+"@example.com").Scan(&w.student))
	must(pool.QueryRow(ctx,
		`INSERT INTO app.classes (name) VALUES ($1) RETURNING id::text`,
		"Lớp "+id).Scan(&w.class))
	must(func() error {
		_, err := pool.Exec(ctx,
			`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
			 VALUES ($1::uuid,$2::uuid,'admin',$3::uuid)`,
			w.class, w.student, w.admin)
		return err
	}())

	version := 1
	if status == "draft" {
		version = 0
	}
	must(pool.QueryRow(ctx,
		`INSERT INTO app.tests (title, status, current_version, created_by)
		 VALUES ($1,$2::app.test_status,$3,$4::uuid) RETURNING id::text`,
		"Đề "+id, status, version, w.admin).Scan(&w.testID))
	must(pool.QueryRow(ctx,
		`INSERT INTO app.test_versions (test_id, version, total_points, published_by)
		 VALUES ($1::uuid,1,'10.00',$2::uuid) RETURNING id::text`,
		w.testID, w.admin).Scan(&w.versionID))

	// By test, not by version: a test may publish a second version mid-case, and
	// an attempt against it would block every delete behind an ON DELETE
	// RESTRICT that these statements ignore -- leaving rows in the dev database
	// that later show up on a screen as if they were real.
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `
			DELETE FROM app.attempts
			 WHERE test_version_id IN (SELECT id FROM app.test_versions WHERE test_id = $1::uuid)`,
			w.testID)
		_, _ = pool.Exec(c, `DELETE FROM app.assignments WHERE test_id = $1::uuid`, w.testID)
		_, _ = pool.Exec(c, `DELETE FROM app.test_versions WHERE test_id = $1::uuid`, w.testID)
		_, _ = pool.Exec(c, `DELETE FROM app.tests WHERE id = $1::uuid`, w.testID)
		_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE class_id = $1::uuid`, w.class)
		_, _ = pool.Exec(c, `DELETE FROM app.classes WHERE id = $1::uuid`, w.class)
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE actor_user_id IN ($1::uuid,$2::uuid)`, w.admin, w.student)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id IN ($1::uuid,$2::uuid)`, w.admin, w.student)
	})
	return w
}

func legalInput(w world) assignments.WriteInput {
	now := time.Now()
	return assignments.WriteInput{
		TestVersionID: w.versionID,
		ClassIDs:      []string{w.class},
		OpensAt:       now.Add(-time.Hour),
		ClosesAt:      now.Add(time.Hour),
		DurationMin:   45,
		MaxAttempts:   1,
		Review:        assignments.Review{ShowScore: true},
		Integrity: assignments.Integrity{
			BlockCopyPaste: true, OnLimitExceeded: "flag", MinAwayMs: 3000,
		},
		Now: now,
	}
}

func request(w world) assignments.Request {
	return assignments.Request{ActorID: w.admin, IP: "203.0.113.7", UserAgent: "go-test"}
}

func fieldsOf(t *testing.T, err error) map[string]string {
	t.Helper()
	var invalid *assignments.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("want a ValidationError, got %v", err)
	}
	out := map[string]string{}
	for _, f := range invalid.Fields {
		out[f.Field] = f.Message
	}
	return out
}

func TestACreatedAssignmentCarriesItsTargetsAndRoster(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	in := legalInput(w)
	in.StudentIDs = []string{w.student}

	created, err := store.Create(ctx, request(w), in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.TestID != w.testID {
		t.Errorf("test id: want %s, got %s", w.testID, created.TestID)
	}
	if len(created.Classes) != 1 || created.Classes[0].ID != w.class {
		t.Errorf("class targets: got %v", created.Classes)
	}
	// The name travels with the target, so no screen has to look it up.
	if len(created.Classes) == 1 && created.Classes[0].Name == "" {
		t.Error("class target carries no name")
	}
	if len(created.StudentIDs) != 1 || created.StudentIDs[0] != w.student {
		t.Errorf("student targets: got %v", created.StudentIDs)
	}
	// The one student is reached both ways; the roster counts them once.
	if created.TargetCount != 1 {
		t.Errorf("target count: want 1, got %d", created.TargetCount)
	}

	got, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID || got.TargetCount != created.TargetCount {
		t.Errorf("get disagrees with create: %+v vs %+v", got, created)
	}

	var actions []string
	rows, err := pool.Query(ctx,
		`SELECT action FROM app.audit_log WHERE entity_id = $1::uuid`, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, a)
	}
	if len(actions) != 1 || actions[0] != "assignment.created" {
		t.Errorf("audit: want one assignment.created, got %v", actions)
	}
}

func TestOnlyAPublishedVersionCanBeAssigned(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	ctx := context.Background()

	for _, status := range []string{"draft", "archived"} {
		t.Run(status, func(t *testing.T) {
			w := seedWorld(t, pool, status)
			_, err := store.Create(ctx, request(w), legalInput(w))
			if !errors.Is(err, assignments.ErrTestNotPublished) {
				t.Fatalf("want ErrTestNotPublished, got %v", err)
			}
		})
	}

	t.Run("unknown version", func(t *testing.T) {
		w := seedWorld(t, pool, "published")
		in := legalInput(w)
		in.TestVersionID = "00000000-0000-7000-8000-00000000dead"
		_, err := store.Create(ctx, request(w), in)
		if !errors.Is(err, assignments.ErrTestNotPublished) {
			t.Fatalf("want ErrTestNotPublished, got %v", err)
		}
	})
}

func TestAnAssignmentNobodyCanTakeIsRejected(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")

	in := legalInput(w)
	in.ClassIDs = nil

	_, err := store.Create(context.Background(), request(w), in)
	if _, ok := fieldsOf(t, err)["targets"]; !ok {
		t.Errorf("want a targets error, got %v", err)
	}
}

func TestTheWindowMustBeAWindow(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")

	in := legalInput(w)
	in.ClosesAt = in.OpensAt

	_, err := store.Create(context.Background(), request(w), in)
	if _, ok := fieldsOf(t, err)["window.closesAt"]; !ok {
		t.Errorf("want a window error, got %v", err)
	}
}

// A bad id must name itself. Left to the FK it would be one opaque 500 for a
// form carrying forty of them.
func TestAnUnknownTargetIsNamed(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")
	const ghost = "00000000-0000-7000-8000-0000000000aa"

	in := legalInput(w)
	in.StudentIDs = []string{ghost}

	_, err := store.Create(context.Background(), request(w), in)
	message, ok := fieldsOf(t, err)["targets.studentIds"]
	if !ok {
		t.Fatalf("want a studentIds error, got %v", err)
	}
	if !strings.Contains(message, ghost) {
		t.Errorf("the message does not name the id: %q", message)
	}
}

// An admin is a real user, so only the role check keeps them out of a roster.
func TestOnlyAStudentCanBeTargetedIndividually(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")

	in := legalInput(w)
	in.StudentIDs = []string{w.admin}

	_, err := store.Create(context.Background(), request(w), in)
	if _, ok := fieldsOf(t, err)["targets.studentIds"]; !ok {
		t.Errorf("want a studentIds error, got %v", err)
	}
}

func TestUpdateReplacesTargetsRatherThanAddingToThem(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	in := legalInput(w)
	in.StudentIDs = []string{w.student}
	created, err := store.Create(ctx, request(w), in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	req := request(w)
	req.ID = created.ID
	next := legalInput(w)
	next.ClassIDs = nil
	next.StudentIDs = []string{w.student}

	saved, err := store.Update(ctx, req, next)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(saved.Classes) != 0 {
		t.Errorf("dropped class target survived: %v", saved.Classes)
	}
	if len(saved.StudentIDs) != 1 {
		t.Errorf("student targets: got %v", saved.StudentIDs)
	}
}

func TestTheVersionIsLockedOnceAnybodyHasStarted(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	created, err := store.Create(ctx, request(w), legalInput(w))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var second string
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.test_versions (test_id, version, total_points, published_by)
		 VALUES ($1::uuid,2,'10.00',$2::uuid) RETURNING id::text`,
		w.testID, w.admin).Scan(&second); err != nil {
		t.Fatal(err)
	}
	req := request(w)
	req.ID = created.ID
	repointed := legalInput(w)
	repointed.TestVersionID = second

	// Untouched, so re-pointing is the legitimate workflow the contract names.
	if _, err := store.Update(ctx, req, repointed); err != nil {
		t.Fatalf("re-point before any attempt: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO app.attempts
		       (assignment_id, test_version_id, student_id, attempt_no, status,
		        session_id, shuffle_seed, beacon_token_hash, deadline_at)
		VALUES ($1::uuid,$2::uuid,$3::uuid,1,'in_progress', gen_random_uuid(), 7,
		        sha256('beacon'::bytea), now() + interval '1 hour')`,
		created.ID, second, w.student); err != nil {
		t.Fatal(err)
	}

	back := legalInput(w)
	back.TestVersionID = w.versionID
	if _, err := store.Update(ctx, req, back); !errors.Is(err, assignments.ErrVersionLocked) {
		t.Fatalf("want ErrVersionLocked, got %v", err)
	}

	// Everything else about a started assignment is still editable.
	sameVersion := legalInput(w)
	sameVersion.TestVersionID = second
	sameVersion.DurationMin = 60
	saved, err := store.Update(ctx, req, sameVersion)
	if err != nil {
		t.Fatalf("edit with the version unchanged: %v", err)
	}
	if saved.DurationMin != 60 {
		t.Errorf("duration: want 60, got %d", saved.DurationMin)
	}
}

func TestClosingEarlyIsRecordedAndDoesNotReopen(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	created, err := store.Create(ctx, request(w), legalInput(w))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ClosedAt != nil {
		t.Fatalf("a new assignment is not closed: %v", created.ClosedAt)
	}

	req := request(w)
	req.ID = created.ID
	closing := legalInput(w)
	closing.CloseNow = true

	closed, err := store.Update(ctx, req, closing)
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if closed.ClosedAt == nil {
		t.Fatal("closeNow did not set closedAt")
	}
	if got := assignments.StatusAt(time.Now(), closed.PublishedAt, closed.OpensAt, closed.ClosesAt, closed.ClosedAt); got != assignments.Closed {
		t.Errorf("status: want closed, got %s", got)
	}

	// closeNow is an action, so its absence must not undo it.
	reopened, err := store.Update(ctx, req, legalInput(w))
	if err != nil {
		t.Fatalf("update after closing: %v", err)
	}
	if reopened.ClosedAt == nil {
		t.Error("a later save reopened a closed assignment")
	}
}

func TestAutoSubmitIsRefusedUntilItExists(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")

	in := legalInput(w)
	in.Integrity.OnLimitExceeded = "auto_submit"

	_, err := store.Create(context.Background(), request(w), in)
	if _, ok := fieldsOf(t, err)["integrity.onLimitExceeded"]; !ok {
		t.Errorf("want an onLimitExceeded error, got %v", err)
	}
}

func TestUpdatingSomethingThatIsNotThereIsNotFound(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")

	req := request(w)
	req.ID = "00000000-0000-7000-8000-0000000000bb"
	if _, err := store.Update(context.Background(), req, legalInput(w)); !errors.Is(err, assignments.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// Disabling a student must not pin an assignment one short for ever.
//
// A suspended account cannot sign in, so it is not a student the teacher is
// still waiting on. Leaving it in the denominator makes "12/13" a number that
// nothing can ever close.
func TestADisabledStudentLeavesTheProgressDenominator(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	// A second student in the same class, so the roster is two.
	var other string
	if err := pool.QueryRow(ctx,
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'Người Thứ Hai','student')
		 RETURNING id::text`, "asg-x-"+nonce(t)+"@example.com").Scan(&other); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
		 VALUES ($1::uuid,$2::uuid,'admin',$3::uuid)`, w.class, other, w.admin); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.class_members WHERE user_id = $1::uuid`, other)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id = $1::uuid`, other)
	})

	created, err := store.Create(ctx, request(w), legalInput(w))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	before, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.TargetCount != 2 {
		t.Fatalf("target count = %d, want 2", before.TargetCount)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, other); err != nil {
		t.Fatal(err)
	}

	after, err := store.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.TargetCount != 1 {
		t.Errorf("target count = %d after disabling one of two, want 1", after.TargetCount)
	}
}

// G-01's "Lưu nháp": saved, targeted or not, given to nobody.
func TestADraftIsSavedWithoutBeingGivenOut(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	in := legalInput(w)
	in.Draft = true
	// The one assignment allowed to reach nobody yet: coming back to it is the
	// point of saving one.
	in.ClassIDs = nil

	draft, err := store.Create(ctx, request(w), in)
	if err != nil {
		t.Fatalf("saving a draft with no targets: %v", err)
	}
	if draft.PublishedAt != nil {
		t.Error("a draft reports a publication time")
	}
	if got := assignments.StatusAt(time.Now(), draft.PublishedAt,
		draft.OpensAt, draft.ClosesAt, draft.ClosedAt); got != assignments.Draft {
		t.Errorf("status = %s, want draft — its window is current", got)
	}

	// Saving again with draft:false is what "Giao bài" sends.
	req := request(w)
	req.ID = draft.ID
	published, err := store.Update(ctx, req, legalInput(w))
	if err != nil {
		t.Fatalf("publishing the draft: %v", err)
	}
	if published.PublishedAt == nil {
		t.Fatal("publishing did not record when")
	}
	if got := assignments.StatusAt(time.Now(), published.PublishedAt,
		published.OpensAt, published.ClosesAt, published.ClosedAt); got != assignments.Open {
		t.Errorf("status = %s, want open", got)
	}

	var action string
	if err := pool.QueryRow(ctx,
		`SELECT action FROM app.audit_log WHERE entity_id = $1::uuid
		  ORDER BY occurred_at DESC LIMIT 1`, draft.ID).Scan(&action); err != nil {
		t.Fatal(err)
	}
	if action != "assignment.published" {
		t.Errorf("audit action = %q, want assignment.published", action)
	}
}

// Publishing is one-way. Students may already be sitting it, so the only way
// back out is closing it.
func TestAPublishedAssignmentCannotBecomeADraftAgain(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	created, err := store.Create(ctx, request(w), legalInput(w))
	if err != nil {
		t.Fatal(err)
	}

	req := request(w)
	req.ID = created.ID
	back := legalInput(w)
	back.Draft = true

	saved, err := store.Update(ctx, req, back)
	if err != nil {
		t.Fatal(err)
	}
	if saved.PublishedAt == nil {
		t.Error("saving with draft:true un-gave an assignment students may be sitting")
	}
}

// The status filter has to know about the new state, or a draft shows up under
// whatever its window happens to say.
func TestTheListFiltersDraftsSeparately(t *testing.T) {
	pool := newPool(t)
	store := assignments.NewStore(pool)
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	in := legalInput(w)
	in.Draft = true
	draft, err := store.Create(ctx, request(w), in)
	if err != nil {
		t.Fatal(err)
	}

	drafts := assignments.Draft
	found, _, err := store.List(ctx, assignments.ListInput{Status: &drafts})
	if err != nil {
		t.Fatal(err)
	}
	var seen bool
	for _, a := range found {
		if a.ID == draft.ID {
			seen = true
		}
	}
	if !seen {
		t.Error("status=draft did not return the draft")
	}

	open := assignments.Open
	opened, _, err := store.List(ctx, assignments.ListInput{Status: &open})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range opened {
		if a.ID == draft.ID {
			t.Error("a draft appeared under status=open because its window is current")
		}
	}
}

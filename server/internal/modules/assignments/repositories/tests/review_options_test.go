//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"quizzivy/internal/modules/assignments/domain"
	"quizzivy/internal/modules/assignments/repositories"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

func secondVersion(t *testing.T, pool *pgxpool.Pool, w world) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO app.test_versions (test_id, version, total_points, published_by)
		 VALUES ($1::uuid, 2, '10.00', $2::uuid) RETURNING id::text`,
		w.testID, w.admin).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func updating(w world, id string) domain.Request {
	req := request(w)
	req.ID = id
	return req
}

func TestAnOpenAssignmentLocksItsTestAndTiming(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")
	second := secondVersion(t, pool, w)
	ctx := context.Background()

	created := createFor(t, store, w, legalInput(w))
	req := updating(w, created.ID)

	for name, change := range map[string]func(*domain.WriteInput){
		"the test version": func(in *domain.WriteInput) { in.TestVersionID = second },
		"the duration":     func(in *domain.WriteInput) { in.DurationMin++ },
		"the attempts":     func(in *domain.WriteInput) { in.MaxAttempts++ },
	} {
		in := legalInput(w)
		change(&in)
		if _, err := store.Update(ctx, req, in); !errors.Is(err, domain.ErrAssignmentLocked) {
			t.Errorf("changing %s of an open assignment: %v, want ErrAssignmentLocked", name, err)
		}
	}
	got, err := store.Get(ctx, req.Scope(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TestVersionID != created.TestVersionID || got.DurationMin != created.DurationMin || got.MaxAttempts != created.MaxAttempts {
		t.Errorf("a refused change was kept: version %s duration %d attempts %d", got.TestVersionID, got.DurationMin, got.MaxAttempts)
	}

	again := legalInput(w)
	again.ClosesAt = again.ClosesAt.Add(time.Hour)
	again.ShuffleQ = true
	again.Review.ShowScore = false
	again.Integrity.MaxFocusLoss = 4
	saved, err := store.Update(ctx, req, again)
	if err != nil {
		t.Fatalf("saving everything else with the three values unchanged: %v", err)
	}
	if !saved.ShuffleQ || saved.Review.ShowScore || saved.Integrity.MaxFocusLoss != 4 {
		t.Errorf("the other fields were not saved: %+v", saved)
	}
}

func TestTheLockAnswersBeforeTheVersionCheck(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")
	second := secondVersion(t, pool, w)
	ctx := context.Background()

	window := func(opens, closes time.Duration) domain.WriteInput {
		in := legalInput(w)
		in.OpensAt, in.ClosesAt = in.Now.Add(opens), in.Now.Add(closes)
		return in
	}
	repoint := func(in domain.WriteInput) domain.WriteInput {
		in.TestVersionID = second
		return in
	}

	open := createFor(t, store, w, window(-time.Hour, time.Hour))
	sitAttempt(t, pool, w, open.ID, "in_progress", "0.00", false)
	if _, err := store.Update(ctx, updating(w, open.ID), repoint(window(-time.Hour, time.Hour))); !errors.Is(err, domain.ErrAssignmentLocked) {
		t.Errorf("an open assignment with an attempt: %v, want ErrAssignmentLocked", err)
	}

	scheduled := createFor(t, store, w, window(time.Hour, 2*time.Hour))
	sitAttempt(t, pool, w, scheduled.ID, "in_progress", "0.00", false)
	if _, err := store.Update(ctx, updating(w, scheduled.ID), repoint(window(time.Hour, 2*time.Hour))); !errors.Is(err, domain.ErrVersionLocked) {
		t.Errorf("a scheduled assignment with an attempt: %v, want ErrVersionLocked", err)
	}

	closed := createFor(t, store, w, window(-2*time.Hour, -time.Hour))
	sitAttempt(t, pool, w, closed.ID, "graded", "5.00", false)
	if _, err := store.Update(ctx, updating(w, closed.ID), repoint(window(-2*time.Hour, -time.Hour))); !errors.Is(err, domain.ErrVersionLocked) {
		t.Errorf("a closed assignment with an attempt: %v, want ErrVersionLocked", err)
	}

	untouched := createFor(t, store, w, window(-2*time.Hour, -time.Hour))
	longer := window(-2*time.Hour, -time.Hour)
	longer.DurationMin, longer.MaxAttempts = 90, 3
	saved, err := store.Update(ctx, updating(w, untouched.ID), longer)
	if err != nil {
		t.Fatalf("a closed assignment's timing: %v", err)
	}
	if saved.DurationMin != 90 || saved.MaxAttempts != 3 {
		t.Errorf("a closed assignment kept duration %d, attempts %d", saved.DurationMin, saved.MaxAttempts)
	}
}

func TestTheLockFlipsExactlyAtTheEdgesOfTheWindow(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	opens := time.Now().Add(time.Hour).Truncate(time.Millisecond)
	closes := opens.Add(time.Hour)
	base := legalInput(w)
	base.OpensAt, base.ClosesAt, base.Now = opens, closes, opens.Add(-time.Hour)
	created := createFor(t, store, w, base)
	req := updating(w, created.ID)

	minutes := 50
	for _, c := range []struct {
		when   string
		now    time.Time
		locked bool
	}{
		{"a microsecond before it opens", opens.Add(-time.Microsecond), false},
		{"the instant it opens", opens, true},
		{"a microsecond before it closes", closes.Add(-time.Microsecond), true},
		{"the instant it closes", closes, false},
	} {
		minutes++
		next := base
		next.Now, next.DurationMin = c.now, minutes
		_, err := store.Update(ctx, req, next)
		if locked := errors.Is(err, domain.ErrAssignmentLocked); locked != c.locked || (!c.locked && err != nil) {
			t.Errorf("%s: locked=%v err=%v, want locked=%v", c.when, locked, err, c.locked)
		}
	}
}

func TestTheNoteRoundTripsFromTheTeacherToTheStudent(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	note := "  Mang theo máy tính cầm tay.\nKhông dùng điện thoại.  "
	in := legalInput(w)
	in.StudentNote, in.StudentNoteSet = domain.StudentNoteOf(&note), true
	in.Review.Release, in.Review.ShowClassAverage = domain.ReleaseAfterClose, true
	created := createFor(t, store, w, in)
	want := "Mang theo máy tính cầm tay.\nKhông dùng điện thoại."
	if created.StudentNote == nil || *created.StudentNote != want {
		t.Fatalf("the teacher's read: %v, want the trimmed note", created.StudentNote)
	}
	if created.Review.Release != domain.ReleaseAfterClose || !created.Review.ShowClassAverage {
		t.Errorf("the teacher's read of the review: %+v", created.Review)
	}

	seen, err := store.StudentDetail(ctx, created.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	if seen.StudentNote == nil || *seen.StudentNote != want {
		t.Errorf("the student's read: %v, want the trimmed note", seen.StudentNote)
	}
	if seen.Review.Release != domain.ReleaseAfterClose || !seen.Review.ShowClassAverage || !seen.Review.ShowScore {
		t.Errorf("the student's read of the review: %+v, want the stored flags", seen.Review)
	}

	omitted := legalInput(w)
	omitted.Review.ShowScore = false
	kept, err := store.Update(ctx, updating(w, created.ID), omitted)
	if err != nil {
		t.Fatal(err)
	}
	if kept.StudentNote == nil || *kept.StudentNote != want || kept.Review.Release != domain.ReleaseAfterClose || !kept.Review.ShowClassAverage {
		t.Errorf("an update that named none of the three changed them: %v %+v", kept.StudentNote, kept.Review)
	}

	cleared := legalInput(w)
	cleared.StudentNoteSet = true
	cleared.ReleaseSet, cleared.ClassAverageSet = true, true
	cleared.Review.Release = domain.ReleaseOnSubmit
	after, err := store.Update(ctx, updating(w, created.ID), cleared)
	if err != nil {
		t.Fatal(err)
	}
	if after.StudentNote != nil || after.Review.Release != domain.ReleaseOnSubmit || after.Review.ShowClassAverage {
		t.Errorf("naming the three did not clear them: %v %+v", after.StudentNote, after.Review)
	}
	seen, err = store.StudentDetail(ctx, created.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	if seen.StudentNote != nil {
		t.Errorf("a cleared note is still read: %q", *seen.StudentNote)
	}
}

func TestTheNoteIsOneToFiveHundredCharactersAfterTrimming(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	create := func(note string) (domain.Assignment, error) {
		in := legalInput(w)
		in.StudentNote, in.StudentNoteSet = domain.StudentNoteOf(&note), true
		return store.Create(ctx, request(w), in)
	}

	full := strings.Repeat("ệ", domain.MaxStudentNote)
	a, err := create("  " + full + "\n")
	if err != nil {
		t.Fatalf("500 characters inside padding: %v", err)
	}
	if a.StudentNote == nil || utf8.RuneCountInString(*a.StudentNote) != domain.MaxStudentNote {
		t.Errorf("stored %v characters, want 500", a.StudentNote)
	}

	if _, err := create(full + "ệ"); err == nil {
		t.Error("501 characters were accepted")
	} else if _, ok := fieldsOf(t, err)["studentNote"]; !ok {
		t.Errorf("501 characters named %v, want studentNote", fieldsOf(t, err))
	}

	for _, blank := range []string{"", "   ", "\n\t "} {
		a, err := create(blank)
		if err != nil {
			t.Fatalf("a blank note %q: %v", blank, err)
		}
		if a.StudentNote != nil {
			t.Errorf("a blank note %q was stored as %q", blank, *a.StudentNote)
		}
	}
}

func TestThePlainTextOfTheNoteIsStoredAsWritten(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")

	note := `<b>Đọc kỹ</b> & "nộp" <script>alert(1)</script> [link](https://example.test)`
	in := legalInput(w)
	in.StudentNote, in.StudentNoteSet = domain.StudentNoteOf(&note), true
	created := createFor(t, store, w, in)

	seen, err := store.StudentDetail(context.Background(), created.ID, w.student)
	if err != nil {
		t.Fatal(err)
	}
	if seen.StudentNote == nil || *seen.StudentNote != note {
		t.Errorf("the note came back as %v, want it byte for byte", seen.StudentNote)
	}
}

func TestACardShowsTheScoreOnlyOnceAnAfterCloseResultIsReleased(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewPostgres(db.NewContext(pool))
	w := seedWorld(t, pool, "published")
	ctx := context.Background()

	onSubmit := createFor(t, store, w, legalInput(w))
	held := legalInput(w)
	held.Review.Release = domain.ReleaseAfterClose
	afterClose := createFor(t, store, w, held)
	sitAttempt(t, pool, w, onSubmit.ID, "graded", "5.00", false)
	sitAttempt(t, pool, w, afterClose.ID, "graded", "5.00", false)

	scoreOf := func(id string) *domain.Score {
		t.Helper()
		sections, err := store.ForStudent(ctx, w.student, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range sections.Completed {
			if c.ID == id {
				return c.Score
			}
		}
		t.Fatalf("assignment %s is not completed", id)
		return nil
	}
	if scoreOf(onSubmit.ID) == nil {
		t.Error("an on_submit card hides the score")
	}
	if scoreOf(afterClose.ID) != nil {
		t.Error("an after_close card shows the score before the close")
	}
	if _, err := pool.Exec(ctx,
		`UPDATE app.assignments SET closed_at = now() - interval '1 second' WHERE id = $1::uuid`, afterClose.ID); err != nil {
		t.Fatal(err)
	}
	if scoreOf(afterClose.ID) == nil {
		t.Error("an after_close card hides the score once the assignment has closed")
	}
}

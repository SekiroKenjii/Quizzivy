//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/internal/modules/assignments/domain"
)

func (w *reachWorld) original(t *testing.T) (domain.Assignment, time.Duration) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	note := "Mang theo bút và giấy nháp."
	in := input(w.versionA, []string{w.classA}, []string{w.studentA})
	in.OpensAt, in.ClosesAt = now.Add(-2*time.Hour), now.Add(-2*time.Hour).Add(3*time.Hour)
	in.DurationMin, in.MaxAttempts = 37, 3
	in.ShuffleQ, in.ShuffleO = true, true
	in.Review = domain.Review{
		ShowScore: true, ShowCorrectAnswers: true, ShowExplanations: false,
		Release: domain.ReleaseAfterClose, ShowClassAverage: true,
	}
	in.Integrity = domain.Integrity{RequireFullscreen: true, BlockCopyPaste: true, MaxFocusLoss: 4, OnLimitExceeded: "auto_submit", MinAwayMs: 5000}
	in.StudentNote, in.StudentNoteSet = &note, true
	created, err := w.store.Create(ctx, as(w.a, false), in)
	if err != nil {
		t.Fatalf("creating the original: %v", err)
	}
	if _, err := w.tx.Exec(ctx, `UPDATE app.assignments SET closed_at = now() - interval '1 minute' WHERE id = $1::uuid`, created.ID); err != nil {
		t.Fatal(err)
	}
	return created, in.ClosesAt.Sub(in.OpensAt)
}

func TestADuplicateIsADraftWithTheOriginalsRulesAndTheClassesNamed(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	original, length := w.original(t)

	req := as(w.a, false)
	req.ID, req.IP, req.UserAgent = original.ID, "203.0.113.7", "go-test"
	now := time.Now()
	copied, err := w.store.Duplicate(ctx, req, []string{w.classA}, now)
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}

	if copied.ID == original.ID {
		t.Fatal("the copy is the original")
	}
	if copied.PublishedAt != nil || domain.Schedule.StatusAt(now, copied.PublishedAt, copied.OpensAt, copied.ClosesAt, copied.ClosedAt) != domain.Draft {
		t.Errorf("the copy is published (%v), want a draft", copied.PublishedAt)
	}
	if copied.TestID != original.TestID || copied.TestVersionID != original.TestVersionID {
		t.Errorf("the copy pins %s/%s, want %s/%s", copied.TestID, copied.TestVersionID, original.TestID, original.TestVersionID)
	}
	if copied.DurationMin != 37 || copied.MaxAttempts != 3 || !copied.ShuffleQ || !copied.ShuffleO {
		t.Errorf("the copy's time limit and attempts read %d/%d shuffle %v/%v, want 37/3 true/true", copied.DurationMin, copied.MaxAttempts, copied.ShuffleQ, copied.ShuffleO)
	}
	if copied.Review != original.Review {
		t.Errorf("the copy's review options read %+v, want %+v", copied.Review, original.Review)
	}
	if copied.Integrity != original.Integrity {
		t.Errorf("the copy's integrity policy reads %+v, want %+v", copied.Integrity, original.Integrity)
	}
	if copied.StudentNote == nil || *copied.StudentNote != "Mang theo bút và giấy nháp." {
		t.Errorf("the copy's note to students reads %v, want the original's", copied.StudentNote)
	}
	if got := copied.ClosesAt.Sub(copied.OpensAt); got != length {
		t.Errorf("the copy's window lasts %v, want the original's %v", got, length)
	}
	if opened := copied.OpensAt.Sub(now).Abs(); opened > time.Millisecond {
		t.Errorf("the copy opens %v from now, want it to open now", opened)
	}
	if copied.ClosedAt != nil {
		t.Errorf("the copy is closed early at %v, want it not", copied.ClosedAt)
	}
	if len(copied.Classes) != 1 || copied.Classes[0].ID != w.classA || len(copied.Students) != 0 {
		t.Errorf("the copy targets %+v and %+v, want only the class named", copied.Classes, copied.Students)
	}

	var owner, source, action string
	if err := w.tx.QueryRow(ctx, `
		SELECT a.created_by::text, l.diff->>'sourceId', l.action
		  FROM app.assignments a JOIN app.audit_log l ON l.entity_id = a.id AND l.action = 'assignment.duplicated'
		 WHERE a.id = $1::uuid`, copied.ID).Scan(&owner, &source, &action); err != nil {
		t.Fatalf("audit row: %v", err)
	}
	if owner != w.a || source != original.ID {
		t.Errorf("the copy is owned by %s and audited from %s, want %s and %s", owner, source, w.a, original.ID)
	}

	kept, err := w.store.Get(ctx, as(w.a, false).Scope(), original.ID)
	if err != nil {
		t.Fatal(err)
	}
	if kept.PublishedAt == nil || kept.ClosedAt == nil || len(kept.Students) != 1 || !kept.ClosesAt.Equal(original.ClosesAt) {
		t.Errorf("the original changed: %+v", kept)
	}
}

func TestADuplicateMayBeAssignedToNoClassYet(t *testing.T) {
	w := newReachWorld(t)
	req := as(w.a, false)
	req.ID = w.mineA
	copied, err := w.store.Duplicate(context.Background(), req, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(copied.Classes) != 0 || len(copied.Students) != 0 || copied.PublishedAt != nil {
		t.Errorf("a copy with no classes reads %+v, want an untargeted draft", copied)
	}
}

func TestADuplicateNamesOnlyWhatTheCallerReaches(t *testing.T) {
	w := newReachWorld(t)
	ctx := context.Background()
	now := time.Now()

	asB := as(w.b, false)
	asB.ID = w.mineA
	if _, err := w.store.Duplicate(ctx, asB, []string{w.classB}, now); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("another teacher's assignment: got %v, want ErrNotFound", err)
	}
	gone := as(w.a, false)
	gone.ID = uuid.NewString()
	if _, err := w.store.Duplicate(ctx, gone, []string{w.classA}, now); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a missing assignment: got %v, want ErrNotFound", err)
	}

	asA := as(w.a, false)
	asA.ID = w.mineA
	for _, class := range []string{w.classB, uuid.NewString()} {
		_, err := w.store.Duplicate(ctx, asA, []string{class}, now)
		if got := fieldsOf(t, err); got["classIds"] == "" || len(got) != 1 {
			t.Fatalf("a class %s the caller does not teach: fields %v, want one on classIds", class, got)
		}
	}

	reachedOnly := as(w.b, false)
	reachedOnly.ID = w.shared
	if _, err := w.store.Duplicate(ctx, reachedOnly, []string{w.classB}, now); !errors.Is(err, domain.ErrTestNotPublished) {
		t.Errorf("a teacher who reaches the assignment through a class but does not own its test: got %v, want ErrTestNotPublished", err)
	}

	everyoneReq := as(w.admin, true)
	everyoneReq.ID = w.shared
	copied, err := w.store.Duplicate(ctx, everyoneReq, []string{w.classA, w.classB}, now)
	if err != nil {
		t.Fatalf("scope.all duplicating a teacher's assignment into two teachers' classes: %v", err)
	}
	got := []string{}
	for _, c := range copied.Classes {
		got = append(got, c.ID)
	}
	if !slices.Equal(sortedIDs(got...), sortedIDs(w.classA, w.classB)) {
		t.Errorf("the copy targets %v, want both classes", got)
	}
}

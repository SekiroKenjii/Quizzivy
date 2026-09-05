//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	"testing"
)

func TestTheTeachersNoteIsKeptTrimmedAndCleared(t *testing.T) {
	pool := newPool(t)
	store := repositories.NewReviews(pool)
	p := seedPaper(t, pool, "submitted")
	ctx := context.Background()

	note := "  đã hỏi Hân, em nói mất điện lúc 10:04  "
	if err := store.SetNote(ctx, p.attempt, &note); err != nil {
		t.Fatalf("set note: %v", err)
	}
	rv, err := store.Get(ctx, p.attempt)
	if err != nil {
		t.Fatal(err)
	}
	if rv.TeacherNote == nil || *rv.TeacherNote != "đã hỏi Hân, em nói mất điện lúc 10:04" {
		t.Errorf("note read back as %v", rv.TeacherNote)
	}

	blank := "   "
	if err := store.SetNote(ctx, p.attempt, &blank); err != nil {
		t.Fatalf("clear note: %v", err)
	}
	if rv, err = store.Get(ctx, p.attempt); err != nil {
		t.Fatal(err)
	}
	if rv.TeacherNote != nil {
		t.Errorf("a blank note was kept as %q", *rv.TeacherNote)
	}

	if err := store.SetNote(ctx, "00000000-0000-7000-8000-000000000000", &note); !errors.Is(err, domain.ErrPaperNotFound) {
		t.Errorf("noting an unknown attempt: %v, want ErrPaperNotFound", err)
	}
}

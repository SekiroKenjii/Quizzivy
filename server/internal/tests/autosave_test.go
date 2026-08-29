package tests_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"quizzivy/internal/tests"
)

// §1.3 has one admin editing at a time, so a stale write is a second tab
// rather than genuine contention -- and failing loudly beats silently
// reverting the outline the other tab was holding.
func TestAStaleUpdatedAtIsRejected(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	created, err := svc.Create(ctx, req(author), "Đề kiểm tra", nil)
	if err != nil {
		t.Fatal(err)
	}
	stale := created.UpdatedAt

	title := "Đã đổi tên"
	saved, err := svc.Update(ctx, reqFor(created.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: stale, Title: &title,
	})
	if err != nil {
		t.Fatalf("the first save should succeed: %v", err)
	}
	if !saved.UpdatedAt.After(stale) {
		t.Fatal("updated_at did not advance, so the guard cannot distinguish saves")
	}

	// The second tab still holds the version it read before the first save.
	other := "Tên từ tab cũ"
	_, err = svc.Update(ctx, reqFor(created.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: stale, Title: &other,
	})
	if !errors.Is(err, tests.ErrStaleWrite) {
		t.Errorf("stale save returned %v, want ErrStaleWrite", err)
	}

	after, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Title != title {
		t.Errorf("title is %q; the refused save overwrote the good one", after.Title)
	}
}

// An outline-only save must still advance the version, or two successive
// autosaves would both pass the same guard and the second would silently
// clobber the first.
func TestAnOutlineOnlySaveStillAdvancesTheVersion(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	created, err := svc.Create(ctx, req(author), "Đề có phần", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := newQuestion(t, pool, author, "Câu hỏi trong đề")

	saved, err := svc.Update(ctx, reqFor(created.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: created.UpdatedAt,
		SetSections:       true,
		Sections: []tests.SectionInput{
			{Title: "Phần nghe", QuestionIDs: []string{q}},
		},
	})
	if err != nil {
		t.Fatalf("outline save: %v", err)
	}
	if !saved.UpdatedAt.After(created.UpdatedAt) {
		t.Error("an outline-only save left updated_at unchanged")
	}

	_, err = svc.Update(ctx, reqFor(created.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: created.UpdatedAt, SetSections: true,
	})
	if !errors.Is(err, tests.ErrStaleWrite) {
		t.Errorf("re-using the pre-outline version returned %v, want ErrStaleWrite", err)
	}
}

func TestUpdatingAMissingTestIsNotFound(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	_, err := svc.Update(context.Background(),
		reqFor("00000000-0000-7000-8000-000000000000", author),
		tests.UpdateInput{ExpectedUpdatedAt: time.Now()})
	if !errors.Is(err, tests.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

// The whole outline is one transaction: a section naming a missing question
// must leave the previous outline exactly as it was.
func TestARejectedOutlineLeavesThePreviousOneIntact(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	created, err := svc.Create(ctx, req(author), "Đề nguyên vẹn", nil)
	if err != nil {
		t.Fatal(err)
	}
	good := newQuestion(t, pool, author, "Câu hỏi tốt")

	saved, err := svc.Update(ctx, reqFor(created.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: created.UpdatedAt,
		SetSections:       true,
		Sections:          []tests.SectionInput{{Title: "Phần 1", QuestionIDs: []string{good}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.Update(ctx, reqFor(created.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: saved.UpdatedAt,
		SetSections:       true,
		Sections: []tests.SectionInput{
			{Title: "Phần 1", QuestionIDs: []string{good}},
			{Title: "Phần 2", QuestionIDs: []string{"00000000-0000-7000-8000-000000000000"}},
		},
	})
	if !errors.Is(err, tests.ErrUnknownQuestion) {
		t.Fatalf("got %v, want ErrUnknownQuestion", err)
	}

	after, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Sections) != 1 || after.Sections[0].Title != "Phần 1" {
		t.Errorf("the refused save changed the outline: %+v", after.Sections)
	}
	if !after.UpdatedAt.Equal(saved.UpdatedAt) {
		t.Error("the refused save advanced updated_at, so the client's version is now wrong")
	}
}

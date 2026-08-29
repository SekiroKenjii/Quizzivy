package tests_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/tests"
)

// Reordering sections rewrites their ordinals one row at a time, which
// transiently collides on the unique constraint whenever two swap places.
// D-13's DEFERRABLE is what lets that be one straightforward pass.
func TestReorderingSectionsKeepsTheirIdentity(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	created, err := svc.Create(ctx, req(author), "Đề sắp xếp lại", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := newQuestion(t, pool, author, "Câu hỏi dùng chung")

	saved, err := svc.Update(ctx, reqFor(created.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: created.UpdatedAt,
		SetSections:       true,
		Sections: []tests.SectionInput{
			{Title: "A", QuestionIDs: []string{q}},
			{Title: "B", QuestionIDs: []string{}},
			{Title: "C", QuestionIDs: []string{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	first, second, third := saved.Sections[0], saved.Sections[1], saved.Sections[2]

	// Drag the first section to the end.
	reordered, err := svc.Update(ctx, reqFor(created.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: saved.UpdatedAt,
		SetSections:       true,
		Sections: []tests.SectionInput{
			{ID: second.ID, Title: "B", QuestionIDs: []string{}},
			{ID: third.ID, Title: "C", QuestionIDs: []string{}},
			{ID: first.ID, Title: "A", QuestionIDs: []string{q}},
		},
	})
	if err != nil {
		t.Fatalf("reorder: %v", err)
	}

	want := []struct{ id, title string }{
		{second.ID, "B"}, {third.ID, "C"}, {first.ID, "A"},
	}
	for i, w := range want {
		got := reordered.Sections[i]
		if got.Ordinal != i {
			t.Errorf("section %d has ordinal %d; ordinals must be dense 0..n-1", i, got.Ordinal)
		}
		if got.ID != w.id {
			t.Errorf("position %d holds section %s, want %s -- ids must survive a reorder",
				i, got.ID, w.id)
		}
		if got.Title != w.title {
			t.Errorf("position %d is %q, want %q", i, got.Title, w.title)
		}
	}
	if len(reordered.Sections[2].QuestionIDs) != 1 {
		t.Error("the moved section lost its questions")
	}
}

// A section dropped from the payload is removed, and its question rows go with
// it, but the bank questions do not.
func TestDroppingASectionRemovesItAndItsQuestionRows(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	created, err := svc.Create(ctx, req(author), "Đề bỏ phần", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := newQuestion(t, pool, author, "Câu hỏi còn lại")

	saved, err := svc.Update(ctx, reqFor(created.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: created.UpdatedAt,
		SetSections:       true,
		Sections: []tests.SectionInput{
			{Title: "Giữ lại", QuestionIDs: []string{q}},
			{Title: "Sẽ bị bỏ", QuestionIDs: []string{q}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	dropped := saved.Sections[1].ID

	after, err := svc.Update(ctx, reqFor(created.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: saved.UpdatedAt,
		SetSections:       true,
		Sections: []tests.SectionInput{
			{ID: saved.Sections[0].ID, Title: "Giữ lại", QuestionIDs: []string{q}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Sections) != 1 {
		t.Fatalf("%d sections after the drop, want 1", len(after.Sections))
	}

	var links int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.test_section_questions WHERE test_section_id = $1`,
		dropped).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if links != 0 {
		t.Errorf("%d question rows survived their section", links)
	}

	var stillThere int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.questions WHERE id = $1 AND deleted_at IS NULL`, q).Scan(&stillThere); err != nil {
		t.Fatal(err)
	}
	if stillThere != 1 {
		t.Error("dropping a section deleted the bank question it referenced")
	}
}

// totalPoints and questionCount are derived from the outline, so they have to
// move with it rather than being stored and drifting.
func TestTotalsFollowTheOutline(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	created, err := svc.Create(ctx, req(author), "Đề tính điểm", nil)
	if err != nil {
		t.Fatal(err)
	}
	if created.TotalPoints != "0" && created.TotalPoints != "0.00" {
		t.Errorf("a new test has totalPoints %q, want zero", created.TotalPoints)
	}

	q1 := newQuestion(t, pool, author, "Câu 2 điểm A")
	q2 := newQuestion(t, pool, author, "Câu 2 điểm B")

	saved, err := svc.Update(ctx, reqFor(created.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: created.UpdatedAt,
		SetSections:       true,
		Sections:          []tests.SectionInput{{Title: "Phần 1", QuestionIDs: []string{q1, q2}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.QuestionCount != 2 {
		t.Errorf("questionCount is %d, want 2", saved.QuestionCount)
	}
	if saved.TotalPoints != "4.00" {
		t.Errorf("totalPoints is %q, want 4.00", saved.TotalPoints)
	}
}

// The same question twice in one section would be indistinguishable to a
// student and would score twice.
func TestAQuestionCannotAppearTwiceInASection(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	created, err := svc.Create(ctx, req(author), "Đề trùng câu", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := newQuestion(t, pool, author, "Câu bị lặp")

	_, err = svc.Update(ctx, reqFor(created.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: created.UpdatedAt,
		SetSections:       true,
		Sections:          []tests.SectionInput{{Title: "Phần 1", QuestionIDs: []string{q, q}}},
	})
	var invalid *tests.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("got %v, want a ValidationError", err)
	}
}

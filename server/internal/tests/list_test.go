package tests_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/tests"
)

// A-03's "Thẻ" filters by the tags of the questions a test CONTAINS. §7 gives
// Test no tags of its own, so this is derived rather than stored -- which also
// means it needs no tag-editing surface, and none is drawn.
func TestTestsAreFilteredByTheirQuestionsTags(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	store := tests.NewStore(pool)
	ctx := context.Background()
	tag := "a03-" + strings.ReplaceAll(author, "-", "")[:10]

	tagged := newTaggedQuestion(t, pool, author, "Câu có thẻ", tag)
	plain := newQuestion(t, pool, author, "Câu không thẻ")

	withTag, err := svc.Create(ctx, req(author), "Đề có thẻ", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, reqFor(withTag.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: withTag.UpdatedAt, SetSections: true,
		Sections: []tests.SectionInput{{Title: "P1", QuestionIDs: []string{tagged}}},
	}); err != nil {
		t.Fatal(err)
	}

	without, err := svc.Create(ctx, req(author), "Đề không thẻ", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(ctx, reqFor(without.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: without.UpdatedAt, SetSections: true,
		Sections: []tests.SectionInput{{Title: "P1", QuestionIDs: []string{plain}}},
	}); err != nil {
		t.Fatal(err)
	}

	found, _, err := store.List(ctx, tests.ListInput{Tags: []string{tag}})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, x := range found {
		seen[x.ID] = true
	}
	if !seen[withTag.ID] {
		t.Error("the test containing the tagged question was not returned")
	}
	if seen[without.ID] {
		t.Error("a test with no matching question was returned")
	}

	// The rail must not offer a chip that returns nothing.
	tagList, err := store.Tags(ctx, tests.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(tagList, tag) {
		t.Errorf("the tag is filterable but not offered: %v", tagList)
	}
}

func newTaggedQuestion(t *testing.T, pool *pgxpool.Pool, author, prompt, tag string) string {
	t.Helper()
	id := newQuestion(t, pool, author, prompt)
	if _, err := pool.Exec(context.Background(),
		`UPDATE app.questions SET tags = ARRAY[$2::text] WHERE id = $1::uuid`, id, tag); err != nil {
		t.Fatal(err)
	}
	return id
}

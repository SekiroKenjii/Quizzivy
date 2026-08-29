package tests_test

import (
	"context"
	"testing"

	"quizzivy/internal/tests"
)

// A copy has not been published, so it must not inherit a history it never
// had -- nor a currentVersion that would let an assignment point at a snapshot
// its test never went through publish validation for.
func TestDuplicateCopiesTheDraftAndNoVersions(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	source, err := svc.Create(ctx, req(author), "Đề gốc", strptr("Mô tả gốc"))
	if err != nil {
		t.Fatal(err)
	}
	q1 := newQuestion(t, pool, author, "Câu một")
	q2 := newQuestion(t, pool, author, "Câu hai")

	source, err = svc.Update(ctx, reqFor(source.ID, author), tests.UpdateInput{
		ExpectedUpdatedAt: source.UpdatedAt,
		SetSections:       true,
		Sections: []tests.SectionInput{
			{Title: "Phần nghe", Instructions: strptr("Nghe kỹ"), QuestionIDs: []string{q1, q2}},
			{Title: "Phần đọc", QuestionIDs: []string{q2}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// The source has been published at some point: the copy must not inherit it.
	if _, err := pool.Exec(ctx,
		`UPDATE app.tests SET status = 'published', current_version = 3 WHERE id = $1`,
		source.ID); err != nil {
		t.Fatal(err)
	}

	copied, err := svc.Duplicate(ctx, reqFor(source.ID, author))
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}

	if copied.ID == source.ID {
		t.Fatal("duplicate returned the source")
	}
	if copied.Status != tests.Draft {
		t.Errorf("copy status is %q, want draft", copied.Status)
	}
	if copied.CurrentVersion != 0 {
		t.Errorf("copy currentVersion is %d, want 0", copied.CurrentVersion)
	}

	// The Done-when item: no version rows follow the copy. test_versions does
	// not exist until T-2.9, so this asserts the property the way it can be
	// asserted today and will keep meaning the same thing afterwards.
	var versionTables int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables
		  WHERE table_schema = 'app' AND table_name = 'test_versions'`).Scan(&versionTables); err != nil {
		t.Fatal(err)
	}
	if versionTables > 0 {
		var rows int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM app.test_versions WHERE test_id = $1`, copied.ID).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if rows != 0 {
			t.Errorf("the copy has %d version rows, want 0", rows)
		}
	}

	// The draft structure IS copied, with fresh ids.
	if len(copied.Sections) != 2 {
		t.Fatalf("copy has %d sections, want 2", len(copied.Sections))
	}
	if copied.Sections[0].Title != "Phần nghe" || copied.Sections[1].Title != "Phần đọc" {
		t.Errorf("section titles did not copy: %+v", copied.Sections)
	}
	if copied.Sections[0].Instructions == nil || *copied.Sections[0].Instructions != "Nghe kỹ" {
		t.Error("section instructions did not copy")
	}
	if len(copied.Sections[0].QuestionIDs) != 2 || copied.Sections[0].QuestionIDs[0] != q1 {
		t.Errorf("question order did not copy: %v", copied.Sections[0].QuestionIDs)
	}
	for i, sec := range copied.Sections {
		if sec.ID == source.Sections[i].ID {
			t.Errorf("section %d reused the source's id; editing the copy would edit the source", i)
		}
	}

	// The source is untouched.
	after, err := svc.Get(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.CurrentVersion != 3 || after.Status != tests.Published {
		t.Errorf("duplicating changed the source: status=%s version=%d", after.Status, after.CurrentVersion)
	}
}

func TestDuplicatingAMissingTestIsNotFound(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)

	_, err := svc.Duplicate(context.Background(),
		reqFor("00000000-0000-7000-8000-000000000000", author))
	if err == nil {
		t.Error("duplicating a missing test succeeded")
	}
}

func strptr(s string) *string { return &s }

package publish_test

import (
	"context"
	"testing"

	"quizzivy/internal/questions"
)

// §7's CORE INVARIANT. If this test is ever deleted the versioning is
// decorative: editing a published test would reach students mid-attempt and
// change what a finished attempt was scored against.
func TestEditingTheBankAfterPublishLeavesTheVersionUnchanged(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)
	ctx := context.Background()

	q := b.shortAnswer("Bản gốc của đề bài", "5.00")
	draft := b.draft("Đề bất biến", q)

	version, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Edit the bank question in every way the snapshot copies.
	if _, err := b.qsvc.Update(ctx, questions.WriteRequest{
		ID: q, ActorID: author,
		Input: questions.Input{
			Type: questions.ShortAnswer, Prompt: "ĐÃ SỬA SAU KHI XUẤT BẢN",
			Points: "99.00", Tags: []string{},
		},
	}); err != nil {
		t.Fatalf("editing the bank question: %v", err)
	}

	var prompt, points string
	if err := pool.QueryRow(ctx, `
		SELECT vq.prompt, vq.points::text
		  FROM app.test_version_questions vq
		  JOIN app.test_version_sections vs ON vs.id = vq.test_version_section_id
		 WHERE vs.test_version_id = $1`, version.ID).Scan(&prompt, &points); err != nil {
		t.Fatal(err)
	}

	if prompt != "Bản gốc của đề bài" {
		t.Errorf("the version's prompt is %q; editing the bank reached a published version", prompt)
	}
	if points != "5.00" {
		t.Errorf("the version's points are %q, want 5.00", points)
	}

	var total string
	if err := pool.QueryRow(ctx,
		`SELECT total_points::text FROM app.test_versions WHERE id = $1`, version.ID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != "5.00" {
		t.Errorf("the version's total_points is %q; the score denominator drifted", total)
	}
}

// The grading key has to survive exactly: an ordinal or an is_correct flag that
// shifted would score the wrong answer.
func TestTheSnapshotPreservesOptionOrdinalsAndCorrectness(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)
	ctx := context.Background()

	q := b.question(questions.Input{
		Type: questions.SingleChoice, Prompt: "Thủ đô của Việt Nam?", Points: "2.00",
		Options: []questions.OptionInput{
			{Text: "Huế", IsCorrect: false},
			{Text: "Hà Nội", IsCorrect: true},
			{Text: "Đà Nẵng", IsCorrect: false},
		},
	})
	draft := b.draft("Đề trắc nghiệm", q)

	version, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	rows, err := pool.Query(ctx, `
		SELECT o.ordinal, o.text, o.is_correct
		  FROM app.test_version_options o
		  JOIN app.test_version_questions vq ON vq.id = o.test_version_question_id
		  JOIN app.test_version_sections vs ON vs.id = vq.test_version_section_id
		 WHERE vs.test_version_id = $1
		 ORDER BY o.ordinal`, version.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type option struct {
		ordinal   int
		text      string
		isCorrect bool
	}
	var got []option
	for rows.Next() {
		var o option
		if err := rows.Scan(&o.ordinal, &o.text, &o.isCorrect); err != nil {
			t.Fatal(err)
		}
		got = append(got, o)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	want := []option{
		{0, "Huế", false},
		{1, "Hà Nội", true},
		{2, "Đà Nẵng", false},
	}
	if len(got) != len(want) {
		t.Fatalf("%d options in the snapshot, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("option %d is %+v, want %+v -- the grading key moved", i, got[i], want[i])
		}
	}
}

// fill_blank carries its accepted answers into the snapshot, since grading
// reads them from there and never from the bank.
func TestTheSnapshotCarriesBlanksAndAcceptedAnswers(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)
	ctx := context.Background()

	q := b.question(questions.Input{
		Type: questions.FillBlank, Prompt: "Tôi {{1}} đi học và {{2}} về nhà", Points: "3.00",
		Blanks: []questions.BlankInput{
			{Ordinal: 1, AcceptedAnswers: []string{"đi", "di"}},
			{Ordinal: 2, AcceptedAnswers: []string{"về"}, CaseSensitive: true},
		},
	})
	draft := b.draft("Đề điền khuyết", q)

	version, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	rows, err := pool.Query(ctx, `
		SELECT bl.ordinal, bl.case_sensitive, count(a.id)
		  FROM app.test_version_blanks bl
		  JOIN app.test_version_questions vq ON vq.id = bl.test_version_question_id
		  JOIN app.test_version_sections vs ON vs.id = vq.test_version_section_id
		  LEFT JOIN app.test_version_blank_answers a ON a.test_version_blank_id = bl.id
		 WHERE vs.test_version_id = $1
		 GROUP BY bl.ordinal, bl.case_sensitive
		 ORDER BY bl.ordinal`, version.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type blank struct {
		ordinal       int
		caseSensitive bool
		answers       int
	}
	var got []blank
	for rows.Next() {
		var bl blank
		if err := rows.Scan(&bl.ordinal, &bl.caseSensitive, &bl.answers); err != nil {
			t.Fatal(err)
		}
		got = append(got, bl)
	}
	want := []blank{{1, false, 2}, {2, true, 1}}
	if len(got) != len(want) {
		t.Fatalf("%d blanks in the snapshot, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("blank %d is %+v, want %+v", i, got[i], want[i])
		}
	}
}

// Versions are an append-only history of what was published and when, not a
// diff: an assignment names a version, so "nothing changed" still needs a row.
func TestRepublishingUnchangedStillCreatesAVersion(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)
	ctx := context.Background()

	q := b.shortAnswer("Không đổi gì cả", "1.00")
	draft := b.draft("Đề xuất bản hai lần", q)

	first, err := b.publish(draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.publish(draft.ID)
	if err != nil {
		t.Fatalf("republish: %v", err)
	}

	if second.Version != first.Version+1 {
		t.Errorf("second publish is version %d, want %d", second.Version, first.Version+1)
	}
	if second.ID == first.ID {
		t.Error("republishing returned the same version row")
	}

	var rows int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.test_versions WHERE test_id = $1`, draft.ID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Errorf("%d version rows after two publishes, want 2", rows)
	}

	// The test now points at the newer one.
	var current int
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT current_version, status::text FROM app.tests WHERE id = $1`, draft.ID).Scan(&current, &status); err != nil {
		t.Fatal(err)
	}
	if current != second.Version || status != "published" {
		t.Errorf("test is at version %d status %s, want %d published", current, status, second.Version)
	}
}

// The whole publish is one transaction, so a failure part-way must leave no
// half-written version behind.
func TestAFailedPublishLeavesNoVersionRow(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)
	ctx := context.Background()

	// A choice question with no correct option fails validation.
	q := b.question(questions.Input{
		Type: questions.SingleChoice, Prompt: "Không có đáp án đúng", Points: "1.00",
		Options: []questions.OptionInput{
			{Text: "A", IsCorrect: true},
			{Text: "B", IsCorrect: false},
		},
	})
	draft := b.draft("Đề sẽ hỏng", q)
	if _, err := pool.Exec(ctx,
		`UPDATE app.question_options SET is_correct = false WHERE question_id = $1`, q); err != nil {
		t.Fatal(err)
	}

	if _, err := b.publish(draft.ID); err == nil {
		t.Fatal("publishing an invalid test succeeded")
	}

	var versions, sections int
	if err := pool.QueryRow(ctx,
		`SELECT (SELECT count(*) FROM app.test_versions WHERE test_id = $1),
		        (SELECT count(*) FROM app.test_version_sections vs
		           JOIN app.test_versions v ON v.id = vs.test_version_id
		          WHERE v.test_id = $1)`, draft.ID).Scan(&versions, &sections); err != nil {
		t.Fatal(err)
	}
	if versions != 0 || sections != 0 {
		t.Errorf("a failed publish left %d versions and %d sections", versions, sections)
	}

	var status string
	var current int
	if err := pool.QueryRow(ctx,
		`SELECT status::text, current_version FROM app.tests WHERE id = $1`, draft.ID).Scan(&status, &current); err != nil {
		t.Fatal(err)
	}
	if status != "draft" || current != 0 {
		t.Errorf("a failed publish left the test at %s version %d", status, current)
	}
}

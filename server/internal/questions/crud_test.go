package questions_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/questions"
)

// TestSoftDeleteLeavesTheQuestionResolvableByID is §13.2's whole point.
//
// A published version references bank rows by id, and those rows can be deleted
// afterwards. If a delete removed the row -- or made it unresolvable -- a
// student would open a published test and find a question missing. Deleted
// means "out of the bank", not "gone".
func TestSoftDeleteLeavesTheQuestionResolvableByID(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	q := write(t, svc, author, "Câu hỏi sắp bị xoá")

	if err := svc.Delete(ctx, questions.WriteRequest{ID: q.ID, ActorID: author}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Absent from the bank.
	listed, _, err := svc.List(ctx, questions.ListInput{Limit: questions.MaxLimit})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range listed {
		if item.ID == q.ID {
			t.Error("a soft-deleted question is still listed in the bank")
		}
	}

	// And not resolvable by the normal path either.
	if _, err := svc.Get(ctx, q.ID); !errors.Is(err, questions.ErrNotFound) {
		t.Errorf("Get on a deleted question returned %v, want ErrNotFound", err)
	}

	// But still resolvable for a version snapshot, which is the property that
	// keeps a published test working.
	revived, err := svc.GetIncludingDeleted(ctx, q.ID)
	if err != nil {
		t.Fatalf("a deleted question must stay resolvable for version snapshots: %v", err)
	}
	if revived.Prompt != q.Prompt {
		t.Errorf("resolved prompt = %q, want %q", revived.Prompt, q.Prompt)
	}

	// The row survives, with deleted_at set.
	var deletedAt *string
	if err := pool.QueryRow(ctx,
		`SELECT deleted_at::text FROM app.questions WHERE id = $1`, q.ID).Scan(&deletedAt); err != nil {
		t.Fatalf("the row was hard-deleted: %v", err)
	}
	if deletedAt == nil {
		t.Error("deleted_at is still null")
	}

	var audited int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.audit_log WHERE action = 'question.deleted' AND entity_id = $1`,
		q.ID).Scan(&audited); err != nil {
		t.Fatal(err)
	}
	if audited != 1 {
		t.Errorf("audit rows for the delete: %d, want 1", audited)
	}
}

func TestDeletingTwiceIsNotFound(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	q := write(t, svc, author, "Xoá hai lần")
	if err := svc.Delete(ctx, questions.WriteRequest{ID: q.ID, ActorID: author}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, questions.WriteRequest{ID: q.ID, ActorID: author}); !errors.Is(err, questions.ErrNotFound) {
		t.Errorf("second delete returned %v, want ErrNotFound", err)
	}
}

// choiceInput builds a single_choice question whose options are in the given
// order, with the first one correct.
func choiceInput(prompt string, texts ...string) questions.Input {
	in := questions.Input{
		Type: questions.SingleChoice, Prompt: prompt, Points: "2.50", Tags: []string{},
	}
	for i, text := range texts {
		in.Options = append(in.Options, questions.OptionInput{Text: text, IsCorrect: i == 0})
	}
	return in
}

// TestReorderingOptionsRoundTrips: array position is the ordinal, and a reorder
// has to survive the write.
//
// Ordinals are normalised dense 0..n-1 rather than taken from the client, so a
// reorder cannot leave a gap -- and a gap matters, because shuffleOptions and
// the grading key both address options by position.
func TestReorderingOptionsRoundTrips(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	created, err := svc.Create(ctx, questions.WriteRequest{
		Input:   choiceInput("Thủ đô của Việt Nam là gì?", "Hà Nội", "Huế", "Đà Nẵng"),
		ActorID: author,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	assertOptionOrder(t, created, []string{"Hà Nội", "Huế", "Đà Nẵng"}, []bool{true, false, false})

	// Drag the correct answer to the end.
	reordered := choiceInput("Thủ đô của Việt Nam là gì?", "Huế", "Đà Nẵng", "Hà Nội")
	reordered.Options = []questions.OptionInput{
		{Text: "Huế", IsCorrect: false},
		{Text: "Đà Nẵng", IsCorrect: false},
		{Text: "Hà Nội", IsCorrect: true},
	}
	updated, err := svc.Update(ctx, questions.WriteRequest{
		ID: created.ID, Input: reordered, ActorID: author,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	assertOptionOrder(t, updated, []string{"Huế", "Đà Nẵng", "Hà Nội"}, []bool{false, false, true})

	// And re-read, so the assertion is about what was STORED rather than what
	// the write happened to return.
	reread, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertOptionOrder(t, reread, []string{"Huế", "Đà Nẵng", "Hà Nội"}, []bool{false, false, true})

	// No orphans: replacing options must not leave the old rows behind.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.question_options WHERE question_id = $1`, created.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("%d option rows after a reorder, want 3", count)
	}
}

func assertOptionOrder(t *testing.T, q questions.Question, texts []string, correct []bool) {
	t.Helper()
	if len(q.Options) != len(texts) {
		t.Fatalf("%d options, want %d", len(q.Options), len(texts))
	}
	for i := range texts {
		if q.Options[i].Ordinal != i {
			t.Errorf("option %d has ordinal %d; ordinals must be dense 0..n-1", i, q.Options[i].Ordinal)
		}
		if q.Options[i].Text != texts[i] {
			t.Errorf("option %d is %q, want %q", i, q.Options[i].Text, texts[i])
		}
		if q.Options[i].IsCorrect != correct[i] {
			t.Errorf("option %d isCorrect = %v, want %v -- the grading key followed the wrong row",
				i, q.Options[i].IsCorrect, correct[i])
		}
	}
}

// TestBlanksAndAnswersRoundTrip covers the other child table, including the
// deduplication that stops a repeated answer aborting the whole save.
func TestBlanksAndAnswersRoundTrip(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	in := questions.Input{
		Type: questions.FillBlank, Prompt: "Tôi {{1}} đi học và {{2}} về nhà",
		Points: "3.00", Tags: []string{},
		Blanks: []questions.BlankInput{
			{Ordinal: 1, AcceptedAnswers: []string{"đi", "di", "đi"}}, // repeated on purpose
			{Ordinal: 2, AcceptedAnswers: []string{"về"}, CaseSensitive: true},
		},
	}
	q, err := svc.Create(ctx, questions.WriteRequest{Input: in, ActorID: author})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(q.Blanks) != 2 {
		t.Fatalf("%d blanks, want 2", len(q.Blanks))
	}
	if q.Blanks[0].Ordinal != 1 || q.Blanks[1].Ordinal != 2 {
		t.Errorf("ordinals %d,%d -- blanks are 1-indexed to match {{1}}, {{2}}",
			q.Blanks[0].Ordinal, q.Blanks[1].Ordinal)
	}
	if len(q.Blanks[0].AcceptedAnswers) != 2 {
		t.Errorf("blank 1 has %v; the repeated answer should have been deduplicated, not rejected",
			q.Blanks[0].AcceptedAnswers)
	}
	if !q.Blanks[1].CaseSensitive {
		t.Error("caseSensitive did not round-trip")
	}
}

// TestUpdateReplacesChildrenAtomically: the editor sends the whole question, so
// a failed write must not leave the new prompt beside the old options -- a
// question that renders one thing and grades another.
func TestUpdateRejectedByTheDatabaseLeavesTheOldVersion(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	svc := newService(t, pool)
	ctx := context.Background()

	created, err := svc.Create(ctx, questions.WriteRequest{
		Input:   choiceInput("Câu gốc", "A", "B"),
		ActorID: author,
	})
	if err != nil {
		t.Fatal(err)
	}

	// points beyond numeric(8,2)'s CHECK: accepted by validation, refused by
	// the database, so the failure lands mid-transaction after the children
	// have already been deleted.
	bad := choiceInput("Câu đã sửa", "C", "D")
	bad.Points = "99999999.00"
	if _, err := svc.Update(ctx, questions.WriteRequest{
		ID: created.ID, Input: bad, ActorID: author,
	}); err == nil {
		t.Fatal("an out-of-range points value was accepted")
	}

	after, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("the question vanished after a failed update: %v", err)
	}
	if after.Prompt != "Câu gốc" {
		t.Errorf("prompt = %q, want the original -- the failed update was not rolled back", after.Prompt)
	}
	assertOptionOrder(t, after, []string{"A", "B"}, []bool{true, false})
}

// TestDraftReferenceCheckIsWiredOnceTestTablesExist is a tripwire, the same
// shape as media's.
//
// CountDraftReferences returns a constant because test_section_questions does
// not exist until migration 00015 (T-2.7). That constant is correct today and
// becomes a bug the moment the table lands: DELETE would stop returning 409 and
// silently break a teacher's draft outline.
func TestDraftReferenceCheckIsWiredOnceTestTablesExist(t *testing.T) {
	pool := newPool(t)

	var exists bool
	err := pool.QueryRow(context.Background(),
		`SELECT EXISTS (
		    SELECT 1 FROM information_schema.tables
		     WHERE table_schema = 'app' AND table_name = 'test_section_questions')`).Scan(&exists)
	if err != nil {
		t.Fatalf("checking for the draft outline table: %v", err)
	}
	if exists {
		t.Fatal(
			"app.test_section_questions now exists, so questions.CountDraftReferences must stop " +
				"returning a constant:\n" +
				"  SELECT count(*) FROM app.test_section_questions WHERE question_id = $1\n" +
				"so DELETE /admin/questions/:id returns 409 for a question a draft still uses (§15).\n" +
				"Then delete this test and replace it with real coverage.")
	}
}

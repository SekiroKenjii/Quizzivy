//go:build integration

package adapters_test

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/internal/core/adapters"
	importsdomain "quizzivy/internal/modules/imports/domain"
	importsrepo "quizzivy/internal/modules/imports/repositories"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionsapp "quizzivy/internal/modules/questions/application"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	testsapp "quizzivy/internal/modules/tests/application"
	testsrepo "quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/actor"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type commitHarness struct {
	pool      *pgxpool.Pool
	imports   *importsrepo.Postgres
	committer adapters.ImportCommitter
	by        actor.Actor
}

func commitSetup(t *testing.T) commitHarness {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var id string
	if err := pool.QueryRow(ctx, `INSERT INTO app.users(email,full_name,role) VALUES($1,'Commit teacher','admin') RETURNING id::text`, uuid.NewString()+"@example.test").Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupCommits(t, pool, id) })
	dbx := db.NewContext(pool)
	committer := adapters.ImportCommitter{
		DB: dbx,
		Tests: func(scoped db.Context) *testsapp.Application {
			questions, media := questionsrepo.NewPostgres(scoped), mediarepo.NewPostgres(scoped)
			return testsapp.New(testsrepo.NewPostgres(scoped, questions, media).WithGroupQuestions(adapters.GroupQuestions{})).WithGroups(testsrepo.NewGroupsPostgres(scoped, adapters.GroupQuestions{}, media), adapters.MediaKinds{})
		},
		Questions: func(scoped db.Context) *questionsapp.Application {
			return questionsapp.New(questionsrepo.NewPostgres(scoped), adapters.MediaKinds{})
		},
	}
	return commitHarness{pool: pool, imports: importsrepo.NewPostgres(dbx), committer: committer, by: actor.Actor{ID: id}}
}

func cleanupCommits(t *testing.T, pool *pgxpool.Pool, userID string) {
	ctx := context.Background()
	for _, sql := range []string{
		`DELETE FROM app.test_section_units WHERE test_section_id IN (SELECT s.id FROM app.test_sections s JOIN app.tests x ON x.id=s.test_id WHERE x.created_by=$1)`,
		`DELETE FROM app.test_section_questions WHERE test_section_id IN (SELECT s.id FROM app.test_sections s JOIN app.tests x ON x.id=s.test_id WHERE x.created_by=$1)`,
		`DELETE FROM app.group_gap_bindings WHERE stimulus_id IN (SELECT m.id FROM app.group_stimuli m JOIN app.question_groups g ON g.id=m.group_id WHERE g.created_by=$1)`,
		`DELETE FROM app.group_stimuli WHERE group_id IN (SELECT id FROM app.question_groups WHERE created_by=$1)`,
		`DELETE FROM app.question_options WHERE question_id IN (SELECT id FROM app.questions WHERE created_by=$1)`,
		`DELETE FROM app.questions WHERE created_by=$1`,
		`DELETE FROM app.question_groups WHERE created_by=$1`,
		`DELETE FROM app.test_sections WHERE test_id IN (SELECT id FROM app.tests WHERE created_by=$1)`,
		`DELETE FROM app.tests WHERE created_by=$1`,
		`DELETE FROM app.word_import_commits WHERE committed_by=$1`,
		`DELETE FROM app.word_import_drafts WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
		`UPDATE app.word_imports SET source_revision=NULL WHERE created_by=$1`,
		`DELETE FROM app.word_import_run_events WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
		`DELETE FROM app.word_import_runs WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
		`DELETE FROM app.word_import_source_set_items WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
		`DELETE FROM app.word_import_sources WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
		`DELETE FROM app.word_import_source_sets WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
		`DELETE FROM app.word_imports WHERE created_by=$1`,
	} {
		if _, err := pool.Exec(ctx, sql, userID); err != nil {
			t.Errorf("cleanup %q: %v", sql, err)
		}
	}
}

func prose(text string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"format": "semantic_v1", "blocks": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text, "marks": []string{}}}}}})
	return raw
}

func choice(id, prompt, correct string) importsdomain.DraftQuestion {
	q := importsdomain.DraftQuestion{ID: id, Label: id, Type: "single_choice", Prompt: prose(prompt), Points: "1", Blanks: []importsdomain.DraftBlank{}, Source: []importsdomain.SourceRef{},
		Origins: importsdomain.Origins{Type: importsdomain.InferredStructure, Prompt: importsdomain.SourceExplicit, Options: importsdomain.SourceExplicit, Answer: importsdomain.SourceExplicit, Points: importsdomain.TeacherEntered}}
	for _, label := range []string{"A", "B"} {
		q.Options = append(q.Options, importsdomain.DraftOption{ID: id + label, Label: label, Content: prose(prompt + " " + label)})
	}
	q.Answer = importsdomain.DraftAnswer{State: importsdomain.AnswerKnown, OptionIDs: []string{id + correct}, Evidence: []importsdomain.SourceRef{}}
	return q
}

func reviewedDraft() importsdomain.Draft {
	standalone := choice("q1", "Which city is larger?", "B")
	group := importsdomain.DraftGroup{ID: "g1", Stimulus: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Tet is ","marks":[]},{"type":"gap","id":"gap-2","label":"2"}]}]}`),
		Gaps: []importsdomain.GapLink{{GapID: "gap-2", QuestionID: "q2"}}, Questions: []importsdomain.DraftQuestion{choice("q2", "(2)", "A")}, Source: []importsdomain.SourceRef{}}
	return importsdomain.Draft{Version: importsdomain.DraftVersion, Title: "TEST 9", Notices: []importsdomain.Finding{}, Acknowledged: []string{},
		Sections: []importsdomain.DraftSection{{ID: "s1", Title: "I. Choose", Origin: importsdomain.SourceExplicit, Source: []importsdomain.SourceRef{},
			Items: []importsdomain.DraftItem{{Question: &standalone}, {Group: &group}}}}}
}

func (h commitHarness) underReview(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	quotas := importsdomain.DefaultQuotas()
	quotas.GlobalImports = 100000
	created, err := h.imports.Create(ctx, importsdomain.Create{RequestID: uuid.NewString(), Title: "Đề nhập", Actor: h.by}, quotas)
	if err != nil {
		t.Fatal(err)
	}
	digest := make([]byte, 32)
	copy(digest, uuid.New().String())
	source, err := h.imports.Reserve(ctx, importsdomain.Reserve{Actor: h.by, Source: importsdomain.Source{ImportID: created.ID, UploadID: uuid.NewString(), ExpectedRevision: created.Revision, Role: "exam", Filename: "de.docx", Format: "docx", Bytes: 10, SHA256: digest}}, quotas)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := h.imports.Finish(ctx, importsdomain.Finish{ImportID: created.ID, SourceID: source.ID, Actor: h.by})
	if err != nil {
		t.Fatal(err)
	}
	version := uuid.NewString()
	if _, err := h.imports.Schedule(ctx, importsdomain.Schedule{ImportID: created.ID, RequestID: uuid.NewString(), PipelineVersion: version, ExpectedRevision: receipt.Import.Revision, SourceRevision: receipt.Import.SourceRevision, Actor: h.by, MaxAttempts: 1}); err != nil {
		t.Fatal(err)
	}
	run, err := h.imports.Claim(ctx, importsdomain.ClaimPolicy{WorkerID: uuid.NewString(), PipelineVersion: version, Lease: 30e9, GlobalLimit: 100, ActorLimit: 100})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(reviewedDraft())
	if err := h.imports.Complete(ctx, run.Claim(), importsdomain.Outcome{Result: json.RawMessage(`{"schemaVersion":1}`), Draft: body}); err != nil {
		t.Fatal(err)
	}
	return created.ID
}

func (h commitHarness) record(importID, requestID string) func(context.Context, importsdomain.CommitStore, string) error {
	return func(ctx context.Context, store importsdomain.CommitStore, testID string) error {
		return store.RecordCommit(ctx, importsdomain.CommitRecord{Commit: importsdomain.Commit{ImportID: importID, RequestID: requestID, DraftRevision: 1, Digest: make([]byte, 32), TestID: &testID}, Actor: h.by})
	}
}

func (h commitHarness) testsCreated(t *testing.T) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.tests WHERE created_by=$1`, h.by.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func plan(t *testing.T) importsdomain.CommitPlan {
	t.Helper()
	p, err := importsdomain.Plan(reviewedDraft(), "fallback", uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCommitCreatesOneTestWithItsSectionsGroupsAndBankQuestionsInOrder(t *testing.T) {
	h := commitSetup(t)
	ctx := context.Background()
	importID := h.underReview(t)
	testID, err := h.committer.Materialize(ctx, plan(t), h.by, h.record(importID, uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	var title string
	var units []string
	if err := h.pool.QueryRow(ctx, `SELECT t.title, ARRAY(SELECT CASE WHEN u.group_id IS NULL THEN 'question' ELSE 'group' END FROM app.test_section_units u WHERE u.test_section_id=s.id ORDER BY u.ordinal)
 FROM app.tests t JOIN app.test_sections s ON s.test_id=t.id WHERE t.id=$1`, testID).Scan(&title, &units); err != nil {
		t.Fatal(err)
	}
	if title != "TEST 9" || len(units) != 2 || units[0] != "question" || units[1] != "group" {
		t.Fatalf("test %q units %v", title, units)
	}
	var members, correct int
	if err := h.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM app.questions q JOIN app.question_groups g ON g.id=q.context_group_id JOIN app.test_sections s ON s.id=g.owner_section_id WHERE s.test_id=$1),
 (SELECT count(*) FROM app.question_options o JOIN app.questions q ON q.id=o.question_id WHERE q.created_by=$2 AND o.is_correct)`, testID, h.by.ID).Scan(&members, &correct); err != nil {
		t.Fatal(err)
	}
	if members != 1 || correct != 2 {
		t.Fatalf("group members %d correct options %d", members, correct)
	}
	current, err := h.imports.Get(ctx, importID)
	if err != nil || current.Status != "committed" || current.TestID == nil || *current.TestID != testID {
		t.Fatalf("import %+v err %v", current, err)
	}
}

func TestAFailedCommitLeavesNoTestBehind(t *testing.T) {
	h := commitSetup(t)
	importID := h.underReview(t)
	refused := errors.New("record refused")
	_, err := h.committer.Materialize(context.Background(), plan(t), h.by, func(context.Context, importsdomain.CommitStore, string) error { return refused })
	if !errors.Is(err, refused) || h.testsCreated(t) != 0 {
		t.Fatalf("err %v tests %d", err, h.testsCreated(t))
	}
	if current, err := h.imports.Get(context.Background(), importID); err != nil || current.Status != "needs_review" {
		t.Fatalf("import %+v err %v", current, err)
	}
}

func TestConcurrentCommitsProduceExactlyOneTest(t *testing.T) {
	h := commitSetup(t)
	importID := h.underReview(t)
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = h.committer.Materialize(context.Background(), plan(t), h.by, h.record(importID, uuid.NewString()))
		}()
	}
	wg.Wait()
	succeeded := 0
	for _, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case !errors.Is(err, importsdomain.ErrConflict):
			t.Fatalf("unexpected error %v", err)
		}
	}
	if succeeded != 1 || h.testsCreated(t) != 1 {
		t.Fatalf("succeeded %d tests %d", succeeded, h.testsCreated(t))
	}
}

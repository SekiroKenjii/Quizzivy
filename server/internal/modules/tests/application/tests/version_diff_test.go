//go:build integration

package application_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"quizzivy/internal/core/adapters"
	questionscommand "quizzivy/internal/modules/questions/application/command"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/repositories"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func diffAuthor(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	t.Cleanup(func() {
		if id == "" {
			return
		}
		for _, statement := range []string{
			`DELETE FROM app.audit_log WHERE actor_user_id = $1`,
			`DELETE FROM app.test_versions WHERE published_by = $1`,
			`DELETE FROM app.tests WHERE created_by = $1`,
			`DELETE FROM app.questions WHERE created_by = $1`,
			`DELETE FROM app.users WHERE id = $1`,
		} {
			if _, err := pool.Exec(context.Background(), statement, id); err != nil {
				t.Errorf("cleanup %q: %v", statement, err)
			}
		}
	})
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO app.users (email, full_name, role_id)
		 VALUES ($1,'Giáo viên',(SELECT id FROM app.roles WHERE builtin_key = 'teacher')) RETURNING id::text`,
		"diff-"+hex.EncodeToString(nonce)+"@example.com").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

type sqlLog struct {
	mu   sync.Mutex
	sql  []string
	hook func(string)
}

func (l *sqlLog) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	l.mu.Lock()
	l.sql = append(l.sql, data.SQL)
	hook := l.hook
	l.mu.Unlock()
	if hook != nil {
		hook(data.SQL)
	}
	return ctx
}

func (l *sqlLog) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (l *sqlLog) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sql, l.hook = nil, nil
}

func (l *sqlLog) statements() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.sql)
}

func loggedPool(t *testing.T) (*pgxpool.Pool, *sqlLog) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	log := &sqlLog{}
	cfg.ConnConfig.Tracer = log
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, log
}

func diffKinds(changes []domain.Change) []domain.ChangeKind {
	out := make([]domain.ChangeKind, len(changes))
	for i, c := range changes {
		out[i] = c.Kind
	}
	return out
}

func (b *builder) get(testID string) domain.Test {
	b.t.Helper()
	test, err := b.tests.Queries.Get.Handle(context.Background(), query.Get{ID: testID, Scope: access.Scope{UserID: b.author}})
	if err != nil {
		b.t.Fatalf("get %s: %v", testID, err)
	}
	return test
}

func (b *builder) diff(testID string, version int, against domain.Against) (query.DiffResult, error) {
	return b.tests.Queries.Diff.Handle(context.Background(), query.Diff{TestID: testID, Version: version, Against: against, Scope: access.Scope{UserID: b.author}})
}

func (b *builder) mustDiff(testID string, version int, against domain.Against) query.DiffResult {
	b.t.Helper()
	result, err := b.diff(testID, version, against)
	if err != nil {
		b.t.Fatalf("diff of version %d: %v", version, err)
	}
	return result
}

func (b *builder) mustPublish(testID string) domain.Version {
	b.t.Helper()
	version, err := b.publish(testID)
	if err != nil {
		b.t.Fatalf("publish: %v", err)
	}
	return version
}

func (b *builder) editQuestion(id, prompt, points string) {
	b.t.Helper()
	_, err := b.qsvc.Commands.Update.Handle(context.Background(), questionscommand.Update{Request: questionsdomain.WriteRequest{
		ID: id, ActorID: b.author,
		Input: questionsdomain.Input{Type: questionsdomain.ShortAnswer, Prompt: prompt, Points: points, Tags: []string{}},
	}})
	if err != nil {
		b.t.Fatalf("edit question %s: %v", id, err)
	}
}

func (b *builder) setOutline(test domain.Test, sections ...domain.SectionInput) domain.Test {
	b.t.Helper()
	saved, err := b.tests.Commands.Update.Handle(context.Background(), command.Update{
		Request: reqFor(test.ID, b.author),
		Input:   domain.UpdateInput{ExpectedUpdatedAt: test.UpdatedAt, SetSections: true, Sections: sections},
	})
	if err != nil {
		b.t.Fatalf("set outline: %v", err)
	}
	return saved
}

var onVersion = func(n int) domain.Against { return domain.Against{Kind: domain.AgainstVersion, Version: n} }

var againstDraft = domain.Against{Kind: domain.AgainstDraft}

var againstPrevious = domain.Against{Kind: domain.AgainstPrevious}

func TestABankEditMakesTheDraftDifferAndMovesNothingOnTheTest(t *testing.T) {
	pool := newPool(t)
	b := newBuilder(t, pool, diffAuthor(t, pool))
	one, two := b.shortAnswer("Câu một", "1.00"), b.shortAnswer("Câu hai", "2.00")
	draft := b.draft("Đề so sánh", one, two)

	if got := b.get(draft.ID).UnpublishedChanges; got != nil {
		t.Fatalf("a test never published counts %d, want null", *got)
	}
	b.mustPublish(draft.ID)
	published := b.get(draft.ID)
	if published.UnpublishedChanges == nil || *published.UnpublishedChanges != 0 {
		t.Fatalf("a draft just published counts %v, want 0", published.UnpublishedChanges)
	}

	b.editQuestion(one, "Câu một đã sửa", "1.00")
	edited := b.get(draft.ID)
	if edited.UnpublishedChanges == nil || *edited.UnpublishedChanges != 1 {
		t.Fatalf("after a bank edit the draft counts %v, want 1", edited.UnpublishedChanges)
	}
	if !edited.UpdatedAt.Equal(published.UpdatedAt) {
		t.Errorf("a bank edit moved the test's updatedAt from %v to %v; the count must not rely on it", published.UpdatedAt, edited.UpdatedAt)
	}

	result := b.mustDiff(draft.ID, 1, againstDraft)
	if result.From == nil || result.From.Draft || result.From.Version != 1 || !result.To.Draft {
		t.Fatalf("sides = %+v and %+v, want version 1 and the draft", result.From, result.To)
	}
	if len(result.Changes) != 1 {
		t.Fatalf("changes = %+v, want one", result.Changes)
	}
	changed := result.Changes[0]
	if changed.Kind != domain.ChangeChanged || changed.Number != 1 || changed.QuestionID != one || !reflect.DeepEqual(changed.Fields, []domain.ChangedField{domain.FieldPrompt}) {
		t.Errorf("change = %+v, want question 1 changed in its prompt, named by its bank id", changed)
	}

	b.mustPublish(draft.ID)
	if again := b.get(draft.ID).UnpublishedChanges; again == nil || *again != 0 {
		t.Errorf("after publishing the edit the draft counts %v, want 0", again)
	}
}

func TestTheCountIsNullInAListRowAndInAWriteAnswerAndAnAnswerOtherwise(t *testing.T) {
	pool := newPool(t)
	author := diffAuthor(t, pool)
	b := newBuilder(t, pool, author)
	question := b.shortAnswer("Câu hỏi", "1.00")
	draft := b.draft("Đề có thay đổi", question)
	b.mustPublish(draft.ID)
	b.editQuestion(question, "Câu hỏi đã sửa", "1.00")
	if got := b.get(draft.ID).UnpublishedChanges; got == nil || *got != 1 {
		t.Fatalf("getTest counts %v, want 1", got)
	}

	listed, err := b.tests.Queries.List.Handle(context.Background(), query.List{Input: domain.ListInput{Scope: access.Scope{UserID: author}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].UnpublishedChanges != nil {
		t.Errorf("list rows = %+v, want one with a null count", listed.Items)
	}

	title := "Đổi tên"
	current := b.get(draft.ID)
	written, err := b.tests.Commands.Update.Handle(context.Background(), command.Update{
		Request: reqFor(draft.ID, author), Input: domain.UpdateInput{ExpectedUpdatedAt: current.UpdatedAt, Title: &title},
	})
	if err != nil {
		t.Fatal(err)
	}
	if written.UnpublishedChanges != nil {
		t.Errorf("a write answered a count of %d, want null", *written.UnpublishedChanges)
	}
	copied, err := b.tests.Commands.Duplicate.Handle(context.Background(), command.Duplicate{Request: reqFor(draft.ID, author)})
	if err != nil {
		t.Fatal(err)
	}
	if copied.UnpublishedChanges != nil {
		t.Errorf("a duplicate answered a count of %d, want null", *copied.UnpublishedChanges)
	}
}

func TestTheDiffReadsTheSidesTheContractNames(t *testing.T) {
	pool := newPool(t)
	b := newBuilder(t, pool, diffAuthor(t, pool))
	one, two := b.shortAnswer("Câu một", "1.00"), b.shortAnswer("Câu hai", "2.00")
	draft := b.draft("Đề hai phiên bản", one, two)
	first := b.mustPublish(draft.ID)

	introduced := b.mustDiff(draft.ID, 1, againstPrevious)
	if introduced.From != nil || introduced.To.Version != 1 || !introduced.To.PublishedAt.Equal(first.PublishedAt) {
		t.Fatalf("sides of the first version = %+v and %+v, want nothing before it", introduced.From, introduced.To)
	}
	if !slices.Equal(diffKinds(introduced.Changes), []domain.ChangeKind{domain.ChangeAdded, domain.ChangeAdded}) {
		t.Errorf("the first version's changes = %v, want both questions added and no total", diffKinds(introduced.Changes))
	}

	three := b.shortAnswer("Câu ba", "3.00")
	b.editQuestion(two, "Câu hai", "4.00")
	b.setOutline(b.get(draft.ID), domain.SectionInput{Title: "Phần 1", QuestionIDs: []string{one, two, three}})
	second := b.mustPublish(draft.ID)

	wantKinds := []domain.ChangeKind{domain.ChangeAdded, domain.ChangeChanged, domain.ChangePoints}
	for name, got := range map[string]query.DiffResult{
		"version 2 against the previous": b.mustDiff(draft.ID, 2, againstPrevious),
		"version 2 against 1":            b.mustDiff(draft.ID, 2, onVersion(1)),
		"version 1 against 2":            b.mustDiff(draft.ID, 1, onVersion(2)),
	} {
		if got.From == nil || got.From.Version != 1 || got.To.Version != 2 || !got.To.PublishedAt.Equal(second.PublishedAt) {
			t.Errorf("%s: sides = %+v and %+v, want 1 then 2", name, got.From, got.To)
		}
		if !slices.Equal(diffKinds(got.Changes), wantKinds) {
			t.Errorf("%s: kinds = %v, want %v", name, diffKinds(got.Changes), wantKinds)
		}
	}
	changes := b.mustDiff(draft.ID, 2, againstPrevious).Changes
	if changes[0].QuestionID == three || changes[0].Number != 3 {
		t.Errorf("added = %+v, want question 3 of version 2 named by its frozen row, not its bank id", changes[0])
	}
	if changes[2].PointsFrom != "3.00" || changes[2].PointsTo != "8.00" {
		t.Errorf("total = %+v, want 3.00 to 8.00", changes[2])
	}

	if flat := b.mustDiff(draft.ID, 2, againstDraft); flat.From.Version != 2 || !flat.To.Draft || len(flat.Changes) != 0 {
		t.Errorf("version 2 against the draft = %+v, want no change: the draft is version 2", flat)
	}
	b.setOutline(b.get(draft.ID), domain.SectionInput{Title: "Phần 1", QuestionIDs: []string{one, three}})
	removed := b.mustDiff(draft.ID, 2, againstDraft)
	if !slices.Equal(diffKinds(removed.Changes), []domain.ChangeKind{domain.ChangeRemoved, domain.ChangePoints}) || removed.Changes[0].Number != 2 {
		t.Errorf("after dropping question 2: %+v, want it removed (numbered in version 2) and the total down", removed.Changes)
	}
}

func TestTheDiffRefusesWhatIsNotThereAndWhatIsNotTheCallers(t *testing.T) {
	pool := newPool(t)
	b := newBuilder(t, pool, diffAuthor(t, pool))
	draft := b.draft("Đề của tôi", b.shortAnswer("Câu hỏi", "1.00"))
	b.mustPublish(draft.ID)

	for name, c := range map[string]struct {
		version int
		against domain.Against
		want    error
	}{
		"a missing version":          {9, againstPrevious, domain.ErrNotFound},
		"a missing target":           {1, onVersion(9), domain.ErrNotFound},
		"the version against itself": {1, onVersion(1), domain.ErrSameVersion},
	} {
		if _, err := b.diff(draft.ID, c.version, c.against); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}

	stranger := access.Scope{UserID: diffAuthor(t, pool)}
	foreign := query.Diff{TestID: draft.ID, Version: 1, Against: againstDraft, Scope: stranger}
	if _, err := b.tests.Queries.Diff.Handle(context.Background(), foreign); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("another teacher's test: err = %v, want not found", err)
	}
	foreign.Scope = access.Scope{All: true}
	if _, err := b.tests.Queries.Diff.Handle(context.Background(), foreign); err != nil {
		t.Errorf("an admin's scope.all: err = %v, want a result", err)
	}
	foreign.TestID = "00000000-0000-7000-8000-000000000000"
	if _, err := b.tests.Queries.Diff.Handle(context.Background(), foreign); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("an unknown test: err = %v, want not found", err)
	}
	if _, err := b.tests.Queries.Diff.Handle(context.Background(), query.Diff{TestID: draft.ID, Version: 0, Against: againstDraft, Scope: access.Scope{UserID: b.author}}); err != nil {
		t.Errorf("the latest version against the draft: %v", err)
	}
}

var lockClause = regexp.MustCompile(`(?i)\bfor\s+(no\s+key\s+update|update|share|key\s+share)\b`)

var writeStatement = regexp.MustCompile(`(?i)^\s*(insert|update|delete|copy|truncate|alter|create|drop)\b`)

func TestTheDiffWritesNothingLocksNothingAndReadsOneSnapshot(t *testing.T) {
	pool, log := loggedPool(t)
	b := newBuilder(t, pool, diffAuthor(t, pool))
	one, two := b.shortAnswer("Câu một", "1.00"), b.shortAnswer("Câu hai", "2.00")
	draft := b.draft("Đề chỉ đọc", one, two)
	b.mustPublish(draft.ID)
	b.editQuestion(one, "Câu một đã sửa", "1.00")

	type rowFacts struct {
		xmin, xmax string
		updatedAt  time.Time
	}
	facts := func() rowFacts {
		var f rowFacts
		if err := pool.QueryRow(context.Background(), `SELECT xmin::text, xmax::text, updated_at FROM app.tests WHERE id = $1`, draft.ID).Scan(&f.xmin, &f.xmax, &f.updatedAt); err != nil {
			t.Fatal(err)
		}
		return f
	}
	before := facts()

	log.reset()
	result := b.mustDiff(draft.ID, 1, againstDraft)
	statements := log.statements()
	if len(result.Changes) != 1 {
		t.Fatalf("changes = %+v, want one", result.Changes)
	}
	if after := facts(); after.xmin != before.xmin || after.xmax != before.xmax || !after.updatedAt.Equal(before.updatedAt) {
		t.Errorf("the test row went from %+v to %+v: the diff wrote or locked it", before, after)
	}
	if len(statements) < 3 || !strings.EqualFold(statements[0], "begin") {
		t.Fatalf("statements = %q, want a transaction", statements)
	}
	if got, want := statements[1], "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY"; got != want {
		t.Errorf("first statement of the transaction = %q, want %q", got, want)
	}
	for _, statement := range statements {
		if lockClause.MatchString(statement) {
			t.Errorf("a locking read in the diff: %s", statement)
		}
		if writeStatement.MatchString(statement) {
			t.Errorf("a write in the diff: %s", statement)
		}
	}

	log.reset()
	if _, err := b.tests.Queries.Get.Handle(context.Background(), query.Get{ID: draft.ID, Scope: access.Scope{UserID: b.author}}); err != nil {
		t.Fatal(err)
	}
	counted := false
	for _, statement := range log.statements() {
		if strings.HasPrefix(statement, "SET TRANSACTION") {
			counted = true
		}
		if writeStatement.MatchString(statement) {
			t.Errorf("a write in getTest: %s", statement)
		}
	}
	if !counted {
		t.Error("getTest did not open the read-only transaction for its count")
	}
}

func TestBothSidesOfADiffComeFromTheSnapshotItOpenedWith(t *testing.T) {
	pool, log := loggedPool(t)
	writer := newPool(t)
	b := newBuilder(t, pool, diffAuthor(t, pool))
	question := b.shortAnswer("Câu hỏi", "1.00")
	draft := b.draft("Đề một ảnh chụp", question)
	b.mustPublish(draft.ID)

	var once sync.Once
	log.reset()
	log.hook = func(sql string) {
		if !strings.Contains(sql, "FROM app.test_sections WHERE test_id") {
			return
		}
		once.Do(func() {
			if _, err := writer.Exec(context.Background(), `UPDATE app.questions SET prompt = 'Sửa giữa chừng' WHERE id = $1`, question); err != nil {
				t.Errorf("concurrent edit: %v", err)
			}
		})
	}
	during := b.mustDiff(draft.ID, 1, againstDraft)
	log.reset()
	if len(during.Changes) != 0 {
		t.Errorf("a bank edit committed after the diff began changed its answer: %+v", during.Changes)
	}
	if after := b.mustDiff(draft.ID, 1, againstDraft); len(after.Changes) != 1 {
		t.Errorf("a later diff = %+v, want to see the edit", after.Changes)
	}
}

func TestADiffOfAHundredQuestionsCostsTheQueriesOfATenQuestionPaper(t *testing.T) {
	pool, log := loggedPool(t)
	author := diffAuthor(t, pool)
	cost := func(questions int) int {
		b := newBuilder(t, pool, author)
		ids := make([]string, questions)
		for i := range ids {
			ids[i] = b.shortAnswer(fmt.Sprintf("Câu %d", i+1), "1.00")
		}
		const sections = 5
		inputs := make([]domain.SectionInput, sections)
		for i := range inputs {
			inputs[i].Title = fmt.Sprintf("Phần %d", i+1)
		}
		for i, id := range ids {
			inputs[i%sections].QuestionIDs = append(inputs[i%sections].QuestionIDs, id)
		}
		draft := b.draft(fmt.Sprintf("Đề %d câu", questions), ids[0])
		b.setOutline(draft, inputs...)
		b.mustPublish(draft.ID)
		b.editQuestion(ids[0], "Câu 1 đã sửa", "1.00")

		log.reset()
		result := b.mustDiff(draft.ID, 1, againstDraft)
		n := len(log.statements())
		if len(result.Changes) != 1 {
			t.Fatalf("%d questions: changes = %+v, want one", questions, result.Changes)
		}
		return n
	}
	small, large := cost(10), cost(100)
	t.Logf("a diff of a 10 question paper runs %d statements, of a 100 question paper %d, in 5 sections each", small, large)
	if small != large {
		t.Errorf("statements: %d for 10 questions, %d for 100: the cost grows with the questions", small, large)
	}
}

func TestRestoringAVersionAsADraftLeavesNothingUnpublished(t *testing.T) {
	pool := newPool(t)
	author := diffAuthor(t, pool)
	b := newBuilder(t, pool, author)
	gap := "g-" + hex.EncodeToString([]byte(author[:4]))
	blank := b.question(questionsdomain.Input{
		Type: questionsdomain.FillBlank, Prompt: "[1]", Points: "1.00",
		PromptContent: []byte(fmt.Sprintf(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":%q,"label":"1"}]}]}`, gap)),
		Blanks:        []questionsdomain.BlankInput{{GapID: &gap, Ordinal: 1, AcceptedAnswers: []string{"x", "y"}}},
	})
	choice := b.question(questionsdomain.Input{
		Type: questionsdomain.SingleChoice, Prompt: "Chọn", Points: "2.00",
		Options: []questionsdomain.OptionInput{{Text: "đúng", IsCorrect: true}, {Text: "sai"}},
	})
	draft := b.draft("Đề khôi phục", blank, choice)
	b.mustPublish(draft.ID)

	current := b.get(draft.ID)
	restored, err := b.tests.Commands.CreateDraftFromVersion.Handle(context.Background(), command.CreateDraftFromVersion{Request: domain.VersionRequest{
		Request: reqFor(draft.ID, author), Version: 1, ExpectedUpdatedAt: current.UpdatedAt,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if restored.Sections[0].QuestionIDs[0] == blank {
		t.Fatal("the restore reused the bank question; this test needs the copies")
	}
	if count := b.get(draft.ID).UnpublishedChanges; count == nil || *count != 0 {
		t.Fatalf("a restored draft counts %v, want 0: it is a copy of the version", count)
	}
	if changes := b.mustDiff(draft.ID, 1, againstDraft).Changes; len(changes) != 0 {
		t.Fatalf("changes = %+v, want none", changes)
	}

	b.editQuestion(restored.Sections[0].QuestionIDs[1], "Chọn (đã sửa)", "2.00")
	edited := b.mustDiff(draft.ID, 1, againstDraft).Changes
	if !slices.Equal(diffKinds(edited), []domain.ChangeKind{domain.ChangeAdded, domain.ChangeRemoved}) {
		t.Errorf("an edited copy: kinds = %v, want it added and the original removed", diffKinds(edited))
	}
}

func removeTest(t *testing.T, repo *repositories.Postgres, testID, author string) {
	t.Helper()
	ctx := context.Background()
	current, err := repo.Get(ctx, everyone, testID)
	if err != nil {
		t.Errorf("cleanup read: %v", err)
		return
	}
	archived := domain.Archived
	if _, err := repo.Update(ctx, domain.UpdateRequest{ID: testID, ActorID: author, Now: time.Now(), Input: domain.UpdateInput{ExpectedUpdatedAt: current.UpdatedAt, Status: &archived}, Scope: access.Scope{UserID: author}}); err != nil {
		t.Errorf("cleanup archive: %v", err)
		return
	}
	if err := repo.Delete(ctx, domain.Request{ID: testID, ActorID: author, Scope: access.Scope{UserID: author}}, time.Now()); err != nil {
		t.Errorf("cleanup delete: %v", err)
	}
}

type groupedWorld struct {
	pool   *pgxpool.Pool
	log    *sqlLog
	repo   *repositories.Postgres
	groups *repositories.GroupsPostgres
	app    *application.Application
	author string
	scope  access.Scope
	testID string
	stored domain.StoredGroup
}

func publishedGroupedTest(t *testing.T) *groupedWorld {
	t.Helper()
	ctx := context.Background()
	pool, log := loggedPool(t)
	author := diffAuthor(t, pool)
	media := mediarepo.NewPostgres(db.NewContext(pool))
	repo := repositories.NewPostgres(db.NewContext(pool), adapters.GroupQuestions{}, media).WithGroupQuestions(adapters.GroupQuestions{})
	w := &groupedWorld{
		pool: pool, log: log, repo: repo, author: author, scope: access.Scope{UserID: author},
		groups: repositories.NewGroupsPostgres(db.NewContext(pool), adapters.GroupQuestions{}, media),
		app:    application.New(repo),
	}
	t.Cleanup(func() {
		if w.testID != "" {
			removeTest(t, repo, w.testID, author)
		}
	})
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id, section, updated := snapshotDraft(t, tx, author)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	w.testID = id
	if w.stored, err = w.groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, ""), OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now(), Scope: w.scope, Grants: bothKeys}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Publish(ctx, domain.PublishRequest{TestID: id, ActorID: author, Scope: w.scope}, time.Now(), domain.Publishing.Validate); err != nil {
		t.Fatal(err)
	}
	return w
}

func TestAGroupedVersionRestoredAsDraftHasNoChangeAndAnEditedPassageChangesItsMembers(t *testing.T) {
	ctx := context.Background()
	w := publishedGroupedTest(t)
	pool, log, repo, groups, app := w.pool, w.log, w.repo, w.groups, w.app
	author, scope, testID, stored := w.author, w.scope, w.testID, w.stored
	count := func() *int {
		t.Helper()
		test, err := app.Queries.Get.Handle(ctx, query.Get{ID: testID, Scope: scope})
		if err != nil {
			t.Fatal(err)
		}
		return test.UnpublishedChanges
	}
	if got := count(); got == nil || *got != 0 {
		t.Fatalf("a grouped draft just published counts %v, want 0", got)
	}

	current, err := repo.Get(ctx, everyone, testID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateDraftFromVersion(ctx, domain.VersionRequest{Request: domain.Request{ID: testID, ActorID: author, Scope: scope}, Version: 1, ExpectedUpdatedAt: current.UpdatedAt}, time.Now()); err != nil {
		t.Fatal(err)
	}
	var copied string
	if err := pool.QueryRow(ctx, `SELECT g.id::text FROM app.question_groups g JOIN app.test_sections s ON s.id=g.owner_section_id WHERE s.test_id=$1`, testID).Scan(&copied); err != nil {
		t.Fatal(err)
	}
	if copied == stored.Bundle.Group.ID {
		t.Fatal("the restore kept the source group; this test needs the copy")
	}
	if got := count(); got == nil || *got != 0 {
		t.Fatalf("a restored grouped draft counts %v, want 0: every id of it was renewed", got)
	}

	log.reset()
	papers, err := repo.DiffPapers(ctx, domain.DiffRequest{TestID: testID, Version: 1, Against: againstDraft, Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("a diff of a paper with one group of 2 members and 1 standalone question runs %d statements", len(log.statements()))
	if changes, err := papers.Changes(); err != nil || len(changes) != 0 {
		t.Fatalf("changes = %+v, %v, want none", changes, err)
	}

	group, err := groups.Get(ctx, everyone, copied)
	if err != nil {
		t.Fatal(err)
	}
	group.Bundle.Group.Stimuli[0].Title = "Bài đọc đã sửa"
	mutation := groupMutation(group, author)
	mutation.ExpectedTestUpdatedAt = *group.TestUpdatedAt
	if _, err := groups.Update(ctx, domain.UpdateGroupInput{GroupMutation: mutation, Bundle: group.Bundle}); err != nil {
		t.Fatal(err)
	}
	edited, err := app.Queries.Diff.Handle(ctx, query.Diff{TestID: testID, Version: 1, Against: againstDraft, Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	if len(edited.Changes) != 2 {
		t.Fatalf("changes after editing a passage = %+v, want one per member", edited.Changes)
	}
	for _, change := range edited.Changes {
		if change.Kind != domain.ChangeChanged || !reflect.DeepEqual(change.Fields, []domain.ChangedField{domain.FieldContext}) {
			t.Errorf("change = %+v, want the context of a member", change)
		}
	}
	if got := count(); got == nil || *got != 2 {
		t.Errorf("the count after editing the passage = %v, want 2", got)
	}

	for _, swap := range []string{
		`UPDATE app.questions SET context_ordinal = 2 WHERE context_group_id = $1 AND context_ordinal = 1`,
		`UPDATE app.questions SET context_ordinal = 1 WHERE context_group_id = $1 AND context_ordinal = 0`,
	} {
		if _, err := pool.Exec(ctx, swap, copied); err != nil {
			t.Fatalf("breaking the group's membership: %v", err)
		}
	}
	if got := count(); got != nil {
		t.Errorf("a draft whose group is refused counts %d, want null", *got)
	}
	_, err = app.Queries.Diff.Handle(ctx, query.Diff{TestID: testID, Version: 1, Against: againstDraft, Scope: scope})
	var refused *domain.GroupError
	if !errors.Is(err, domain.ErrDraftUnreadable) || !errors.As(err, &refused) {
		t.Errorf("diff of a refused draft = %v, want ErrDraftUnreadable wrapping the group error", err)
	}
	if _, err := app.Queries.Diff.Handle(ctx, query.Diff{TestID: testID, Version: 1, Against: onVersion(1), Scope: scope}); !errors.Is(err, domain.ErrSameVersion) {
		t.Errorf("a version against itself = %v", err)
	}
	for _, restore := range []string{
		`UPDATE app.questions SET context_ordinal = 0 WHERE context_group_id = $1 AND context_ordinal = 1`,
		`UPDATE app.questions SET context_ordinal = 1 WHERE context_group_id = $1 AND context_ordinal = 2`,
	} {
		if _, err := pool.Exec(ctx, restore, copied); err != nil {
			t.Fatalf("restoring the group's membership: %v", err)
		}
	}
}

func TestAVersionWhoseGroupNoLongerReadsLeavesTheCountNullAndTheDiffUnreadable(t *testing.T) {
	ctx := context.Background()
	w := publishedGroupedTest(t)
	frozenGroup := `SELECT g.id::text FROM app.test_version_groups g
		JOIN app.test_version_sections s ON s.id = g.test_version_section_id
		JOIN app.test_versions v ON v.id = s.test_version_id WHERE v.test_id = $1 AND v.version = 1`
	var frozen string
	if err := w.pool.QueryRow(ctx, frozenGroup, w.testID).Scan(&frozen); err != nil {
		t.Fatal(err)
	}
	if got, err := w.app.Queries.Get.Handle(ctx, query.Get{ID: w.testID, Scope: w.scope}); err != nil || got.UnpublishedChanges == nil || *got.UnpublishedChanges != 0 {
		t.Fatalf("before the damage getTest = %v, %v, want a count of 0", got.UnpublishedChanges, err)
	}

	for _, shift := range []string{
		`UPDATE app.test_version_group_members SET ordinal = 2 WHERE group_id = $1 AND ordinal = 1`,
		`UPDATE app.test_version_group_members SET ordinal = 1 WHERE group_id = $1 AND ordinal = 0`,
	} {
		if _, err := w.pool.Exec(ctx, shift, frozen); err != nil {
			t.Fatalf("breaking the frozen group: %v", err)
		}
	}

	got, err := w.app.Queries.Get.Handle(ctx, query.Get{ID: w.testID, Scope: w.scope})
	if err != nil {
		t.Fatalf("getTest failed for a version it cannot read: %v", err)
	}
	if got.UnpublishedChanges != nil {
		t.Errorf("the count is %d, want null: the latest version does not read back", *got.UnpublishedChanges)
	}
	for name, against := range map[string]domain.Against{"the draft": againstDraft, "the previous version": againstPrevious} {
		_, err := w.app.Queries.Diff.Handle(ctx, query.Diff{TestID: w.testID, Version: 1, Against: against, Scope: w.scope})
		var refused *domain.GroupError
		if !errors.Is(err, domain.ErrVersionUnreadable) || !errors.As(err, &refused) {
			t.Errorf("diff of version 1 against %s = %v, want ErrVersionUnreadable wrapping the group error", name, err)
		}
		if errors.Is(err, domain.ErrNotFound) {
			t.Errorf("diff against %s answered not found for a version that exists", name)
		}
	}

	for _, shift := range []string{
		`UPDATE app.test_version_group_members SET ordinal = 0 WHERE group_id = $1 AND ordinal = 1`,
		`UPDATE app.test_version_group_members SET ordinal = 1 WHERE group_id = $1 AND ordinal = 2`,
	} {
		if _, err := w.pool.Exec(ctx, shift, frozen); err != nil {
			t.Fatalf("mending the frozen group: %v", err)
		}
	}
	if got, err := w.app.Queries.Get.Handle(ctx, query.Get{ID: w.testID, Scope: w.scope}); err != nil || got.UnpublishedChanges == nil || *got.UnpublishedChanges != 0 {
		t.Errorf("after mending, getTest = %v, %v, want a count of 0", got.UnpublishedChanges, err)
	}
}

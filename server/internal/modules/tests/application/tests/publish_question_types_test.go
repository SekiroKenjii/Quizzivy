//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionscommand "quizzivy/internal/modules/questions/application/command"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func choices(texts ...string) []questions.OptionInput {
	out := make([]questions.OptionInput, len(texts))
	for i, text := range texts {
		out[i] = questions.OptionInput{Text: text}
	}
	return out
}

func marked(options []questions.OptionInput, correct ...int) []questions.OptionInput {
	out := slices.Clone(options)
	for _, i := range correct {
		out[i].IsCorrect = true
	}
	return out
}

func number(n int) *int { return &n }

func flag(b bool) *bool { return &b }

func tfngInput(correct int) questions.Input {
	return questions.Input{Type: questions.TFNG, Prompt: "The author grew up by the sea.", Points: "1.00",
		Options: marked(choices("True", "False", "Not given"), correct)}
}

func chooseNInput(n int, correct ...int) questions.Input {
	return questions.Input{Type: questions.MultipleChoice, Prompt: "Choose TWO letters.", Points: "2.00", SelectCount: number(n),
		Options: marked(choices("A", "B", "C", "D", "E"), correct...)}
}

func wordLimitInput(limit int) questions.Input {
	return questions.Input{Type: questions.ShortAnswer, Prompt: "Name the river.", Points: "1.00", MaxWords: number(limit)}
}

func essayInput(sample string, labels ...string) questions.Input {
	in := questions.Input{Type: questions.Essay, Prompt: "Discuss both views.", Points: "9.00", SampleAnswer: &sample}
	for _, label := range labels {
		in.Rubric = append(in.Rubric, questions.RubricCriterionInput{Label: label})
	}
	return in
}

func matchingInput(reuse bool, texts []string, keys ...int) questions.Input {
	in := questions.Input{Type: questions.Matching, Prompt: "Match each paragraph to a heading.", Points: "3.00", MatchAllowReuse: flag(reuse),
		Options: choices("Early life", "A turning point", "Later years", "Legacy")}
	for i, key := range keys {
		in.MatchItems = append(in.MatchItems, questions.MatchItemInput{Text: texts[i], CorrectOption: key})
	}
	return in
}

func (b *builder) rewrite(id string, in questions.Input) {
	b.t.Helper()
	if in.Tags == nil {
		in.Tags = []string{}
	}
	if _, err := b.qsvc.Commands.Update.Handle(context.Background(), questionscommand.Update{Request: questions.WriteRequest{ID: id, ActorID: b.author, Input: in}}); err != nil {
		b.t.Fatalf("rewrite %s: %v", id, err)
	}
}

// frozenTypeDigest is every R6 field a version holds, in paper order, with
// each matching item keyed by its choice's ordinal.
func frozenTypeDigest(t *testing.T, q db.Querier, versionID string) string {
	t.Helper()
	var digest string
	if err := q.QueryRow(context.Background(), `
		SELECT coalesce(string_agg(concat_ws('|', q.type, q.select_count, q.max_words, q.match_allow_reuse, q.sample_answer,
		       (SELECT string_agg(o.ordinal || ':' || o.text || ':' || o.is_correct, ',' ORDER BY o.ordinal)
		          FROM app.test_version_options o WHERE o.test_version_question_id = q.id),
		       (SELECT string_agg(i.ordinal || ':' || i.text || '->' || o.ordinal, ',' ORDER BY i.ordinal)
		          FROM app.test_version_match_items i JOIN app.test_version_options o ON o.id = i.correct_option_id
		         WHERE i.test_version_question_id = q.id),
		       (SELECT string_agg(c.ordinal || ':' || c.label, ',' ORDER BY c.ordinal)
		          FROM app.test_version_rubric_criteria c WHERE c.test_version_question_id = q.id)), E'\n' ORDER BY s.ordinal, q.ordinal), '')
		  FROM app.test_version_questions q
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1`, versionID).Scan(&digest); err != nil {
		t.Fatal(err)
	}
	return digest
}

func bankTypeDigest(t *testing.T, q db.Querier, ids []string) string {
	t.Helper()
	var digest string
	if err := q.QueryRow(context.Background(), `
		SELECT coalesce(string_agg(concat_ws('|', q.type, q.select_count, q.max_words, q.match_allow_reuse, q.sample_answer,
		       (SELECT string_agg(o.ordinal || ':' || o.text || ':' || o.is_correct, ',' ORDER BY o.ordinal)
		          FROM app.question_options o WHERE o.question_id = q.id),
		       (SELECT string_agg(i.ordinal || ':' || i.text || '->' || o.ordinal, ',' ORDER BY i.ordinal)
		          FROM app.question_match_items i JOIN app.question_options o ON o.id = i.correct_option_id
		         WHERE i.question_id = q.id),
		       (SELECT string_agg(c.ordinal || ':' || c.label, ',' ORDER BY c.ordinal)
		          FROM app.question_rubric_criteria c WHERE c.question_id = q.id)), E'\n' ORDER BY m.ord), '')
		  FROM unnest($1::uuid[]) WITH ORDINALITY AS m(id, ord)
		  JOIN app.questions q ON q.id = m.id`, ids).Scan(&digest); err != nil {
		t.Fatal(err)
	}
	return digest
}

type typedPaper struct {
	ids   []string
	draft domain.Test
}

func newTypedPaper(b *builder) typedPaper {
	ids := []string{
		b.question(tfngInput(2)),
		b.question(chooseNInput(2, 1, 3)),
		b.question(wordLimitInput(3)),
		b.question(essayInput("Some people think…", "Task response", "Coherence and cohesion")),
		b.question(matchingInput(false, []string{"Paragraph A", "Paragraph B", "Paragraph C"}, 2, 0, 3)),
	}
	return typedPaper{ids: ids, draft: b.draft("Đề có mọi loại câu hỏi", ids...)}
}

func TestPublishingFreezesEveryNewFieldAndLaterEditsDoNotReachTheVersion(t *testing.T) {
	f := newSnapshotFixture(t)
	b := f.builder
	paper := newTypedPaper(b)
	before := bankTypeDigest(t, f.conn, paper.ids)

	version, err := b.publish(paper.draft.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if version.ManualCount != 2 {
		t.Errorf("the version counts %d answers marked by hand, want the short answer and the essay", version.ManualCount)
	}
	frozen := frozenTypeDigest(t, f.conn, version.ID)
	if frozen != before {
		t.Fatalf("the version froze\n%s\nwant the bank's\n%s", frozen, before)
	}

	b.rewrite(paper.ids[0], tfngInput(0))
	b.rewrite(paper.ids[1], chooseNInput(3, 0, 1, 2))
	b.rewrite(paper.ids[2], wordLimitInput(50))
	b.rewrite(paper.ids[3], essayInput("Changed sample", "Lexical resource"))
	b.rewrite(paper.ids[4], matchingInput(true, []string{"Edited A", "Edited B"}, 1, 1))
	if after := bankTypeDigest(t, f.conn, paper.ids); after == before {
		t.Fatal("the edits did not change the bank, so this test proves nothing")
	}
	if got := frozenTypeDigest(t, f.conn, version.ID); got != frozen {
		t.Fatalf("editing the bank reached the version:\n%s\nwant\n%s", got, frozen)
	}
}

func TestEditFromThisVersionCopiesTheNewFieldsIntoNewBankRows(t *testing.T) {
	f := newSnapshotFixture(t)
	b := f.builder
	ctx := context.Background()
	paper := newTypedPaper(b)
	version, err := b.publish(paper.draft.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	frozen := frozenTypeDigest(t, f.conn, version.ID)
	b.rewrite(paper.ids[4], matchingInput(true, []string{"Edited A", "Edited B"}, 1, 1))

	request := domain.VersionRequest{Request: reqFor(paper.draft.ID, b.author), Version: 1}
	if err := f.conn.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id = $1`, paper.draft.ID).Scan(&request.ExpectedUpdatedAt); err != nil {
		t.Fatal(err)
	}
	restored, err := b.tests.Commands.CreateDraftFromVersion.Handle(ctx, command.CreateDraftFromVersion{Request: request})
	if err != nil {
		t.Fatalf("restore as draft: %v", err)
	}
	copies := restored.Sections[0].QuestionIDs
	for i, id := range copies {
		if id == paper.ids[i] {
			t.Fatalf("question %d: the restore reused the bank row", i)
		}
	}
	if got := bankTypeDigest(t, f.conn, copies); got != frozen {
		t.Fatalf("the restored draft holds\n%s\nwant the version's\n%s", got, frozen)
	}
	republished, err := b.publish(paper.draft.ID)
	if err != nil {
		t.Fatalf("publishing the restored draft: %v", err)
	}
	if got := frozenTypeDigest(t, f.conn, republished.ID); got != frozen {
		t.Fatalf("version 2 froze\n%s\nwant\n%s", got, frozen)
	}
}

func TestPublishRefusesARuleTheBankNoLongerHolds(t *testing.T) {
	for _, c := range []struct {
		name   string
		input  questions.Input
		breaks string
	}{
		{"choose N", chooseNInput(2, 1, 3), `UPDATE app.question_options SET is_correct = true WHERE question_id = $1 AND ordinal = 0`},
		{"tfng", tfngInput(2), `UPDATE app.question_options SET is_correct = true WHERE question_id = $1 AND ordinal = 0`},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newSnapshotFixture(t)
			b := f.builder
			id := b.question(c.input)
			draft := b.draft("Đề lỗi", id)
			if _, err := f.conn.Exec(context.Background(), c.breaks, id); err != nil {
				t.Fatal(err)
			}
			_, err := b.publish(draft.ID)
			var invalid *domain.PublishValidationError
			if !errors.As(err, &invalid) || len(invalid.Violations) != 1 {
				t.Fatalf("publish = %v, want one violation", err)
			}
			if v := invalid.Violations[0]; v.Rule != domain.ChoiceHasCorrectOption || v.QuestionID != id || v.Message == "" || v.English == "" {
				t.Fatalf("violation %+v, want %s on %s", v, domain.ChoiceHasCorrectOption, id)
			}
		})
	}
}

func TestThePreviewCarriesRulesAndKeysButNoAnswerKey(t *testing.T) {
	f := newSnapshotFixture(t)
	b := f.builder
	paper := newTypedPaper(b)
	if _, err := b.publish(paper.draft.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	preview, err := b.tests.Queries.Preview.Handle(context.Background(), query.Preview{TestID: paper.draft.ID, Scope: access.Scope{UserID: b.author}})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Questions) != 5 {
		t.Fatalf("%d preview questions, want 5", len(preview.Questions))
	}
	tfng, chooseN, short, essay, matching := preview.Questions[0], preview.Questions[1], preview.Questions[2], preview.Questions[3], preview.Questions[4]
	if got := texts(tfng.Options); !slices.Equal(got, questions.TFNGTexts[:]) {
		t.Errorf("tfng options %v, want the fixed %v", got, questions.TFNGTexts)
	}
	if chooseN.SelectCount == nil || *chooseN.SelectCount != 2 {
		t.Errorf("choose N selectCount = %v, want 2", chooseN.SelectCount)
	}
	if short.MaxWords == nil || *short.MaxWords != 3 {
		t.Errorf("maxWords = %v, want 3", short.MaxWords)
	}
	if len(essay.Options) != 0 || len(essay.MatchItems) != 0 {
		t.Errorf("an essay previews options %v and items %v", essay.Options, essay.MatchItems)
	}
	if matching.MatchReuse == nil || *matching.MatchReuse {
		t.Errorf("matchAllowReuse = %v, want false", matching.MatchReuse)
	}
	var optionKeys, itemKeys []string
	for _, option := range matching.Options {
		optionKeys = append(optionKeys, option.Key)
	}
	for _, item := range matching.MatchItems {
		itemKeys = append(itemKeys, item.Key+":"+item.Text)
	}
	if !slices.Equal(optionKeys, []string{"i", "ii", "iii", "iv"}) {
		t.Errorf("choice keys %v, want i to iv", optionKeys)
	}
	if !slices.Equal(itemKeys, []string{"A:Paragraph A", "B:Paragraph B", "C:Paragraph C"}) {
		t.Errorf("items %v", itemKeys)
	}
	for _, option := range tfng.Options {
		if option.Key != "" {
			t.Errorf("a tfng option carries the key %q", option.Key)
		}
	}
}

func texts(options []domain.PreviewOption) []string {
	out := make([]string, len(options))
	for i, option := range options {
		out[i] = option.Text
	}
	return out
}

func TestAGroupOfNewTypesSurvivesPublishDuplicateAndRestore(t *testing.T) {
	ctx := context.Background()
	tx, author, groups := groupTransaction(t)
	testID, section, updated := snapshotDraft(t, tx, author)
	bundle := storedGroupFixture(t, "")
	matchingID, essayID := groupIdentity(t), groupIdentity(t)
	bundle.Questions = append(bundle.Questions,
		domain.GroupQuestion{ID: matchingID, Input: matchingInput(false, []string{"Paragraph A", "Paragraph B"}, 1, 0)},
		domain.GroupQuestion{ID: essayID, Input: essayInput("Sample", "Task", "Lexis")})
	bundle.Group.Members = append(bundle.Group.Members,
		domain.GroupMember{QuestionID: matchingID, OptionOrder: "fixed"}, domain.GroupMember{QuestionID: essayID, OptionOrder: "shuffle"})
	for i := range bundle.Questions {
		bundle.Questions[i].Input.Tags = []string{}
	}
	scope := access.Scope{UserID: author}
	if _, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &section, ExpectedTestUpdatedAt: updated, ActorID: author, Now: time.Now(), Scope: scope, Grants: bothKeys}); err != nil {
		t.Fatalf("create the group: %v", err)
	}
	repo := repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
	published, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author, Scope: scope}, time.Now(), domain.Publishing.Validate)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	frozen := frozenTypeDigest(t, tx, published.ID)
	for _, want := range []string{"matching|f|0:Early life:false", "|0:Paragraph A->1,1:Paragraph B->0", "essay|Sample|0:Task,1:Lexis"} {
		if !strings.Contains(frozen, want) {
			t.Fatalf("the version froze\n%s\nwhich lacks %q", frozen, want)
		}
	}

	copied, err := repo.Duplicate(ctx, domain.DuplicateInput{ID: testID, ActorID: author, Now: time.Now(), Scope: scope})
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if got := draftGroupDigest(t, tx, copied.ID); got != draftGroupDigest(t, tx, testID) {
		t.Fatalf("the duplicate's group holds\n%s\nwant\n%s", got, draftGroupDigest(t, tx, testID))
	}

	req := domain.VersionRequest{Request: domain.Request{ID: testID, ActorID: author, Scope: scope}, Version: 1}
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, testID).Scan(&req.ExpectedUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateDraftFromVersion(ctx, req, time.Now()); err != nil {
		t.Fatalf("restore: %v", err)
	}
	republished, err := repo.Publish(ctx, domain.PublishRequest{TestID: testID, ActorID: author, Scope: scope}, time.Now(), domain.Publishing.Validate)
	if err != nil {
		t.Fatalf("publish the restored draft: %v", err)
	}
	if got := frozenTypeDigest(t, tx, republished.ID); got != frozen {
		t.Fatalf("the restored group froze\n%s\nwant\n%s", got, frozen)
	}
}

func draftGroupDigest(t *testing.T, tx pgx.Tx, testID string) string {
	t.Helper()
	rows, err := tx.Query(context.Background(), `SELECT q.id::text FROM app.questions q
		JOIN app.question_groups g ON g.id = q.context_group_id
		JOIN app.test_sections s ON s.id = g.owner_section_id
		WHERE s.test_id = $1 ORDER BY q.prompt, q.type`, testID)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return bankTypeDigest(t, tx, ids)
}

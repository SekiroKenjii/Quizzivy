//go:build integration

package repositories_test

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionsapp "quizzivy/internal/modules/questions/application"
	questionscommand "quizzivy/internal/modules/questions/application/command"
	questions "quizzivy/internal/modules/questions/domain"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"reflect"
	"testing"
	"time"
)

func TestGroupedMetadataRestoreAndDraftSkillsKeepLegacyContent(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	owner := uuid.NewString()
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil {
			t.Errorf("lifecycle rollback: %v", err)
			return
		}
		var remaining int
		err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM app.users WHERE id=$1)+(SELECT count(*) FROM app.tests WHERE created_by=$1)+(SELECT count(*) FROM app.questions WHERE created_by=$1)+(SELECT count(*) FROM app.question_groups WHERE created_by=$1)+(SELECT count(*) FROM app.audit_log WHERE actor_user_id=$1)`, owner).Scan(&remaining)
		if err != nil || remaining != 0 {
			t.Errorf("lifecycle owned absence=%d %v", remaining, err)
		}
		t.Logf("lifecycle rollback checked owner=%s", owner)
	})
	if _, err := tx.Exec(ctx, `INSERT INTO app.users(id,email,full_name,role_id) VALUES($1,$2,'Metadata teacher',(SELECT id FROM app.roles WHERE builtin_key='teacher'))`, owner, owner+"@example.test"); err != nil {
		t.Fatal(err)
	}
	dbx := db.NewContext(tx)
	bank := questionsrepo.NewPostgres(dbx)
	media := mediarepo.NewPostgres(dbx)
	repo := repositories.NewPostgres(dbx, bank, media).WithGroupQuestions(adapters.GroupQuestions{})
	groups := repositories.NewGroupsPostgres(dbx, adapters.GroupQuestions{}, media)
	scope := access.Scope{UserID: owner}
	now := time.Now()
	created, err := repo.Create(ctx, domain.CreateInput{Title: "Skills", ActorID: owner, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if created.Skills == nil || len(created.Skills) != 0 {
		t.Fatalf("empty skills %+v", created.Skills)
	}
	reading, grammar := questions.Skill("reading"), questions.Skill("grammar")
	level := questions.Level("c2")
	standalone, err := bank.Create(ctx, questions.WriteInput{Input: questions.Input{Type: questions.ShortAnswer, Prompt: "Standalone", Points: "1", Tags: []string{}, Skill: &reading}, ActorID: owner, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	draft, err := repo.Update(ctx, domain.UpdateRequest{ID: created.ID, ActorID: owner, Scope: scope, Input: domain.UpdateInput{ExpectedUpdatedAt: created.UpdatedAt, SetSections: true, Sections: []domain.SectionInput{{Title: "Part", QuestionIDs: []string{standalone.ID}}}}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	groupID, memberID, nullID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	input := questions.Input{Type: questions.SingleChoice, Prompt: "Grouped", Points: "1", Tags: []string{}, Level: &level, Skill: &grammar}
	for i := range 9 {
		input.Options = append(input.Options, questions.OptionInput{Text: "Choice", IsCorrect: i == 0})
	}
	bundle := domain.GroupBundle{Group: domain.QuestionGroup{ID: groupID, Title: "Group", Members: []domain.GroupMember{{QuestionID: memberID, OptionOrder: "fixed"}, {QuestionID: nullID, OptionOrder: "shuffle"}}}, Questions: []domain.GroupQuestion{{ID: memberID, Input: input}, {ID: nullID, Input: questions.Input{Type: questions.ShortAnswer, Prompt: "Unset", Points: "1", Tags: []string{}}}}}
	section := draft.Sections[0].ID
	stored, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &section, ExpectedTestUpdatedAt: draft.UpdatedAt, ActorID: owner, Scope: scope, Grants: access.NewSet(access.ContentTestsWrite), Now: now})
	if err != nil {
		t.Fatal(err)
	}
	current, err := repo.Get(ctx, scope, draft.ID)
	if err != nil || !reflect.DeepEqual(current.Skills, []string{"grammar", "reading"}) {
		t.Fatalf("draft distinct sorted skills=%v %v", current.Skills, err)
	}
	version, err := repo.Publish(ctx, domain.PublishRequest{TestID: draft.ID, ActorID: owner, Scope: scope}, now, domain.Publishing.Validate)
	if err != nil {
		t.Fatal(err)
	}
	var frozenGroup string
	if err := tx.QueryRow(ctx, `SELECT g.id::text FROM app.test_version_groups g JOIN app.test_version_sections s ON s.id=g.test_version_section_id WHERE s.test_version_id=$1`, version.ID).Scan(&frozenGroup); err != nil {
		t.Fatal(err)
	}
	frozen, err := groups.Frozen(ctx, frozenGroup)
	if err != nil {
		t.Fatal(err)
	}
	assertRestoredMetadata(t, frozen)
	stored.Bundle.Questions[0].Input.Level = nil
	stored.Bundle.Questions[0].Input.Skill = &reading
	mutation := domain.GroupMutation{ID: stored.Bundle.Group.ID, ExpectedRevision: stored.Revision, ActorID: owner, Scope: scope, Grants: access.NewSet(access.ContentTestsWrite)}
	if err := tx.QueryRow(ctx, `SELECT updated_at FROM app.tests WHERE id=$1`, draft.ID).Scan(&mutation.ExpectedTestUpdatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := groups.Update(ctx, domain.UpdateGroupInput{GroupMutation: mutation, Bundle: stored.Bundle}); err != nil {
		t.Fatal(err)
	}
	current, err = repo.Get(ctx, scope, draft.ID)
	if err != nil || !reflect.DeepEqual(current.Skills, []string{"reading"}) {
		t.Fatalf("current draft not frozen skills=%v %v", current.Skills, err)
	}
	restored, err := repo.CreateDraftFromVersion(ctx, domain.VersionRequest{Request: domain.Request{ID: draft.ID, ActorID: owner, Scope: scope}, Version: version.Version, ExpectedUpdatedAt: current.UpdatedAt}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.Skills, []string{"grammar", "reading"}) {
		t.Fatalf("restore skills %v", restored.Skills)
	}
	var restoredID string
	if err := tx.QueryRow(ctx, `SELECT g.id::text FROM app.question_groups g JOIN app.test_sections s ON s.id=g.owner_section_id WHERE s.test_id=$1`, draft.ID).Scan(&restoredID); err != nil {
		t.Fatal(err)
	}
	actual, err := groups.Get(ctx, scope, restoredID)
	if err != nil {
		t.Fatal(err)
	}
	assertRestoredMetadata(t, actual.Bundle)
}

func assertRestoredMetadata(t *testing.T, bundle domain.GroupBundle) {
	t.Helper()
	if len(bundle.Questions) != 2 {
		t.Fatalf("member count %d", len(bundle.Questions))
	}
	first, second := bundle.Questions[0].Input, bundle.Questions[1].Input
	if first.Level == nil || *first.Level != "c2" || first.Skill == nil || *first.Skill != "grammar" || len(first.Options) != 9 || second.Level != nil || second.Skill != nil {
		t.Fatalf("group metadata/legacy content lost: %+v %+v", first, second)
	}
}

func metadataCopyFixture(t *testing.T) (context.Context, db.Context, string) {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var version int
	if err := pool.QueryRow(ctx, `SELECT current_setting('server_version_num')::integer`).Scan(&version); err != nil || version < 180000 {
		t.Fatalf("PG18 required: %d %v", version, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	owner := uuid.NewString()
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil {
			t.Errorf("copy rollback: %v", err)
			return
		}
		var remaining int
		err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM app.users WHERE id=$1)+(SELECT count(*) FROM app.tests WHERE created_by=$1)+(SELECT count(*) FROM app.questions WHERE created_by=$1)+(SELECT count(*) FROM app.question_groups WHERE created_by=$1)+(SELECT count(*) FROM app.audit_log WHERE actor_user_id=$1)`, owner).Scan(&remaining)
		if err != nil || remaining != 0 {
			t.Errorf("copy exact-owned absence=%d err=%v", remaining, err)
		}
		t.Logf("copy rollback checked owner=%s", owner)
	})
	t.Logf("copy fixture owner=%s", owner)
	if _, err := tx.Exec(ctx, `INSERT INTO app.users(id,email,full_name,role_id) VALUES($1,$2,'Metadata teacher',(SELECT id FROM app.roles WHERE builtin_key='teacher'))`, owner, owner+"@example.test"); err != nil {
		t.Fatal(err)
	}
	return ctx, db.NewContext(tx), owner
}

func TestMetadataCopyAndStandaloneRestorePersistLegacyNineOptions(t *testing.T) {
	ctx, dbx, owner := metadataCopyFixture(t)
	bank := questionsrepo.NewPostgres(dbx)
	media := mediarepo.NewPostgres(dbx)
	repo := repositories.NewPostgres(dbx, bank, media).WithGroupQuestions(adapters.GroupQuestions{})
	groups := repositories.NewGroupsPostgres(dbx, adapters.GroupQuestions{}, media)
	scope := access.Scope{UserID: owner}
	now := time.Now()
	level := questions.Level("c2")
	grammar, reading := questions.Skill("grammar"), questions.Skill("reading")
	input := questions.Input{Type: questions.SingleChoice, Prompt: "Legacy standalone", Points: "1", Tags: []string{}, Level: &level, Skill: &reading}
	for i := range 9 {
		input.Options = append(input.Options, questions.OptionInput{Text: fmt.Sprintf("Legacy %d", i), IsCorrect: i == 0})
	}
	source, err := bank.Create(ctx, questions.WriteInput{Input: input, ActorID: owner, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	app := questionsapp.New(bank, nil)
	duplicate, err := app.Commands.Duplicate.Handle(ctx, questionscommand.Duplicate{Request: questions.WriteRequest{ID: source.ID, ActorID: owner}})
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := bank.Get(ctx, scope, duplicate.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertFreshLegacyQuestion(t, source, persisted)
	created, err := repo.Create(ctx, domain.CreateInput{Title: "Metadata copies", ActorID: owner, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	draft, err := repo.Update(ctx, domain.UpdateRequest{ID: created.ID, ActorID: owner, Scope: scope, Input: domain.UpdateInput{ExpectedUpdatedAt: created.UpdatedAt, SetSections: true, Sections: []domain.SectionInput{{Title: "Part", QuestionIDs: []string{source.ID}}}}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	memberID := uuid.NewString()
	groupInput := input
	groupInput.Skill = &grammar
	group, err := groups.Create(ctx, domain.CreateGroupInput{Bundle: domain.GroupBundle{Group: domain.QuestionGroup{ID: uuid.NewString(), Title: "Legacy group", Members: []domain.GroupMember{{QuestionID: memberID, OptionOrder: "fixed"}}}, Questions: []domain.GroupQuestion{{ID: memberID, Input: groupInput}}}, ActorID: owner, Scope: scope, Grants: access.NewSet(access.ContentQuestionsWrite), Now: now})
	if err != nil {
		t.Fatal(err)
	}
	section := draft.Sections[0].ID
	copiedGroup, err := groups.Copy(ctx, domain.CopyGroupInput{SourceID: group.Bundle.Group.ID, ExpectedSourceRevision: group.Revision, OwnerSectionID: &section, ExpectedTestUpdatedAt: draft.UpdatedAt, ActorID: owner, Scope: scope, Grants: access.NewSet(access.ContentTestsWrite), Now: now})
	if err != nil {
		t.Fatal(err)
	}
	storedGroup, err := groups.Get(ctx, scope, copiedGroup.Bundle.Group.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertFreshLegacyGroup(t, group.Bundle, storedGroup.Bundle)
	copiedTest, err := repo.Duplicate(ctx, domain.DuplicateInput{ID: draft.ID, ActorID: owner, Scope: scope, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	copiedTest, err = repo.Get(ctx, scope, copiedTest.ID)
	if err != nil || copiedTest.ID == draft.ID || len(copiedTest.Sections) != 1 || copiedTest.Sections[0].ID == section || !reflect.DeepEqual(copiedTest.Skills, []string{"grammar", "reading"}) {
		t.Fatalf("test copy identity/metadata=%+v err=%v", copiedTest, err)
	}
	units := copiedTest.Sections[0].Units
	if len(units) != 2 || units[0] != (domain.SectionUnit{Kind: "question", ID: source.ID}) || units[1].Kind != "group" || units[1].ID == storedGroup.Bundle.Group.ID {
		t.Fatalf("copy keeps standalone reference and remaps group units=%+v", units)
	}
	copiedMember, err := groups.Get(ctx, scope, units[1].ID)
	if err != nil {
		t.Fatal(err)
	}
	assertFreshLegacyGroup(t, storedGroup.Bundle, copiedMember.Bundle)
	sharedQuestion, err := bank.Get(ctx, scope, units[0].ID)
	if err != nil || sharedQuestion.Level == nil || *sharedQuestion.Level != level || sharedQuestion.Skill == nil || *sharedQuestion.Skill != reading || len(sharedQuestion.Options) != 9 || !reflect.DeepEqual(sharedQuestion.Options, source.Options) {
		t.Fatalf("test copy standalone metadata/legacy options=%+v err=%v", sharedQuestion, err)
	}
	version, err := repo.Publish(ctx, domain.PublishRequest{TestID: draft.ID, ActorID: owner, Scope: scope}, now, domain.Publishing.Validate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bank.Update(ctx, questions.WriteInput{ID: source.ID, Input: questions.Input{Type: questions.ShortAnswer, Prompt: "Changed bank", Points: "1", Tags: []string{}}, ActorID: owner, Now: now}); err != nil {
		t.Fatal(err)
	}
	current, err := repo.Get(ctx, scope, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := repo.CreateDraftFromVersion(ctx, domain.VersionRequest{Request: domain.Request{ID: draft.ID, ActorID: owner, Scope: scope}, Version: version.Version, ExpectedUpdatedAt: current.UpdatedAt}, now)
	if err != nil || len(restored.Sections) != 1 || !reflect.DeepEqual(restored.Skills, []string{"grammar", "reading"}) {
		t.Fatalf("restored test metadata=%+v err=%v", restored, err)
	}
	units = restored.Sections[0].Units
	if len(units) != 2 || units[0].Kind != "question" || units[1].Kind != "group" || units[0].ID == source.ID {
		t.Fatalf("restored standalone identity/unit mapping=%+v", units)
	}
	restoredQuestion, err := bank.Get(ctx, scope, units[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	assertFreshLegacyQuestion(t, source, restoredQuestion)
}

func assertFreshLegacyQuestion(t *testing.T, source, copied questions.Question) {
	t.Helper()
	if copied.ID == source.ID || source.Level == nil || source.Skill == nil || !reflect.DeepEqual(copied.Level, source.Level) || !reflect.DeepEqual(copied.Skill, source.Skill) || copied.Type != source.Type || copied.Prompt != source.Prompt || copied.Points != source.Points || len(source.Options) != 9 || len(copied.Options) != 9 {
		t.Fatalf("copied legacy question source=%+v copied=%+v", source, copied)
	}
	ids := map[string]bool{source.ID: true}
	for _, option := range source.Options {
		ids[option.ID] = true
	}
	if ids[copied.ID] {
		t.Fatal("copied question reuses a source option identity")
	}
	ids[copied.ID] = true
	for i, option := range copied.Options {
		before := source.Options[i]
		if option.ID == "" || ids[option.ID] || option.Text != before.Text || option.Ordinal != before.Ordinal || option.IsCorrect != before.IsCorrect {
			t.Fatalf("copied option %d source=%+v copied=%+v", i, before, option)
		}
		ids[option.ID] = true
	}
}

func assertFreshLegacyGroup(t *testing.T, source, copied domain.GroupBundle) {
	t.Helper()
	if copied.Group.ID == source.Group.ID || len(source.Questions) != 1 || len(copied.Questions) != 1 || len(copied.Group.Members) != 1 || copied.Group.Members[0].QuestionID != copied.Questions[0].ID || copied.Group.Members[0].OptionOrder != source.Group.Members[0].OptionOrder || copied.Questions[0].ID == source.Questions[0].ID {
		t.Fatalf("copied group identity/member mapping source=%+v copied=%+v", source, copied)
	}
	before, after := source.Questions[0].Input, copied.Questions[0].Input
	if before.Level == nil || before.Skill == nil || !reflect.DeepEqual(before.Level, after.Level) || !reflect.DeepEqual(before.Skill, after.Skill) || before.Type != after.Type || before.Prompt != after.Prompt || len(before.Options) != 9 || len(after.Options) != 9 {
		t.Fatalf("copied group metadata/legacy options before=%+v after=%+v", before, after)
	}
	ids := map[string]bool{source.Group.ID: true, source.Questions[0].ID: true}
	for _, option := range before.Options {
		if option.ID == nil {
			t.Fatal("source option identity missing")
		}
		ids[*option.ID] = true
	}
	if ids[copied.Group.ID] || ids[copied.Questions[0].ID] || copied.Group.ID == copied.Questions[0].ID {
		t.Fatal("copied group reuses a source graph identity")
	}
	ids[copied.Group.ID] = true
	ids[copied.Questions[0].ID] = true
	for i, option := range after.Options {
		if option.ID == nil || *option.ID == "" || ids[*option.ID] || option.Text != before.Options[i].Text || option.IsCorrect != before.Options[i].IsCorrect {
			t.Fatalf("copied group option %d=%+v", i, option)
		}
		ids[*option.ID] = true
	}
}

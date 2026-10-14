//go:build integration

package application_test

import (
	"context"
	"fmt"
	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
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

type moveWorld struct {
	t        *testing.T
	ctx      context.Context
	tx       pgx.Tx
	author   string
	scope    access.Scope
	tests    *repositories.Postgres
	groups   *repositories.GroupsPostgres
	bank     *questionsrepo.Postgres
	testID   string
	sections []string
}

func newMoveWorld(t *testing.T, sectionQuestions ...[]string) *moveWorld {
	t.Helper()
	tx, author, groups := groupTransaction(t)
	w := &moveWorld{t: t, ctx: context.Background(), tx: tx, author: author, scope: access.Scope{UserID: author}, groups: groups,
		bank: questionsrepo.NewPostgres(db.NewContext(tx))}
	w.tests = repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
	created, err := w.tests.Create(w.ctx, domain.CreateInput{Title: "Move fixture", ActorID: author, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	w.testID = created.ID
	w.outline(sectionQuestions...)
	return w
}

func (w *moveWorld) outline(sectionQuestions ...[]string) {
	w.t.Helper()
	sections := make([]domain.SectionInput, len(sectionQuestions))
	for i, ids := range sectionQuestions {
		sections[i] = domain.SectionInput{Title: fmt.Sprintf("Part %d", i+1), QuestionIDs: ids}
		if i < len(w.sections) {
			sections[i].ID = w.sections[i]
		}
	}
	saved, err := w.tests.Update(w.ctx, domain.UpdateRequest{ID: w.testID, ActorID: w.author, Now: time.Now(), Scope: w.scope,
		Input: domain.UpdateInput{ExpectedUpdatedAt: w.test().UpdatedAt, SetSections: true, Sections: sections}})
	if err != nil {
		w.t.Fatal(err)
	}
	w.sections = w.sections[:0]
	for _, section := range saved.Sections {
		w.sections = append(w.sections, section.ID)
	}
}

func (w *moveWorld) test() domain.Test {
	w.t.Helper()
	test, err := w.tests.Get(w.ctx, w.scope, w.testID)
	if err != nil {
		w.t.Fatal(err)
	}
	return test
}

func (w *moveWorld) question(prompt string) string {
	w.t.Helper()
	created, err := w.bank.Create(w.ctx, questionsdomain.WriteInput{ActorID: w.author, Now: time.Now(), Input: questionsdomain.Input{
		Type: questionsdomain.SingleChoice, Prompt: prompt, Points: "1.5", Tags: []string{"đọc hiểu"},
		Options: []questionsdomain.OptionInput{{Text: "đúng", IsCorrect: true}, {Text: "sai"}},
	}})
	if err != nil {
		w.t.Fatal(err)
	}
	return created.ID
}

func (w *moveWorld) plainGroup(section int, prompts ...string) domain.StoredGroup {
	w.t.Helper()
	bundle := domain.GroupBundle{Group: domain.QuestionGroup{ID: groupIdentity(w.t), Title: "Nhóm " + prompts[0], Stimuli: []domain.GroupStimulus{}, Recordings: []domain.GroupRecording{}}}
	for _, prompt := range prompts {
		id := groupIdentity(w.t)
		bundle.Group.Members = append(bundle.Group.Members, domain.GroupMember{QuestionID: id, OptionOrder: "shuffle"})
		bundle.Questions = append(bundle.Questions, domain.GroupQuestion{ID: id, Input: plainInput(prompt)})
	}
	return w.mount(section, bundle)
}

func (w *moveWorld) mount(section int, bundle domain.GroupBundle) domain.StoredGroup {
	w.t.Helper()
	stored, err := w.groups.Create(w.ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &w.sections[section], ExpectedTestUpdatedAt: w.test().UpdatedAt,
		ActorID: w.author, Now: time.Now(), Scope: w.scope, Grants: bothKeys})
	if err != nil {
		w.t.Fatal(err)
	}
	return stored
}

func (w *moveWorld) revision(group string) int64 {
	w.t.Helper()
	stored, err := w.groups.Get(w.ctx, w.scope, group)
	if err != nil {
		w.t.Fatal(err)
	}
	return stored.Revision
}

func (w *moveWorld) input(question string, section int, group *string, index int, touched ...string) domain.MoveQuestionInput {
	w.t.Helper()
	expected := make([]domain.GroupRevision, len(touched))
	for i, id := range touched {
		expected[i] = domain.GroupRevision{GroupID: id, Revision: w.revision(id)}
	}
	return domain.MoveQuestionInput{TestID: w.testID, QuestionID: question, ExpectedTestUpdatedAt: w.test().UpdatedAt,
		To: domain.MoveTarget{SectionID: w.sections[section], GroupID: group, Index: index}, ExpectedGroups: expected,
		ActorID: w.author, Now: time.Now(), Scope: w.scope, Grants: bothKeys}
}

func (w *moveWorld) move(question string, section int, group *string, index int, touched ...string) (domain.MoveQuestionResult, error) {
	w.t.Helper()
	return w.groups.MoveQuestion(w.ctx, w.input(question, section, group, index, touched...))
}

func (w *moveWorld) mustMove(question string, section int, group *string, index int, touched ...string) domain.MoveQuestionResult {
	w.t.Helper()
	result, err := w.move(question, section, group, index, touched...)
	if err != nil {
		w.t.Fatalf("move: %v", err)
	}
	return result
}

func (w *moveWorld) members(group string) []string {
	w.t.Helper()
	return w.ordered(`SELECT id::text, context_ordinal FROM app.questions WHERE context_group_id=$1 ORDER BY context_ordinal`, group)
}

func (w *moveWorld) standalone(section int) []string {
	w.t.Helper()
	return w.ordered(`SELECT question_id::text, ordinal FROM app.test_section_questions WHERE test_section_id=$1 ORDER BY ordinal`, w.sections[section])
}

func (w *moveWorld) units(section int) []string {
	w.t.Helper()
	return w.ordered(`SELECT CASE WHEN group_id IS NULL THEN 'q:'||question_id::text ELSE 'g:'||group_id::text END, ordinal
		FROM app.test_section_units WHERE test_section_id=$1 ORDER BY ordinal`, w.sections[section])
}

func (w *moveWorld) ordered(query string, args ...any) []string {
	w.t.Helper()
	rows, err := w.tx.Query(w.ctx, query, args...)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var item string
		var ordinal int
		if err := rows.Scan(&item, &ordinal); err != nil {
			w.t.Fatal(err)
		}
		if ordinal != len(out) {
			w.t.Fatalf("ordinals are not contiguous: %q is at %d, want %d", item, ordinal, len(out))
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		w.t.Fatal(err)
	}
	return out
}

func (w *moveWorld) state() string {
	w.t.Helper()
	var out strings.Builder
	fmt.Fprintf(&out, "test %s\n", w.test().UpdatedAt.Format(time.RFC3339Nano))
	for i := range w.sections {
		fmt.Fprintf(&out, "section %d units %v standalone %v\n", i, w.units(i), w.standalone(i))
	}
	rows, err := w.tx.Query(w.ctx, `SELECT g.id::text, g.revision FROM app.question_groups g JOIN app.test_sections s ON s.id=g.owner_section_id WHERE s.test_id=$1 ORDER BY g.id`, w.testID)
	if err != nil {
		w.t.Fatal(err)
	}
	type groupRow struct {
		id       string
		revision int64
	}
	found, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (groupRow, error) {
		var g groupRow
		return g, row.Scan(&g.id, &g.revision)
	})
	if err != nil {
		w.t.Fatal(err)
	}
	for _, g := range found {
		fmt.Fprintf(&out, "group %s revision %d members %v\n", g.id, g.revision, w.members(g.id))
	}
	var events int
	if err := w.tx.QueryRow(w.ctx, `SELECT count(*) FROM app.audit_log WHERE entity_id=$1 AND action='test.question_moved'`, w.testID).Scan(&events); err != nil {
		w.t.Fatal(err)
	}
	fmt.Fprintf(&out, "move audit rows %d\n", events)
	return out.String()
}

func (w *moveWorld) eventCount(action string) int {
	w.t.Helper()
	var count int
	if err := w.tx.QueryRow(w.ctx, `SELECT count(*) FROM app.audit_log WHERE actor_user_id=$1 AND action=$2`, w.author, action).Scan(&count); err != nil {
		w.t.Fatal(err)
	}
	return count
}

func (w *moveWorld) asBank(question string) (standalone bool, deleted bool) {
	w.t.Helper()
	if err := w.tx.QueryRow(w.ctx, `SELECT context_group_id IS NULL AND context_ordinal IS NULL AND context_option_order IS NULL, deleted_at IS NOT NULL
		FROM app.questions WHERE id=$1`, question).Scan(&standalone, &deleted); err != nil {
		w.t.Fatal(err)
	}
	return standalone, deleted
}

func ptr(id string) *string { return &id }

func plainInput(prompt string) questionsdomain.Input {
	return questionsdomain.Input{Type: questionsdomain.ShortAnswer, Prompt: prompt, Points: "1", Tags: []string{}}
}

func (w *moveWorld) publish() int {
	w.t.Helper()
	version, err := w.tests.Publish(w.ctx, domain.PublishRequest{TestID: w.testID, ActorID: w.author, Scope: w.scope}, time.Now(), func(domain.DraftContent) error { return nil })
	if err != nil {
		w.t.Fatalf("publish: %v", err)
	}
	return version.Version
}

func memberIDs(group domain.StoredGroup) []string {
	ids := make([]string, len(group.Bundle.Group.Members))
	for i, member := range group.Bundle.Group.Members {
		ids[i] = member.QuestionID
	}
	return ids
}

func requireOrder(t *testing.T, name string, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("%s:\n got %v\nwant %v", name, got, want)
	}
}

type stranger struct {
	testID, sectionID, groupID, memberID string
}

func (w *moveWorld) stranger() stranger {
	w.t.Helper()
	other := scopedUser(w.t, w.tx, "teacher")
	var s stranger
	var updated time.Time
	if err := w.tx.QueryRow(w.ctx, `INSERT INTO app.tests (title,created_by,owner_id) VALUES ('Stranger',$1,$1) RETURNING id::text,updated_at`, other).Scan(&s.testID, &updated); err != nil {
		w.t.Fatal(err)
	}
	if err := w.tx.QueryRow(w.ctx, `INSERT INTO app.test_sections (test_id,ordinal,title) VALUES ($1,0,'Part') RETURNING id::text`, s.testID).Scan(&s.sectionID); err != nil {
		w.t.Fatal(err)
	}
	bundle := domain.GroupBundle{Group: domain.QuestionGroup{ID: groupIdentity(w.t), Title: "Nhóm của người khác", Stimuli: []domain.GroupStimulus{}, Recordings: []domain.GroupRecording{}}}
	member := groupIdentity(w.t)
	bundle.Group.Members = []domain.GroupMember{{QuestionID: member, OptionOrder: "shuffle"}}
	bundle.Questions = []domain.GroupQuestion{{ID: member, Input: plainInput("x")}}
	stored, err := w.groups.Create(w.ctx, domain.CreateGroupInput{Bundle: bundle, OwnerSectionID: &s.sectionID, ExpectedTestUpdatedAt: updated,
		ActorID: other, Now: time.Now(), Scope: access.Scope{UserID: other}, Grants: bothKeys})
	if err != nil {
		w.t.Fatal(err)
	}
	s.groupID, s.memberID = stored.Bundle.Group.ID, member
	return s
}

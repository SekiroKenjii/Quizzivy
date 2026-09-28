//go:build integration

package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"quizzivy/internal/core/adapters"
	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/modules/tests/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

type scopeWorld struct {
	tx                     pgx.Tx
	tests                  *repositories.Postgres
	groups                 *repositories.GroupsPostgres
	a, b, admin            string
	scopeA, scopeB, anyone access.Scope
	testA, sectionA        string
	questionA, questionB   string
	bankGroupA             domain.StoredGroup
	sectionGroupA          domain.StoredGroup
}

func scopedUser(t *testing.T, tx pgx.Tx, builtin string) string {
	t.Helper()
	var id string
	if err := tx.QueryRow(context.Background(),
		`INSERT INTO app.users (email, full_name, role_id) VALUES ($1, 'Scope fixture', (SELECT id FROM app.roles WHERE builtin_key = $2)) RETURNING id::text`,
		uuid.NewString()+"@example.test", builtin).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func scopedQuestion(t *testing.T, tx pgx.Tx, owner, tag string) string {
	t.Helper()
	var id string
	if err := tx.QueryRow(context.Background(),
		`INSERT INTO app.questions (type, prompt, points, created_by, owner_id, tags) VALUES ('short_answer', 'Scope fixture', 1, $1, $1, ARRAY[$2]) RETURNING id::text`,
		owner, tag).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func newScopeWorld(t *testing.T) *scopeWorld {
	t.Helper()
	ctx := context.Background()
	tx, _, groups := groupTransaction(t)
	w := &scopeWorld{tx: tx, groups: groups}
	w.tests = repositories.NewPostgres(db.NewContext(tx), adapters.GroupQuestions{}, mediarepo.NewPostgres(db.NewContext(tx))).WithGroupQuestions(adapters.GroupQuestions{})
	w.a, w.b, w.admin = scopedUser(t, tx, "teacher"), scopedUser(t, tx, "teacher"), scopedUser(t, tx, "admin")
	w.scopeA, w.scopeB, w.anyone = access.Scope{UserID: w.a}, access.Scope{UserID: w.b}, access.Scope{UserID: w.admin, All: true}
	w.questionA = scopedQuestion(t, tx, w.a, "tag-of-a-"+groupIdentity(t))
	w.questionB = scopedQuestion(t, tx, w.b, "tag-of-b-"+groupIdentity(t))

	created, err := w.tests.Create(ctx, domain.CreateInput{Title: "A's test", ActorID: w.a, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := w.tests.Update(ctx, domain.UpdateRequest{ID: created.ID, ActorID: w.a, Now: time.Now(), Scope: w.scopeA,
		Input: domain.UpdateInput{ExpectedUpdatedAt: created.UpdatedAt, SetSections: true, Sections: []domain.SectionInput{{Title: "Part 1", QuestionIDs: []string{w.questionA}}}}})
	if err != nil {
		t.Fatal(err)
	}
	w.testA, w.sectionA = updated.ID, updated.Sections[0].ID
	if _, err := w.tests.Publish(ctx, domain.PublishRequest{TestID: w.testA, ActorID: w.a, Scope: w.scopeA}, time.Now(), func(domain.DraftContent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	w.bankGroupA, err = groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, ""), ActorID: w.a, Now: time.Now(), Scope: w.scopeA, Grants: bothKeys})
	if err != nil {
		t.Fatal(err)
	}
	w.sectionGroupA, err = groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, ""), OwnerSectionID: &w.sectionA,
		ExpectedTestUpdatedAt: w.current(t, w.testA).UpdatedAt, ActorID: w.a, Now: time.Now(), Scope: w.scopeA, Grants: bothKeys})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func (w *scopeWorld) current(t *testing.T, id string) domain.Test {
	t.Helper()
	test, err := w.tests.Get(context.Background(), w.anyone, id)
	if err != nil {
		t.Fatal(err)
	}
	return test
}

func (w *scopeWorld) request(scope access.Scope, actorID string) domain.Request {
	return domain.Request{ID: w.testA, ActorID: actorID, Scope: scope}
}

func expectNotFound(t *testing.T, name string, err error) {
	t.Helper()
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("%s: %v, want not found", name, err)
	}
}

func TestAnotherTeachersTestAnswersAsAMissingOneWould(t *testing.T) {
	ctx := context.Background()
	w := newScopeWorld(t)
	now := time.Now()
	current := w.current(t, w.testA)
	versionOf := func(scope access.Scope, actorID string) domain.VersionRequest {
		return domain.VersionRequest{Request: w.request(scope, actorID), Version: 1, ExpectedUpdatedAt: current.UpdatedAt}
	}

	_, err := w.tests.Get(ctx, w.scopeB, w.testA)
	expectNotFound(t, "get", err)
	_, err = w.tests.Update(ctx, domain.UpdateRequest{ID: w.testA, ActorID: w.b, Now: now, Scope: w.scopeB,
		Input: domain.UpdateInput{ExpectedUpdatedAt: current.UpdatedAt}})
	expectNotFound(t, "update", err)
	_, err = w.tests.Update(ctx, domain.UpdateRequest{ID: w.testA, ActorID: w.b, Now: now, Scope: w.scopeB,
		Input: domain.UpdateInput{ExpectedUpdatedAt: now.Add(-time.Hour)}})
	expectNotFound(t, "update with a stale revision, before the revision is compared", err)
	_, err = w.tests.Duplicate(ctx, domain.DuplicateInput{ID: w.testA, ActorID: w.b, Now: now, Scope: w.scopeB})
	expectNotFound(t, "duplicate", err)
	_, err = w.tests.SetCurrentVersion(ctx, versionOf(w.scopeB, w.b), now)
	expectNotFound(t, "set current version", err)
	_, err = w.tests.CreateDraftFromVersion(ctx, versionOf(w.scopeB, w.b), now)
	expectNotFound(t, "restore as draft", err)
	expectNotFound(t, "delete version", w.tests.DeleteVersion(ctx, versionOf(w.scopeB, w.b), now))
	expectNotFound(t, "delete, before the archive check", w.tests.Delete(ctx, w.request(w.scopeB, w.b), now))
	if _, err := w.tests.Publish(ctx, domain.PublishRequest{TestID: w.testA, ActorID: w.b, Scope: w.scopeB}, now, func(domain.DraftContent) error { return nil }); !errors.Is(err, domain.ErrDraftNotFound) {
		t.Errorf("publish: %v, want the draft not found", err)
	}
	if _, err := w.tests.Preview(ctx, w.scopeB, w.testA, 0); !errors.Is(err, domain.ErrNotPublished) {
		t.Errorf("preview: %v, want not published, as an unknown id answers", err)
	}
	if versions, err := w.tests.ListVersions(ctx, w.scopeB, w.testA); err != nil || len(versions) != 0 {
		t.Errorf("versions: %d, %v, want none, as an unknown id has", len(versions), err)
	}
	if versions, err := w.tests.ListVersions(ctx, w.scopeA, w.testA); err != nil || len(versions) != 1 {
		t.Errorf("the owner's versions: %d, %v, want 1", len(versions), err)
	}
	_, err = w.tests.Get(ctx, access.Scope{}, w.testA)
	expectNotFound(t, "get with no scope at all", err)
}

func TestListsFacetsAndTagsOmitAnotherTeachersTests(t *testing.T) {
	ctx := context.Background()
	w := newScopeWorld(t)
	for name, c := range map[string]struct {
		scope access.Scope
		sees  bool
	}{"another teacher": {w.scopeB, false}, "the owner": {w.scopeA, true}, "the Admin": {w.anyone, true}, "no scope": {access.Scope{}, false}} {
		found, _, err := w.tests.List(ctx, domain.ListInput{Limit: 100, Query: "A's test", Scope: c.scope})
		if err != nil {
			t.Fatal(err)
		}
		listed := slices.ContainsFunc(found, func(test domain.Test) bool { return test.ID == w.testA })
		tags, err := w.tests.Tags(ctx, domain.ListInput{Scope: c.scope})
		if err != nil {
			t.Fatal(err)
		}
		tagged := slices.ContainsFunc(tags, func(tag string) bool { return len(tag) > 9 && tag[:9] == "tag-of-a-" })
		facets, err := w.tests.Facets(ctx, domain.ListInput{Query: "A's test", Scope: c.scope})
		if err != nil {
			t.Fatal(err)
		}
		if listed != c.sees || tagged != c.sees || (facets.All > 0) != c.sees {
			t.Errorf("%s: listed %v, tag offered %v, facets %d; want the test seen: %v", name, listed, tagged, facets.All, c.sees)
		}
	}
}

func TestAnotherTeachersGroupsAnswerAsMissingOnes(t *testing.T) {
	ctx := context.Background()
	w := newScopeWorld(t)
	now := time.Now()
	for name, group := range map[string]domain.StoredGroup{"bank group": w.bankGroupA, "section group": w.sectionGroupA} {
		mutation := domain.GroupMutation{ID: group.Bundle.Group.ID, ExpectedRevision: group.Revision, ActorID: w.b, Now: now, Scope: w.scopeB, Grants: bothKeys}
		if group.OwnerSectionID != nil {
			mutation.ExpectedTestUpdatedAt = w.current(t, w.testA).UpdatedAt
		}
		_, err := w.groups.Get(ctx, w.scopeB, group.Bundle.Group.ID)
		expectNotFound(t, name+": get", err)
		_, err = w.groups.Update(ctx, domain.UpdateGroupInput{GroupMutation: mutation, Bundle: group.Bundle})
		expectNotFound(t, name+": update", err)
		expectNotFound(t, name+": delete", w.groups.Delete(ctx, mutation))
		_, err = w.groups.Copy(ctx, domain.CopyGroupInput{SourceID: group.Bundle.Group.ID, ExpectedSourceRevision: group.Revision, ActorID: w.b, Now: now, Scope: w.scopeB, Grants: bothKeys})
		expectNotFound(t, name+": copy from", err)
		if _, err := w.groups.Get(ctx, w.scopeA, group.Bundle.Group.ID); err != nil {
			t.Errorf("%s: the owner cannot read it: %v", name, err)
		}
		if _, err := w.groups.Get(ctx, w.anyone, group.Bundle.Group.ID); err != nil {
			t.Errorf("%s: the Admin cannot read it: %v", name, err)
		}
	}
	bank := domain.GroupMutation{ID: w.bankGroupA.Bundle.Group.ID, ExpectedRevision: w.bankGroupA.Revision, ActorID: w.b, Now: now, Scope: w.scopeB, Grants: bothKeys}
	_, err := w.groups.SetArchived(ctx, bank, true)
	expectNotFound(t, "archive", err)
	_, err = w.groups.Create(ctx, domain.CreateGroupInput{Bundle: storedGroupFixture(t, ""), OwnerSectionID: &w.sectionA,
		ExpectedTestUpdatedAt: w.current(t, w.testA).UpdatedAt, ActorID: w.b, Now: now, Scope: w.scopeB, Grants: bothKeys})
	expectNotFound(t, "create in another teacher's section", err)
	listed, _, err := w.groups.List(ctx, domain.GroupListInput{Status: "all", Limit: 100, Scope: w.scopeB})
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(listed, func(g domain.GroupSummary) bool { return g.ID == w.bankGroupA.Bundle.Group.ID }) {
		t.Error("another teacher's bank list shows the group")
	}
}

func TestADraftReferencesOnlyItsOwnersQuestions(t *testing.T) {
	ctx := context.Background()
	w := newScopeWorld(t)
	created, err := w.tests.Create(ctx, domain.CreateInput{Title: "B's test", ActorID: w.b, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	outline := func(scope access.Scope, testID string, updated time.Time, question string) error {
		_, err := w.tests.Update(ctx, domain.UpdateRequest{ID: testID, ActorID: scope.UserID, Now: time.Now(), Scope: scope,
			Input: domain.UpdateInput{ExpectedUpdatedAt: updated, SetSections: true, Sections: []domain.SectionInput{{Title: "Part 1", QuestionIDs: []string{question}}}}})
		return err
	}
	foreign := outline(w.scopeB, created.ID, created.UpdatedAt, w.questionA)
	missing := outline(w.scopeB, created.ID, created.UpdatedAt, uuid.NewString())
	if !errors.Is(foreign, domain.ErrUnknownQuestion) || !errors.Is(missing, domain.ErrUnknownQuestion) {
		t.Errorf("another teacher's question: %v; a missing one: %v; want the same unknown-question error for both", foreign, missing)
	}
	if err := outline(w.scopeB, created.ID, created.UpdatedAt, w.questionB); err != nil {
		t.Errorf("B's own question: %v", err)
	}
	another, err := w.tests.Create(ctx, domain.CreateInput{Title: "A's second test", ActorID: w.a, Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if err := outline(w.anyone, another.ID, another.UpdatedAt, w.questionB); !errors.Is(err, domain.ErrUnknownQuestion) {
		t.Errorf("the Admin putting B's question in A's test: %v, want unknown question: a draft holds only its owner's questions", err)
	}
}

func TestTheAdminEditsAnotherTeachersTestWithoutTakingIt(t *testing.T) {
	ctx := context.Background()
	w := newScopeWorld(t)
	title := "Sửa bởi quản trị viên"
	current := w.current(t, w.testA)
	edited, err := w.tests.Update(ctx, domain.UpdateRequest{ID: w.testA, ActorID: w.admin, Now: time.Now(), Scope: w.anyone,
		Input: domain.UpdateInput{ExpectedUpdatedAt: current.UpdatedAt, Title: &title}})
	if err != nil {
		t.Fatal(err)
	}
	var owner string
	if err := w.tx.QueryRow(ctx, `SELECT owner_id::text FROM app.tests WHERE id = $1`, w.testA).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if edited.Title != title || owner != w.a {
		t.Errorf("title %q, owner %s; want the edit applied and A still the owner", edited.Title, owner)
	}
}

type keyCase struct {
	name string
	err  error
	want error
}

func TestGroupWritesNeedTheKeyTheirTargetNeeds(t *testing.T) {
	ctx := context.Background()
	w := newScopeWorld(t)
	questionsOnly := access.NewSet(access.ContentQuestionsWrite)
	testsOnly := access.NewSet(access.ContentTestsWrite)
	now := time.Now()
	create := func(section *string, grants access.Set, scope access.Scope) error {
		in := domain.CreateGroupInput{Bundle: storedGroupFixture(t, ""), OwnerSectionID: section, ActorID: scope.UserID, Now: now, Scope: scope, Grants: grants}
		if section != nil {
			in.ExpectedTestUpdatedAt = w.current(t, w.testA).UpdatedAt
		}
		_, err := w.groups.Create(ctx, in)
		return err
	}
	cases := []keyCase{
		{"a bank group with only the tests key", create(nil, testsOnly, w.scopeA), domain.ErrForbidden},
		{"a section group with only the questions key", create(&w.sectionA, questionsOnly, w.scopeA), domain.ErrForbidden},
		{"another teacher's section with the wrong key", create(&w.sectionA, questionsOnly, w.scopeB), domain.ErrNotFound},
		{"a bank group with the questions key", create(nil, questionsOnly, w.scopeA), nil},
		{"a section group with the tests key", create(&w.sectionA, testsOnly, w.scopeA), nil},
	}
	mutate := func(group domain.StoredGroup, grants access.Set, scope access.Scope) domain.GroupMutation {
		m := domain.GroupMutation{ID: group.Bundle.Group.ID, ExpectedRevision: group.Revision, ActorID: scope.UserID, Now: now, Scope: scope, Grants: grants}
		if group.OwnerSectionID != nil {
			m.ExpectedTestUpdatedAt = w.current(t, w.testA).UpdatedAt
		}
		return m
	}
	update := func(group domain.StoredGroup, grants access.Set, scope access.Scope) error {
		_, err := w.groups.Update(ctx, domain.UpdateGroupInput{GroupMutation: mutate(group, grants, scope), Bundle: group.Bundle})
		return err
	}
	copyTo := func(section *string, grants access.Set) error {
		in := domain.CopyGroupInput{SourceID: w.bankGroupA.Bundle.Group.ID, ExpectedSourceRevision: w.bankGroupA.Revision, OwnerSectionID: section, ActorID: w.a, Now: now, Scope: w.scopeA, Grants: grants}
		if section != nil {
			in.ExpectedTestUpdatedAt = w.current(t, w.testA).UpdatedAt
		}
		_, err := w.groups.Copy(ctx, in)
		return err
	}
	cases = append(cases,
		keyCase{"updating a bank group with only the tests key", update(w.bankGroupA, testsOnly, w.scopeA), domain.ErrForbidden},
		keyCase{"updating a section group with only the questions key", update(w.sectionGroupA, questionsOnly, w.scopeA), domain.ErrForbidden},
		keyCase{"updating another teacher's group with the wrong key", update(w.bankGroupA, testsOnly, w.scopeB), domain.ErrNotFound},
		keyCase{"deleting a section group with only the questions key", w.groups.RemoveFromSection(ctx, mutate(w.sectionGroupA, questionsOnly, w.scopeA)), domain.ErrForbidden},
		keyCase{"deleting a bank group with only the tests key", w.groups.Delete(ctx, mutate(w.bankGroupA, testsOnly, w.scopeA)), domain.ErrForbidden},
		keyCase{"copying into the bank with only the tests key", copyTo(nil, testsOnly), domain.ErrForbidden},
		keyCase{"copying into a section with only the questions key", copyTo(&w.sectionA, questionsOnly), domain.ErrForbidden},
		keyCase{"updating a bank group with the questions key", update(w.bankGroupA, questionsOnly, w.scopeA), nil},
	)
	for _, c := range cases {
		if c.want == nil && c.err != nil || c.want != nil && !errors.Is(c.err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, c.err, c.want)
		}
	}
}

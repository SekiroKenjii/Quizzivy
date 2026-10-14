//go:build integration

package application_test

import (
	"errors"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
	"reflect"
	"testing"
	"time"
)

func TestMoveInCopiesAndLeavesTheBankQuestion(t *testing.T) {
	w := newMoveWorld(t)
	bank, other := w.question("Câu hỏi ngân hàng"), w.question("Câu khác")
	w.outline([]string{bank, other})
	group := w.plainGroup(0, "m0", "m1")
	members := memberIDs(group)
	id := group.Bundle.Group.ID
	before, err := w.bank.Get(w.ctx, w.scope, bank)
	if err != nil {
		t.Fatal(err)
	}

	result := w.mustMove(bank, 0, ptr(id), 1, id)

	if !result.Copied || result.QuestionID == bank {
		t.Fatalf("a copy-in must say so and name the copy: copied=%v id=%s", result.Copied, result.QuestionID)
	}
	requireOrder(t, "members", w.members(id), members[0], result.QuestionID, members[1])
	requireOrder(t, "units", w.units(0), "q:"+other, "g:"+id)
	requireOrder(t, "standalone", w.standalone(0), other)
	requireOrder(t, "the answer's outline", result.Test.Sections[0].QuestionIDs, other)
	if len(result.Groups) != 1 || result.Groups[0].Revision != group.Revision+1 || result.Groups[0].Bundle.Group.Members[1].QuestionID != result.QuestionID {
		t.Fatalf("the answer must carry the group at its new revision: %+v", result.Groups)
	}
	if result.Test.UpdatedAt.IsZero() {
		t.Fatal("the answer has no test revision")
	}

	after, err := w.bank.Get(w.ctx, w.scope, bank)
	if err != nil {
		t.Fatalf("the bank question is gone: %v", err)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) || !reflect.DeepEqual(after.Options, before.Options) || after.Prompt != before.Prompt || after.Points != before.Points {
		t.Fatalf("the bank question was touched:\nbefore %+v\nafter  %+v", before, after)
	}
	if standalone, deleted := w.asBank(bank); !standalone || deleted {
		t.Fatalf("the bank question left the bank: standalone=%v deleted=%v", standalone, deleted)
	}

	stored, err := w.groups.Get(w.ctx, w.scope, id)
	if err != nil {
		t.Fatal(err)
	}
	copied := stored.Bundle.Questions[1]
	if copied.Input.Prompt != before.Prompt || copied.Input.Points != before.Points || !reflect.DeepEqual(copied.Input.Tags, before.Tags) || stored.Bundle.Group.Members[1].OptionOrder != "shuffle" {
		t.Fatalf("the copy changed the question: %+v", copied)
	}
	for i, option := range copied.Input.Options {
		if *option.ID == before.Options[i].ID || option.Text != before.Options[i].Text || option.IsCorrect != before.Options[i].IsCorrect {
			t.Fatalf("option %d: identity reused or content changed", i)
		}
	}
	var owner string
	if err := w.tx.QueryRow(w.ctx, `SELECT owner_id::text FROM app.questions WHERE id=$1`, result.QuestionID).Scan(&owner); err != nil || owner != w.author {
		t.Fatalf("the copy's owner is %q, %v", owner, err)
	}
	if refs, err := questionsrepo.DraftReferences(w.ctx, w.tx, bank); err != nil || len(refs) != 0 {
		t.Fatalf("the draft still references the bank question: %v, %v", refs, err)
	}
	if moved, updated := w.eventCount("test.question_moved"), w.eventCount("question_group.updated"); moved != 1 || updated != 1 {
		t.Fatalf("audit rows: moved=%d group=%d", moved, updated)
	}
}

func TestMoveOutKeepsTheIdAndReturnsItToTheBank(t *testing.T) {
	w := newMoveWorld(t)
	lead := w.question("Câu dẫn")
	w.outline([]string{lead})
	group := w.plainGroup(0, "m0", "m1", "m2")
	members := memberIDs(group)
	id := group.Bundle.Group.ID

	result := w.mustMove(members[1], 0, nil, 0, id)

	if result.Copied || result.QuestionID != members[1] {
		t.Fatalf("a member keeps its id on the way out: copied=%v id=%s", result.Copied, result.QuestionID)
	}
	requireOrder(t, "members", w.members(id), members[0], members[2])
	requireOrder(t, "units", w.units(0), "q:"+members[1], "q:"+lead, "g:"+id)
	requireOrder(t, "standalone", w.standalone(0), members[1], lead)
	if standalone, deleted := w.asBank(members[1]); !standalone || deleted {
		t.Fatalf("the member is not an ordinary bank question: standalone=%v deleted=%v", standalone, deleted)
	}
	if got, err := w.bank.Get(w.ctx, w.scope, members[1]); err != nil || got.Prompt != "m1" {
		t.Fatalf("the bank cannot read it: %+v, %v", got, err)
	}
	err := w.bank.SoftDelete(w.ctx, questionsdomain.WriteInput{ID: members[1], ActorID: w.author, Now: time.Now()})
	if !errors.Is(err, questionsdomain.ErrReferenced) {
		t.Fatalf("deleting the question the draft now references must be refused: %v", err)
	}
	if result.Groups[0].Revision != group.Revision+1 {
		t.Fatalf("revision %d, want %d", result.Groups[0].Revision, group.Revision+1)
	}

	saved := result.Test.Sections[0]
	units := make([]domain.SectionUnit, len(saved.Units))
	copy(units, saved.Units)
	if _, err := w.tests.Update(w.ctx, domain.UpdateRequest{ID: w.testID, ActorID: w.author, Now: time.Now(), Scope: w.scope, Input: domain.UpdateInput{
		ExpectedUpdatedAt: result.Test.UpdatedAt, SetSections: true, GroupOutline: true,
		Sections: []domain.SectionInput{{ID: saved.ID, Title: saved.Title, QuestionIDs: saved.QuestionIDs, Units: units, SetUnits: true}},
	}}); err != nil {
		t.Fatalf("the next autosave refused the moved question: %v", err)
	}
}

func TestMoveBetweenGroupsBumpsBothRevisions(t *testing.T) {
	w := newMoveWorld(t)
	w.outline(nil)
	a, b := w.plainGroup(0, "a0", "a1", "a2"), w.plainGroup(0, "b0", "b1")
	ma, mb := memberIDs(a), memberIDs(b)
	aid, bid := a.Bundle.Group.ID, b.Bundle.Group.ID
	testBefore := w.test().UpdatedAt

	result := w.mustMove(ma[1], 0, ptr(bid), 1, aid, bid)

	if result.Copied || result.QuestionID != ma[1] {
		t.Fatalf("a member keeps its id between groups: copied=%v id=%s", result.Copied, result.QuestionID)
	}
	requireOrder(t, "source members", w.members(aid), ma[0], ma[2])
	requireOrder(t, "target members", w.members(bid), mb[0], ma[1], mb[1])
	if len(result.Groups) != 2 || result.Groups[0].Bundle.Group.ID != aid || result.Groups[1].Bundle.Group.ID != bid {
		t.Fatalf("the answer must carry the source group, then the target")
	}
	if w.revision(aid) != a.Revision+1 || w.revision(bid) != b.Revision+1 || result.Groups[0].Revision != a.Revision+1 || result.Groups[1].Revision != b.Revision+1 {
		t.Fatalf("both revisions must move once: %d %d", w.revision(aid), w.revision(bid))
	}
	if !w.test().UpdatedAt.After(testBefore) && !w.test().UpdatedAt.Equal(testBefore) {
		t.Fatal("the test revision went backwards")
	}
	requireOrder(t, "units", w.units(0), "g:"+aid, "g:"+bid)

	again := w.mustMove(ma[1], 0, ptr(bid), 0, bid)
	requireOrder(t, "reordered members", w.members(bid), ma[1], mb[0], mb[1])
	if len(again.Groups) != 1 || w.revision(bid) != b.Revision+2 || w.revision(aid) != a.Revision+1 {
		t.Fatalf("reordering inside a group bumps that group once and no other: a=%d b=%d", w.revision(aid), w.revision(bid))
	}
}

func TestMoveClampsIndexAndCompactsOrdinals(t *testing.T) {
	w := newMoveWorld(t)
	q0, q1, q2 := w.question("q0"), w.question("q1"), w.question("q2")
	w.outline([]string{q0, q1, q2})
	group := w.plainGroup(0, "m0", "m1")
	members := memberIDs(group)
	id := group.Bundle.Group.ID

	in := w.mustMove(q1, 0, ptr(id), 99, id)
	copied := in.QuestionID
	requireOrder(t, "members after a copy-in past the end", w.members(id), members[0], members[1], copied)
	requireOrder(t, "units after the standalone left", w.units(0), "q:"+q0, "q:"+q2, "g:"+id)
	requireOrder(t, "standalone after the standalone left", w.standalone(0), q0, q2)

	w.mustMove(members[0], 0, nil, 99, id)
	requireOrder(t, "units after a move-out past the end", w.units(0), "q:"+q0, "q:"+q2, "g:"+id, "q:"+members[0])
	requireOrder(t, "standalone after a move-out past the end", w.standalone(0), q0, q2, members[0])
	requireOrder(t, "members after a move-out", w.members(id), members[1], copied)

	w.mustMove(copied, 0, nil, 1, id)
	requireOrder(t, "units in the middle, the unedited copy having gone back as its original", w.units(0), "q:"+q0, "q:"+q1, "q:"+q2, "g:"+id, "q:"+members[0])
	requireOrder(t, "standalone in the middle", w.standalone(0), q0, q1, q2, members[0])

	w.mustMove(members[1], 0, nil, -5, id)
	requireOrder(t, "units at the front", w.units(0), "q:"+members[1], "q:"+q0, "q:"+q1, "q:"+q2, "g:"+id, "q:"+members[0])
	requireOrder(t, "members of the emptied group", w.members(id))
	requireOrder(t, "standalone at the front", w.standalone(0), members[1], q0, q1, q2, members[0])
	if got := w.revision(id); got != group.Revision+4 {
		t.Fatalf("an emptied group stays, at revision %d, want %d", got, group.Revision+4)
	}
}

func TestMoveOutOfAGapBoundMemberIsRefused(t *testing.T) {
	w := newMoveWorld(t)
	w.outline(nil)
	bound := w.mount(0, storedGroupFixture(t, ""))
	other := w.plainGroup(0, "free")
	id := bound.Bundle.Group.ID
	stimulus := bound.Bundle.Group.Stimuli[0].ID
	before := w.state()

	for name, attempt := range map[string]func() (domain.MoveQuestionResult, error){
		"to the section": func() (domain.MoveQuestionResult, error) { return w.move(memberIDs(bound)[0], 0, nil, 0, id) },
		"to another group": func() (domain.MoveQuestionResult, error) {
			return w.move(memberIDs(bound)[0], 0, ptr(other.Bundle.Group.ID), 0, id, other.Bundle.Group.ID)
		},
		"a blank-bound one": func() (domain.MoveQuestionResult, error) { return w.move(memberIDs(bound)[1], 0, nil, 0, id) },
	} {
		_, err := attempt()
		var refused *domain.GroupError
		if !errors.As(err, &refused) || refused.Rule != domain.RuleGroupGapBound || refused.StimulusID != stimulus || refused.QuestionID == "" || refused.GapID == "" {
			t.Fatalf("%s: %v, want group_gap_bound with the stimulus, gap and question", name, err)
		}
		if after := w.state(); after != before {
			t.Fatalf("%s: a refused move changed the draft:\nbefore %s\nafter  %s", name, before, after)
		}
	}

	if _, err := w.move(memberIDs(bound)[0], 0, ptr(id), 1, id); err != nil {
		t.Fatalf("a bound member can still be reordered inside its group: %v", err)
	}
	requireOrder(t, "reordered", w.members(id), memberIDs(bound)[1], memberIDs(bound)[0])
}

func TestMoveSeedsALegacySectionItTouches(t *testing.T) {
	t.Run("a standalone question leaves a section that has no units", func(t *testing.T) {
		w := newMoveWorld(t)
		q1, q2 := w.question("q1"), w.question("q2")
		w.outline([]string{q1, q2}, nil)
		group := w.plainGroup(1, "m0")
		id := group.Bundle.Group.ID
		if got := w.units(0); len(got) != 0 {
			t.Fatalf("the untouched section already has units: %v", got)
		}

		copied := w.mustMove(q1, 1, ptr(id), 0, id)

		requireOrder(t, "units", w.units(0), "q:"+q2)
		requireOrder(t, "standalone", w.standalone(0), q2)
		requireOrder(t, "members", w.members(id), copied.QuestionID, memberIDs(group)[0])
	})
	t.Run("a member lands in a section that has no units", func(t *testing.T) {
		w := newMoveWorld(t)
		q1, q2 := w.question("q1"), w.question("q2")
		w.outline([]string{q1, q2}, nil)
		group := w.plainGroup(1, "m0")
		member := memberIDs(group)[0]

		w.mustMove(member, 0, nil, 1, group.Bundle.Group.ID)

		requireOrder(t, "units", w.units(0), "q:"+q1, "q:"+member, "q:"+q2)
		requireOrder(t, "standalone", w.standalone(0), q1, member, q2)
		requireOrder(t, "the other section", w.units(1), "g:"+group.Bundle.Group.ID)
	})
}

func TestMoveByAnAdminKeepsTheTeachersOwnership(t *testing.T) {
	w := newMoveWorld(t)
	bank := w.question("bank")
	w.outline([]string{bank})
	group := w.plainGroup(0, "m0", "m1")
	members := memberIDs(group)
	id := group.Bundle.Group.ID
	admin := scopedUser(t, w.tx, "admin")
	as := func(in domain.MoveQuestionInput) domain.MoveQuestionInput {
		in.ActorID, in.Scope = admin, access.Scope{UserID: admin, All: true}
		return in
	}

	copied, err := w.groups.MoveQuestion(w.ctx, as(w.input(bank, 0, ptr(id), 1, id)))
	if err != nil {
		t.Fatalf("copy-in as an admin: %v", err)
	}
	if _, err := w.groups.MoveQuestion(w.ctx, as(w.input(members[0], 0, nil, 0, id))); err != nil {
		t.Fatalf("move-out as an admin: %v", err)
	}

	for name, question := range map[string]string{"the copy": copied.QuestionID, "the member that left": members[0]} {
		var owner string
		if err := w.tx.QueryRow(w.ctx, `SELECT owner_id::text FROM app.questions WHERE id=$1`, question).Scan(&owner); err != nil || owner != w.author {
			t.Fatalf("%s is owned by %q, want the test's owner: %v", name, owner, err)
		}
	}
}

func TestMoveOutNamesTheTestsOwnerWhoseverRowTheMemberWas(t *testing.T) {
	w := newMoveWorld(t)
	lead := w.question("lead")
	w.outline([]string{lead})
	group := w.plainGroup(0, "m0", "m1")
	member := memberIDs(group)[0]
	elsewhere := scopedUser(t, w.tx, "teacher")
	if _, err := w.tx.Exec(w.ctx, `UPDATE app.questions SET owner_id=$2 WHERE id=$1`, member, elsewhere); err != nil {
		t.Fatal(err)
	}

	result := w.mustMove(member, 0, nil, 0, group.Bundle.Group.ID)

	var owner string
	if err := w.tx.QueryRow(w.ctx, `SELECT owner_id::text FROM app.questions WHERE id=$1`, member).Scan(&owner); err != nil || owner != w.author {
		t.Fatalf("the question is owned by %q, want the test's owner %q: %v", owner, w.author, err)
	}
	if got, err := w.bank.Get(w.ctx, w.scope, member); err != nil || got.Prompt != "m0" {
		t.Fatalf("the test's owner cannot read the question in the bank: %+v, %v", got, err)
	}
	saved := result.Test.Sections[0]
	if _, err := w.tests.Update(w.ctx, domain.UpdateRequest{ID: w.testID, ActorID: w.author, Now: time.Now(), Scope: w.scope, Input: domain.UpdateInput{
		ExpectedUpdatedAt: result.Test.UpdatedAt, SetSections: true, GroupOutline: true,
		Sections: []domain.SectionInput{{ID: saved.ID, Title: saved.Title, QuestionIDs: saved.QuestionIDs, Units: saved.Units, SetUnits: true}},
	}}); err != nil {
		t.Fatalf("the next autosave refused the question: %v", err)
	}
}

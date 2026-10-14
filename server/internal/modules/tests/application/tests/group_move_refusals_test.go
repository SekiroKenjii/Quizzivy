//go:build integration

package application_test

import (
	"errors"
	"fmt"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
	"testing"
	"time"

	"github.com/google/uuid"
)

type refusalFixture struct {
	w                  *moveWorld
	bank, dup, stray   string
	g, h               domain.StoredGroup
	bankGroup          domain.StoredGroup
	stranger           stranger
	gMembers, hMembers []string
}

func newRefusalFixture(t *testing.T) refusalFixture {
	t.Helper()
	w := newMoveWorld(t)
	f := refusalFixture{w: w, bank: w.question("bank"), dup: w.question("dup"), stray: w.question("stray")}
	w.outline([]string{f.bank, f.dup}, []string{f.dup})
	f.g, f.h = w.plainGroup(0, "m0", "m1"), w.plainGroup(1, "h0")
	f.gMembers, f.hMembers = memberIDs(f.g), memberIDs(f.h)
	var err error
	f.bankGroup, err = w.groups.Create(w.ctx, domain.CreateGroupInput{Bundle: plainBundle(t, "bank group"), ActorID: w.author, Now: time.Now(), Scope: w.scope, Grants: bothKeys})
	if err != nil {
		t.Fatal(err)
	}
	f.stranger = w.stranger()
	return f
}

func plainBundle(t *testing.T, prompt string) domain.GroupBundle {
	t.Helper()
	member := groupIdentity(t)
	return domain.GroupBundle{
		Group: domain.QuestionGroup{ID: groupIdentity(t), Title: prompt, Members: []domain.GroupMember{{QuestionID: member, OptionOrder: "shuffle"}},
			Stimuli: []domain.GroupStimulus{}, Recordings: []domain.GroupRecording{}},
		Questions: []domain.GroupQuestion{{ID: member, Input: plainInput(prompt)}},
	}
}

func TestMoveRefusalsWriteNothing(t *testing.T) {
	questionsOnly := access.NewSet(access.ContentQuestionsWrite)
	cases := []struct {
		name  string
		input func(f refusalFixture) domain.MoveQuestionInput
		err   error
		rule  string
	}{
		{"a move between two standalone places", func(f refusalFixture) domain.MoveQuestionInput {
			return f.w.input(f.bank, 1, nil, 0)
		}, nil, domain.RuleMoveOutlineOnly},
		{"a destination group the section does not own", func(f refusalFixture) domain.MoveQuestionInput {
			return f.w.input(f.bank, 1, ptr(f.g.Bundle.Group.ID), 0, f.g.Bundle.Group.ID)
		}, nil, domain.RuleMoveGroupSection},
		{"a revision for a group the move does not touch", func(f refusalFixture) domain.MoveQuestionInput {
			return f.w.input(f.gMembers[0], 0, nil, 0, f.g.Bundle.Group.ID, f.h.Bundle.Group.ID)
		}, nil, domain.RuleMoveRevisions},
		{"a revision repeated", func(f refusalFixture) domain.MoveQuestionInput {
			in := f.w.input(f.bank, 0, ptr(f.g.Bundle.Group.ID), 0, f.g.Bundle.Group.ID)
			in.ExpectedGroups = append(in.ExpectedGroups, in.ExpectedGroups[0])
			return in
		}, nil, domain.RuleMoveRevisions},
		{"a touched group missing from the revisions", func(f refusalFixture) domain.MoveQuestionInput {
			return f.w.input(f.bank, 0, ptr(f.g.Bundle.Group.ID), 0)
		}, domain.ErrStaleWrite, ""},
		{"a stale group revision", func(f refusalFixture) domain.MoveQuestionInput {
			in := f.w.input(f.bank, 0, ptr(f.g.Bundle.Group.ID), 0, f.g.Bundle.Group.ID)
			in.ExpectedGroups[0].Revision--
			return in
		}, domain.ErrStaleWrite, ""},
		{"a stale test revision", func(f refusalFixture) domain.MoveQuestionInput {
			in := f.w.input(f.bank, 0, ptr(f.g.Bundle.Group.ID), 0, f.g.Bundle.Group.ID)
			in.ExpectedTestUpdatedAt = in.ExpectedTestUpdatedAt.Add(-time.Hour)
			return in
		}, domain.ErrStaleWrite, ""},
		{"another teacher's test", func(f refusalFixture) domain.MoveQuestionInput {
			in := f.w.input(f.bank, 0, ptr(f.g.Bundle.Group.ID), 0, f.g.Bundle.Group.ID)
			in.TestID = f.stranger.testID
			return in
		}, domain.ErrNotFound, ""},
		{"another teacher's question", func(f refusalFixture) domain.MoveQuestionInput {
			return f.w.input(f.stranger.memberID, 0, ptr(f.g.Bundle.Group.ID), 0, f.g.Bundle.Group.ID)
		}, domain.ErrNotFound, ""},
		{"another teacher's section", func(f refusalFixture) domain.MoveQuestionInput {
			in := f.w.input(f.bank, 0, ptr(f.g.Bundle.Group.ID), 0, f.g.Bundle.Group.ID)
			in.To.SectionID = f.stranger.sectionID
			return in
		}, domain.ErrNotFound, ""},
		{"another teacher's destination group", func(f refusalFixture) domain.MoveQuestionInput {
			in := f.w.input(f.bank, 0, ptr(f.stranger.groupID), 0)
			in.ExpectedGroups = []domain.GroupRevision{{GroupID: f.stranger.groupID, Revision: 1}}
			return in
		}, domain.ErrNotFound, ""},
		{"another teacher's group among the revisions", func(f refusalFixture) domain.MoveQuestionInput {
			in := f.w.input(f.bank, 0, ptr(f.g.Bundle.Group.ID), 0, f.g.Bundle.Group.ID)
			in.ExpectedGroups = append(in.ExpectedGroups, domain.GroupRevision{GroupID: f.stranger.groupID, Revision: 1})
			return in
		}, domain.ErrNotFound, ""},
		{"a missing group among the revisions", func(f refusalFixture) domain.MoveQuestionInput {
			in := f.w.input(f.bank, 0, ptr(f.g.Bundle.Group.ID), 0, f.g.Bundle.Group.ID)
			in.ExpectedGroups = append(in.ExpectedGroups, domain.GroupRevision{GroupID: uuid.NewString(), Revision: 1})
			return in
		}, domain.ErrNotFound, ""},
		{"a bank group as the destination", func(f refusalFixture) domain.MoveQuestionInput {
			in := f.w.input(f.bank, 0, ptr(f.bankGroup.Bundle.Group.ID), 0)
			in.ExpectedGroups = []domain.GroupRevision{{GroupID: f.bankGroup.Bundle.Group.ID, Revision: f.bankGroup.Revision}}
			return in
		}, domain.ErrNotFound, ""},
		{"a question the test does not hold", func(f refusalFixture) domain.MoveQuestionInput {
			return f.w.input(f.stray, 0, ptr(f.g.Bundle.Group.ID), 0, f.g.Bundle.Group.ID)
		}, domain.ErrNotFound, ""},
		{"a missing question", func(f refusalFixture) domain.MoveQuestionInput {
			return f.w.input(uuid.NewString(), 0, ptr(f.g.Bundle.Group.ID), 0, f.g.Bundle.Group.ID)
		}, domain.ErrNotFound, ""},
		{"a standalone question held by two sections", func(f refusalFixture) domain.MoveQuestionInput {
			return f.w.input(f.dup, 0, ptr(f.g.Bundle.Group.ID), 0, f.g.Bundle.Group.ID)
		}, nil, domain.RuleMoveSourceAmbiguous},
		{"only the questions key", func(f refusalFixture) domain.MoveQuestionInput {
			in := f.w.input(f.bank, 0, ptr(f.g.Bundle.Group.ID), 0, f.g.Bundle.Group.ID)
			in.Grants = questionsOnly
			return in
		}, domain.ErrForbidden, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newRefusalFixture(t)
			in := c.input(f)
			before := f.w.state()

			_, err := f.w.groups.MoveQuestion(f.w.ctx, in)

			var refused *domain.GroupError
			switch {
			case c.rule != "":
				if !errors.As(err, &refused) || refused.Rule != c.rule {
					t.Fatalf("got %v, want rule %s", err, c.rule)
				}
			case !errors.Is(err, c.err):
				t.Fatalf("got %v, want %v", err, c.err)
			}
			if after := f.w.state(); after != before {
				t.Fatalf("a refused move changed the draft:\nbefore %s\nafter  %s", before, after)
			}
		})
	}
}

func TestMoveRefusesAnArchivedTest(t *testing.T) {
	w := newMoveWorld(t)
	bank := w.question("bank")
	w.outline([]string{bank})
	group := w.plainGroup(0, "m0")
	id := group.Bundle.Group.ID
	w.publish()
	if _, err := w.tx.Exec(w.ctx, `UPDATE app.tests SET status='archived' WHERE id=$1`, w.testID); err != nil {
		t.Fatal(err)
	}
	before := w.state()

	_, err := w.move(bank, 0, ptr(id), 0, id)

	var refused *domain.GroupError
	if !errors.As(err, &refused) || refused.Rule != "group_owner_archived" {
		t.Fatalf("got %v, want the archived-test rule", err)
	}
	if after := w.state(); after != before {
		t.Fatal("a refused move changed the draft")
	}
}

func TestMoveRefusesAFullGroupAndStillReordersIt(t *testing.T) {
	w := newMoveWorld(t)
	bank := w.question("bank")
	w.outline([]string{bank})
	prompts := make([]string, domain.MaxGroupMembers)
	for i := range prompts {
		prompts[i] = fmt.Sprintf("m%03d", i)
	}
	full := w.plainGroup(0, prompts...)
	small := w.plainGroup(0, "s0")
	fid, sid := full.Bundle.Group.ID, small.Bundle.Group.ID
	members := memberIDs(full)
	before := w.state()

	for name, attempt := range map[string]func() (domain.MoveQuestionResult, error){
		"a copy into it": func() (domain.MoveQuestionResult, error) { return w.move(bank, 0, ptr(fid), 0, fid) },
		"a member from another": func() (domain.MoveQuestionResult, error) {
			return w.move(memberIDs(small)[0], 0, ptr(fid), 0, sid, fid)
		},
	} {
		_, err := attempt()
		var refused *domain.GroupError
		if !errors.As(err, &refused) || refused.Rule != domain.RuleGroupLimits {
			t.Fatalf("%s: %v, want group_limits", name, err)
		}
		if after := w.state(); after != before {
			t.Fatalf("%s: a refused move changed the draft", name)
		}
	}

	w.mustMove(members[199], 0, ptr(fid), 0, fid)
	got := w.members(fid)
	if len(got) != domain.MaxGroupMembers || got[0] != members[199] || got[1] != members[0] {
		t.Fatalf("a full group could not be reordered: %d members, first %s", len(got), got[0])
	}
	w.mustMove(members[0], 0, nil, 0, fid)
	if _, err := w.move(bank, 0, ptr(fid), 0, fid); err != nil {
		t.Fatalf("a group with room refused a copy: %v", err)
	}
}

func TestMoveRefusesAQuestionWhoseAudioIsTheGroupsSharedRecording(t *testing.T) {
	w := newMoveWorld(t)
	asset := storedGroupAsset(t, w.tx, w.author, "audio")
	kind := "audio"
	narrated, err := w.bank.Create(w.ctx, questionsdomain.WriteInput{ActorID: w.author, Now: time.Now(), MediaAssetKind: &kind, Input: questionsdomain.Input{
		Type: questionsdomain.ShortAnswer, Prompt: "Nghe và trả lời", Points: "1", Tags: []string{}, MediaAssetID: &asset, Audio: &questionsdomain.AudioPolicy{AllowSeek: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	w.outline([]string{narrated.ID})
	group := w.mount(0, storedGroupFixture(t, asset))
	id := group.Bundle.Group.ID
	before := w.state()

	_, err = w.move(narrated.ID, 0, ptr(id), 0, id)

	var refused *domain.GroupError
	if !errors.As(err, &refused) || refused.Rule != "group_audio_scope" {
		t.Fatalf("got %v, want group_audio_scope", err)
	}
	if after := w.state(); after != before {
		t.Fatalf("a refused move changed the draft:\nbefore %s\nafter  %s", before, after)
	}
	if standalone, deleted := w.asBank(narrated.ID); !standalone || deleted {
		t.Fatal("the bank question was touched")
	}
}

//go:build integration

package application_test

import (
	"encoding/json"
	"errors"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
	"reflect"
	"strings"
	"testing"
	"time"
)

func (w *moveWorld) copiedFrom(question string) *string {
	w.t.Helper()
	var origin *string
	if err := w.tx.QueryRow(w.ctx, `SELECT copied_from_id::text FROM app.questions WHERE id=$1`, question).Scan(&origin); err != nil {
		w.t.Fatal(err)
	}
	return origin
}

func (w *moveWorld) exists(question string) bool {
	w.t.Helper()
	var found bool
	if err := w.tx.QueryRow(w.ctx, `SELECT EXISTS (SELECT 1 FROM app.questions WHERE id=$1)`, question).Scan(&found); err != nil {
		w.t.Fatal(err)
	}
	return found
}

func (w *moveWorld) bankQuestions() int {
	w.t.Helper()
	var count int
	if err := w.tx.QueryRow(w.ctx, `SELECT count(*) FROM app.questions WHERE owner_id=$1 AND context_group_id IS NULL AND deleted_at IS NULL`, w.author).Scan(&count); err != nil {
		w.t.Fatal(err)
	}
	return count
}

func (w *moveWorld) bankQuestionsWithPrompt(prompt string) int {
	w.t.Helper()
	var count int
	if err := w.tx.QueryRow(w.ctx, `SELECT count(*) FROM app.questions WHERE owner_id=$1 AND context_group_id IS NULL AND deleted_at IS NULL AND prompt=$2`, w.author, prompt).Scan(&count); err != nil {
		w.t.Fatal(err)
	}
	return count
}

func (w *moveWorld) editGroup(group string, edit func(*domain.GroupBundle)) domain.StoredGroup {
	w.t.Helper()
	stored, err := w.groups.Get(w.ctx, w.scope, group)
	if err != nil {
		w.t.Fatal(err)
	}
	bundle := stored.Bundle
	edit(&bundle)
	saved, err := w.groups.Update(w.ctx, domain.UpdateGroupInput{GroupMutation: domain.GroupMutation{ID: group, ExpectedRevision: stored.Revision,
		ExpectedTestUpdatedAt: *stored.TestUpdatedAt, ActorID: w.author, Now: time.Now(), Scope: w.scope, Grants: bothKeys}, Bundle: bundle})
	if err != nil {
		w.t.Fatalf("saving the group: %v", err)
	}
	return saved
}

func (w *moveWorld) editMember(group, member string, edit func(*questionsdomain.Input)) {
	w.t.Helper()
	w.editGroup(group, func(bundle *domain.GroupBundle) {
		for i := range bundle.Questions {
			if bundle.Questions[i].ID == member {
				edit(&bundle.Questions[i].Input)
				return
			}
		}
		w.t.Fatalf("%s is not a member of %s", member, group)
	})
}

func (w *moveWorld) copiedIn(bank string, section int, group string, index int) string {
	w.t.Helper()
	result := w.mustMove(bank, section, ptr(group), index, group)
	if !result.Copied {
		w.t.Fatal("the move did not copy")
	}
	return result.QuestionID
}

func (w *moveWorld) richBank(prompt string) string {
	w.t.Helper()
	level, skill := questionsdomain.Level("b1"), questionsdomain.Skill("reading")
	created, err := w.bank.Create(w.ctx, questionsdomain.WriteInput{ActorID: w.author, Now: time.Now(), Input: questionsdomain.Input{
		Type: questionsdomain.MultipleChoice, Level: &level, Skill: &skill, Prompt: prompt, Points: "2.5", Tags: []string{"đọc hiểu", "b1"},
		Explanation: ptr("Vì đoạn văn nói vậy"),
		Options: []questionsdomain.OptionInput{
			{Text: "một", IsCorrect: true}, {Text: "hai"}, {Text: "ba", IsCorrect: true},
		},
	}})
	if err != nil {
		w.t.Fatal(err)
	}
	return created.ID
}

func TestMoveInRemembersTheBankQuestionItWasCopiedFrom(t *testing.T) {
	w := newMoveWorld(t)
	bank := w.question("Câu hỏi ngân hàng")
	w.outline([]string{bank})
	group := w.plainGroup(0, "m0")
	id := group.Bundle.Group.ID

	copied := w.copiedIn(bank, 0, id, 0)

	if got := w.copiedFrom(copied); got == nil || *got != bank {
		t.Fatalf("the copy names %v as its origin, want %s", got, bank)
	}
	if got := w.copiedFrom(bank); got != nil {
		t.Fatalf("the bank question names an origin: %s", *got)
	}
	if got := w.copiedFrom(memberIDs(group)[0]); got != nil {
		t.Fatalf("a member born in the group names an origin: %s", *got)
	}
}

func TestMoveOutOfAnUneditedCopyPutsTheOriginalBack(t *testing.T) {
	w := newMoveWorld(t)
	lead, bank := w.question("Câu dẫn"), w.question("Câu hỏi ngân hàng")
	w.outline([]string{lead, bank})
	group := w.plainGroup(0, "m0", "m1")
	members := memberIDs(group)
	id := group.Bundle.Group.ID
	original, err := w.bank.Get(w.ctx, w.scope, bank)
	if err != nil {
		t.Fatal(err)
	}
	copied := w.copiedIn(bank, 0, id, 1)
	requireOrder(t, "units after the copy-in", w.units(0), "q:"+lead, "g:"+id)
	revision, banks := w.revision(id), w.bankQuestions()

	result := w.mustMove(copied, 0, nil, 1, id)

	if !result.Restored || result.QuestionID != bank || result.Copied || result.EditedCopy {
		t.Fatalf("restored=%v id=%s copied=%v edited=%v, want the original back and nothing else said", result.Restored, result.QuestionID, result.Copied, result.EditedCopy)
	}
	requireOrder(t, "units", w.units(0), "q:"+lead, "q:"+bank, "g:"+id)
	requireOrder(t, "standalone", w.standalone(0), lead, bank)
	requireOrder(t, "the answer's outline", result.Test.Sections[0].QuestionIDs, lead, bank)
	requireOrder(t, "members", w.members(id), members[0], members[1])
	if w.exists(copied) {
		t.Fatal("the copy is still in the database")
	}
	if got := w.bankQuestions(); got != banks {
		t.Fatalf("%d bank questions, want %d: restoring adds no bank question", got, banks)
	}
	if got := w.bankQuestionsWithPrompt("Câu hỏi ngân hàng"); got != 1 {
		t.Fatalf("%d bank questions with the prompt, want exactly one", got)
	}
	after, err := w.bank.Get(w.ctx, w.scope, bank)
	if err != nil || !after.UpdatedAt.Equal(original.UpdatedAt) || !reflect.DeepEqual(after.Options, original.Options) {
		t.Fatalf("the original was written to: %+v, %v", after, err)
	}
	if len(result.Groups) != 1 || result.Groups[0].Revision != revision+1 || len(result.Groups[0].Bundle.Group.Members) != 2 {
		t.Fatalf("the answer must carry the group at its new revision without the copy: %+v", result.Groups)
	}
	if err := w.bank.SoftDelete(w.ctx, questionsdomain.WriteInput{ID: bank, ActorID: w.author, Now: time.Now()}); !errors.Is(err, questionsdomain.ErrReferenced) {
		t.Fatalf("deleting the question the draft references again must be refused: %v", err)
	}
	var recorded int
	if err := w.tx.QueryRow(w.ctx, `SELECT count(*) FROM app.audit_log WHERE actor_user_id=$1 AND action='test.question_moved'
		AND diff->>'restored'='true' AND diff->>'questionId'=$2 AND diff->>'removedMemberId'=$3`, w.author, bank, copied).Scan(&recorded); err != nil || recorded != 1 {
		t.Fatalf("the audit entry names the restored question and the removed copy: %d, %v", recorded, err)
	}
	saved := result.Test.Sections[0]
	if _, err := w.tests.Update(w.ctx, domain.UpdateRequest{ID: w.testID, ActorID: w.author, Now: time.Now(), Scope: w.scope, Input: domain.UpdateInput{
		ExpectedUpdatedAt: result.Test.UpdatedAt, SetSections: true, GroupOutline: true,
		Sections: []domain.SectionInput{{ID: saved.ID, Title: saved.Title, QuestionIDs: saved.QuestionIDs, Units: saved.Units, SetUnits: true}},
	}}); err != nil {
		t.Fatalf("the next autosave refused the restored question: %v", err)
	}
}

func TestSavingTheGroupWithoutEditingTheCopyStillRestoresTheOriginal(t *testing.T) {
	w := newMoveWorld(t)
	bank := w.richBank("Câu nhiều lựa chọn")
	w.outline([]string{bank})
	group := w.plainGroup(0, "m0")
	id := group.Bundle.Group.ID
	copied := w.copiedIn(bank, 0, id, 0)

	w.editGroup(id, func(bundle *domain.GroupBundle) { bundle.Group.Title = "Nhóm đã đổi tên" })
	result := w.mustMove(copied, 0, nil, 0, id)

	if !result.Restored || result.QuestionID != bank {
		t.Fatalf("a save that did not touch the copy made it an edited one: restored=%v edited=%v", result.Restored, result.EditedCopy)
	}
}

func TestEveryKindOfQuestionComesBackAsItself(t *testing.T) {
	gaps := `{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Điền ","marks":[]},{"type":"gap","id":"first-gap","label":"1"},{"type":"text","text":" và ","marks":[]},{"type":"gap","id":"second-gap","label":"2"}]}]}`
	rich := func(text string, marks ...string) json.RawMessage {
		encoded, err := json.Marshal(append([]string{}, marks...))
		if err != nil {
			t.Fatal(err)
		}
		return json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":` + quote(t, text) + `,"marks":` + string(encoded) + `}]}]}`)
	}
	level, skill := questionsdomain.Level("c1"), questionsdomain.Skill("listening")
	kinds := map[string]func(w *moveWorld) questionsdomain.WriteInput{
		"a single choice with rich prompt, options and explanation": func(_ *moveWorld) questionsdomain.WriteInput {
			return questionsdomain.WriteInput{Input: questionsdomain.Input{
				Type: questionsdomain.SingleChoice, Prompt: "Chọn đáp án", PromptContent: rich("Chọn đáp án", "bold"), Points: "1.25",
				Level: &level, Skill: &skill, Tags: []string{"b", "a"}, Explanation: ptr("Giải thích"), ExplanationContent: rich("Giải thích"),
				Options: []questionsdomain.OptionInput{{Text: "đúng", Content: rich("đúng", "italic"), IsCorrect: true}, {Text: "sai"}},
			}}
		},
		"a true or false": func(_ *moveWorld) questionsdomain.WriteInput {
			return questionsdomain.WriteInput{Input: questionsdomain.Input{
				Type: questionsdomain.TrueFalse, Prompt: "Trái đất tròn", Points: "1",
				Options: []questionsdomain.OptionInput{{Text: "Đúng", IsCorrect: true}, {Text: "Sai"}},
			}}
		},
		"a fill in the blanks with numbered placeholders": func(_ *moveWorld) questionsdomain.WriteInput {
			return questionsdomain.WriteInput{Input: questionsdomain.Input{
				Type: questionsdomain.FillBlank, Prompt: "Điền {{1}} và {{2}}", Points: "2",
				Blanks: []questionsdomain.BlankInput{
					{Ordinal: 1, AcceptedAnswers: []string{"một", "1"}},
					{Ordinal: 2, CaseSensitive: true, AcceptedAnswers: []string{"Hai"}},
				},
			}}
		},
		"a fill in the blanks with gaps in a rich prompt": func(_ *moveWorld) questionsdomain.WriteInput {
			return questionsdomain.WriteInput{Input: questionsdomain.Input{
				Type: questionsdomain.FillBlank, Prompt: "Điền [1] và [2]", PromptContent: json.RawMessage(gaps), Points: "2",
				Blanks: []questionsdomain.BlankInput{
					{GapID: ptr("first-gap"), Ordinal: 1, AcceptedAnswers: []string{"một", "1"}},
					{GapID: ptr("second-gap"), Ordinal: 2, CaseSensitive: true, AcceptedAnswers: []string{"Hai"}},
				},
			}}
		},
		"a short answer with a sample answer and an image": func(w *moveWorld) questionsdomain.WriteInput {
			asset, kind := storedGroupAsset(w.t, w.tx, w.author, "image"), "image"
			return questionsdomain.WriteInput{MediaAssetKind: &kind, Input: questionsdomain.Input{
				Type: questionsdomain.ShortAnswer, Prompt: "Mô tả bức hình", Points: "3", SampleAnswer: ptr("Một bức hình"),
				MediaAssetID: &asset, MediaAlt: ptr("Hình minh hoạ"),
			}}
		},
		"a listening question with a recording, a policy and a transcript": func(w *moveWorld) questionsdomain.WriteInput {
			asset, kind := storedGroupAsset(w.t, w.tx, w.author, "audio"), "audio"
			return questionsdomain.WriteInput{MediaAssetKind: &kind, Input: questionsdomain.Input{
				Type: questionsdomain.SingleChoice, Prompt: "Nghe và chọn", Points: "1", MediaAssetID: &asset, Transcript: ptr("Lời thoại"),
				Audio:   &questionsdomain.AudioPolicy{MaxPlays: groupValue(2), AllowSeek: true, ShowTranscriptAfterSubmit: true, AllowPause: groupValue(false)},
				Options: []questionsdomain.OptionInput{{Text: "a", IsCorrect: true}, {Text: "b"}},
			}}
		},
	}
	for name, build := range kinds {
		t.Run(name, func(t *testing.T) {
			w := newMoveWorld(t)
			write := build(w)
			write.ActorID, write.Now = w.author, time.Now()
			if write.Input.Tags == nil {
				write.Input.Tags = []string{}
			}
			created, err := w.bank.Create(w.ctx, write)
			if err != nil {
				t.Fatal(err)
			}
			bank := created.ID
			w.outline([]string{bank})
			group := w.plainGroup(0, "m0")
			id := group.Bundle.Group.ID
			copied := w.copiedIn(bank, 0, id, 0)
			banks := w.bankQuestions()

			result := w.mustMove(copied, 0, nil, 0, id)

			if !result.Restored || result.QuestionID != bank || result.EditedCopy {
				t.Fatalf("the unedited copy of %s did not come back: restored=%v edited=%v", name, result.Restored, result.EditedCopy)
			}
			if got := w.bankQuestions(); got != banks {
				t.Fatalf("%d bank questions, want %d", got, banks)
			}
		})
	}
}

func quote(t *testing.T, text string) string {
	t.Helper()
	encoded, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestMoveOutOfAnEditedCopyKeepsItAsANewBankQuestion(t *testing.T) {
	edits := map[string]func(*questionsdomain.Input){
		"the prompt":       func(in *questionsdomain.Input) { in.Prompt += " (sửa)" },
		"the points":       func(in *questionsdomain.Input) { in.Points = "4" },
		"an option's text": func(in *questionsdomain.Input) { in.Options[1].Text = "bốn" },
		"which options are right": func(in *questionsdomain.Input) {
			in.Options[1].IsCorrect = true
		},
		"the tags":        func(in *questionsdomain.Input) { in.Tags = append(in.Tags, "mới") },
		"the level":       func(in *questionsdomain.Input) { in.Level = groupValue(questionsdomain.Level("c2")) },
		"the skill":       func(in *questionsdomain.Input) { in.Skill = nil },
		"the explanation": func(in *questionsdomain.Input) { in.Explanation = ptr("Giải thích khác") },
		"an option removed": func(in *questionsdomain.Input) {
			in.Options = in.Options[:2]
		},
	}
	for name, edit := range edits {
		t.Run(name, func(t *testing.T) {
			w := newMoveWorld(t)
			bank := w.richBank("Câu nhiều lựa chọn")
			w.outline([]string{bank})
			group := w.plainGroup(0, "m0")
			id := group.Bundle.Group.ID
			copied := w.copiedIn(bank, 0, id, 0)
			w.editMember(id, copied, edit)
			banks := w.bankQuestions()

			result := w.mustMove(copied, 0, nil, 0, id)

			if result.Restored || !result.EditedCopy || result.Copied || result.QuestionID != copied {
				t.Fatalf("restored=%v edited=%v id=%s, want the edited copy kept under its own id", result.Restored, result.EditedCopy, result.QuestionID)
			}
			if standalone, deleted := w.asBank(copied); !standalone || deleted {
				t.Fatalf("the edited copy is not an ordinary bank question: standalone=%v deleted=%v", standalone, deleted)
			}
			if got := w.copiedFrom(copied); got != nil {
				t.Fatalf("the question that became independent still names %s as its origin", *got)
			}
			if got := w.bankQuestions(); got != banks+1 {
				t.Fatalf("%d bank questions, want %d: the edited copy is a new one", got, banks+1)
			}
			if standalone, deleted := w.asBank(bank); !standalone || deleted {
				t.Fatalf("the original left the bank: standalone=%v deleted=%v", standalone, deleted)
			}
			requireOrder(t, "units", w.units(0), "q:"+copied, "g:"+id)
			if refs := w.refsTo(bank); refs != 0 {
				t.Fatalf("the draft references the original %d times", refs)
			}
		})
	}
}

func (w *moveWorld) refsTo(question string) int {
	w.t.Helper()
	var count int
	if err := w.tx.QueryRow(w.ctx, `SELECT count(*) FROM app.test_section_questions WHERE question_id=$1`, question).Scan(&count); err != nil {
		w.t.Fatal(err)
	}
	return count
}

func TestMoveOutOfAnEditedBlankCopyKeepsItAsANewBankQuestion(t *testing.T) {
	w := newMoveWorld(t)
	created, err := w.bank.Create(w.ctx, questionsdomain.WriteInput{ActorID: w.author, Now: time.Now(), Input: questionsdomain.Input{
		Type: questionsdomain.FillBlank, Prompt: "Điền {{1}}", Points: "1", Tags: []string{},
		Blanks: []questionsdomain.BlankInput{{Ordinal: 1, AcceptedAnswers: []string{"một"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	w.outline([]string{created.ID})
	group := w.plainGroup(0, "m0")
	id := group.Bundle.Group.ID
	copied := w.copiedIn(created.ID, 0, id, 0)
	w.editMember(id, copied, func(in *questionsdomain.Input) { in.Blanks[0].AcceptedAnswers = []string{"một", "1"} })

	result := w.mustMove(copied, 0, nil, 0, id)

	if result.Restored || !result.EditedCopy || result.QuestionID != copied {
		t.Fatalf("restored=%v edited=%v, want the edited copy kept", result.Restored, result.EditedCopy)
	}
}

func TestMoveOutOfACopyWhoseOriginCannotComeBackLeavesAsAnOrdinaryBankQuestion(t *testing.T) {
	cases := map[string]func(w *moveWorld, bank string) (as func(domain.MoveQuestionInput) domain.MoveQuestionInput){
		"the original was deleted": func(w *moveWorld, bank string) func(domain.MoveQuestionInput) domain.MoveQuestionInput {
			if err := w.bank.SoftDelete(w.ctx, questionsdomain.WriteInput{ID: bank, ActorID: w.author, Now: time.Now()}); err != nil {
				w.t.Fatalf("the draft no longer references the original, so it can be deleted: %v", err)
			}
			return func(in domain.MoveQuestionInput) domain.MoveQuestionInput { return in }
		},
		"the original is another teacher's": func(w *moveWorld, bank string) func(domain.MoveQuestionInput) domain.MoveQuestionInput {
			elsewhere := scopedUser(w.t, w.tx, "teacher")
			if _, err := w.tx.Exec(w.ctx, `UPDATE app.questions SET owner_id=$2 WHERE id=$1`, bank, elsewhere); err != nil {
				w.t.Fatal(err)
			}
			return func(in domain.MoveQuestionInput) domain.MoveQuestionInput { return in }
		},
		"an admin moves it and the original is another teacher's": func(w *moveWorld, bank string) func(domain.MoveQuestionInput) domain.MoveQuestionInput {
			elsewhere, admin := scopedUser(w.t, w.tx, "teacher"), scopedUser(w.t, w.tx, "admin")
			if _, err := w.tx.Exec(w.ctx, `UPDATE app.questions SET owner_id=$2 WHERE id=$1`, bank, elsewhere); err != nil {
				w.t.Fatal(err)
			}
			return func(in domain.MoveQuestionInput) domain.MoveQuestionInput {
				in.ActorID, in.Scope = admin, access.Scope{UserID: admin, All: true}
				return in
			}
		},
		"the original is in the test again": func(w *moveWorld, bank string) func(domain.MoveQuestionInput) domain.MoveQuestionInput {
			w.addToSection(0, bank)
			return func(in domain.MoveQuestionInput) domain.MoveQuestionInput { return in }
		},
		"the original is in another section of the test": func(w *moveWorld, bank string) func(domain.MoveQuestionInput) domain.MoveQuestionInput {
			w.addToSection(1, bank)
			return func(in domain.MoveQuestionInput) domain.MoveQuestionInput { return in }
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			w := newMoveWorld(t)
			bank := w.question("Câu hỏi ngân hàng")
			w.outline([]string{bank}, nil)
			group := w.plainGroup(0, "m0")
			id := group.Bundle.Group.ID
			copied := w.copiedIn(bank, 0, id, 0)
			as := arrange(w, bank)
			held := w.refsTo(bank)

			result, err := w.groups.MoveQuestion(w.ctx, as(w.input(copied, 0, nil, 0, id)))
			if err != nil {
				t.Fatalf("the move must not fail: %v", err)
			}

			if result.Restored || result.EditedCopy || result.QuestionID != copied {
				t.Fatalf("restored=%v edited=%v id=%s, want the copy to leave under its own id and say nothing about an edit", result.Restored, result.EditedCopy, result.QuestionID)
			}
			if standalone, deleted := w.asBank(copied); !standalone || deleted {
				t.Fatalf("the copy is not an ordinary bank question: standalone=%v deleted=%v", standalone, deleted)
			}
			if got := w.copiedFrom(copied); got != nil {
				t.Fatalf("the question that became independent still names %s as its origin", *got)
			}
			if got := w.refsTo(bank); got != held {
				t.Fatalf("the original is referenced %d times, was %d: it must not be placed", got, held)
			}
		})
	}
}

func (w *moveWorld) addToSection(section int, question string) {
	w.t.Helper()
	id := w.sections[section]
	if _, err := w.tx.Exec(w.ctx, `INSERT INTO app.test_section_questions (test_section_id,ordinal,question_id)
		SELECT $1,count(*),$2 FROM app.test_section_questions WHERE test_section_id=$1`, id, question); err != nil {
		w.t.Fatal(err)
	}
	if _, err := w.tx.Exec(w.ctx, `INSERT INTO app.test_section_units (test_section_id,ordinal,question_id)
		SELECT $1,count(*),$2 FROM app.test_section_units WHERE test_section_id=$1`, id, question); err != nil {
		w.t.Fatal(err)
	}
}

func TestMovingACopyBetweenGroupsAndWithinOneKeepsItsOrigin(t *testing.T) {
	w := newMoveWorld(t)
	bank := w.question("Câu hỏi ngân hàng")
	w.outline([]string{bank})
	a, b := w.plainGroup(0, "a0"), w.plainGroup(0, "b0")
	aid, bid := a.Bundle.Group.ID, b.Bundle.Group.ID
	copied := w.copiedIn(bank, 0, aid, 1)

	w.mustMove(copied, 0, ptr(bid), 0, aid, bid)
	if got := w.copiedFrom(copied); got == nil || *got != bank {
		t.Fatalf("moving the copy to another group changed its origin to %v", got)
	}
	w.mustMove(copied, 0, ptr(bid), 1, bid)
	if got := w.copiedFrom(copied); got == nil || *got != bank {
		t.Fatalf("reordering the copy changed its origin to %v", got)
	}

	result := w.mustMove(copied, 0, nil, 0, bid)

	if !result.Restored || result.QuestionID != bank {
		t.Fatalf("a copy that changed groups did not come back as the original: restored=%v edited=%v", result.Restored, result.EditedCopy)
	}
	requireOrder(t, "units", w.units(0), "q:"+bank, "g:"+aid, "g:"+bid)
}

func TestARestoreSeedsASectionWithNoUnitsAndPlacesTheOriginal(t *testing.T) {
	w := newMoveWorld(t)
	q1, q2, bank := w.question("q1"), w.question("q2"), w.question("Câu hỏi ngân hàng")
	w.outline([]string{q1, q2}, []string{bank})
	group := w.plainGroup(1, "m0")
	id := group.Bundle.Group.ID
	copied := w.copiedIn(bank, 1, id, 0)
	if got := w.units(0); len(got) != 0 {
		t.Fatalf("the destination section already has units: %v", got)
	}

	result := w.mustMove(copied, 0, nil, 1, id)

	if !result.Restored || result.QuestionID != bank {
		t.Fatalf("restored=%v id=%s", result.Restored, result.QuestionID)
	}
	requireOrder(t, "units", w.units(0), "q:"+q1, "q:"+bank, "q:"+q2)
	requireOrder(t, "standalone", w.standalone(0), q1, bank, q2)
	requireOrder(t, "the group's section", w.units(1), "g:"+id)
}

func TestARestoreClampsItsIndexLikeAnyMoveOut(t *testing.T) {
	w := newMoveWorld(t)
	q0, q1, bank := w.question("q0"), w.question("q1"), w.question("Câu hỏi ngân hàng")
	w.outline([]string{q0, q1, bank})
	group := w.plainGroup(0, "m0")
	id := group.Bundle.Group.ID
	copied := w.copiedIn(bank, 0, id, 0)

	w.mustMove(copied, 0, nil, 99, id)

	requireOrder(t, "units", w.units(0), "q:"+q0, "q:"+q1, "g:"+id, "q:"+bank)
	requireOrder(t, "standalone", w.standalone(0), q0, q1, bank)
}

func TestARestoreIsRefusedWhileTheCopyIsBoundToAGap(t *testing.T) {
	w := newMoveWorld(t)
	bank := w.question("Câu hỏi ngân hàng")
	w.outline([]string{bank})
	group := w.plainGroup(0, "m0")
	id := group.Bundle.Group.ID
	copied := w.copiedIn(bank, 0, id, 0)
	w.editGroup(id, func(bundle *domain.GroupBundle) {
		bundle.Group.Stimuli = []domain.GroupStimulus{{ID: groupIdentity(w.t), Title: "Bài đọc",
			Content: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"choice-gap","label":"1"}]}]}`),
			Gaps:    []domain.GroupGapBinding{{Kind: "question", GapID: "choice-gap", QuestionID: copied}}}}
	})
	before := w.state()

	_, err := w.move(copied, 0, nil, 0, id)

	var refused *domain.GroupError
	if !errors.As(err, &refused) || refused.Rule != domain.RuleGroupGapBound {
		t.Fatalf("%v, want group_gap_bound", err)
	}
	if after := w.state(); after != before {
		t.Fatalf("a refused move changed the draft:\nbefore %s\nafter  %s", before, after)
	}
	if !w.exists(copied) {
		t.Fatal("the refused move removed the copy")
	}
}

func TestRestoringTheOriginalLeavesAPublishedVersionWorking(t *testing.T) {
	w := newMoveWorld(t)
	lead, bank := w.question("Câu dẫn"), w.question("Câu hỏi ngân hàng")
	w.outline([]string{lead, bank})
	group := w.plainGroup(0, "m0")
	id := group.Bundle.Group.ID
	copied := w.copiedIn(bank, 0, id, 0)
	first := w.publish()
	before := w.versionRows()
	requireOrder(t, "the first version's questions", w.frozenQuestions(first), lead, copied, memberIDs(group)[0])

	result := w.mustMove(copied, 0, nil, 1, id)
	if !result.Restored {
		t.Fatal("the copy did not come back as the original")
	}

	after := w.versionRows()
	for table := range before {
		if table == "test_version_questions" {
			continue
		}
		if !reflect.DeepEqual(after[table], before[table]) {
			t.Errorf("%s changed:\nbefore %v\nafter  %v", table, before[table], after[table])
		}
	}
	if len(after["test_version_questions"]) != len(before["test_version_questions"]) {
		t.Fatalf("a frozen question was lost: %d rows, was %d", len(after["test_version_questions"]), len(before["test_version_questions"]))
	}
	differing := 0
	for i := range before["test_version_questions"] {
		if before["test_version_questions"][i] == after["test_version_questions"][i] {
			continue
		}
		differing++
		was, now := frozenRow(t, before["test_version_questions"][i]), frozenRow(t, after["test_version_questions"][i])
		if was["source_question_id"] != copied || now["source_question_id"] != nil {
			t.Fatalf("the frozen copy's link was %v and became %v, want %s and null", was["source_question_id"], now["source_question_id"], copied)
		}
		delete(was, "source_question_id")
		delete(now, "source_question_id")
		if !reflect.DeepEqual(was, now) {
			t.Fatalf("the frozen copy's content changed:\nbefore %v\nafter  %v", was, now)
		}
	}
	if differing != 1 {
		t.Fatalf("%d frozen questions differ, want only the copy's link", differing)
	}

	second := w.publish()
	requireOrder(t, "the second version's units", w.frozenUnits(second), "q:"+lead, "q:"+bank, "g:Nhóm m0")
	requireOrder(t, "the second version's questions", w.frozenQuestions(second), lead, bank, memberIDs(group)[0])
}

func frozenRow(t *testing.T, encoded string) map[string]any {
	t.Helper()
	var row map[string]any
	if err := json.Unmarshal([]byte(encoded), &row); err != nil {
		t.Fatal(err)
	}
	return row
}

func TestAnOriginThatLeavesTheBankWhileACopyStandsIsForgottenNotLost(t *testing.T) {
	w := newMoveWorld(t)
	bank := w.question("Câu hỏi ngân hàng")
	w.outline([]string{bank})
	group := w.plainGroup(0, "m0")
	id := group.Bundle.Group.ID
	copied := w.copiedIn(bank, 0, id, 0)

	if _, err := w.tx.Exec(w.ctx, `DELETE FROM app.questions WHERE id=$1`, bank); err != nil {
		t.Fatalf("a copy must not veto the removal of its origin: %v", err)
	}

	if got := w.copiedFrom(copied); got != nil {
		t.Fatalf("the copy still names the removed origin %s", *got)
	}
	if result := w.mustMove(copied, 0, nil, 0, id); result.Restored || result.QuestionID != copied {
		t.Fatalf("restored=%v id=%s", result.Restored, result.QuestionID)
	}
}

func TestCopiedFromIsIndexedForItsReferentialAction(t *testing.T) {
	w := newMoveWorld(t)
	var definition string
	if err := w.tx.QueryRow(w.ctx, `SELECT indexdef FROM pg_indexes WHERE schemaname='app' AND indexname='questions_copied_from_idx'`).Scan(&definition); err != nil {
		t.Fatalf("the partial index is missing: %v", err)
	}
	if !strings.Contains(definition, "(copied_from_id)") || !strings.Contains(definition, "WHERE (copied_from_id IS NOT NULL)") {
		t.Fatalf("index definition %q", definition)
	}
	var action string
	if err := w.tx.QueryRow(w.ctx, `SELECT confdeltype::text FROM pg_constraint WHERE conname='questions_copied_from_id_fkey'`).Scan(&action); err != nil || action != "n" {
		t.Fatalf("the foreign key's ON DELETE action is %q, %v, want n (SET NULL)", action, err)
	}
}

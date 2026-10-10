package domain_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"

	"golang.org/x/text/unicode/norm"
)

func decomposed(s string) string { return norm.NFD.String(s) }

func material(text, label string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":%q,"marks":["bold"]},{"type":"gap","id":"Gap-1","label":%q}]}]}`, text, label))
}

func TestAnOutlineWriteIsComposed(t *testing.T) {
	title, description, instructions := decomposed("Đề kiểm tra"), decomposed("Mô tả đề"), decomposed("Làm bài cẩn thận")
	in := domain.UpdateInput{
		Title: &title, Description: &description, SetDescription: true, SetSections: true,
		Sections: []domain.SectionInput{{ID: "section-id", Title: decomposed("Phần nghe"), Instructions: &instructions, QuestionIDs: []string{"q-1"}, Units: []domain.SectionUnit{{Kind: "question", ID: "q-1"}}}},
	}
	got, err := in.Composed()
	if err != nil {
		t.Fatal(err)
	}
	section := got.Sections[0]
	if *got.Title != "Đề kiểm tra" || *got.Description != "Mô tả đề" || section.Title != "Phần nghe" || *section.Instructions != "Làm bài cẩn thận" {
		t.Fatalf("not composed: %+v", got)
	}
	if section.ID != "section-id" || section.QuestionIDs[0] != "q-1" || section.Units[0].ID != "q-1" || !got.SetSections || !got.SetDescription {
		t.Fatalf("an identity or a flag changed: %+v", got)
	}
	if in.Sections[0].Title != decomposed("Phần nghe") {
		t.Fatal("Composed rewrote the sections of its receiver")
	}
}

func TestAnOutlineWriteWithNothingToComposeStaysAbsent(t *testing.T) {
	got, err := (domain.UpdateInput{}).Composed()
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != nil || got.Description != nil || got.Sections != nil {
		t.Fatalf("an absent field appeared: %+v", got)
	}
}

func TestATitleComposingLeavesOverItsLimitIsRefusedByName(t *testing.T) {
	title := strings.Repeat("क़", domain.MaxTestTitle)
	_, err := (domain.UpdateInput{Title: &title}).Composed()
	var invalid *domain.ValidationError
	if !errors.As(err, &invalid) || invalid.Fields[0].Field != "title" {
		t.Fatalf("err=%v", err)
	}
}

func bundle() domain.GroupBundle {
	transcript := decomposed("Lời thoại")
	alt := decomposed("Ảnh")
	asset := "01935000-0000-7000-8000-0000000000a1"
	return domain.GroupBundle{
		Group: domain.QuestionGroup{
			ID: "group-id", Title: decomposed("Nhóm đọc hiểu"), Instructions: material(decomposed("Đọc kỹ"), decomposed("Điền")),
			Members: []domain.GroupMember{{QuestionID: "q-1", OptionOrder: "shuffle"}},
			Stimuli: []domain.GroupStimulus{{
				ID: "stimulus-id", Title: decomposed("Bài đọc"), Content: material(decomposed("Nội dung bài đọc"), decomposed("Điền")),
				Gaps: []domain.GroupGapBinding{{Kind: "question", GapID: "Gap-1", QuestionID: "q-1"}},
			}},
			Recordings: []domain.GroupRecording{{ID: "recording-id", AssetID: asset, Transcript: &transcript}},
		},
		Questions: []domain.GroupQuestion{{ID: "q-1", Input: questions.Input{Type: questions.ShortAnswer, Prompt: decomposed("Câu một"), Points: "1", MediaAlt: &alt, MediaAssetID: &asset}}},
	}
}

func TestAGroupIsComposedWithItsMaterialsAndMembers(t *testing.T) {
	in := bundle()
	got, err := in.Composed()
	if err != nil {
		t.Fatal(err)
	}
	group := got.Group
	if group.Title != "Nhóm đọc hiểu" || group.Stimuli[0].Title != "Bài đọc" || *group.Recordings[0].Transcript != "Lời thoại" {
		t.Fatalf("not composed: %+v", group)
	}
	for name, raw := range map[string]json.RawMessage{"instructions": group.Instructions, "stimulus content": group.Stimuli[0].Content} {
		if string(raw) != norm.NFC.String(string(raw)) {
			t.Errorf("%s is still decomposed: %s", name, raw)
		}
	}
	question := got.Questions[0]
	if question.Input.Prompt != "Câu một" || *question.Input.MediaAlt != "Ảnh" || *question.Input.MediaAssetID != "01935000-0000-7000-8000-0000000000a1" || question.ID != "q-1" {
		t.Fatalf("member not composed or an identity changed: %+v", question)
	}
	binding := group.Stimuli[0].Gaps[0]
	if group.ID != "group-id" || group.Stimuli[0].ID != "stimulus-id" || group.Recordings[0].ID != "recording-id" || binding.GapID != "Gap-1" || binding.QuestionID != "q-1" || group.Members[0].OptionOrder != "shuffle" {
		t.Fatalf("an identity changed: %+v", group)
	}
	if in.Group.Title != decomposed("Nhóm đọc hiểu") || in.Questions[0].Input.Prompt != decomposed("Câu một") {
		t.Fatal("Composed rewrote its receiver")
	}
}

func TestAGroupWithoutMaterialsComposesToAnEmptyOne(t *testing.T) {
	got, err := (domain.GroupBundle{Group: domain.QuestionGroup{ID: "g", Title: decomposed("Nhóm")}}).Composed()
	if err != nil {
		t.Fatal(err)
	}
	if got.Group.Title != "Nhóm" || got.Group.Instructions != nil || got.Group.Stimuli != nil || got.Group.Recordings != nil || got.Questions != nil {
		t.Fatalf("%+v", got)
	}
}

func TestAMemberComposingLeavesOverALimitIsRefusedAsGroupContent(t *testing.T) {
	in := bundle()
	alt := strings.Repeat("क़", questions.MaxMediaAltLength)
	in.Questions[0].Input.MediaAlt = &alt
	_, err := in.Composed()
	var invalid *domain.GroupError
	if !errors.As(err, &invalid) || invalid.Rule != "group_content" || invalid.QuestionID != "q-1" {
		t.Fatalf("err=%v", err)
	}
}

func TestAStimulusComposingMakesInvalidIsRefusedAsGroupContent(t *testing.T) {
	in := bundle()
	in.Group.Stimuli[0].Content = material("Nội dung", strings.Repeat("क़", 32))
	_, err := in.Composed()
	var invalid *domain.GroupError
	if !errors.As(err, &invalid) || invalid.Rule != "group_content" || invalid.StimulusID != "stimulus-id" {
		t.Fatalf("err=%v", err)
	}
}

func TestAnInvalidMaterialIsLeftToValidation(t *testing.T) {
	in := bundle()
	in.Group.Instructions = json.RawMessage(`{"format":"nope"}`)
	got, err := in.Composed()
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Group.Instructions) != `{"format":"nope"}` {
		t.Fatalf("an invalid document was rewritten: %s", got.Group.Instructions)
	}
	if err := got.Validate(); err == nil {
		t.Fatal("the invalid document was accepted")
	}
}

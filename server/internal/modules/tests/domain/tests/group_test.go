package domain_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/content"
	"reflect"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
)

func groupID(n int) string { return fmt.Sprintf("019535d9-3df7-79fb-b466-%012d", n) }

func groupPointer[T any](value T) *T { return &value }

func groupFixture() domain.GroupBundle {
	return domain.GroupBundle{
		Group: domain.QuestionGroup{
			ID: groupID(1), Title: "Bài đọc và bài nghe",
			Instructions: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Đọc rồi trả lời.","marks":[]}]}]}`),
			Members:      []domain.GroupMember{{QuestionID: groupID(2), OptionOrder: "fixed"}, {QuestionID: groupID(3), OptionOrder: "shuffle"}},
			Stimuli: []domain.GroupStimulus{
				{
					ID: groupID(4), Title: "Điền vào bài đọc",
					Content: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Tiếng Việt: ","marks":["underline"]},{"type":"gap","id":"choice-gap","label":"1"},{"type":"gap","id":"material-blank","label":"1"}]}]}`),
					Gaps: []domain.GroupGapBinding{
						{Kind: "question", GapID: "choice-gap", QuestionID: groupID(2)},
						{Kind: "blank", GapID: "material-blank", QuestionID: groupID(3), BlankGapID: groupPointer("answer-gap")},
					},
				},
				{
					ID: groupID(5), Title: "Đoạn hội thoại",
					Content: json.RawMessage(fmt.Sprintf(`{"format":"semantic_v1","blocks":[{"type":"audio","assetId":%q,"label":"Hội thoại"},{"type":"image","assetId":%q,"alt":"Sơ đồ"}]}`, groupID(6), groupID(7))),
					Gaps:    []domain.GroupGapBinding{},
				},
			},
			Recordings: []domain.GroupRecording{{ID: groupID(8), AssetID: groupID(6), Policy: questions.AudioPolicy{MaxPlays: groupPointer(2)}, Transcript: groupPointer("Lời thoại riêng của giáo viên")}},
		},
		Questions: []domain.GroupQuestion{
			{ID: groupID(2), Input: questions.Input{
				Type: questions.SingleChoice, Prompt: "Chọn đáp án theo bài đọc", Points: "0.25", Tags: []string{"Đọc hiểu"},
				Options: []questions.OptionInput{{ID: groupPointer(groupID(9)), Text: "đúng", IsCorrect: true}, {ID: groupPointer(groupID(10)), Text: "sai"}},
			}},
			{ID: groupID(3), Input: questions.Input{
				Type: questions.FillBlank, Prompt: "[5]", Points: "0.75",
				PromptContent: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"answer-gap","label":"5"}]}]}`),
				Blanks:        []questions.BlankInput{{ID: groupPointer(groupID(11)), GapID: groupPointer("answer-gap"), Ordinal: 5, AcceptedAnswers: []string{"chính xác", "chính xác"}}},
			}},
		},
	}
}

func TestGroupUsesExplicitBindingsAndAllowsEmptyDrafts(t *testing.T) {
	bundle := groupFixture()
	if err := bundle.ValidateForPublish(false); err != nil {
		t.Fatal(err)
	}
	if err := bundle.ValidateForPublish(true); err == nil {
		t.Fatal("option-label dependencies were ignored")
	}
	bundle.Group.Members[0].OptionOrder = "shuffle"
	if err := bundle.ValidateForPublish(true); err != nil {
		t.Fatal(err)
	}
	bundle.Group.Members = nil
	bundle.Group.Stimuli = nil
	bundle.Group.Recordings = nil
	bundle.Questions = nil
	if err := bundle.Validate(); err != nil {
		t.Fatalf("empty draft: %v", err)
	}
	if err := bundle.ValidateForPublish(false); err == nil {
		t.Fatal("empty group could publish")
	}
}

func TestGroupRejectsBrokenContext(t *testing.T) {
	cases := []struct {
		name   string
		change func(*domain.GroupBundle)
	}{
		{"missing member data", func(b *domain.GroupBundle) { b.Questions = b.Questions[:1] }},
		{"extra member data", func(b *domain.GroupBundle) { b.Questions = append(b.Questions, b.Questions[0]) }},
		{"same member twice", func(b *domain.GroupBundle) { b.Group.Members[1] = b.Group.Members[0] }},
		{"member outside catalog", func(b *domain.GroupBundle) { b.Group.Members[1].QuestionID = groupID(99) }},
		{"unknown option order", func(b *domain.GroupBundle) { b.Group.Members[0].OptionOrder = "random" }},
		{"fixed non choice options", func(b *domain.GroupBundle) { b.Group.Members[1].OptionOrder = "fixed" }},
		{"duplicate material", func(b *domain.GroupBundle) { b.Group.Stimuli = append(b.Group.Stimuli, b.Group.Stimuli[0]) }},
		{"identity shared across types", func(b *domain.GroupBundle) { b.Group.Stimuli[0].ID = b.Group.ID }},
		{"malformed identity", func(b *domain.GroupBundle) { b.Group.ID = "group 1" }},
		{"duplicate child identity", func(b *domain.GroupBundle) { b.Questions[0].Input.Options[1].ID = b.Questions[0].Input.Options[0].ID }},
		{"child reuses parent identity", func(b *domain.GroupBundle) { b.Questions[1].Input.Blanks[0].ID = groupPointer(b.Group.ID) }},
		{"blank title", func(b *domain.GroupBundle) { b.Group.Title = "\t" }},
		{"oversized title", func(b *domain.GroupBundle) { b.Group.Title = strings.Repeat("ế", 201) }},
		{"invalid title unicode", func(b *domain.GroupBundle) { b.Group.Title = string([]byte{0xff}) }},
		{"nul title", func(b *domain.GroupBundle) { b.Group.Title = "hidden\x00text" }},
		{"material without title", func(b *domain.GroupBundle) { b.Group.Stimuli[0].Title = " " }},
		{"material without content", func(b *domain.GroupBundle) { b.Group.Stimuli[0].Content = nil }},
		{"duplicate content key", func(b *domain.GroupBundle) {
			b.Group.Stimuli[0].Content = json.RawMessage(`{"format":"semantic_v1","format":"legacy_markdown_v1","markdown":"x"}`)
		}},
		{"key in learner material", func(b *domain.GroupBundle) {
			b.Group.Stimuli[0].Content = json.RawMessage(`{"format":"legacy_markdown_v1","markdown":"x","isCorrect":true}`)
		}},
		{"gap in group instructions", func(b *domain.GroupBundle) { b.Group.Instructions = b.Group.Stimuli[0].Content }},
		{"unknown material gap", func(b *domain.GroupBundle) { b.Group.Stimuli[0].Gaps[0].GapID = "not-in-content" }},
		{"missing material binding", func(b *domain.GroupBundle) { b.Group.Stimuli[0].Gaps = b.Group.Stimuli[0].Gaps[:1] }},
		{"duplicate binding", func(b *domain.GroupBundle) {
			b.Group.Stimuli[0].Gaps = append(b.Group.Stimuli[0].Gaps, b.Group.Stimuli[0].Gaps[0])
		}},
		{"outside group target", func(b *domain.GroupBundle) { b.Group.Stimuli[0].Gaps[0].QuestionID = groupID(99) }},
		{"unknown target kind", func(b *domain.GroupBundle) { b.Group.Stimuli[0].Gaps[0].Kind = "option" }},
		{"blank metadata on choice", func(b *domain.GroupBundle) { b.Group.Stimuli[0].Gaps[0].BlankGapID = groupPointer("answer-gap") }},
		{"choice points to fill blank", func(b *domain.GroupBundle) { b.Group.Stimuli[0].Gaps[0].QuestionID = groupID(3) }},
		{"blank points to choice", func(b *domain.GroupBundle) { b.Group.Stimuli[0].Gaps[1].QuestionID = groupID(2) }},
		{"missing stable blank target", func(b *domain.GroupBundle) { b.Group.Stimuli[0].Gaps[1].BlankGapID = nil }},
		{"answer row is not stable target", func(b *domain.GroupBundle) { b.Group.Stimuli[0].Gaps[1].BlankGapID = groupPointer(groupID(11)) }},
		{"printed label is not target", func(b *domain.GroupBundle) { b.Group.Stimuli[0].Gaps[1].BlankGapID = groupPointer("5") }},
		{"wrong blank target", func(b *domain.GroupBundle) { b.Group.Stimuli[0].Gaps[1].BlankGapID = groupPointer("other") }},
		{"same response twice", func(b *domain.GroupBundle) {
			b.Group.Stimuli[0].Gaps[1] = domain.GroupGapBinding{Kind: "question", GapID: "material-blank", QuestionID: groupID(2)}
		}},
		{"missing recording policy", func(b *domain.GroupBundle) { b.Group.Recordings = nil }},
		{"orphan recording", func(b *domain.GroupBundle) { b.Group.Recordings[0].AssetID = groupID(99) }},
		{"image recording", func(b *domain.GroupBundle) { b.Group.Recordings[0].AssetID = groupID(7) }},
		{"zero playback limit", func(b *domain.GroupBundle) { b.Group.Recordings[0].Policy.MaxPlays = groupPointer(0) }},
		{"negative playback limit", func(b *domain.GroupBundle) { b.Group.Recordings[0].Policy.MaxPlays = groupPointer(-1) }},
		{"two allowances for one file", func(b *domain.GroupBundle) {
			r := b.Group.Recordings[0]
			r.ID = groupID(99)
			b.Group.Recordings = append(b.Group.Recordings, r)
		}},
		{"oversized transcript", func(b *domain.GroupBundle) {
			b.Group.Recordings[0].Transcript = groupPointer(strings.Repeat("a", 100001))
		}},
		{"question and shared audio scope", func(b *domain.GroupBundle) {
			b.Questions[0].Input.MediaAssetID = groupPointer(groupID(6))
			b.Questions[0].MediaAssetKind = groupPointer("audio")
			b.Questions[0].Input.Audio = &questions.AudioPolicy{}
		}},
		{"missing asset kind", func(b *domain.GroupBundle) { b.Questions[0].Input.MediaAssetID = groupPointer(groupID(99)) }},
		{"orphan asset kind", func(b *domain.GroupBundle) { b.Questions[0].MediaAssetKind = groupPointer("image") }},
		{"bad grading data", func(b *domain.GroupBundle) { b.Questions[0].Input.Options[0].IsCorrect = false }},
		{"blank key lost", func(b *domain.GroupBundle) { b.Questions[1].Input.Blanks[0].AcceptedAnswers = nil }},
		{"invalid question unicode", func(b *domain.GroupBundle) { b.Questions[0].Input.Prompt = string([]byte{0xff}) }},
		{"nul answer", func(b *domain.GroupBundle) { b.Questions[1].Input.Blanks[0].AcceptedAnswers[0] = "abc\x00def" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle := groupFixture()
			tc.change(&bundle)
			err := bundle.Validate()
			var invalid *domain.GroupError
			if !errors.As(err, &invalid) {
				t.Fatalf("wanted anchored group error, got %v", err)
			}
			if strings.Contains(err.Error(), "Lời thoại") || strings.Contains(err.Error(), "chính xác") {
				t.Fatal("private content escaped into diagnostics")
			}
		})
	}
}

func TestGroupCopyRemapsFullGraphWithoutChangingMeaning(t *testing.T) {
	source := groupFixture()
	before, _ := json.Marshal(source)
	copied, err := source.Copy(uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(source)
	if string(before) != string(after) {
		t.Fatal("copy mutated the source")
	}
	if copied.Group.ID == source.Group.ID || copied.Group.Stimuli[0].ID == source.Group.Stimuli[0].ID || copied.Group.Recordings[0].ID == source.Group.Recordings[0].ID {
		t.Fatal("editable graph identity was reused")
	}
	for i, member := range copied.Group.Members {
		if member.QuestionID == source.Group.Members[i].QuestionID || member.QuestionID != copied.Questions[i].ID || member.OptionOrder != source.Group.Members[i].OptionOrder {
			t.Fatal("member identity or ordering changed incorrectly")
		}
	}
	for i, material := range copied.Group.Stimuli {
		a, _ := content.Parse(source.Group.Stimuli[i].Content)
		b, _ := content.Parse(material.Content)
		if a.PlainText() != b.PlainText() || !reflect.DeepEqual(a.Assets(), b.Assets()) {
			t.Fatal("copy changed material meaning or immutable asset reference")
		}
		for j, gap := range material.Gaps {
			if gap.GapID == source.Group.Stimuli[i].Gaps[j].GapID || gap.QuestionID != copied.Group.Members[j].QuestionID {
				t.Fatal("material response binding was not remapped")
			}
		}
	}
	if *copied.Group.Stimuli[0].Gaps[1].BlankGapID != *copied.Questions[1].Input.Blanks[0].GapID || *copied.Questions[1].Input.Blanks[0].GapID == "answer-gap" {
		t.Fatal("material and question blank ends diverged")
	}
	if copied.Group.Recordings[0].AssetID != source.Group.Recordings[0].AssetID || *copied.Group.Recordings[0].Policy.MaxPlays != 2 || *copied.Group.Recordings[0].Transcript != *source.Group.Recordings[0].Transcript {
		t.Fatal("independent playback binding lost its policy or recording")
	}
	if !reflect.DeepEqual(copied.Questions[1].Input.Blanks[0].AcceptedAnswers, source.Questions[1].Input.Blanks[0].AcceptedAnswers) || !copied.Questions[0].Input.Options[0].IsCorrect || copied.Questions[0].Input.Points != "0.25" {
		t.Fatal("copy changed grading data")
	}
	if *copied.Questions[0].Input.Options[0].ID == *source.Questions[0].Input.Options[0].ID || *copied.Questions[1].Input.Blanks[0].ID == *source.Questions[1].Input.Blanks[0].ID {
		t.Fatal("answer identities were reused")
	}
	copyBeforeMutation, _ := json.Marshal(copied)
	source.Group.Instructions[0] = '!'
	source.Group.Stimuli[0].Content[0] = '!'
	source.Questions[1].Input.PromptContent[0] = '!'
	source.Questions[1].Input.Blanks[0].AcceptedAnswers[0] = "changed"
	source.Questions[0].Input.Options[0].IsCorrect = false
	source.Questions[0].Input.Tags[0] = "changed"
	*source.Group.Recordings[0].Policy.MaxPlays = 99
	*source.Group.Recordings[0].Transcript = "changed"
	copyAfterMutation, _ := json.Marshal(copied)
	if string(copyBeforeMutation) != string(copyAfterMutation) {
		t.Fatal("source edits changed the independent copy")
	}
}

func TestGroupCopyFailsAtomicallyOnUnsafeIdentityProvider(t *testing.T) {
	for _, generate := range []func() string{nil, func() string { return "invalid" }, func() string { return groupID(1) }, func() string { return groupID(6) }, func() string { return groupID(9) }, func() string { return groupID(99) }} {
		bundle := groupFixture()
		before, _ := json.Marshal(bundle)
		copied, err := bundle.Copy(generate)
		if err == nil || !reflect.DeepEqual(copied, domain.GroupBundle{}) {
			t.Fatal("unsafe identity provider returned a partial copy")
		}
		after, _ := json.Marshal(bundle)
		if string(before) != string(after) {
			t.Fatal("failed copy mutated source")
		}
	}
}

func TestGroupBudgetIncludesResolvedQuestions(t *testing.T) {
	bundle := groupFixture()
	bundle.Questions[0].Input.Prompt = strings.Repeat("a", domain.MaxGroupBytes)
	if err := bundle.Validate(); err == nil {
		t.Fatal("oversized complete context was accepted")
	}
}

func TestGroupCopyResolvesIdentityInsteadOfCatalogPosition(t *testing.T) {
	source := groupFixture()
	source.Questions[0], source.Questions[1] = source.Questions[1], source.Questions[0]
	source.Group.Members[0].QuestionID = strings.ToUpper(source.Group.Members[0].QuestionID)
	source.Group.Stimuli[0].Gaps[0].QuestionID = strings.ToUpper(source.Group.Stimuli[0].Gaps[0].QuestionID)
	copied, err := source.Copy(uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	if copied.Group.Members[0].QuestionID != copied.Questions[1].ID || copied.Group.Stimuli[0].Gaps[0].QuestionID != copied.Questions[1].ID {
		t.Fatal("catalog order or UUID case changed member binding")
	}
}

func TestGroupScopesLocalGapNamesToTheirMaterialAndQuestion(t *testing.T) {
	source := groupFixture()
	material := source.Group.Stimuli[0]
	material.ID = groupID(90)
	material.Content = json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"gap","id":"material-blank","label":"1"}]}]}`)
	material.Gaps = []domain.GroupGapBinding{{Kind: "blank", GapID: "material-blank", QuestionID: groupID(91), BlankGapID: groupPointer("answer-gap")}}
	source.Group.Stimuli = append(source.Group.Stimuli, material)
	question := groupFixture().Questions[1]
	question.ID = groupID(91)
	question.Input.Blanks[0].ID = groupPointer(groupID(92))
	source.Questions = append(source.Questions, question)
	source.Group.Members = append(source.Group.Members, domain.GroupMember{QuestionID: question.ID, OptionOrder: "shuffle"})
	copied, err := source.Copy(uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	first := copied.Group.Stimuli[0].Gaps[1]
	second := copied.Group.Stimuli[2].Gaps[0]
	if first.GapID == second.GapID || *first.BlankGapID == *second.BlankGapID || first.QuestionID == second.QuestionID {
		t.Fatal("locally repeated names collapsed distinct response bindings")
	}
	if *first.BlankGapID != *copied.Questions[1].Input.Blanks[0].GapID || *second.BlankGapID != *copied.Questions[2].Input.Blanks[0].GapID {
		t.Fatal("local gap was attached to another question")
	}
}

func TestGroupRecordingCoversRepeatedAssetButNotSeparateMemberAudio(t *testing.T) {
	bundle := groupFixture()
	material := bundle.Group.Stimuli[1]
	material.ID = groupID(90)
	bundle.Group.Stimuli = append(bundle.Group.Stimuli, material)
	bundle.Group.Recordings[0].Policy.MaxPlays = nil
	bundle.Questions[0].Input.MediaAssetID = groupPointer(groupID(91))
	bundle.Questions[0].Input.Audio = &questions.AudioPolicy{MaxPlays: groupPointer(1)}
	bundle.Questions[0].MediaAssetKind = groupPointer("audio")
	if err := bundle.Validate(); err != nil {
		t.Fatalf("one shared recording with independent member audio: %v", err)
	}
	bundle.Questions[0].Input.MediaAssetID = groupPointer(groupID(7))
	if err := bundle.Validate(); err == nil {
		t.Fatal("one immutable asset claimed incompatible kinds")
	}
}

func TestGroupRejectsAggregateCountsBeforeWalkingContent(t *testing.T) {
	for _, change := range []func(*domain.GroupBundle){
		func(b *domain.GroupBundle) {
			b.Group.Members = make([]domain.GroupMember, domain.MaxGroupMembers+1)
			b.Questions = make([]domain.GroupQuestion, len(b.Group.Members))
		},
		func(b *domain.GroupBundle) { b.Group.Stimuli = make([]domain.GroupStimulus, domain.MaxGroupStimuli+1) },
		func(b *domain.GroupBundle) {
			b.Group.Recordings = make([]domain.GroupRecording, domain.MaxGroupRecordings+1)
		},
	} {
		bundle := groupFixture()
		change(&bundle)
		var invalid *domain.GroupError
		if err := bundle.Validate(); !errors.As(err, &invalid) || invalid.Rule != "group_limits" {
			t.Fatalf("expected early budget failure, got %v", err)
		}
	}
}

func TestGroupLimitsMatchOpenAPI(t *testing.T) {
	spec, err := openapi3.NewLoader().LoadFromFile("../../../../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(spec.Components.Schemas["QuestionGroup"].Value.Extensions["x-group-limits"])
	if err != nil {
		t.Fatal(err)
	}
	var contract map[string]int
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatal(err)
	}
	limits := map[string]int{"bytes": domain.MaxGroupBytes, "members": domain.MaxGroupMembers, "stimuli": domain.MaxGroupStimuli, "recordings": domain.MaxGroupRecordings, "title": domain.MaxGroupTitle, "transcript": domain.MaxGroupTranscript}
	if !maps.Equal(contract, limits) {
		t.Fatalf("group limits %v differ from contract %v", limits, contract)
	}
}

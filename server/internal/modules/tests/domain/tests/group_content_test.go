package domain_test

import (
	"encoding/json"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

const sharedPrompt = `{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Điền ","marks":[]},{"type":"gap","id":"first-gap","label":"1"},{"type":"text","text":" và ","marks":[]},{"type":"gap","id":"second-gap","label":"2"}]}]}`

func richChoice() domain.GroupQuestion {
	kind := "audio"
	level, skill := questions.Level("b1"), questions.Skill("listening")
	return domain.GroupQuestion{ID: groupID(20), MediaAssetKind: &kind, Input: questions.Input{
		Type: questions.MultipleChoice, Level: &level, Skill: &skill, Prompt: "Nghe rồi chọn", Points: "1.50",
		PromptContent:      json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Nghe rồi chọn","marks":[]}]}]}`),
		Explanation:        groupPointer("Vì bài nghe nói vậy"),
		ExplanationContent: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Vì bài nghe nói vậy","marks":[]}]}]}`),
		MediaAssetID:       groupPointer(groupID(40)),
		Audio:              &questions.AudioPolicy{MaxPlays: groupPointer(2), AllowSeek: true, ShowTranscriptAfterSubmit: true, AllowPause: groupPointer(false)},
		Transcript:         groupPointer("Lời thoại"),
		Tags:               []string{"nghe", "b1"},
		Options: []questions.OptionInput{
			{ID: groupPointer(groupID(21)), Text: "một", IsCorrect: true, Content: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"một","marks":["bold"]}]}]}`)},
			{ID: groupPointer(groupID(22)), Text: "hai"},
			{ID: groupPointer(groupID(23)), Text: "ba", IsCorrect: true},
		},
	}}
}

func richBlanks() domain.GroupQuestion {
	return domain.GroupQuestion{ID: groupID(30), Input: questions.Input{
		Type: questions.FillBlank, Prompt: "[1] [2]", Points: "2", PromptContent: json.RawMessage(sharedPrompt),
		Blanks: []questions.BlankInput{
			{ID: groupPointer(groupID(31)), GapID: groupPointer("first-gap"), Ordinal: 1, AcceptedAnswers: []string{"một", "1"}},
			{ID: groupPointer(groupID(32)), GapID: groupPointer("second-gap"), Ordinal: 2, CaseSensitive: true, AcceptedAnswers: []string{"Hai"}},
		},
	}}
}

func shortAnswer() domain.GroupQuestion {
	return domain.GroupQuestion{ID: groupID(50), Input: questions.Input{
		Type: questions.ShortAnswer, Prompt: "Viết một câu", Points: "3", SampleAnswer: groupPointer("Một câu mẫu"),
		MediaAssetID: groupPointer(groupID(41)), MediaAlt: groupPointer("Hình minh hoạ"),
	}}
}

func matching() domain.GroupQuestion {
	return domain.GroupQuestion{ID: groupID(60), Input: questions.Input{
		Type: questions.Matching, Prompt: "Nối", Points: "4", MatchAllowReuse: groupPointer(false),
		Options: []questions.OptionInput{
			{ID: groupPointer(groupID(61)), Text: "Đoạn một"},
			{ID: groupPointer(groupID(62)), Text: "Đoạn hai"},
		},
		MatchItems: []questions.MatchItemInput{
			{ID: groupPointer(groupID(63)), Text: "Tiêu đề A", CorrectOption: 0},
			{ID: groupPointer(groupID(64)), Text: "Tiêu đề B", CorrectOption: 1},
		},
	}}
}

func essay() domain.GroupQuestion {
	return domain.GroupQuestion{ID: groupID(70), Input: questions.Input{
		Type: questions.Essay, Prompt: "Viết một bài", Points: "9", SampleAnswer: groupPointer("Bài mẫu"),
		Rubric: []questions.RubricCriterionInput{{ID: groupPointer(groupID(71)), Label: "Nội dung"}, {ID: groupPointer(groupID(72)), Label: "Ngữ pháp"}},
	}}
}

func sources() map[string]domain.GroupQuestion {
	fixture := groupFixture()
	return map[string]domain.GroupQuestion{
		"a choice question":                       fixture.Questions[0],
		"a blank with a rich prompt":              fixture.Questions[1],
		"two blanks with a rich prompt":           richBlanks(),
		"every field of a multiple choice":        richChoice(),
		"a short answer with a sample and a note": shortAnswer(),
		"a matching question":                     matching(),
		"an essay with a rubric":                  essay(),
	}
}

func copyOf(t *testing.T, source domain.GroupQuestion) domain.GroupQuestion {
	t.Helper()
	copied, err := source.Copy(uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	return copied
}

func TestACopyNobodyTouchedHasTheSameContentAsItsSource(t *testing.T) {
	for name, source := range sources() {
		t.Run(name, func(t *testing.T) {
			copied := copyOf(t, source)
			if !source.SameContent(copied) || !copied.SameContent(source) {
				t.Fatal("an unedited copy compared as a different content")
			}
			if again := copyOf(t, copied); !source.SameContent(again) {
				t.Fatal("a copy of a copy compared as a different content")
			}
		})
	}
}

func TestContentIsTheSameAfterAStorageRoundTrip(t *testing.T) {
	source := richBlanks()
	copied := copyOf(t, source)
	copied.Input.PromptContent = json.RawMessage(" " + string(copied.Input.PromptContent) + "\n")
	copied.Input.ExplanationContent = json.RawMessage("null")
	copied.Input.Tags = []string{}
	source.Input.Tags = nil
	source.Input.Blanks[0].AcceptedAnswers = []string{"1", "một"}
	if !source.SameContent(copied) {
		t.Fatal("whitespace, a null document, empty against missing tags and the order of the accepted answers are not edits")
	}
}

func TestContentIgnoresTheOrderOfTagsAndReadsNoTranscriptAsAnEmptyOne(t *testing.T) {
	source := richChoice()
	copied := copyOf(t, source)
	copied.Input.Tags = []string{"b1", "nghe"}
	if !source.SameContent(copied) {
		t.Fatal("the order of the tags is not an edit")
	}
	source.Input.Transcript, copied.Input.Transcript = nil, groupPointer("")
	if !source.SameContent(copied) {
		t.Fatal("no transcript and an empty one read the same to a student")
	}
}

func TestAnyEditToTheContentIsADifferentContent(t *testing.T) {
	edits := []struct {
		name   string
		source func() domain.GroupQuestion
		edit   func(*domain.GroupQuestion)
	}{
		{"the type", richChoice, func(q *domain.GroupQuestion) { q.Input.Type = questions.SingleChoice }},
		{"the prompt", richChoice, func(q *domain.GroupQuestion) { q.Input.Prompt += "." }},
		{"the rich prompt", richChoice, func(q *domain.GroupQuestion) {
			q.Input.PromptContent = json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Nghe rồi chọn","marks":["bold"]}]}]}`)
		}},
		{"the points", richChoice, func(q *domain.GroupQuestion) { q.Input.Points = "2.00" }},
		{"the level", richChoice, func(q *domain.GroupQuestion) { q.Input.Level = nil }},
		{"the skill", richChoice, func(q *domain.GroupQuestion) { q.Input.Skill = groupPointer(questions.Skill("reading")) }},
		{"a tag", richChoice, func(q *domain.GroupQuestion) { q.Input.Tags = append(q.Input.Tags, "mới") }},
		{"the tags removed", richChoice, func(q *domain.GroupQuestion) { q.Input.Tags = nil }},
		{"the explanation", richChoice, func(q *domain.GroupQuestion) { q.Input.Explanation = groupPointer("Khác") }},
		{"the rich explanation", richChoice, func(q *domain.GroupQuestion) { q.Input.ExplanationContent = nil }},
		{"the sample answer", shortAnswer, func(q *domain.GroupQuestion) { q.Input.SampleAnswer = groupPointer("Khác") }},
		{"the image's alt text", shortAnswer, func(q *domain.GroupQuestion) { q.Input.MediaAlt = groupPointer("Khác") }},
		{"the image", shortAnswer, func(q *domain.GroupQuestion) { q.Input.MediaAssetID = groupPointer(groupID(42)) }},
		{"the image removed", shortAnswer, func(q *domain.GroupQuestion) {
			q.Input.MediaAssetID, q.Input.MediaAlt, q.MediaAssetKind = nil, nil, nil
		}},
		{"the number of plays", richChoice, func(q *domain.GroupQuestion) { q.Input.Audio.MaxPlays = groupPointer(3) }},
		{"unlimited plays", richChoice, func(q *domain.GroupQuestion) { q.Input.Audio.MaxPlays = nil }},
		{"seeking", richChoice, func(q *domain.GroupQuestion) { q.Input.Audio.AllowSeek = false }},
		{"the transcript's reveal", richChoice, func(q *domain.GroupQuestion) { q.Input.Audio.ShowTranscriptAfterSubmit = false }},
		{"pausing", richChoice, func(q *domain.GroupQuestion) { q.Input.Audio.AllowPause = groupPointer(true) }},
		{"the transcript", richChoice, func(q *domain.GroupQuestion) { q.Input.Transcript = groupPointer("Khác") }},
		{"an option's text", richChoice, func(q *domain.GroupQuestion) { q.Input.Options[1].Text = "bốn" }},
		{"which option is correct", richChoice, func(q *domain.GroupQuestion) { q.Input.Options[1].IsCorrect = true }},
		{"an option's rich content", richChoice, func(q *domain.GroupQuestion) { q.Input.Options[0].Content = nil }},
		{"the order of the options", richChoice, func(q *domain.GroupQuestion) {
			q.Input.Options[0], q.Input.Options[1] = q.Input.Options[1], q.Input.Options[0]
		}},
		{"an option removed", richChoice, func(q *domain.GroupQuestion) { q.Input.Options = q.Input.Options[:2] }},
		{"an accepted answer", richBlanks, func(q *domain.GroupQuestion) {
			q.Input.Blanks[0].AcceptedAnswers = append(q.Input.Blanks[0].AcceptedAnswers, "I")
		}},
		{"case sensitivity", richBlanks, func(q *domain.GroupQuestion) { q.Input.Blanks[0].CaseSensitive = true }},
		{"a blank's ordinal", richBlanks, func(q *domain.GroupQuestion) { q.Input.Blanks[1].Ordinal = 3 }},
		{"the gaps the blanks answer", richBlanks, func(q *domain.GroupQuestion) {
			first, second := q.Input.Blanks[0].GapID, q.Input.Blanks[1].GapID
			q.Input.Blanks[0].GapID, q.Input.Blanks[1].GapID = second, first
		}},
		{"a blank removed", richBlanks, func(q *domain.GroupQuestion) { q.Input.Blanks = q.Input.Blanks[:1] }},
		{"the number to choose", richChoice, func(q *domain.GroupQuestion) { q.Input.SelectCount = groupPointer(2) }},
		{"the word limit", shortAnswer, func(q *domain.GroupQuestion) { q.Input.MaxWords = groupPointer(3) }},
		{"reuse of a choice", matching, func(q *domain.GroupQuestion) { q.Input.MatchAllowReuse = groupPointer(true) }},
		{"an item's text", matching, func(q *domain.GroupQuestion) { q.Input.MatchItems[0].Text = "Khác" }},
		{"an item's choice", matching, func(q *domain.GroupQuestion) { q.Input.MatchItems[0].CorrectOption = 1 }},
		{"an item removed", matching, func(q *domain.GroupQuestion) { q.Input.MatchItems = q.Input.MatchItems[:1] }},
		{"a criterion's label", essay, func(q *domain.GroupQuestion) { q.Input.Rubric[0].Label = "Khác" }},
		{"a criterion removed", essay, func(q *domain.GroupQuestion) { q.Input.Rubric = nil }},
	}
	for _, edit := range edits {
		t.Run(edit.name, func(t *testing.T) {
			source := edit.source()
			edited := copyOf(t, source)
			edit.edit(&edited)
			if source.SameContent(edited) || edited.SameContent(source) {
				t.Fatalf("editing %s was not noticed", edit.name)
			}
		})
	}
}

func TestEveryFieldOfAQuestionIsEitherComparedOrAnIdentity(t *testing.T) {
	known := map[reflect.Type]map[string]bool{
		reflect.TypeFor[questions.Input](): {
			"PromptContent": true, "ExplanationContent": true, "Type": true, "Level": true, "Skill": true, "Prompt": true, "MediaAssetID": true,
			"MediaAlt": true, "Audio": true, "Transcript": true, "Options": true, "Blanks": true, "Points": true, "Explanation": true,
			"SampleAnswer": true, "Tags": true, "SelectCount": true, "MaxWords": true, "MatchAllowReuse": true,
			"MatchItems": true, "Rubric": true,
		},
		reflect.TypeFor[questions.MatchItemInput]():       {"ID": false, "Text": true, "CorrectOption": true},
		reflect.TypeFor[questions.RubricCriterionInput](): {"ID": false, "Label": true},
		reflect.TypeFor[questions.OptionInput]():          {"Content": true, "ID": false, "Text": true, "IsCorrect": true},
		reflect.TypeFor[questions.BlankInput]():           {"GapID": false, "ID": false, "Ordinal": true, "AcceptedAnswers": true, "CaseSensitive": true},
		reflect.TypeFor[questions.AudioPolicy]():          {"MaxPlays": true, "AllowSeek": true, "ShowTranscriptAfterSubmit": true, "AllowPause": true},
		reflect.TypeFor[domain.GroupQuestion]():           {"ID": false, "Input": true, "MediaAssetKind": true},
	}
	for kind, fields := range known {
		for i := range kind.NumField() {
			if _, listed := fields[kind.Field(i).Name]; !listed {
				t.Errorf("%s.%s is new: compare it in SameContent, or list it here as an identity, so that a restore never discards an edit to it", kind.Name(), kind.Field(i).Name)
			}
		}
		for name := range fields {
			if _, found := kind.FieldByName(name); !found {
				t.Errorf("%s.%s is listed here and no longer exists", kind.Name(), name)
			}
		}
	}
}

func TestContentOfAQuestionThatCannotBeCopiedIsNeverTheSame(t *testing.T) {
	broken := richBlanks()
	broken.Input.Blanks[0].GapID = nil
	if broken.SameContent(broken) {
		t.Fatal("a question with a gap-less blank under rich content compared as the same as itself")
	}
	invalid := richChoice()
	invalid.Input.PromptContent = json.RawMessage(`{`)
	if invalid.SameContent(invalid) {
		t.Fatal("a question whose rich content does not parse compared as the same as itself")
	}
}

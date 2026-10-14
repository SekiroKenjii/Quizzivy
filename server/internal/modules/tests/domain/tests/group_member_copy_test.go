package domain_test

import (
	"encoding/json"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/content"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestGroupQuestionCopyRemapsEveryIdentityWithoutChangingMeaning(t *testing.T) {
	source := groupFixture().Questions[1]
	before, _ := json.Marshal(source)
	copied, err := source.Copy(uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(source)
	if string(before) != string(after) {
		t.Fatal("copy mutated the source")
	}
	blank, original := copied.Input.Blanks[0], source.Input.Blanks[0]
	if copied.ID == source.ID || *blank.ID == *original.ID || *blank.GapID == *original.GapID {
		t.Fatal("an identity of the question was reused")
	}
	prompt, err := content.ParseQuestionPrompt(copied.Input.PromptContent)
	if err != nil {
		t.Fatal(err)
	}
	if gaps := prompt.GapIDs(); len(gaps) != 1 || gaps[0] != *blank.GapID {
		t.Fatalf("prompt gap %v does not follow the blank's new gap %s", gaps, *blank.GapID)
	}
	if !reflect.DeepEqual(blank.AcceptedAnswers, original.AcceptedAnswers) || blank.Ordinal != original.Ordinal || copied.Input.Points != source.Input.Points || copied.Input.Type != source.Input.Type {
		t.Fatal("copy changed grading data")
	}
}

func TestGroupQuestionCopyRemapsOptionIdentities(t *testing.T) {
	source := groupFixture().Questions[0]
	copied, err := source.Copy(uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	for i, option := range copied.Input.Options {
		if *option.ID == *source.Input.Options[i].ID || option.Text != source.Input.Options[i].Text || option.IsCorrect != source.Input.Options[i].IsCorrect {
			t.Fatalf("option %d: identity reused or content changed", i)
		}
	}
	source.Input.Options[0].IsCorrect = false
	source.Input.Tags[0] = "changed"
	if !copied.Input.Options[0].IsCorrect || copied.Input.Tags[0] == "changed" {
		t.Fatal("source edits changed the independent copy")
	}
}

func TestGroupQuestionCopyKeepsTheBankQuestionsAssetAndKind(t *testing.T) {
	kind, asset := "audio", groupID(40)
	source := groupFixture().Questions[0]
	source.Input.MediaAssetID, source.MediaAssetKind = &asset, &kind
	copied, err := source.Copy(uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	if copied.Input.MediaAssetID == nil || *copied.Input.MediaAssetID != asset || copied.MediaAssetKind == nil || *copied.MediaAssetKind != kind {
		t.Fatal("an immutable asset reference was changed")
	}
}

func TestGroupQuestionCopyFailsOnUnsafeIdentityProviders(t *testing.T) {
	for _, generate := range []func() string{nil, func() string { return "invalid" }, func() string { return groupID(2) }, func() string { return groupID(9) }, func() string { return groupID(11) }} {
		source := groupFixture().Questions[0]
		copied, err := source.Copy(generate)
		if err == nil || !reflect.DeepEqual(copied, domain.GroupQuestion{}) {
			t.Fatal("an unsafe identity provider returned a partial copy")
		}
	}
}

package application_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	questions "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/domain"
	"testing"
)

func TestSubmittedGroupCommandsRefuseOversizedMemberBeforePersistence(t *testing.T) {
	app := application.New(nil)
	bundle := domain.GroupBundle{Questions: []domain.GroupQuestion{{ID: "first", Input: questions.Input{Type: questions.ShortAnswer}}, {ID: "second", Input: questions.Input{Type: questions.MultipleChoice, Options: make([]questions.OptionInput, 9)}}}}
	for name, run := range map[string]func() error{
		"create": func() error {
			_, err := app.Commands.CreateGroup.Handle(context.Background(), command.CreateGroup{Bundle: bundle})
			return err
		},
		"update": func() error {
			_, err := app.Commands.UpdateGroup.Handle(context.Background(), command.UpdateGroup{Bundle: bundle})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			var invalid *questions.ValidationError
			if err := run(); !errors.As(err, &invalid) || invalid.Fields[0].Field != "options" {
				t.Fatalf("expected member cap: %v", err)
			}
		})
	}
}

func TestStructuralGroupCopyPreservesNullableMetadataAndStoredNineOptions(t *testing.T) {
	level, skill := questions.Level("c2"), questions.Skill("grammar")
	first, second := uuid.NewString(), uuid.NewString()
	in := questions.Input{Type: questions.SingleChoice, Prompt: "Legacy", Points: "1", Level: &level, Skill: &skill}
	for i := range 9 {
		in.Options = append(in.Options, questions.OptionInput{Text: "Choice", IsCorrect: i == 0})
	}
	bundle := domain.GroupBundle{Group: domain.QuestionGroup{ID: uuid.NewString(), Title: "Group", Members: []domain.GroupMember{{QuestionID: first, OptionOrder: "fixed"}, {QuestionID: second, OptionOrder: "shuffle"}}}, Questions: []domain.GroupQuestion{{ID: first, Input: in}, {ID: second, Input: questions.Input{Type: questions.ShortAnswer, Prompt: "Unset", Points: "1"}}}}
	copied, err := bundle.Copy(uuid.NewString)
	if err != nil {
		t.Fatal(err)
	}
	actual, unset := copied.Questions[0].Input, copied.Questions[1].Input
	if copied.Group.ID == bundle.Group.ID || copied.Questions[0].ID == first || actual.Level == nil || *actual.Level != level || actual.Skill == nil || *actual.Skill != skill || len(actual.Options) != 9 || unset.Level != nil || unset.Skill != nil {
		t.Fatalf("structural copy changed metadata/content: %+v", copied)
	}
	*actual.Level = "a1"
	if *bundle.Questions[0].Input.Level != "c2" {
		t.Fatal("copied metadata aliases original")
	}
}

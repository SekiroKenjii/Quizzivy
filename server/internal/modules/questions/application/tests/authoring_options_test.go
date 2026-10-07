package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/questions/application"
	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/access"
	"testing"
)

func TestSubmittedBankCommandsRefuseNineBeforePersistence(t *testing.T) {
	repo := &authoringRepository{}
	app := application.New(repo, nil)
	in := domain.Input{Type: domain.SingleChoice, Prompt: "Choose", Points: "1"}
	for i := range 9 {
		in.Options = append(in.Options, domain.OptionInput{Text: "Choice", IsCorrect: i == 0})
	}
	for name, run := range map[string]func() error{
		"create": func() error {
			_, err := app.Commands.Create.Handle(context.Background(), command.Create{Request: domain.WriteRequest{Input: in}})
			return err
		},
		"update": func() error {
			_, err := app.Commands.Update.Handle(context.Background(), command.Update{Request: domain.WriteRequest{ID: "known", Input: in}})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			var invalid *domain.ValidationError
			if err := run(); !errors.As(err, &invalid) || invalid.Fields[0].Field != "options" {
				t.Fatalf("expected authoring refusal: %v", err)
			}
		})
	}
	if repo.writes != 0 {
		t.Fatalf("refused content persisted: %d", repo.writes)
	}
	in.Options = in.Options[:8]
	if _, err := app.Commands.Create.Handle(context.Background(), command.Create{Request: domain.WriteRequest{Input: in}}); err != nil || repo.writes != 1 {
		t.Fatalf("eight options refused or not persisted: %v writes=%d", err, repo.writes)
	}
}

type authoringRepository struct {
	domain.Repository
	writes    int
	source    domain.Question
	lastInput domain.Input
}

func (r *authoringRepository) Create(_ context.Context, in domain.WriteInput) (domain.Question, error) {
	r.writes++
	r.lastInput = in.Input
	return domain.Question{Type: in.Input.Type}, nil
}

func (r *authoringRepository) Update(ctx context.Context, in domain.WriteInput) (domain.Question, error) {
	return r.Create(ctx, in)
}

func (r *authoringRepository) Get(_ context.Context, _ access.Scope, _ string) (domain.Question, error) {
	return r.source, nil
}

func TestBodylessDuplicatePreservesStoredNineOptionsAndMetadata(t *testing.T) {
	level, skill := domain.Level("c2"), domain.Skill("speaking")
	repo := &authoringRepository{source: domain.Question{ID: "source", Type: domain.SingleChoice, Prompt: "Legacy", Points: "1", Level: &level, Skill: &skill, Tags: []string{"legacy"}}}
	for i := range 9 {
		repo.source.Options = append(repo.source.Options, domain.Option{Text: "Choice", IsCorrect: i == 0})
	}
	app := application.New(repo, nil)
	_, err := app.Commands.Duplicate.Handle(context.Background(), command.Duplicate{Request: domain.WriteRequest{ID: repo.source.ID}})
	actual := repo.lastInput
	if err != nil || repo.writes != 1 || len(actual.Options) != 9 || actual.Level == nil || *actual.Level != level || actual.Skill == nil || *actual.Skill != skill || len(actual.Tags) != 1 || actual.Tags[0] != "legacy" {
		t.Fatalf("stored duplicate changed content: %+v writes=%d err=%v", actual, repo.writes, err)
	}
}

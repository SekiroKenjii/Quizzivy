package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"quizzivy/internal/modules/questions/application"
	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/access"

	"golang.org/x/text/unicode/norm"
)

type taggingRepository struct {
	domain.Repository
	tags []string
}

func (r *taggingRepository) AddTags(_ context.Context, _ access.Scope, _ []string, tags []string) (int, error) {
	r.tags = tags
	return 1, nil
}

func decomposedInput() domain.Input {
	alt, sample := norm.NFD.String("Ảnh cái nón"), norm.NFD.String("Hà Nội")
	asset := "01935000-0000-7000-8000-0000000000a1"
	return domain.Input{
		Type: domain.ShortAnswer, Prompt: norm.NFD.String("Mô tả bức tranh"), Points: "1",
		SampleAnswer: &sample, MediaAlt: &alt, MediaAssetID: &asset,
		Tags: []string{norm.NFD.String("nghé"), "shared"},
	}
}

func assertComposed(t *testing.T, got domain.Input) {
	t.Helper()
	if got.Prompt != norm.NFC.String(got.Prompt) || *got.SampleAnswer != norm.NFC.String(*got.SampleAnswer) ||
		*got.MediaAlt != norm.NFC.String(*got.MediaAlt) || got.Tags[0] != norm.NFC.String(got.Tags[0]) {
		t.Fatalf("the repository was handed decomposed text: %+v", got)
	}
	if got.Prompt != "Mô tả bức tranh" || *got.SampleAnswer != "Hà Nội" || got.Tags[0] != "nghé" || got.Tags[1] != "shared" {
		t.Fatalf("composing changed more than the form: %+v", got)
	}
	if *got.MediaAssetID != "01935000-0000-7000-8000-0000000000a1" {
		t.Fatalf("an identity changed: %s", *got.MediaAssetID)
	}
}

func TestCreateAndUpdateHandTheRepositoryComposedText(t *testing.T) {
	for name, run := range map[string]func(*application.Application) error{
		"create": func(app *application.Application) error {
			_, err := app.Commands.Create.Handle(context.Background(), command.Create{Request: domain.WriteRequest{Input: decomposedInput()}})
			return err
		},
		"update": func(app *application.Application) error {
			_, err := app.Commands.Update.Handle(context.Background(), command.Update{Request: domain.WriteRequest{ID: "known", Input: decomposedInput()}})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &authoringRepository{}
			if err := run(application.New(repo, fixedMediaKind("image"))); err != nil {
				t.Fatal(err)
			}
			if repo.writes != 1 {
				t.Fatalf("writes=%d", repo.writes)
			}
			assertComposed(t, repo.lastInput)
		})
	}
}

func TestAddTagsHandsTheRepositoryComposedTags(t *testing.T) {
	repo := &taggingRepository{}
	_, err := application.New(repo, nil).Commands.AddTags.Handle(context.Background(), command.AddTags{
		IDs: []string{"a"}, Tags: []string{norm.NFD.String("nghé"), "shared"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.tags) != 2 || repo.tags[0] != "nghé" || repo.tags[0] != norm.NFC.String(repo.tags[0]) || repo.tags[1] != "shared" {
		t.Fatalf("tags=%q", repo.tags)
	}
}

func TestACommandRefusesWhatComposingLeavesOverALimitBeforeAnyWrite(t *testing.T) {
	asset := "01935000-0000-7000-8000-0000000000a1"
	alt := strings.Repeat("क़", domain.MaxMediaAltLength)
	in := domain.Input{Type: domain.ShortAnswer, Prompt: "Mô tả", Points: "1", MediaAssetID: &asset, MediaAlt: &alt}
	repo := &authoringRepository{}
	app := application.New(repo, fixedMediaKind("image"))
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
			if err := run(); !errors.As(err, &invalid) || invalid.Fields[0].Field != "mediaAlt" {
				t.Fatalf("err=%v", err)
			}
		})
	}
	if repo.writes != 0 {
		t.Fatalf("a refused value reached the repository %d times", repo.writes)
	}
}

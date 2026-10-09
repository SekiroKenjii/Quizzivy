package application_test

import (
	"context"
	"quizzivy/internal/modules/questions/application"
	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/access"
	"testing"
)

type fixedMediaKind string

func (k fixedMediaKind) Kind(context.Context, access.Scope, string) (string, error) {
	return string(k), nil
}

func TestBodylessDuplicateKeepsTheAltTextOfAnImageQuestion(t *testing.T) {
	asset, alt := "01a04900-0000-7000-8000-000000000001", "Một chú mèo ngồi trên ghế"
	repo := &authoringRepository{source: domain.Question{ID: "source", Type: domain.ShortAnswer, Prompt: "Mô tả", Points: "1", MediaAssetID: &asset, MediaAlt: &alt}}
	_, err := application.New(repo, fixedMediaKind("image")).Commands.Duplicate.Handle(context.Background(), command.Duplicate{Request: domain.WriteRequest{ID: repo.source.ID}})
	if err != nil || repo.writes != 1 {
		t.Fatalf("duplicate: %v, writes = %d", err, repo.writes)
	}
	if got := repo.lastInput.MediaAlt; got == nil || *got != alt {
		t.Fatalf("the copy's alt text = %v, want %q", got, alt)
	}
}

func TestACommandRefusesAltTextOnAnAudioFileBeforeAnyWrite(t *testing.T) {
	asset, alt := "01a04900-0000-7000-8000-000000000001", "Một đoạn hội thoại"
	repo := &authoringRepository{}
	in := domain.Input{Type: domain.ShortAnswer, Prompt: "Nghe", Points: "1", MediaAssetID: &asset, MediaAlt: &alt, Audio: &domain.AudioPolicy{}}
	app := application.New(repo, fixedMediaKind("audio"))
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
		if err := run(); err == nil {
			t.Fatalf("%s accepted alt text on an audio file", name)
		}
	}
	if repo.writes != 0 {
		t.Fatalf("refused alt text reached the repository %d times", repo.writes)
	}
}

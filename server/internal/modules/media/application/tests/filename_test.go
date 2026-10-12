package application_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/domain"
)

func TestAnUploadWhoseFileNameHoldsANulIsRefusedBeforeAnythingIsReadOrStored(t *testing.T) {
	b := newBench()
	body := imageOf(t, 64)

	_, err := b.app.Commands.Upload.Handle(context.Background(), command.Upload{Filename: "a\x00b.png", Body: body, UploaderID: uploader})

	if !errors.Is(err, domain.ErrInvalidFilename) {
		t.Fatalf("err = %v, want ErrInvalidFilename", err)
	}
	if body.read != 0 {
		t.Errorf("the refused upload's body was read: %d bytes", body.read)
	}
	b.nothingStored(t)
}

func TestTheSameUploadWithoutTheNulIsStored(t *testing.T) {
	b := newBench()

	asset, err := b.app.Commands.Upload.Handle(context.Background(), command.Upload{Filename: "ab.png", Body: imageOf(t, 64), UploaderID: uploader})

	if err != nil || asset.OriginalFilename != "ab.png" {
		t.Fatalf("asset = %+v, err = %v, want the file stored under its name", asset, err)
	}
}

func TestAReplacementWhoseFileNameHoldsANulIsRefusedBeforeAnythingIsReadOrStored(t *testing.T) {
	app, repo, store := replacementBench()
	cmd := replacementCommand(t)
	body := imageOf(t, 32)
	cmd.Filename, cmd.Body = "a\x00b.png", body

	_, err := app.Commands.Replace.Handle(context.Background(), cmd)

	if !errors.Is(err, domain.ErrInvalidFilename) {
		t.Fatalf("err = %v, want ErrInvalidFilename", err)
	}
	if body.read != 0 || len(store.puts) != 0 || len(repo.replacements) != 0 {
		t.Errorf("the refused replacement read %d bytes, put %v and replaced %v", body.read, store.puts, repo.replacements)
	}
}

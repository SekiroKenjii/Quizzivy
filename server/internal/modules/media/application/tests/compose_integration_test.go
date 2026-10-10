//go:build integration

package application_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/domain"

	"golang.org/x/text/unicode/norm"
)

func TestAnUploadedFilenameIsStoredComposed(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	svc := newService(t, pool, newFakeStore())

	asset, err := svc.Commands.Upload.Handle(context.Background(), command.Upload{
		Filename: norm.NFD.String("bài nghe số một.mp3"), Body: bytes.NewReader(fixture(t, "cbr-128k.mp3")), UploaderID: uploader,
	})
	if err != nil {
		t.Fatal(err)
	}
	if asset.OriginalFilename != "bài nghe số một.mp3" {
		t.Fatalf("filename=%q", asset.OriginalFilename)
	}
	if want := "audio/" + asset.ID + ".mp3"; asset.StorageKey != want {
		t.Fatalf("the storage key must come from the id alone: %q", asset.StorageKey)
	}
}

func TestRenamingAnAssetStoresTheNameComposed(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	svc := newService(t, pool, newFakeStore())
	asset, err := svc.Commands.Upload.Handle(context.Background(), command.Upload{
		Filename: "bai-nghe.mp3", Body: bytes.NewReader(fixture(t, "cbr-128k.mp3")), UploaderID: uploader,
	})
	if err != nil {
		t.Fatal(err)
	}
	name := "  " + norm.NFD.String("Bài nghe số hai") + "  "
	renamed, err := svc.Commands.Update.Handle(context.Background(), command.Update{Input: domain.UpdateInput{ID: asset.ID, ActorID: uploader, DisplayName: &name}})
	if err != nil {
		t.Fatal(err)
	}
	if renamed.DisplayName != "Bài nghe số hai" || renamed.StorageKey != asset.StorageKey || renamed.OriginalFilename != "bai-nghe.mp3" {
		t.Fatalf("renamed=%+v", renamed)
	}
}

func TestANameComposingLeavesOverItsLimitIsRefusedAsInvalid(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	svc := newService(t, pool, newFakeStore())
	asset, err := svc.Commands.Upload.Handle(context.Background(), command.Upload{
		Filename: "bai-nghe.mp3", Body: bytes.NewReader(fixture(t, "cbr-128k.mp3")), UploaderID: uploader,
	})
	if err != nil {
		t.Fatal(err)
	}
	name := strings.Repeat("क़", domain.MaxDisplayNameLength)
	_, err = svc.Commands.Update.Handle(context.Background(), command.Update{Input: domain.UpdateInput{ID: asset.ID, ActorID: uploader, DisplayName: &name}})
	if !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("err=%v", err)
	}
}

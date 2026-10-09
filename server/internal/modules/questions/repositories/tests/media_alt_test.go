//go:build integration

package repositories_test

import (
	"context"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func altAsset(t *testing.T, tx pgx.Tx, owner, kind string) string {
	t.Helper()
	var id string
	err := tx.QueryRow(context.Background(), `INSERT INTO app.media_assets
		(kind,storage_key,mime_type,bytes,duration_ms,original_filename,checksum_sha256,uploaded_by)
		VALUES ($1::app.media_kind,$2,CASE WHEN $1='audio' THEN 'audio/mpeg' ELSE 'image/png' END,100,
		CASE WHEN $1='audio' THEN 1000 ELSE NULL END,'fixture',$3,$4) RETURNING id::text`,
		kind, kind+"/"+uuid.NewString(), make([]byte, 32), owner).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTheBankKeepsAltTextForAnImageAndNeverForAnythingElse(t *testing.T) {
	ctx, _, tx, owner, _ := metadataBank(t)
	repo := repositories.NewPostgres(db.NewContext(tx))
	image, audio := altAsset(t, tx, owner, "image"), altAsset(t, tx, owner, "audio")
	imageKind, audioKind := "image", "audio"
	text := func(s string) *string { return &s }

	input := func(asset *string, alt *string) domain.Input {
		return domain.Input{Type: domain.ShortAnswer, Prompt: "Mô tả", Points: "1", Tags: []string{}, MediaAssetID: asset, MediaAlt: alt}
	}
	write := func(id string, in domain.Input, kind *string) domain.Question {
		t.Helper()
		w := domain.WriteInput{ID: id, Input: in, MediaAssetKind: kind, ActorID: owner, Now: time.Now()}
		var q domain.Question
		var err error
		if id == "" {
			q, err = repo.Create(ctx, w)
		} else {
			q, err = repo.Update(ctx, w)
		}
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	want := func(step string, q domain.Question, alt *string) {
		t.Helper()
		got, err := repo.Get(ctx, access.Scope{UserID: owner}, q.ID)
		if err != nil {
			t.Fatal(err)
		}
		if (got.MediaAlt == nil) != (alt == nil) || (alt != nil && *got.MediaAlt != *alt) {
			t.Fatalf("%s: stored alt text = %v, want %v", step, deref(got.MediaAlt), deref(alt))
		}
		if (q.MediaAlt == nil) != (alt == nil) || (alt != nil && *q.MediaAlt != *alt) {
			t.Fatalf("%s: the write answered alt text %v, want %v", step, deref(q.MediaAlt), deref(alt))
		}
	}

	q := write("", input(&image, text("Một chú mèo")), &imageKind)
	want("created with an image", q, text("Một chú mèo"))

	q = write(q.ID, input(&image, text("Một chú chó")), &imageKind)
	want("edited", q, text("Một chú chó"))

	q = write(q.ID, input(&image, nil), &imageKind)
	want("a write that leaves it out replaces it", q, nil)

	q = write(q.ID, input(&image, text("Một chú mèo")), &imageKind)
	audioInput := input(&audio, nil)
	audioInput.Audio = &domain.AudioPolicy{}
	q = write(q.ID, audioInput, &audioKind)
	want("changed to audio", q, nil)

	q = write(q.ID, input(&image, text("Một chú mèo")), &imageKind)
	q = write(q.ID, input(nil, nil), nil)
	want("media cleared", q, nil)

	bypassed := input(&audio, text("kept by a caller that skipped validation"))
	bypassed.Audio = &domain.AudioPolicy{}
	q = write(q.ID, bypassed, &audioKind)
	want("alt text handed to the store with an audio file", q, nil)

	created := write("", bypassed, &audioKind)
	want("created with alt text and an audio file", created, nil)
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

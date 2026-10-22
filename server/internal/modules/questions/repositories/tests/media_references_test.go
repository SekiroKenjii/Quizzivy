//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

func mediaReferences(t *testing.T, tx pgx.Tx, question string) []string {
	t.Helper()
	rows, err := tx.Query(context.Background(), `SELECT field || ':' || media_asset_kind::text || ':' || media_asset_id::text
		FROM app.question_media_references WHERE question_id = $1 ORDER BY 1`, question)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return refs
}

func prose(blocks ...string) json.RawMessage {
	out := `{"format":"semantic_v1","blocks":[`
	for i, b := range blocks {
		if i > 0 {
			out += ","
		}
		out += b
	}
	return json.RawMessage(out + `]}`)
}

const lookParagraph = `{"type":"paragraph","content":[{"type":"text","text":"Look.","marks":[]}]}`

func imageBlock(id string) string {
	return fmt.Sprintf(`{"type":"image","assetId":%q,"alt":""}`, id)
}

// TestWriteMediaReferencesMakesTheRowsWhatTheProseNames is D-064 ruling 4:
// the rows follow the content they were written with, field by field, and
// the prompt's audio marker, which places the question's own audio, has none.
func TestWriteMediaReferencesMakesTheRowsWhatTheProseNames(t *testing.T) {
	ctx, _, tx, owner, _ := metadataBank(t)
	image, audio := altAsset(t, tx, owner, "image"), altAsset(t, tx, owner, "audio")
	var question string
	if err := tx.QueryRow(ctx, `INSERT INTO app.questions (type, prompt, points, created_by, owner_id) VALUES ('short_answer', 'Look.', 1, $1, $1) RETURNING id::text`, owner).Scan(&question); err != nil {
		t.Fatal(err)
	}
	prompt := prose(`{"type":"audio"}`, imageBlock(image), lookParagraph, imageBlock(image))
	explanation := prose(lookParagraph, imageBlock(image), fmt.Sprintf(`{"type":"audio","assetId":%q}`, audio))
	if err := repositories.WriteMediaReferences(ctx, tx, question, prompt, explanation); err != nil {
		t.Fatal(err)
	}
	want := []string{"explanation:audio:" + audio, "explanation:image:" + image, "prompt:image:" + image}
	if got := mediaReferences(t, tx, question); !slices.Equal(got, want) {
		t.Fatalf("references %v, want %v", got, want)
	}
	if err := repositories.WriteMediaReferences(ctx, tx, question, prompt, json.RawMessage(`null`)); err != nil {
		t.Fatal(err)
	}
	if got := mediaReferences(t, tx, question); !slices.Equal(got, want[2:]) {
		t.Fatalf("after the explanation went: %v, want %v", got, want[2:])
	}
	nested := prose(fmt.Sprintf(`{"type":"list","ordered":false,"start":1,"items":[[%s,%s]]}`, lookParagraph, imageBlock(image)))
	if err := repositories.WriteMediaReferences(ctx, tx, question, nested, nil); err == nil {
		t.Fatal("a prompt breaking the media rules was read as naming nothing")
	}
	if got := mediaReferences(t, tx, question); !slices.Equal(got, want[2:]) {
		t.Fatalf("a refused write changed the references: %v", got)
	}
}

// TestTheBankRefusesMediaAndClearsReferencesInTheContentsTransaction shows
// the bank write runs the reference write: an update with plain prose clears
// rows the question held, and a write with media is refused with the field
// error until R6-13 (D-064 ruling 7) and leaves nothing behind.
func TestTheBankRefusesMediaAndClearsReferencesInTheContentsTransaction(t *testing.T) {
	ctx, _, tx, owner, _ := metadataBank(t)
	repo := repositories.NewPostgres(db.NewContext(tx))
	image := altAsset(t, tx, owner, "image")
	write := func(id string, prompt json.RawMessage) (domain.Question, error) {
		in := domain.WriteInput{ID: id, ActorID: owner, Now: time.Now(),
			Input: domain.Input{Type: domain.ShortAnswer, Prompt: "Look.", PromptContent: prompt, Points: "1", Tags: []string{}}}
		if id == "" {
			return repo.Create(ctx, in)
		}
		return repo.Update(ctx, in)
	}

	var invalid *domain.ValidationError
	if _, err := write("", prose(lookParagraph, imageBlock(image))); !errors.As(err, &invalid) || invalid.Fields[0].Field != "promptContent" {
		t.Fatalf("a create with an image answered %v, want the promptContent field error", err)
	}
	if n := countRows(t, tx, `SELECT count(*) FROM app.question_media_references WHERE media_asset_id = $1`, image); n != 0 {
		t.Fatalf("the refused create left %d references", n)
	}

	q, err := write("", prose(lookParagraph))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO app.question_media_references (question_id, field, media_asset_id, media_asset_kind) VALUES ($1, 'prompt', $2, 'image')`, q.ID, image); err != nil {
		t.Fatal(err)
	}
	if _, err := write(q.ID, prose(lookParagraph)); err != nil {
		t.Fatal(err)
	}
	if got := mediaReferences(t, tx, q.ID); len(got) != 0 {
		t.Fatalf("an update whose prose names no media kept %v", got)
	}
}

func countRows(t *testing.T, tx pgx.Tx, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := tx.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

//go:build integration

package repositories_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/core/adapters"
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

// TestTheBankWriteStoresProseMediaItsWriterMayUse is R6-13's bank half of
// D-064: a write may show media in its prompt and explanation, each asset
// checked as the question-media attach path checks one, under its lock in the
// write's transaction. Another teacher's asset is refused exactly as a
// missing one is (ruling 5), a block naming the other kind is refused, the
// prompt's audio marker needs the question's own audio (ruling 3), and an
// update that omits its prose keeps the stored media (composedProse). The
// refused field never tells whether an id exists (QC-R6M-1): it is the first
// unusable reference in field order, whatever order the locks ran in.
func TestTheBankWriteStoresProseMediaItsWriterMayUse(t *testing.T) {
	ctx, _, tx, owner, foreign := metadataBank(t)
	repo := repositories.NewPostgres(db.NewContext(tx)).WithMedia(adapters.QuestionMedia{})
	image, audio := altAsset(t, tx, owner, "image"), altAsset(t, tx, owner, "audio")
	theirs := altAsset(t, tx, foreign, "image")
	audioKind := "audio"
	explained := "Look."
	write := func(id string, in domain.Input) (domain.Question, error) {
		in.Type, in.Points, in.Tags = domain.ShortAnswer, "1", []string{}
		if in.Prompt == "" {
			in.Prompt = "Look."
		}
		write := domain.WriteInput{ID: id, ActorID: owner, Now: time.Now(), Input: in}
		if in.MediaAssetID != nil {
			write.MediaAssetKind = &audioKind
		}
		if id == "" {
			return repo.Create(ctx, write)
		}
		return repo.Update(ctx, write)
	}
	refused := func(err error, field string, wrongKind bool) bool {
		var prose *domain.ProseMediaError
		return errors.As(err, &prose) && prose.Field == field && prose.WrongKind == wrongKind && errors.Is(err, domain.ErrMediaNotFound)
	}

	q, err := write("", domain.Input{PromptContent: prose(lookParagraph, imageBlock(image)),
		Explanation: &explained, ExplanationContent: prose(lookParagraph, fmt.Sprintf(`{"type":"audio","assetId":%q}`, audio))})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := mediaReferences(t, tx, q.ID), []string{"explanation:audio:" + audio, "prompt:image:" + image}; !slices.Equal(got, want) {
		t.Fatalf("references %v, want %v", got, want)
	}

	_, theirErr := write("", domain.Input{PromptContent: prose(lookParagraph, imageBlock(theirs))})
	_, missingErr := write("", domain.Input{PromptContent: prose(lookParagraph, imageBlock("01935000-0000-7000-8000-0000000000ff"))})
	if !refused(theirErr, "promptContent", false) || !refused(missingErr, "promptContent", false) || theirErr.Error() != missingErr.Error() {
		t.Fatalf("another teacher's image answered %v, a missing one %v: want the same refusal", theirErr, missingErr)
	}
	low, high := "00000000-0000-7000-8000-000000000001", "ffffffff-ffff-7fff-bfff-ffffffffffff"
	for _, probe := range []string{theirs, low} {
		for _, pair := range [][3]string{{probe, high, "promptContent"}, {high, probe, "promptContent"}, {image, probe, "explanationContent"}} {
			_, err := write("", domain.Input{PromptContent: prose(lookParagraph, imageBlock(pair[0])),
				Explanation: &explained, ExplanationContent: prose(lookParagraph, imageBlock(pair[1]))})
			if !refused(err, pair[2], false) {
				t.Fatalf("prompt %s and explanation %s answered %v: want %s whether or not the probe exists", pair[0], pair[1], err, pair[2])
			}
		}
	}
	if _, err := write(q.ID, domain.Input{PromptContent: prose(lookParagraph), Explanation: &explained, ExplanationContent: prose(lookParagraph, imageBlock(theirs))}); !refused(err, "explanationContent", false) {
		t.Fatalf("another teacher's image in an explanation answered %v", err)
	}
	if _, err := write("", domain.Input{PromptContent: prose(lookParagraph, imageBlock(theirs)),
		Explanation: &explained, ExplanationContent: prose(lookParagraph, imageBlock(audio))}); !refused(err, "promptContent", false) {
		t.Fatalf("a foreign prompt image before a wrong-kind explanation block answered %v", err)
	}
	if _, err := write("", domain.Input{PromptContent: prose(lookParagraph, imageBlock(audio))}); !refused(err, "promptContent", true) {
		t.Fatalf("an image block naming a recording answered %v", err)
	}

	marker := prose(`{"type":"audio"}`, lookParagraph)
	var invalid *domain.ValidationError
	if _, err := write("", domain.Input{PromptContent: marker}); !errors.As(err, &invalid) || invalid.Fields[0].Field != "promptContent" {
		t.Fatalf("a marker without the question's audio answered %v", err)
	}
	policy := &domain.AudioPolicy{AllowSeek: true, ShowTranscriptAfterSubmit: false}
	placed, err := write("", domain.Input{PromptContent: marker, MediaAssetID: &audio, Audio: policy})
	if err != nil {
		t.Fatalf("a marker with the question's audio, in one write: %v", err)
	}
	if got := mediaReferences(t, tx, placed.ID); len(got) != 0 {
		t.Fatalf("the marker names no asset, yet left %v", got)
	}
	if _, err := write(placed.ID, domain.Input{}); !errors.As(err, &invalid) || invalid.Fields[0].Field != "promptContent" {
		t.Fatalf("dropping the audio under a kept marker answered %v", err)
	}

	kept, err := write(q.ID, domain.Input{Explanation: &explained})
	if err != nil {
		t.Fatalf("an update omitting its prose: %v", err)
	}
	if !strings.Contains(string(kept.PromptContent), image) || !strings.Contains(string(kept.ExplanationContent), audio) {
		t.Fatalf("the omitted prose lost its media: %s / %s", kept.PromptContent, kept.ExplanationContent)
	}
	if got := mediaReferences(t, tx, q.ID); len(got) != 2 {
		t.Fatalf("the omitted prose kept references %v", got)
	}

	bare := repositories.NewPostgres(db.NewContext(tx))
	if _, err := bare.Create(ctx, domain.WriteInput{ActorID: owner, Now: time.Now(), Input: domain.Input{Type: domain.ShortAnswer, Prompt: "Look.", Points: "1", Tags: []string{},
		PromptContent: prose(lookParagraph, imageBlock(image))}}); err == nil || errors.Is(err, domain.ErrMediaNotFound) {
		t.Fatalf("a store without media locks answered %v", err)
	}
}

// TestTheBankClearsReferencesInTheContentsTransaction shows the bank write
// runs the reference write: an update with plain prose clears rows the
// question held.
func TestTheBankClearsReferencesInTheContentsTransaction(t *testing.T) {
	ctx, _, tx, owner, _ := metadataBank(t)
	repo := repositories.NewPostgres(db.NewContext(tx)).WithMedia(adapters.QuestionMedia{})
	image := altAsset(t, tx, owner, "image")
	write := func(id string, prompt json.RawMessage) (domain.Question, error) {
		in := domain.WriteInput{ID: id, ActorID: owner, Now: time.Now(),
			Input: domain.Input{Type: domain.ShortAnswer, Prompt: "Look.", PromptContent: prompt, Points: "1", Tags: []string{}}}
		if id == "" {
			return repo.Create(ctx, in)
		}
		return repo.Update(ctx, in)
	}
	q, err := write("", prose(lookParagraph, imageBlock(image)))
	if err != nil {
		t.Fatal(err)
	}
	if got := mediaReferences(t, tx, q.ID); len(got) != 1 {
		t.Fatalf("references %v", got)
	}
	if _, err := write(q.ID, prose(lookParagraph)); err != nil {
		t.Fatal(err)
	}
	if got := mediaReferences(t, tx, q.ID); len(got) != 0 {
		t.Fatalf("an update whose prose names no media kept %v", got)
	}
	if n := countRows(t, tx, `SELECT count(*) FROM app.question_media_references WHERE media_asset_id = $1`, image); n != 0 {
		t.Fatalf("%d references left", n)
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

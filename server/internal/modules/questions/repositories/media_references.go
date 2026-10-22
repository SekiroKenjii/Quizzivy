package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"quizzivy/internal/shared/content"

	"github.com/jackc/pgx/v5"
)

// MediaReferenceFields names the prose fields whose media are rows of app.question_media_references (00154).
const (
	PromptField      = "prompt"
	ExplanationField = "explanation"
)

// WriteMediaReferences makes a question's media reference rows exactly the
// assets its prompt and explanation name (DG-109, D-064 ruling 4), inside the
// transaction that writes the content, so "Used in", the delete check and the
// sweep never disagree with the stored JSON. The prompt's audio marker names no
// asset and has no row. Content that fails the media rules is an error, never
// read as naming nothing. It neither authorizes nor locks the assets: the
// write's validation does that before it runs (R6-13).
func WriteMediaReferences(ctx context.Context, tx pgx.Tx, questionID string, prompt, explanation json.RawMessage) error {
	var fields, ids, kinds []string
	for _, prose := range []struct {
		field string
		raw   json.RawMessage
		parse func([]byte) (content.Document, error)
	}{
		{PromptField, prompt, content.ParsePromptWithMedia},
		{ExplanationField, explanation, content.ParseExplanationWithMedia},
	} {
		raw := nullableContent(prose.raw)
		if len(raw) == 0 {
			continue
		}
		document, err := prose.parse(raw)
		if err != nil {
			return fmt.Errorf("questions: media references: %s: %w", prose.field, err)
		}
		for _, asset := range document.Assets() {
			fields = append(fields, prose.field)
			ids = append(ids, asset.ID)
			kinds = append(kinds, asset.Kind)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app.question_media_references WHERE question_id = $1`, questionID); err != nil {
		return fmt.Errorf("questions: clear media references: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO app.question_media_references (question_id, field, media_asset_id, media_asset_kind)
		SELECT $1, r.field, r.id, r.kind::app.media_kind
		  FROM unnest($2::text[], $3::uuid[], $4::text[]) AS r(field, id, kind)`,
		questionID, fields, ids, kinds); err != nil {
		return fmt.Errorf("questions: write media references: %w", err)
	}
	return nil
}

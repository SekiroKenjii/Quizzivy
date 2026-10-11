package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/content"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/validation"

	"github.com/jackc/pgx/v5"
)

func prepareQuestionContent(ctx context.Context, tx pgx.Tx, write *domain.WriteInput, update bool, groupID *string) error {
	in := &write.Input
	if !update {
		return in.ValidateContent()
	}
	var prompt string
	var explanation *string
	var promptContent, explanationContent json.RawMessage
	err := tx.QueryRow(ctx, `SELECT prompt, explanation, prompt_content, explanation_content
        FROM app.questions WHERE id = $1 AND deleted_at IS NULL
          AND context_group_id IS NOT DISTINCT FROM $2::uuid AND ($3::boolean OR owner_id = $4::uuid) FOR UPDATE`,
		write.ID, groupID, write.All || groupID != nil, opt.String(write.ActorID)).
		Scan(&prompt, &explanation, &promptContent, &explanationContent)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := preserveProse("promptContent", &in.PromptContent, promptContent, &in.Prompt, &prompt); err != nil {
		return err
	}
	if err := preserveProse("explanationContent", &in.ExplanationContent, explanationContent, in.Explanation, explanation); err != nil {
		return err
	}
	return in.ValidateContent()
}

func preserveProse(field string, next *json.RawMessage, old json.RawMessage, text, previous *string) error {
	if *next != nil || old == nil {
		return nil
	}
	if text == nil || previous == nil || content.NFC(*text) != content.NFC(*previous) {
		return &validation.Error{Fields: []validation.Field{{
			Field: field, Message: "Câu hỏi đã có định dạng. Hãy tải lại trước khi sửa nội dung.",
		}}}
	}
	*next, *text = composedProse(old, *text, *previous)
	return nil
}

func composedProse(old json.RawMessage, text, previous string) (json.RawMessage, string) {
	document, err := content.Parse(old)
	if err != nil {
		return old, previous
	}
	composed, err := content.Normalize(document)
	if err != nil || content.NFC(composed.PlainText()) != content.NFC(text) {
		return old, previous
	}
	encoded, err := composed.MarshalJSON()
	if err != nil {
		return old, previous
	}
	return encoded, composed.PlainText()
}

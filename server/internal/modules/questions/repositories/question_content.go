package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/validation"

	"github.com/jackc/pgx/v5"
)

func prepareQuestionContent(ctx context.Context, tx pgx.Tx, id string, in *domain.Input, update bool, groupID *string) error {
	if !update {
		return in.ValidateContent()
	}
	var prompt string
	var explanation *string
	var promptContent, explanationContent json.RawMessage
	err := tx.QueryRow(ctx, `SELECT prompt, explanation, prompt_content, explanation_content
        FROM app.questions WHERE id = $1 AND deleted_at IS NULL
          AND context_group_id IS NOT DISTINCT FROM $2::uuid FOR UPDATE`, id, groupID).
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
	if text == nil || previous == nil || *text != *previous {
		return &validation.Error{Fields: []validation.Field{{
			Field: field, Message: "Câu hỏi đã có định dạng. Hãy tải lại trước khi sửa nội dung.",
		}}}
	}
	*next = old
	return nil
}

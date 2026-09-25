package repositories

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/validation"

	"github.com/jackc/pgx/v5"
)

func nullableContent(raw json.RawMessage) json.RawMessage {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	return raw
}

func preserveOptionContent(ctx context.Context, tx pgx.Tx, questionID string, options []domain.OptionInput) error {
	rows, err := tx.Query(ctx, `SELECT id::text, ordinal, text, content
        FROM app.question_options WHERE question_id = $1 ORDER BY ordinal`, questionID)
	if err != nil {
		return err
	}
	defer rows.Close()
	byID := map[string]domain.Option{}
	byOrdinal := map[int]domain.Option{}
	for rows.Next() {
		var old domain.Option
		if err := rows.Scan(&old.ID, &old.Ordinal, &old.Text, &old.Content); err != nil {
			return err
		}
		byID[old.ID], byOrdinal[old.Ordinal] = old, old
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range options {
		next := &options[i]
		if next.Content != nil {
			continue
		}
		old, matched := byID[opt.Deref(next.ID)]
		if !matched {
			old = byOrdinal[i]
		}
		if old.Content == nil {
			continue
		}
		if !matched || old.Text != next.Text {
			return &validation.Error{Fields: []validation.Field{{
				Field:   fmt.Sprintf("options[%d].content", i),
				Message: "Phương án đã có định dạng. Hãy tải lại câu hỏi trước khi sửa nội dung.",
			}}}
		}
		next.Content = old.Content
	}
	return nil
}

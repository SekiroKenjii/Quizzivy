package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/shared/opt"
	"strings"

	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"
)

const (
	optionsTable = "app.question_options"
	blanksTable  = "app.question_blanks"
)

// heldChildren returns the ids of the rows a question holds in an options or
// blanks table.
func heldChildren(ctx context.Context, tx pgx.Tx, table, questionID string) (map[string]bool, error) {
	rows, err := tx.Query(ctx, `SELECT id::text FROM `+table+` WHERE question_id = $1`, questionID)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	held := make(map[string]bool, len(ids))
	for _, id := range ids {
		held[id] = true
	}
	return held, nil
}

// keptIDs gives, for each requested row in order, the id of the held row it
// updates, or "" when it is created: no id, an id the question does not hold
// (another question's, or one already deleted), or an id an earlier entry of
// the same request already took.
func keptIDs[T any](items []T, id func(T) *string, held map[string]bool) []string {
	kept := make([]string, len(items))
	taken := make(map[string]bool, len(items))
	for i, item := range items {
		want := strings.ToLower(opt.Deref(id(item)))
		if want == "" || !held[want] || taken[want] {
			continue
		}
		taken[want] = true
		kept[i] = want
	}
	return kept
}

func deleteOtherChildren(ctx context.Context, tx pgx.Tx, table, questionID string, kept []string) error {
	keep := make([]string, 0, len(kept))
	for _, id := range kept {
		if id != "" {
			keep = append(keep, id)
		}
	}
	_, err := tx.Exec(ctx,
		`DELETE FROM `+table+` WHERE question_id = $1 AND id <> ALL($2::uuid[])`, questionID, keep)
	return err
}

// replaceOptions makes the question's options the ones in the input. An option
// sent with an id the question holds is updated in place, so its id survives
// the save; one sent without an id, or with an id the question does not hold,
// is created; one not sent is deleted. Ordinals are the array positions,
// written under the deferred ordinal constraint so a reorder can move rows one
// statement at a time.
func replaceOptions(ctx context.Context, tx pgx.Tx, questionID string, in domain.Input) error {
	if err := preserveOptionContent(ctx, tx, questionID, in.Options); err != nil {
		return err
	}
	options := in.Options
	if !in.Type.IsChoice() {
		options = nil
	}
	held, err := heldChildren(ctx, tx, optionsTable, questionID)
	if err != nil {
		return fmt.Errorf("questions: read options: %w", err)
	}
	kept := keptIDs(options, func(o domain.OptionInput) *string { return o.ID }, held)
	if err := deleteOtherChildren(ctx, tx, optionsTable, questionID, kept); err != nil {
		return fmt.Errorf("questions: clear options: %w", err)
	}
	if len(options) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS app.question_options_ordinal_key DEFERRED`); err != nil {
		return fmt.Errorf("questions: defer option order: %w", err)
	}
	updates := &pgx.Batch{}
	var created [][]any
	for i, o := range options {
		if kept[i] == "" {
			created = append(created, []any{questionID, i, o.Text, o.IsCorrect, nullableContent(o.Content)})
			continue
		}
		updates.Queue(`UPDATE app.question_options
			   SET ordinal = $2, text = $3, is_correct = $4, content = $5
			 WHERE id = $1 AND question_id = $6`,
			kept[i], i, o.Text, o.IsCorrect, nullableContent(o.Content), questionID)
	}
	if err := tx.SendBatch(ctx, updates).Close(); err != nil {
		return fmt.Errorf("questions: update options: %w", err)
	}
	if len(created) > 0 {
		_, err := tx.CopyFrom(ctx,
			pgx.Identifier{"app", "question_options"},
			[]string{"question_id", "ordinal", "text", "is_correct", "content"},
			pgx.CopyFromRows(created))
		if err != nil {
			return fmt.Errorf("questions: write options: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS app.question_options_ordinal_key IMMEDIATE`); err != nil {
		return fmt.Errorf("questions: order options: %w", err)
	}
	return nil
}

// replaceBlanks makes the question's blanks the ones in the input, by the same
// rule as replaceOptions: a blank sent with an id the question holds keeps it,
// with its accepted answers replaced; any other is created; one not sent is
// deleted.
func replaceBlanks(ctx context.Context, tx pgx.Tx, questionID string, in domain.Input) error {
	blanks := in.Blanks
	if in.Type != domain.FillBlank {
		blanks = nil
	}
	held, err := heldChildren(ctx, tx, blanksTable, questionID)
	if err != nil {
		return fmt.Errorf("questions: read blanks: %w", err)
	}
	kept := keptIDs(blanks, func(b domain.BlankInput) *string { return b.ID }, held)
	if err := deleteOtherChildren(ctx, tx, blanksTable, questionID, kept); err != nil {
		return fmt.Errorf("questions: clear blanks: %w", err)
	}
	if len(blanks) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS app.question_blanks_ordinal_key DEFERRED`); err != nil {
		return fmt.Errorf("questions: defer blank order: %w", err)
	}
	if err := releaseMovedGaps(ctx, tx, blanks, kept); err != nil {
		return err
	}
	for i, b := range blanks {
		id, err := writeBlank(ctx, tx, questionID, kept[i], b)
		if err != nil {
			return err
		}
		if err := writeBlankAnswers(ctx, tx, id, b.AcceptedAnswers); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `SET CONSTRAINTS app.question_blanks_ordinal_key IMMEDIATE`); err != nil {
		return fmt.Errorf("questions: order blanks: %w", err)
	}
	return nil
}

func releaseMovedGaps(ctx context.Context, tx pgx.Tx, blanks []domain.BlankInput, kept []string) error {
	var ids []string
	var gaps []*string
	for i, b := range blanks {
		if kept[i] != "" {
			ids = append(ids, kept[i])
			gaps = append(gaps, b.GapID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE app.question_blanks b SET gap_id = NULL
		  FROM unnest($1::uuid[], $2::text[]) AS want(id, gap_id)
		 WHERE b.id = want.id AND b.gap_id IS NOT NULL AND b.gap_id IS DISTINCT FROM want.gap_id`,
		ids, gaps)
	if err != nil {
		return fmt.Errorf("questions: release blank gaps: %w", err)
	}
	return nil
}

func writeBlank(ctx context.Context, tx pgx.Tx, questionID, kept string, b domain.BlankInput) (string, error) {
	if kept == "" {
		var id string
		if err := tx.QueryRow(ctx,
			`INSERT INTO app.question_blanks (question_id, ordinal, case_sensitive, gap_id)
			 VALUES ($1, $2, $3, $4) RETURNING id::text`,
			questionID, b.Ordinal, b.CaseSensitive, b.GapID).Scan(&id); err != nil {
			return "", fmt.Errorf("questions: write blank: %w", err)
		}
		return id, nil
	}
	if _, err := tx.Exec(ctx,
		`UPDATE app.question_blanks SET ordinal = $2, case_sensitive = $3, gap_id = $4
		  WHERE id = $1 AND question_id = $5`,
		kept, b.Ordinal, b.CaseSensitive, b.GapID, questionID); err != nil {
		return "", fmt.Errorf("questions: update blank: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM app.question_blank_answers WHERE blank_id = $1`, kept); err != nil {
		return "", fmt.Errorf("questions: clear blank answers: %w", err)
	}
	return kept, nil
}

func writeBlankAnswers(ctx context.Context, tx pgx.Tx, blankID string, answers []string) error {
	seen := map[string]bool{}
	for _, answer := range answers {
		answer = norm.NFC.String(answer)
		if seen[answer] {
			continue
		}
		seen[answer] = true
		if _, err := tx.Exec(ctx,
			`INSERT INTO app.question_blank_answers (blank_id, answer) VALUES ($1, $2)`,
			blankID, answer); err != nil {
			return fmt.Errorf("questions: write blank answer: %w", err)
		}
	}
	return nil
}

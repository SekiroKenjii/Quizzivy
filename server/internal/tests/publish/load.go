package publish

import (
	"context"
	"fmt"
	"quizzivy/internal/db"

	"github.com/jackc/pgx/v5"
)

// loadDraft resolves the outline against the bank: every section in order, with
// each question's full content as the bank holds it right now.
//
// Read inside the publish transaction, after the test row is locked, so what is
// validated is exactly what is frozen.
func loadDraft(ctx context.Context, tx pgx.Tx, testID string) (Draft, error) {
	d := Draft{TestID: testID}

	sections, order, err := loadSections(ctx, tx, testID)
	if err != nil {
		return Draft{}, err
	}
	if len(order) == 0 {
		return Draft{}, ErrNoContent
	}

	byQuestion, err := loadQuestions(ctx, tx, testID)
	if err != nil {
		return Draft{}, err
	}
	for _, id := range order {
		section := sections[id]
		section.Questions = byQuestion[id]
		d.Sections = append(d.Sections, section)
	}
	return d, nil
}

func loadSections(ctx context.Context, tx pgx.Tx, testID string) (map[string]Section, []string, error) {
	rows, err := tx.Query(ctx,
		`SELECT id::text, ordinal, title, instructions
		   FROM app.test_sections WHERE test_id = $1 ORDER BY ordinal`, testID)
	if err != nil {
		return nil, nil, fmt.Errorf("publish: load sections: %w", err)
	}
	defer rows.Close()

	sections := map[string]Section{}
	var order []string
	for rows.Next() {
		var s Section
		if err := rows.Scan(&s.ID, &s.Ordinal, &s.Title, &s.Instructions); err != nil {
			return nil, nil, fmt.Errorf("publish: scan section: %w", err)
		}
		sections[s.ID] = s
		order = append(order, s.ID)
	}
	return sections, order, rows.Err()
}

// loadQuestions reads every question in the outline with its bank content, keyed
// by section id and already in outline order.
func loadQuestions(ctx context.Context, tx pgx.Tx, testID string) (map[string][]Question, error) {
	rows, err := tx.Query(ctx, `
		SELECT sq.test_section_id::text, sq.ordinal, q.id::text, q.type::text, q.prompt,
		       q.media_asset_id::text, q.media_asset_kind::text,
		       q.audio_max_plays, q.audio_allow_seek, q.audio_show_transcript_after,
		       q.transcript, q.points::text, q.explanation, q.sample_answer
		  FROM app.test_section_questions sq
		  JOIN app.test_sections s ON s.id = sq.test_section_id
		  JOIN app.questions q ON q.id = sq.question_id
		 WHERE s.test_id = $1
		 ORDER BY s.ordinal, sq.ordinal`, testID)
	if err != nil {
		return nil, fmt.Errorf("publish: load questions: %w", err)
	}
	defer rows.Close()

	byQuestion := map[string][]Question{}
	var ids []string
	for rows.Next() {
		var sectionID string
		var q Question
		if err := rows.Scan(&sectionID, &q.Ordinal, &q.SourceID, &q.Type, &q.Prompt,
			&q.MediaAssetID, &q.MediaAssetKind, &q.MaxPlays, &q.AllowSeek, &q.ShowTranscript,
			&q.Transcript, &q.Points, &q.Explanation, &q.SampleAnswer); err != nil {
			return nil, fmt.Errorf("publish: scan question: %w", err)
		}
		byQuestion[sectionID] = append(byQuestion[sectionID], q)
		ids = append(ids, q.SourceID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("publish: load questions: %w", err)
	}

	options, err := loadOptions(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	blanks, err := loadBlanks(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	for sectionID, list := range byQuestion {
		for i := range list {
			list[i].Options = options[list[i].SourceID]
			list[i].Blanks = blanks[list[i].SourceID]
		}
		byQuestion[sectionID] = list
	}
	return byQuestion, nil
}

func loadOptions(ctx context.Context, tx pgx.Tx, questionIDs []string) (map[string][]Option, error) {
	if len(questionIDs) == 0 {
		return map[string][]Option{}, nil
	}
	byQuestion, err := db.GroupBy(ctx, tx,
		`SELECT question_id::text, ordinal, text, is_correct
		   FROM app.question_options
		  WHERE question_id = ANY($1::uuid[])
		  ORDER BY question_id, ordinal`, []any{questionIDs},
		func(rows pgx.Rows) (string, Option, error) {
			var questionID string
			var o Option
			err := rows.Scan(&questionID, &o.Ordinal, &o.Text, &o.IsCorrect)
			return questionID, o, err
		})
	if err != nil {
		return nil, fmt.Errorf("publish: load options: %w", err)
	}
	return byQuestion, nil
}

func loadBlanks(ctx context.Context, tx pgx.Tx, questionIDs []string) (map[string][]Blank, error) {
	if len(questionIDs) == 0 {
		return map[string][]Blank{}, nil
	}
	byQuestion, err := db.GroupBy(ctx, tx,
		`SELECT b.question_id::text, b.ordinal, b.case_sensitive,
		        coalesce(array_agg(a.answer ORDER BY a.answer)
		                 FILTER (WHERE a.answer IS NOT NULL), '{}')
		   FROM app.question_blanks b
		   LEFT JOIN app.question_blank_answers a ON a.blank_id = b.id
		  WHERE b.question_id = ANY($1::uuid[])
		  GROUP BY b.question_id, b.id, b.ordinal, b.case_sensitive
		  ORDER BY b.question_id, b.ordinal`, []any{questionIDs},
		func(rows pgx.Rows) (string, Blank, error) {
			var questionID string
			var b Blank
			err := rows.Scan(&questionID, &b.Ordinal, &b.CaseSensitive, &b.AcceptedAnswers)
			return questionID, b, err
		})
	if err != nil {
		return nil, fmt.Errorf("publish: load blanks: %w", err)
	}
	return byQuestion, nil
}

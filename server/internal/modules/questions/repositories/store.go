package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"

	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"
)

type Postgres struct{ db.Repository }

func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

const questionColumns = `
	       q.id::text, q.type::text, q.prompt, q.prompt_content, q.explanation_content,
	       q.media_asset_id::text, q.media_asset_kind::text,
	       q.audio_max_plays, q.audio_allow_seek, q.audio_show_transcript_after,
	       q.transcript, q.points::text, q.explanation, q.sample_answer,
	       q.tags,
	       (SELECT count(DISTINCT s.test_id)
	          FROM app.test_section_questions sq
	          JOIN app.test_sections s ON s.id = sq.test_section_id
	          JOIN app.tests t ON t.id = s.test_id AND t.deleted_at IS NULL
	         WHERE sq.question_id = q.id),
	       q.created_at, q.updated_at`

func scanQuestion(row pgx.Row) (domain.Question, error) {
	var q domain.Question
	var typ string
	var maxPlays *int
	var allowSeek, showTranscript *bool

	err := row.Scan(&q.ID, &typ, &q.Prompt, &q.PromptContent, &q.ExplanationContent, &q.MediaAssetID, &q.MediaAssetKind,
		&maxPlays, &allowSeek, &showTranscript, &q.Transcript, &q.Points,
		&q.Explanation, &q.SampleAnswer, &q.Tags, &q.UsedInTests, &q.CreatedAt, &q.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Question{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Question{}, fmt.Errorf("questions: scan: %w", err)
	}
	q.Type = domain.Type(typ)

	if allowSeek != nil && showTranscript != nil {
		q.Audio = &domain.AudioPolicy{
			MaxPlays:                  maxPlays,
			AllowSeek:                 *allowSeek,
			ShowTranscriptAfterSubmit: *showTranscript,
		}
	}
	return q, nil
}

// Get returns one live question with its children.
func (s *Postgres) Get(ctx context.Context, id string) (domain.Question, error) {
	question, err := s.get(ctx, s.Conn(), id, false)
	if err != nil {
		return domain.Question{}, err
	}
	question.UsedIn, err = s.questionUses(ctx, id)
	return question, err
}

// GetIncludingDeleted resolves a question by id whether or not it is deleted,
// so a soft delete cannot break a published version snapshot.
func (s *Postgres) GetIncludingDeleted(ctx context.Context, id string) (domain.Question, error) {
	return s.get(ctx, s.Conn(), id, true)
}

func (s *Postgres) get(ctx context.Context, q db.Querier, id string, includeDeleted bool) (domain.Question, error) {
	filter := ` AND q.deleted_at IS NULL`
	if includeDeleted {
		filter = ``
	}
	question, err := scanQuestion(q.QueryRow(ctx,
		`SELECT`+questionColumns+` FROM app.questions q WHERE q.id = $1`+filter, id))
	if err != nil {
		return domain.Question{}, err
	}
	if question.Options, err = s.loadOptions(ctx, q, id); err != nil {
		return domain.Question{}, err
	}
	if question.Blanks, err = s.loadBlanks(ctx, q, id); err != nil {
		return domain.Question{}, err
	}
	return question, nil
}

func (s *Postgres) loadOptions(ctx context.Context, q db.Querier, questionID string) ([]domain.Option, error) {
	byQuestion, err := s.loadOptionsFor(ctx, q, []string{questionID})
	if err != nil {
		return nil, err
	}
	return byQuestion[questionID], nil
}

func (s *Postgres) loadOptionsFor(ctx context.Context, q db.Querier, questionIDs []string) (map[string][]domain.Option, error) {
	if len(questionIDs) == 0 {
		return map[string][]domain.Option{}, nil
	}
	byQuestion, err := db.GroupBy(ctx, q,
		`SELECT question_id::text, id::text, ordinal, text, is_correct, content
		   FROM app.question_options
		  WHERE question_id = ANY($1::uuid[])
		  ORDER BY question_id, ordinal`, []any{questionIDs},
		func(rows pgx.Rows) (string, domain.Option, error) {
			var questionID string
			var o domain.Option
			err := rows.Scan(&questionID, &o.ID, &o.Ordinal, &o.Text, &o.IsCorrect, &o.Content)
			return questionID, o, err
		})
	if err != nil {
		return nil, fmt.Errorf("questions: load options: %w", err)
	}
	return byQuestion, nil
}

func (s *Postgres) loadBlanks(ctx context.Context, q db.Querier, questionID string) ([]domain.Blank, error) {
	byQuestion, err := s.loadBlanksFor(ctx, q, []string{questionID})
	if err != nil {
		return nil, err
	}
	return byQuestion[questionID], nil
}

func (s *Postgres) loadBlanksFor(ctx context.Context, q db.Querier, questionIDs []string) (map[string][]domain.Blank, error) {
	if len(questionIDs) == 0 {
		return map[string][]domain.Blank{}, nil
	}
	byQuestion, err := db.GroupBy(ctx, q,
		`SELECT b.question_id::text, b.id::text, b.ordinal, b.gap_id, b.case_sensitive,
		        coalesce(array_agg(a.answer ORDER BY a.answer)
		                 FILTER (WHERE a.answer IS NOT NULL), '{}')
		   FROM app.question_blanks b
		   LEFT JOIN app.question_blank_answers a ON a.blank_id = b.id
		  WHERE b.question_id = ANY($1::uuid[])
		  GROUP BY b.question_id, b.id, b.ordinal, b.gap_id, b.case_sensitive
		  ORDER BY b.question_id, b.ordinal`, []any{questionIDs},
		func(rows pgx.Rows) (string, domain.Blank, error) {
			var questionID string
			var b domain.Blank
			err := rows.Scan(&questionID, &b.ID, &b.Ordinal, &b.GapID, &b.CaseSensitive, &b.AcceptedAnswers)
			return questionID, b, err
		})
	if err != nil {
		return nil, fmt.Errorf("questions: load blanks: %w", err)
	}
	return byQuestion, nil
}

// Create inserts a question and its children in one transaction.
func (s *Postgres) Create(ctx context.Context, in domain.WriteInput) (domain.Question, error) {
	return s.write(ctx, in, false)
}

// Update replaces a question and its children. Edits the bank copy only;
// published versions hold their own snapshot.
func (s *Postgres) Update(ctx context.Context, in domain.WriteInput) (domain.Question, error) {
	return s.write(ctx, in, true)
}

func (s *Postgres) write(ctx context.Context, in domain.WriteInput, update bool) (domain.Question, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Question{}, fmt.Errorf("questions: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := prepareQuestionContent(ctx, tx, in.ID, &in.Input, update); err != nil {
		return domain.Question{}, err
	}

	var maxPlays *int
	var allowSeek, showTranscript *bool
	if in.Input.Audio != nil {
		maxPlays = in.Input.Audio.MaxPlays
		allowSeek = &in.Input.Audio.AllowSeek
		showTranscript = &in.Input.Audio.ShowTranscriptAfterSubmit
	}

	var kind *string
	if in.Input.MediaAssetID != nil {
		kind = in.MediaAssetKind
	}

	id := in.ID
	if update {
		err = tx.QueryRow(ctx, `
			UPDATE app.questions
			   SET type = $2::app.question_type, prompt = $3,
			       media_asset_id = $4, media_asset_kind = $5::app.media_kind,
			       audio_max_plays = $6, audio_allow_seek = $7,
			       audio_show_transcript_after = $8, transcript = $9,
			       points = $10::numeric, explanation = $11, sample_answer = $12,
			       tags = $13, prompt_content = $14, explanation_content = $15
			 WHERE id = $1 AND deleted_at IS NULL
			 RETURNING id::text`,
			id, string(in.Input.Type), in.Input.Prompt, in.Input.MediaAssetID, kind,
			maxPlays, allowSeek, showTranscript, in.Input.Transcript,
			in.Input.Points, in.Input.Explanation, in.Input.SampleAnswer,
			in.Input.Tags, nullableContent(in.Input.PromptContent), nullableContent(in.Input.ExplanationContent)).Scan(&id)
	} else {
		err = tx.QueryRow(ctx, `
			INSERT INTO app.questions
			       (type, prompt, media_asset_id, media_asset_kind,
			        audio_max_plays, audio_allow_seek, audio_show_transcript_after,
			        transcript, points, explanation, sample_answer, tags, created_by, prompt_content, explanation_content)
			VALUES ($1::app.question_type, $2, $3, $4::app.media_kind, $5, $6, $7,
			        $8, $9::numeric, $10, $11, $12, $13, $14, $15)
			RETURNING id::text`,
			string(in.Input.Type), in.Input.Prompt, in.Input.MediaAssetID, kind,
			maxPlays, allowSeek, showTranscript, in.Input.Transcript,
			in.Input.Points, in.Input.Explanation, in.Input.SampleAnswer,
			in.Input.Tags, in.ActorID, nullableContent(in.Input.PromptContent), nullableContent(in.Input.ExplanationContent)).Scan(&id)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Question{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Question{}, fmt.Errorf("questions: write: %w", err)
	}

	if err := replaceOptions(ctx, tx, id, in.Input); err != nil {
		return domain.Question{}, err
	}
	if err := replaceBlanks(ctx, tx, id, in.Input); err != nil {
		return domain.Question{}, err
	}

	action := "question.created"
	if update {
		action = "question.updated"
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorID,
		Action:      action,
		Entity:      "question",
		EntityID:    &id,
		OccurredAt:  in.Now,
		IP:          opt.String(in.IP),
		UserAgent:   opt.String(in.UserAgent),
	}); err != nil {
		return domain.Question{}, err
	}

	written, err := s.get(ctx, tx, id, false)
	if err != nil {
		return domain.Question{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Question{}, fmt.Errorf("questions: commit: %w", err)
	}
	return written, nil
}

func replaceOptions(ctx context.Context, tx pgx.Tx, questionID string, in domain.Input) error {
	if err := preserveOptionContent(ctx, tx, questionID, in.Options); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM app.question_options WHERE question_id = $1`, questionID); err != nil {
		return fmt.Errorf("questions: clear options: %w", err)
	}
	if !in.Type.IsChoice() || len(in.Options) == 0 {
		return nil
	}
	rows := make([][]any, len(in.Options))
	for i, o := range in.Options {
		rows[i] = []any{questionID, i, o.Text, o.IsCorrect, nullableContent(o.Content)}
	}
	_, err := tx.CopyFrom(ctx,
		pgx.Identifier{"app", "question_options"},
		[]string{"question_id", "ordinal", "text", "is_correct", "content"},
		pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("questions: write options: %w", err)
	}
	return nil
}

func replaceBlanks(ctx context.Context, tx pgx.Tx, questionID string, in domain.Input) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM app.question_blanks WHERE question_id = $1`, questionID); err != nil {
		return fmt.Errorf("questions: clear blanks: %w", err)
	}
	if in.Type != domain.FillBlank || len(in.Blanks) == 0 {
		return nil
	}

	for _, b := range in.Blanks {
		var blankID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO app.question_blanks (question_id, ordinal, case_sensitive, gap_id)
			 VALUES ($1, $2, $3, $4) RETURNING id::text`,
			questionID, b.Ordinal, b.CaseSensitive, b.GapID).Scan(&blankID); err != nil {
			return fmt.Errorf("questions: write blank: %w", err)
		}
		seen := map[string]bool{}
		for _, answer := range b.AcceptedAnswers {
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
	}
	return nil
}

// SoftDelete marks a question deleted and audits it.
func (s *Postgres) SoftDelete(ctx context.Context, in domain.WriteInput) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return fmt.Errorf("questions: begin delete: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var alreadyDeleted bool
	err = tx.QueryRow(ctx,
		`SELECT deleted_at IS NOT NULL FROM app.questions WHERE id = $1 FOR UPDATE`,
		in.ID).Scan(&alreadyDeleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("questions: lock: %w", err)
	}
	if alreadyDeleted {
		return domain.ErrNotFound
	}

	refs, err := DraftReferences(ctx, tx, in.ID)
	if err != nil {
		return err
	}
	if len(refs) > 0 {
		return &domain.ReferencedError{Tests: refs}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE app.questions SET deleted_at = $2 WHERE id = $1`, in.ID, in.Now); err != nil {
		return fmt.Errorf("questions: soft delete: %w", err)
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorID,
		Action:      "question.deleted",
		Entity:      "question",
		EntityID:    &in.ID,
		OccurredAt:  in.Now,
		IP:          opt.String(in.IP),
		UserAgent:   opt.String(in.UserAgent),
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("questions: commit delete: %w", err)
	}
	return nil
}

// AddTags attaches tags to several bank questions at once (A-06's "Gắn thẻ").
func (s *Postgres) AddTags(ctx context.Context, ids []string, tags []string) (int, error) {
	rows, err := s.Query(ctx, `
		UPDATE app.questions q
		   SET tags = (
		         SELECT array_agg(DISTINCT t ORDER BY t)
		           FROM unnest(q.tags || $2::text[]) AS t
		       ),
		       updated_at = now()
		 WHERE q.id = ANY($1::uuid[])
		   AND q.deleted_at IS NULL
		   AND NOT (q.tags @> $2::text[])
		RETURNING q.id`, ids, tags)
	if err != nil {
		return 0, fmt.Errorf("questions: add tags: %w", err)
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		n++
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("questions: add tags: %w", err)
	}
	return n, nil
}

package questions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/audit"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// questionColumns is spelled out rather than `SELECT *` (§13.8). The bank row
// carries sample_answer, which §7 marks admin-only -- an implicit column list
// is how that ends up in a student payload the day someone adds a join.
const questionColumns = `
	       q.id::text, q.type::text, q.prompt,
	       q.media_asset_id::text, q.media_asset_kind::text,
	       q.audio_max_plays, q.audio_allow_seek, q.audio_show_transcript_after,
	       q.transcript, q.points::text, q.explanation, q.sample_answer,
	       q.tags, q.created_at, q.updated_at`

func scanQuestion(row pgx.Row) (Question, error) {
	var q Question
	var typ string
	var maxPlays *int
	var allowSeek, showTranscript *bool

	err := row.Scan(&q.ID, &typ, &q.Prompt, &q.MediaAssetID, &q.MediaAssetKind,
		&maxPlays, &allowSeek, &showTranscript, &q.Transcript, &q.Points,
		&q.Explanation, &q.SampleAnswer, &q.Tags, &q.CreatedAt, &q.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Question{}, ErrNotFound
	}
	if err != nil {
		return Question{}, fmt.Errorf("questions: scan: %w", err)
	}
	q.Type = Type(typ)

	// [D-04] The two booleans are non-null exactly when the asset is audio, so
	// their presence is what says whether there is a policy at all. MaxPlays is
	// nullable within a present policy and means unlimited.
	if allowSeek != nil && showTranscript != nil {
		q.Audio = &AudioPolicy{
			MaxPlays:                  maxPlays,
			AllowSeek:                 *allowSeek,
			ShowTranscriptAfterSubmit: *showTranscript,
		}
	}
	return q, nil
}

// Get returns one live question with its children.
func (s *Store) Get(ctx context.Context, id string) (Question, error) {
	return s.get(ctx, s.pool, id, false)
}

// GetIncludingDeleted resolves a question by id whether or not it is deleted.
//
// Version snapshots reference bank rows that may since have been deleted, and a
// soft delete must not break a published test (§13.2). List results exclude
// them; resolution by id does not.
func (s *Store) GetIncludingDeleted(ctx context.Context, id string) (Question, error) {
	return s.get(ctx, s.pool, id, true)
}

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *Store) get(ctx context.Context, q querier, id string, includeDeleted bool) (Question, error) {
	filter := ` AND q.deleted_at IS NULL`
	if includeDeleted {
		filter = ``
	}
	question, err := scanQuestion(q.QueryRow(ctx,
		`SELECT`+questionColumns+` FROM app.questions q WHERE q.id = $1`+filter, id))
	if err != nil {
		return Question{}, err
	}
	if question.Options, err = s.loadOptions(ctx, q, id); err != nil {
		return Question{}, err
	}
	if question.Blanks, err = s.loadBlanks(ctx, q, id); err != nil {
		return Question{}, err
	}
	return question, nil
}

func (s *Store) loadOptions(ctx context.Context, q querier, questionID string) ([]Option, error) {
	rows, err := q.Query(ctx,
		`SELECT id::text, ordinal, text, is_correct
		   FROM app.question_options WHERE question_id = $1 ORDER BY ordinal`, questionID)
	if err != nil {
		return nil, fmt.Errorf("questions: load options: %w", err)
	}
	defer rows.Close()

	options := []Option{}
	for rows.Next() {
		var o Option
		if err := rows.Scan(&o.ID, &o.Ordinal, &o.Text, &o.IsCorrect); err != nil {
			return nil, fmt.Errorf("questions: scan option: %w", err)
		}
		options = append(options, o)
	}
	return options, rows.Err()
}

func (s *Store) loadBlanks(ctx context.Context, q querier, questionID string) ([]Blank, error) {
	// One query rather than one per blank: the answers are aggregated in SQL,
	// ordered so the array is stable between reads.
	rows, err := q.Query(ctx,
		`SELECT b.id::text, b.ordinal, b.case_sensitive,
		        coalesce(array_agg(a.answer ORDER BY a.answer)
		                 FILTER (WHERE a.answer IS NOT NULL), '{}')
		   FROM app.question_blanks b
		   LEFT JOIN app.question_blank_answers a ON a.blank_id = b.id
		  WHERE b.question_id = $1
		  GROUP BY b.id, b.ordinal, b.case_sensitive
		  ORDER BY b.ordinal`, questionID)
	if err != nil {
		return nil, fmt.Errorf("questions: load blanks: %w", err)
	}
	defer rows.Close()

	blanks := []Blank{}
	for rows.Next() {
		var b Blank
		if err := rows.Scan(&b.ID, &b.Ordinal, &b.CaseSensitive, &b.AcceptedAnswers); err != nil {
			return nil, fmt.Errorf("questions: scan blank: %w", err)
		}
		blanks = append(blanks, b)
	}
	return blanks, rows.Err()
}

// WriteInput is a create or an update, depending on whether ID is set.
type WriteInput struct {
	ID    string // empty to create
	Input Input
	// MediaAssetKind is read from media_assets by the service, never taken from
	// the request. It is half of the composite FK [D-05], and letting a caller
	// state it would let them declare an image to be audio -- which is exactly
	// what the composite FK exists to make impossible.
	MediaAssetKind *string
	ActorID        string
	Now            time.Time
	IP             string
	UserAgent      string
}

// Create inserts a question and its children in ONE transaction.
//
// §8's editor sends the whole question on every save, so children are replaced
// wholesale rather than diffed. Doing that outside a transaction would leave a
// question with the old options and the new prompt if the second statement
// failed -- a question that renders one thing and grades another.
func (s *Store) Create(ctx context.Context, in WriteInput) (Question, error) {
	return s.write(ctx, in, false)
}

// Update replaces a question and its children. Edits the BANK copy only:
// published versions hold their own snapshot and are unaffected (§7).
func (s *Store) Update(ctx context.Context, in WriteInput) (Question, error) {
	return s.write(ctx, in, true)
}

func (s *Store) write(ctx context.Context, in WriteInput, update bool) (Question, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Question{}, fmt.Errorf("questions: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var maxPlays *int
	var allowSeek, showTranscript *bool
	if in.Input.Audio != nil {
		maxPlays = in.Input.Audio.MaxPlays
		allowSeek = &in.Input.Audio.AllowSeek
		showTranscript = &in.Input.Audio.ShowTranscriptAfterSubmit
	}
	// The composite FK needs both halves or neither [D-05]. The kind is not
	// taken from the request: it is read from media_assets by the service, so a
	// caller cannot declare an image to be audio.
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
			       tags = $13
			 WHERE id = $1 AND deleted_at IS NULL
			 RETURNING id::text`,
			id, string(in.Input.Type), in.Input.Prompt, in.Input.MediaAssetID, kind,
			maxPlays, allowSeek, showTranscript, in.Input.Transcript,
			in.Input.Points, in.Input.Explanation, in.Input.SampleAnswer,
			in.Input.Tags).Scan(&id)
	} else {
		err = tx.QueryRow(ctx, `
			INSERT INTO app.questions
			       (type, prompt, media_asset_id, media_asset_kind,
			        audio_max_plays, audio_allow_seek, audio_show_transcript_after,
			        transcript, points, explanation, sample_answer, tags, created_by)
			VALUES ($1::app.question_type, $2, $3, $4::app.media_kind, $5, $6, $7,
			        $8, $9::numeric, $10, $11, $12, $13)
			RETURNING id::text`,
			string(in.Input.Type), in.Input.Prompt, in.Input.MediaAssetID, kind,
			maxPlays, allowSeek, showTranscript, in.Input.Transcript,
			in.Input.Points, in.Input.Explanation, in.Input.SampleAnswer,
			in.Input.Tags, in.ActorID).Scan(&id)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Question{}, ErrNotFound
	}
	if err != nil {
		return Question{}, fmt.Errorf("questions: write: %w", err)
	}

	if err := replaceOptions(ctx, tx, id, in.Input); err != nil {
		return Question{}, err
	}
	if err := replaceBlanks(ctx, tx, id, in.Input); err != nil {
		return Question{}, err
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
		IP:          optional(in.IP),
		UserAgent:   optional(in.UserAgent),
	}); err != nil {
		return Question{}, err
	}

	written, err := s.get(ctx, tx, id, false)
	if err != nil {
		return Question{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Question{}, fmt.Errorf("questions: commit: %w", err)
	}
	return written, nil
}

// replaceOptions writes the options with DENSE ordinals, 0..n-1.
//
// Array position is the ordinal, per the contract. Normalising rather than
// trusting an ordinal from the client means a reorder cannot leave gaps, and a
// gap matters: shuffleOptions and the grading key both address options by
// position.
//
// Delete-then-insert rather than diffing. The editor sends the whole question
// on every save, so there is no reliable identity to diff against, and a diff
// that gets it wrong silently re-points a correct answer.
func replaceOptions(ctx context.Context, tx pgx.Tx, questionID string, in Input) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM app.question_options WHERE question_id = $1`, questionID); err != nil {
		return fmt.Errorf("questions: clear options: %w", err)
	}
	if !in.Type.isChoice() || len(in.Options) == 0 {
		return nil
	}
	rows := make([][]any, len(in.Options))
	for i, o := range in.Options {
		rows[i] = []any{questionID, i, o.Text, o.IsCorrect}
	}
	_, err := tx.CopyFrom(ctx,
		pgx.Identifier{"app", "question_options"},
		[]string{"question_id", "ordinal", "text", "is_correct"},
		pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("questions: write options: %w", err)
	}
	return nil
}

// replaceBlanks writes blanks with dense ordinals 1..n, in the order given.
//
// 1-indexed because the prompt addresses them as `{{1}}`, `{{2}}` -- a
// 0-ordinal blank would be unreachable from the text that references it.
// Validation has already checked that the ordinal set matches the prompt's, so
// renumbering here would break that agreement: the blanks are sorted by their
// stated ordinal and keep it.
func replaceBlanks(ctx context.Context, tx pgx.Tx, questionID string, in Input) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM app.question_blanks WHERE question_id = $1`, questionID); err != nil {
		return fmt.Errorf("questions: clear blanks: %w", err)
	}
	if in.Type != FillBlank || len(in.Blanks) == 0 {
		return nil
	}

	for _, b := range in.Blanks {
		var blankID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO app.question_blanks (question_id, ordinal, case_sensitive)
			 VALUES ($1, $2, $3) RETURNING id::text`,
			questionID, b.Ordinal, b.CaseSensitive).Scan(&blankID); err != nil {
			return fmt.Errorf("questions: write blank: %w", err)
		}
		// Deduplicated: UNIQUE (blank_id, answer) is exact, so two identical
		// accepted answers would abort the whole save over a typo in the form.
		seen := map[string]bool{}
		for _, answer := range b.AcceptedAnswers {
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

// SoftDelete marks a question deleted and audits it (§13.2).
func (s *Store) SoftDelete(ctx context.Context, in WriteInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("questions: begin delete: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Locked before the reference count, so a draft outline cannot start
	// referencing this question between the check and the update.
	var alreadyDeleted bool
	err = tx.QueryRow(ctx,
		`SELECT deleted_at IS NOT NULL FROM app.questions WHERE id = $1 FOR UPDATE`,
		in.ID).Scan(&alreadyDeleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("questions: lock: %w", err)
	}
	if alreadyDeleted {
		return ErrNotFound
	}

	refs, err := CountDraftReferences(ctx, tx, in.ID)
	if err != nil {
		return err
	}
	if refs > 0 {
		return ErrReferenced
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
		IP:          optional(in.IP),
		UserAgent:   optional(in.UserAgent),
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("questions: commit delete: %w", err)
	}
	return nil
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

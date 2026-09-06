package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/opt"
	"time"

	"github.com/jackc/pgx/v5"
)

type Postgres struct{ db.Repository }

func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

const rulesQuery = `
	SELECT a.test_version_id, a.opens_at, a.closes_at, a.closed_at, a.published_at,
	       a.duration_minutes, a.max_attempts, a.shuffle_questions, a.shuffle_options,
	       a.integrity_require_fullscreen, a.integrity_block_copy_paste,
	       a.integrity_max_focus_loss, a.integrity_on_limit_exceeded,
	       a.integrity_min_away_ms,
	       -- Targeted by class or by name is one answer, not two: EXISTS over
	       -- the union rather than two counts, for the same reason the roster
	       -- count is a union (a student reached both ways is one person).
	       EXISTS (
	         SELECT 1 FROM (
	             SELECT m.user_id
	               FROM app.assignment_classes ac
	               JOIN app.class_members m ON m.class_id = ac.class_id
	              WHERE ac.assignment_id = a.id
	             UNION
	             SELECT ast.user_id FROM app.assignment_students ast
	              WHERE ast.assignment_id = a.id
	         ) roster
	         JOIN app.users u ON u.id = roster.user_id AND u.disabled_at IS NULL
	        WHERE roster.user_id = $2::uuid
	       )
	  FROM app.assignments a
	 WHERE a.id = $1::uuid`

func (s *Postgres) Rules(ctx context.Context, assignmentID, studentID string) (domain.Rules, error) {
	var r domain.Rules
	err := s.QueryRow(ctx, rulesQuery, assignmentID, studentID).Scan(
		&r.TestVersionID, &r.OpensAt, &r.ClosesAt, &r.ClosedAt, &r.PublishedAt,
		&r.DurationMinutes, &r.MaxAttempts, &r.ShuffleQuestions, &r.ShuffleOptions,
		&r.Integrity.RequireFullscreen, &r.Integrity.BlockCopyPaste,
		&r.Integrity.MaxFocusLoss, &r.Integrity.OnLimitExceeded, &r.Integrity.MinAwayMs,
		&r.Targeted,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Rules{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Rules{}, fmt.Errorf("attempts: read rules: %w", err)
	}
	return r, nil
}

const attemptColumns = `
	id, assignment_id, student_id, test_version_id, attempt_no, status,
	started_at, deadline_at, submitted_at, graded_at,
	focus_loss_count, flagged, session_id, shuffle_seed`

func scanAttempt(r pgx.Row) (domain.AttemptRecord, error) {
	var out domain.AttemptRecord
	err := r.Scan(
		&out.ID, &out.AssignmentID, &out.StudentID, &out.TestVersionID, &out.AttemptNo,
		&out.Status, &out.StartedAt, &out.DeadlineAt, &out.SubmittedAt, &out.GradedAt,
		&out.FocusLossCount, &out.Flagged, &out.SessionID, &out.Seed,
	)
	return out, err
}

// Live finds the one attempt a student may still be working on. The partial
// unique index guarantees there is at most one.
func (s *Postgres) Live(ctx context.Context, assignmentID, studentID string) (domain.AttemptRecord, error) {
	q := `SELECT ` + attemptColumns + `
	        FROM app.attempts
	       WHERE assignment_id = $1::uuid AND student_id = $2::uuid
	         AND status = 'in_progress'`
	out, err := scanAttempt(s.QueryRow(ctx, q, assignmentID, studentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AttemptRecord{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.AttemptRecord{}, fmt.Errorf("attempts: read live attempt: %w", err)
	}
	return out, nil
}

func (s *Postgres) Tally(ctx context.Context, assignmentID, studentID string) (domain.Tally, error) {
	var t domain.Tally
	err := s.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status <> 'voided'),
		       coalesce(max(attempt_no), 0) + 1
		  FROM app.attempts
		 WHERE assignment_id = $1::uuid AND student_id = $2::uuid`,
		assignmentID, studentID).Scan(&t.Spent, &t.Next)
	if err != nil {
		return domain.Tally{}, fmt.Errorf("attempts: tally attempts: %w", err)
	}
	return t, nil
}

func (s *Postgres) Create(ctx context.Context, in domain.CreateInput) (domain.AttemptRecord, error) {
	q := `
		INSERT INTO app.attempts
		  (assignment_id, test_version_id, student_id, attempt_no, session_id,
		   shuffle_seed, beacon_token_hash, started_at, deadline_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, $6, $7, $8, $9)
		RETURNING ` + attemptColumns
	out, err := scanAttempt(s.QueryRow(ctx, q,
		in.AssignmentID, in.TestVersionID, in.StudentID, in.AttemptNo, in.SessionID,
		in.Seed, in.BeaconHash, in.StartedAt, in.DeadlineAt))
	if db.IsUniqueViolation(err, "") {
		return domain.AttemptRecord{}, domain.ErrRaceLost
	}
	if err != nil {
		return domain.AttemptRecord{}, fmt.Errorf("attempts: create: %w", err)
	}
	return out, nil
}

// Resume hands the attempt to a new tab and records why, in one transaction:
// the session swap and the events explaining it are the same fact, and a
// timeline missing the takeover it caused is worse than no timeline.
func (s *Postgres) Resume(ctx context.Context, in domain.ResumeInput) (domain.AttemptRecord, bool, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.AttemptRecord{}, false, fmt.Errorf("attempts: begin resume: %w", err)
	}
	defer tx.Rollback(ctx)

	var previous string
	err = tx.QueryRow(ctx, `
		SELECT session_id FROM app.attempts
		 WHERE id = $1::uuid AND status = 'in_progress'
		   FOR UPDATE`, in.AttemptID).Scan(&previous)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AttemptRecord{}, false, domain.ErrNotFound
	}
	if err != nil {
		return domain.AttemptRecord{}, false, fmt.Errorf("attempts: lock attempt: %w", err)
	}

	takeover, err := sessionWasLive(ctx, tx, in.AttemptID, previous, in.Now)
	if err != nil {
		return domain.AttemptRecord{}, false, err
	}

	updated, err := scanAttempt(tx.QueryRow(ctx, `
		UPDATE app.attempts
		   SET session_id = $2::uuid, beacon_token_hash = $3
		 WHERE id = $1::uuid
		RETURNING `+attemptColumns, in.AttemptID, in.SessionID, in.BeaconHash))
	if err != nil {
		return domain.AttemptRecord{}, false, fmt.Errorf("attempts: swap session: %w", err)
	}

	if err := appendEvent(ctx, tx, in.AttemptID, in.SessionID, domain.KindResume, in.Now); err != nil {
		return domain.AttemptRecord{}, false, err
	}
	if takeover {
		if err := appendEvent(ctx, tx, in.AttemptID, previous, domain.KindSessionTakeover, in.Now); err != nil {
			return domain.AttemptRecord{}, false, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.AttemptRecord{}, false, fmt.Errorf("attempts: commit resume: %w", err)
	}
	return updated, takeover, nil
}

func sessionWasLive(ctx context.Context, q db.Querier, attemptID, sessionID string, now time.Time) (bool, error) {
	var live bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM app.attempt_events
		   WHERE attempt_id = $1::uuid AND session_id = $2::uuid
		     AND kind NOT IN ('resume', 'session_takeover')
		     AND received_at > $3
		)`, attemptID, sessionID, now.Add(-domain.SessionLiveWindow)).Scan(&live)
	if err != nil {
		return false, fmt.Errorf("attempts: check session liveness: %w", err)
	}
	return live, nil
}

func appendEvent(ctx context.Context, q db.Querier, attemptID, sessionID, kind string, now time.Time) error {
	_, err := q.Exec(ctx, `
		INSERT INTO app.attempt_events (attempt_id, session_id, kind, occurred_at)
		VALUES ($1::uuid, $2::uuid, $3, $4)`,
		attemptID, sessionID, kind, now)
	if err != nil {
		return fmt.Errorf("attempts: append %s event: %w", kind, err)
	}
	return nil
}

const sectionsQuery = `
	SELECT id, title, instructions
	  FROM app.test_version_sections
	 WHERE test_version_id = $1::uuid
	 ORDER BY ordinal`

const questionsQuery = `
	SELECT q.id, q.test_version_section_id, q.type, q.prompt, q.points,
	       q.media_asset_id, q.media_asset_kind, m.mime_type, m.original_filename,
	       m.bytes, m.duration_ms, m.created_at,
	       q.audio_max_plays, q.audio_allow_seek, q.audio_show_transcript_after
	  FROM app.test_version_questions q
	  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
	  LEFT JOIN app.media_assets m ON m.id = q.media_asset_id
	 WHERE s.test_version_id = $1::uuid
	 ORDER BY s.ordinal, q.ordinal`

type questionRow struct {
	mediaID, mediaKind, mimeType, filename *string
	mediaBytes, durationMs, maxPlays       *int
	createdAt                              *time.Time
	allowSeek, showTranscript              *bool
}

func (r questionRow) media() *domain.Media {
	if r.mediaID == nil {
		return nil
	}
	m := &domain.Media{
		ID: *r.mediaID, Kind: opt.Deref(r.mediaKind), MimeType: opt.Deref(r.mimeType),
		Filename: opt.Deref(r.filename), DurationMs: r.durationMs,
	}
	if r.mediaBytes != nil {
		m.Bytes = *r.mediaBytes
	}
	if r.createdAt != nil {
		m.CreatedAt = *r.createdAt
	}
	return m
}

func (r questionRow) audio() *domain.AudioPolicy {
	if r.allowSeek == nil || r.showTranscript == nil {
		return nil
	}
	return &domain.AudioPolicy{
		MaxPlays:                  r.maxPlays,
		AllowSeek:                 *r.allowSeek,
		ShowTranscriptAfterSubmit: *r.showTranscript,
	}
}

func (s *Postgres) Sections(ctx context.Context, testVersionID string) ([]domain.Section, error) {
	rows, err := s.Query(ctx, sectionsQuery, testVersionID)
	if err != nil {
		return nil, fmt.Errorf("attempts: read sections: %w", err)
	}
	defer rows.Close()

	var out []domain.Section
	for rows.Next() {
		var sec domain.Section
		if err := rows.Scan(&sec.ID, &sec.Title, &sec.Instructions); err != nil {
			return nil, fmt.Errorf("attempts: scan section: %w", err)
		}
		out = append(out, sec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("attempts: read sections: %w", err)
	}
	return out, nil
}

func (s *Postgres) Questions(ctx context.Context, testVersionID string) ([]domain.Question, error) {
	rows, err := s.Query(ctx, questionsQuery, testVersionID)
	if err != nil {
		return nil, fmt.Errorf("attempts: read questions: %w", err)
	}
	defer rows.Close()

	var out []domain.Question
	byID := map[string]int{}
	for rows.Next() {
		var q domain.Question
		var r questionRow
		if err := rows.Scan(&q.ID, &q.SectionID, &q.Type, &q.Prompt, &q.Points,
			&r.mediaID, &r.mediaKind, &r.mimeType, &r.filename, &r.mediaBytes,
			&r.durationMs, &r.createdAt,
			&r.maxPlays, &r.allowSeek, &r.showTranscript); err != nil {
			return nil, fmt.Errorf("attempts: scan question: %w", err)
		}
		q.Media = r.media()
		q.Audio = r.audio()
		byID[q.ID] = len(out)
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("attempts: read questions: %w", err)
	}

	if err := s.attachOptions(ctx, testVersionID, out, byID); err != nil {
		return nil, err
	}
	return out, s.attachBlanks(ctx, testVersionID, out, byID)
}

func (s *Postgres) attachOptions(ctx context.Context, versionID string, qs []domain.Question, at map[string]int) error {
	byQuestion, err := db.GroupBy(ctx, s.Conn(), `
		SELECT o.test_version_question_id, o.id, o.text
		  FROM app.test_version_options o
		  JOIN app.test_version_questions q ON q.id = o.test_version_question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1::uuid
		 ORDER BY o.ordinal`, []any{versionID},
		func(rows pgx.Rows) (string, domain.Option, error) {
			var questionID string
			var o domain.Option
			err := rows.Scan(&questionID, &o.ID, &o.Text)
			return questionID, o, err
		})
	if err != nil {
		return fmt.Errorf("attempts: read options: %w", err)
	}
	for questionID, options := range byQuestion {
		if i, ok := at[questionID]; ok {
			qs[i].Options = options
		}
	}
	return nil
}

func (s *Postgres) attachBlanks(ctx context.Context, versionID string, qs []domain.Question, at map[string]int) error {
	byQuestion, err := db.GroupBy(ctx, s.Conn(), `
		SELECT b.test_version_question_id, b.id, b.ordinal, b.case_sensitive
		  FROM app.test_version_blanks b
		  JOIN app.test_version_questions q ON q.id = b.test_version_question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1::uuid
		 ORDER BY b.ordinal`, []any{versionID},
		func(rows pgx.Rows) (string, domain.Blank, error) {
			var questionID string
			var b domain.Blank
			err := rows.Scan(&questionID, &b.ID, &b.Ordinal, &b.CaseSensitive)
			return questionID, b, err
		})
	if err != nil {
		return fmt.Errorf("attempts: read blanks: %w", err)
	}
	for questionID, blanks := range byQuestion {
		if i, ok := at[questionID]; ok {
			qs[i].Blanks = blanks
		}
	}
	return nil
}

// Answers is the base for the resume merge: what the server already holds,
// which the client reconciles against its own unflushed edits (§1.2).
func (s *Postgres) Answers(ctx context.Context, attemptID string) (map[string][]byte, error) {
	rows, err := s.Query(ctx, `
		SELECT question_id, payload FROM app.attempt_answers
		 WHERE attempt_id = $1::uuid`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("attempts: read answers: %w", err)
	}
	defer rows.Close()

	out := map[string][]byte{}
	for rows.Next() {
		var questionID string
		var payload []byte
		if err := rows.Scan(&questionID, &payload); err != nil {
			return nil, fmt.Errorf("attempts: scan answer: %w", err)
		}
		out[questionID] = payload
	}
	return out, rows.Err()
}

// ByID reads one attempt for its owner. The student id is part of the
// predicate rather than a check afterwards, so a caller cannot forget it and
// there is no window where the AttemptRecord exists in a variable belonging to nobody.
func (s *Postgres) ByID(ctx context.Context, attemptID, studentID string) (domain.AttemptRecord, error) {
	q := `SELECT ` + attemptColumns + `
	        FROM app.attempts
	       WHERE id = $1::uuid AND student_id = $2::uuid`
	out, err := scanAttempt(s.QueryRow(ctx, q, attemptID, studentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AttemptRecord{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.AttemptRecord{}, fmt.Errorf("attempts: read attempt: %w", err)
	}
	return out, nil
}

// RulesFor loads the assignment behind an attempt. Separate from Rules because
// the attempt already proves which assignment applies -- re-deriving it from a
// client-supplied id would be a way to render one paper under another's rules.
func (s *Postgres) RulesFor(ctx context.Context, assignmentID string) (domain.Rules, error) {
	var r domain.Rules
	err := s.QueryRow(ctx, `
		SELECT test_version_id, opens_at, closes_at, closed_at, published_at,
		       duration_minutes, max_attempts, shuffle_questions, shuffle_options,
		       integrity_require_fullscreen, integrity_block_copy_paste,
		       integrity_max_focus_loss, integrity_on_limit_exceeded,
		       integrity_min_away_ms
		  FROM app.assignments WHERE id = $1::uuid`, assignmentID).Scan(
		&r.TestVersionID, &r.OpensAt, &r.ClosesAt, &r.ClosedAt, &r.PublishedAt,
		&r.DurationMinutes, &r.MaxAttempts, &r.ShuffleQuestions, &r.ShuffleOptions,
		&r.Integrity.RequireFullscreen, &r.Integrity.BlockCopyPaste,
		&r.Integrity.MaxFocusLoss, &r.Integrity.OnLimitExceeded, &r.Integrity.MinAwayMs,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Rules{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Rules{}, fmt.Errorf("attempts: read rules for attempt: %w", err)
	}
	r.Targeted = true
	return r, nil
}

// Rebeacon issues a fresh append-only token WITHOUT touching session_id.
func (s *Postgres) Rebeacon(ctx context.Context, attemptID string, hash []byte) error {
	_, err := s.Exec(ctx,
		`UPDATE app.attempts SET beacon_token_hash = $2 WHERE id = $1::uuid`, attemptID, hash)
	if err != nil {
		return fmt.Errorf("attempts: reissue beacon token: %w", err)
	}
	return nil
}

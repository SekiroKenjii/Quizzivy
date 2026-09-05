package attempts

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/db"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Rules is what starting an attempt needs from the assignment: whether this
// student may start at all, and under what constraints if so.
type Rules struct {
	TestVersionID    string
	OpensAt          time.Time
	ClosesAt         time.Time
	ClosedAt         *time.Time
	PublishedAt      *time.Time
	DurationMinutes  int
	MaxAttempts      int
	ShuffleQuestions bool
	ShuffleOptions   bool
	Integrity        Integrity
	Targeted         bool
}

// Deadline is the §9 rule, server-side and authoritative: a student gets their
// full duration unless the assignment closes first.
//
// 40-open-items.md P3 settles the other direction -- once started, deadline_at
// wins and the student finishes even if closes_at passes mid-attempt. That is
// why this is computed once at creation and never recomputed on resume.
func (r Rules) Deadline(now time.Time) time.Time {
	full := now.Add(time.Duration(r.DurationMinutes) * time.Minute)
	if r.ClosesAt.Before(full) {
		return r.ClosesAt
	}
	return full
}

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

func (s *Store) Rules(ctx context.Context, assignmentID, studentID string) (Rules, error) {
	var r Rules
	err := s.pool.QueryRow(ctx, rulesQuery, assignmentID, studentID).Scan(
		&r.TestVersionID, &r.OpensAt, &r.ClosesAt, &r.ClosedAt, &r.PublishedAt,
		&r.DurationMinutes, &r.MaxAttempts, &r.ShuffleQuestions, &r.ShuffleOptions,
		&r.Integrity.RequireFullscreen, &r.Integrity.BlockCopyPaste,
		&r.Integrity.MaxFocusLoss, &r.Integrity.OnLimitExceeded, &r.Integrity.MinAwayMs,
		&r.Targeted,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Rules{}, ErrNotFound
	}
	if err != nil {
		return Rules{}, fmt.Errorf("attempts: read rules: %w", err)
	}
	return r, nil
}

// Unaliased so the identical list serves both SELECT and RETURNING; a
// RETURNING clause has no table alias in scope.
const attemptColumns = `
	id, assignment_id, student_id, test_version_id, attempt_no, status,
	started_at, deadline_at, submitted_at, graded_at,
	focus_loss_count, flagged, session_id, shuffle_seed`

// row carries the two columns Attempt deliberately does not: the seed is the
// server's business and the session id is handed over separately.
type row struct {
	Attempt
	SessionID string
	Seed      int64
}

func scanAttempt(r pgx.Row) (row, error) {
	var out row
	err := r.Scan(
		&out.ID, &out.AssignmentID, &out.StudentID, &out.TestVersionID, &out.AttemptNo,
		&out.Status, &out.StartedAt, &out.DeadlineAt, &out.SubmittedAt, &out.GradedAt,
		&out.FocusLossCount, &out.Flagged, &out.SessionID, &out.Seed,
	)
	return out, err
}

// Live finds the one attempt a student may still be working on. The partial
// unique index guarantees there is at most one.
func (s *Store) Live(ctx context.Context, assignmentID, studentID string) (row, error) {
	q := `SELECT ` + attemptColumns + `
	        FROM app.attempts
	       WHERE assignment_id = $1::uuid AND student_id = $2::uuid
	         AND status = 'in_progress'`
	out, err := scanAttempt(s.pool.QueryRow(ctx, q, assignmentID, studentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return row{}, ErrNotFound
	}
	if err != nil {
		return row{}, fmt.Errorf("attempts: read live attempt: %w", err)
	}
	return out, nil
}

// Tally answers the two questions the create path asks, which look like one
// and are not.
type Tally struct {
	Spent int
	Next  int
}

func (s *Store) Tally(ctx context.Context, assignmentID, studentID string) (Tally, error) {
	var t Tally
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status <> 'voided'),
		       coalesce(max(attempt_no), 0) + 1
		  FROM app.attempts
		 WHERE assignment_id = $1::uuid AND student_id = $2::uuid`,
		assignmentID, studentID).Scan(&t.Spent, &t.Next)
	if err != nil {
		return Tally{}, fmt.Errorf("attempts: tally attempts: %w", err)
	}
	return t, nil
}

type CreateInput struct {
	AssignmentID  string
	TestVersionID string
	StudentID     string
	AttemptNo     int
	SessionID     string
	Seed          int64
	BeaconHash    []byte
	StartedAt     time.Time
	DeadlineAt    time.Time
}

// ErrRaceLost means a concurrent create won. The caller resumes rather than
// erroring: from the student's side a double-tap produced one attempt, which is
// exactly what they wanted.
var ErrRaceLost = errors.New("attempts: concurrent create won")

func (s *Store) Create(ctx context.Context, in CreateInput) (row, error) {
	q := `
		INSERT INTO app.attempts
		  (assignment_id, test_version_id, student_id, attempt_no, session_id,
		   shuffle_seed, beacon_token_hash, started_at, deadline_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::uuid, $6, $7, $8, $9)
		RETURNING ` + attemptColumns
	out, err := scanAttempt(s.pool.QueryRow(ctx, q,
		in.AssignmentID, in.TestVersionID, in.StudentID, in.AttemptNo, in.SessionID,
		in.Seed, in.BeaconHash, in.StartedAt, in.DeadlineAt))
	if isUniqueViolation(err) {
		return row{}, ErrRaceLost
	}
	if err != nil {
		return row{}, fmt.Errorf("attempts: create: %w", err)
	}
	return out, nil
}

// isUniqueViolation deliberately does not name which index lost. Both
// attempts_one_live and the (assignment, student, attempt_no) unique mean the
// same thing here -- someone else got there first -- and PostgreSQL does not
// promise which one it reports when a row violates both.
func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

type ResumeInput struct {
	AttemptID  string
	SessionID  string
	BeaconHash []byte
	Now        time.Time
}

// Resume hands the attempt to a new tab and records why, in one transaction:
// the session swap and the events explaining it are the same fact, and a
// timeline missing the takeover it caused is worse than no timeline.
//
// It returns whether the superseded session still looked alive, so the caller
// can say so; the decision itself is made here because it depends on rows.
func (s *Store) Resume(ctx context.Context, in ResumeInput) (row, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return row{}, false, fmt.Errorf("attempts: begin resume: %w", err)
	}
	defer tx.Rollback(ctx)

	var previous string
	err = tx.QueryRow(ctx, `
		SELECT session_id FROM app.attempts
		 WHERE id = $1::uuid AND status = 'in_progress'
		   FOR UPDATE`, in.AttemptID).Scan(&previous)
	if errors.Is(err, pgx.ErrNoRows) {
		return row{}, false, ErrNotFound
	}
	if err != nil {
		return row{}, false, fmt.Errorf("attempts: lock attempt: %w", err)
	}

	takeover, err := sessionWasLive(ctx, tx, in.AttemptID, previous, in.Now)
	if err != nil {
		return row{}, false, err
	}

	updated, err := scanAttempt(tx.QueryRow(ctx, `
		UPDATE app.attempts
		   SET session_id = $2::uuid, beacon_token_hash = $3
		 WHERE id = $1::uuid
		RETURNING `+attemptColumns, in.AttemptID, in.SessionID, in.BeaconHash))
	if err != nil {
		return row{}, false, fmt.Errorf("attempts: swap session: %w", err)
	}

	if err := appendEvent(ctx, tx, in.AttemptID, in.SessionID, KindResume, in.Now); err != nil {
		return row{}, false, err
	}
	if takeover {
		if err := appendEvent(ctx, tx, in.AttemptID, previous, KindSessionTakeover, in.Now); err != nil {
			return row{}, false, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return row{}, false, fmt.Errorf("attempts: commit resume: %w", err)
	}
	return updated, takeover, nil
}

// sessionWasLive asks whether the session being superseded still had a tab
// open. There is no heartbeat, so the last thing it actually sent stands in.
//
// Server-written kinds are excluded: they are written on a session's behalf,
// not by it, so counting them would make two reloads in a row read the first
// reload's own `resume` as a live rival and report a takeover that never was.
func sessionWasLive(ctx context.Context, q querier, attemptID, sessionID string, now time.Time) (bool, error) {
	var live bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM app.attempt_events
		   WHERE attempt_id = $1::uuid AND session_id = $2::uuid
		     AND kind NOT IN ('resume', 'session_takeover')
		     AND received_at > $3
		)`, attemptID, sessionID, now.Add(-sessionLiveWindow)).Scan(&live)
	if err != nil {
		return false, fmt.Errorf("attempts: check session liveness: %w", err)
	}
	return live, nil
}

// appendEvent writes one of this package's own events. Client events take a
// different path (T-3.8): they carry a client clock and a sequence number that
// have to be reconciled, and these have neither -- client_seq is left NULL,
// which is what keeps a server row out of the client's dedup space.
func appendEvent(ctx context.Context, q querier, attemptID, sessionID, kind string, now time.Time) error {
	_, err := q.Exec(ctx, `
		INSERT INTO app.attempt_events (attempt_id, session_id, kind, occurred_at)
		VALUES ($1::uuid, $2::uuid, $3, $4)`,
		attemptID, sessionID, kind, now)
	if err != nil {
		return fmt.Errorf("attempts: append %s event: %w", kind, err)
	}
	return nil
}

type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// questionsQuery is §13.5's rule made structural: an explicit column list, so
// the grading key cannot arrive by accident.
const questionsQuery = `
	SELECT q.id, q.type, q.prompt, q.points,
	       q.media_asset_id, q.media_asset_kind, m.mime_type, m.original_filename,
	       m.bytes, m.duration_ms, m.created_at,
	       q.audio_max_plays, q.audio_allow_seek, q.audio_show_transcript_after
	  FROM app.test_version_questions q
	  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
	  LEFT JOIN app.media_assets m ON m.id = q.media_asset_id
	 WHERE s.test_version_id = $1::uuid
	 ORDER BY s.ordinal, q.ordinal`

// questionRow is the nullable half of questionsQuery.
type questionRow struct {
	mediaID, mediaKind, mimeType, filename *string
	mediaBytes, durationMs, maxPlays       *int
	createdAt                              *time.Time
	allowSeek, showTranscript              *bool
}

func (r questionRow) media() *Media {
	if r.mediaID == nil {
		return nil
	}
	m := &Media{
		ID: *r.mediaID, Kind: deref(r.mediaKind), MimeType: deref(r.mimeType),
		Filename: deref(r.filename), DurationMs: r.durationMs,
	}
	if r.mediaBytes != nil {
		m.Bytes = *r.mediaBytes
	}
	if r.createdAt != nil {
		m.CreatedAt = *r.createdAt
	}
	return m
}

func (r questionRow) audio() *AudioPolicy {
	if r.allowSeek == nil || r.showTranscript == nil {
		return nil
	}
	return &AudioPolicy{
		MaxPlays:                  r.maxPlays,
		AllowSeek:                 *r.allowSeek,
		ShowTranscriptAfterSubmit: *r.showTranscript,
	}
}

func (s *Store) Questions(ctx context.Context, testVersionID string) ([]Question, error) {
	rows, err := s.pool.Query(ctx, questionsQuery, testVersionID)
	if err != nil {
		return nil, fmt.Errorf("attempts: read questions: %w", err)
	}
	defer rows.Close()

	var out []Question
	byID := map[string]int{}
	for rows.Next() {
		var q Question
		var r questionRow
		if err := rows.Scan(&q.ID, &q.Type, &q.Prompt, &q.Points,
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

// attachOptions selects id and text. Not is_correct -- see questionsQuery.
func (s *Store) attachOptions(ctx context.Context, versionID string, qs []Question, at map[string]int) error {
	byQuestion, err := db.GroupBy(ctx, s.pool, `
		SELECT o.test_version_question_id, o.id, o.text
		  FROM app.test_version_options o
		  JOIN app.test_version_questions q ON q.id = o.test_version_question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1::uuid
		 ORDER BY o.ordinal`, []any{versionID},
		func(rows pgx.Rows) (string, Option, error) {
			var questionID string
			var o Option
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

// attachBlanks selects id and ordinal. Not case_sensitive, and never the
// accepted answers -- see questionsQuery.
func (s *Store) attachBlanks(ctx context.Context, versionID string, qs []Question, at map[string]int) error {
	byQuestion, err := db.GroupBy(ctx, s.pool, `
		SELECT b.test_version_question_id, b.id, b.ordinal
		  FROM app.test_version_blanks b
		  JOIN app.test_version_questions q ON q.id = b.test_version_question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $1::uuid
		 ORDER BY b.ordinal`, []any{versionID},
		func(rows pgx.Rows) (string, Blank, error) {
			var questionID string
			var b Blank
			err := rows.Scan(&questionID, &b.ID, &b.Ordinal)
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
func (s *Store) Answers(ctx context.Context, attemptID string) (map[string][]byte, error) {
	rows, err := s.pool.Query(ctx, `
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

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ByID reads one attempt for its owner. The student id is part of the
// predicate rather than a check afterwards, so a caller cannot forget it and
// there is no window where the row exists in a variable belonging to nobody.
func (s *Store) ByID(ctx context.Context, attemptID, studentID string) (row, error) {
	q := `SELECT ` + attemptColumns + `
	        FROM app.attempts
	       WHERE id = $1::uuid AND student_id = $2::uuid`
	out, err := scanAttempt(s.pool.QueryRow(ctx, q, attemptID, studentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return row{}, ErrNotFound
	}
	if err != nil {
		return row{}, fmt.Errorf("attempts: read attempt: %w", err)
	}
	return out, nil
}

// RulesFor loads the assignment behind an attempt. Separate from Rules because
// the attempt already proves which assignment applies -- re-deriving it from a
// client-supplied id would be a way to render one paper under another's rules.
func (s *Store) RulesFor(ctx context.Context, assignmentID string) (Rules, error) {
	var r Rules
	err := s.pool.QueryRow(ctx, `
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
		return Rules{}, ErrNotFound
	}
	if err != nil {
		return Rules{}, fmt.Errorf("attempts: read rules for attempt: %w", err)
	}
	r.Targeted = true
	return r, nil
}

// Rebeacon issues a fresh append-only token WITHOUT touching session_id.
//
// Refetching the payload is not taking the attempt over, so it must not
// supersede the tab doing it. The token still has to change: it is stored
// hashed and cannot be read back, so a response that must carry one can only
// carry a new one.
func (s *Store) Rebeacon(ctx context.Context, attemptID string, hash []byte) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE app.attempts SET beacon_token_hash = $2 WHERE id = $1::uuid`, attemptID, hash)
	if err != nil {
		return fmt.Errorf("attempts: reissue beacon token: %w", err)
	}
	return nil
}

package attempts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Save writes a batch of answers and the events that accompanied them, in one
// transaction.
//
// One transaction is the §15 requirement and the reason is ordering: a partial
// flush that recorded a `paste` event for an answer it failed to save would put
// a claim about the student in the teacher's timeline with no work behind it.
func (s *Store) Save(ctx context.Context, in SaveInput, now time.Time) (SaveResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SaveResult{}, fmt.Errorf("attempts: begin save: %w", err)
	}
	defer tx.Rollback(ctx)

	versionID, err := writable(ctx, tx, in, now)
	if err != nil {
		return SaveResult{}, err
	}
	saved, dropped, err := upsertAnswers(ctx, tx, in, versionID)
	if err != nil {
		return SaveResult{}, err
	}
	if err := insertEvents(ctx, tx, in.AttemptID, in.SessionID, in.Events, versionID); err != nil {
		return SaveResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return SaveResult{}, fmt.Errorf("attempts: commit save: %w", err)
	}
	return SaveResult{SavedAt: now, Saved: saved, Dropped: dropped}, nil
}

// writable locks the attempt and decides whether this write may happen at all,
// returning the version whose questions it is allowed to touch.
//
// The order of the refusals is the order of what the client should do about
// them. A closed attempt is over and nothing else is worth saying. A superseded
// session must hear that first even if the deadline has also passed, because
// "submit" is advice it cannot act on -- it is not the session any more.
func writable(ctx context.Context, tx pgx.Tx, in SaveInput, now time.Time) (string, error) {
	var (
		session    string
		status     Status
		deadlineAt time.Time
		versionID  string
	)
	err := tx.QueryRow(ctx, `
		SELECT session_id, status, deadline_at, test_version_id
		  FROM app.attempts
		 WHERE id = $1::uuid AND student_id = $2::uuid
		   FOR UPDATE`, in.AttemptID, in.StudentID).Scan(&session, &status, &deadlineAt, &versionID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Someone else's attempt and an attempt that does not exist are one
		// answer, as everywhere else in this package.
		return "", ErrForbidden
	}
	if err != nil {
		return "", fmt.Errorf("attempts: lock attempt for save: %w", err)
	}

	switch {
	case status != InProgress:
		return "", ErrAttemptClosed
	case session != in.SessionID:
		return "", ErrSessionSuperseded
	case now.After(deadlineAt):
		return "", ErrDeadlinePassed
	}
	return versionID, nil
}

// upsertAnswers writes by (attempt, question), so a retried batch overwrites
// rather than duplicating.
//
// The join to the version is what stops one student writing an answer against a
// question on somebody else's paper, and the option check is the same rule one
// level down: a choice answer naming an id that is not an option of THAT
// question -- malformed, or a real option lifted from another question -- is
// treated exactly like an unknown question. Rows that do not survive are
// DROPPED rather than failing the batch, and that is deliberate: the only ways
// to send such an id are a client bug or an attempt at exactly what the checks
// block, and in both cases refusing the whole batch would throw away the real
// answers sitting beside it. Losing a student's work is the one outcome this
// feature exists to prevent. The count comes back so a caller can notice the
// gap, and nothing that would grade as nonsense is stored.
//
// requires_manual is set from the question type rather than left to grading
// (D-19): final_score is VIRTUAL and unindexable, so §7's pendingManual needs a
// real column to filter on, and the type is known here.
func upsertAnswers(ctx context.Context, tx pgx.Tx, in SaveInput, versionID string) (int, []string, error) {
	if len(in.Answers) == 0 {
		return 0, nil, nil
	}
	ids := make([]string, len(in.Answers))
	payloads := make([]string, len(in.Answers))
	for i, a := range in.Answers {
		ids[i] = a.QuestionID
		payloads[i] = string(a.Payload)
	}

	rows, err := tx.Query(ctx, `
		INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual)
		SELECT $1::uuid, q.id, submitted.payload::jsonb, q.type = 'short_answer'
		  FROM unnest($2::uuid[], $3::text[]) AS submitted(question_id, payload)
		  JOIN app.test_version_questions q ON q.id = submitted.question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $4::uuid
		   AND NOT EXISTS (
		         SELECT 1
		           FROM jsonb_array_elements_text(
		                  CASE WHEN jsonb_typeof(submitted.payload::jsonb->'optionIds') = 'array'
		                       THEN submitted.payload::jsonb->'optionIds'
		                       ELSE '[]'::jsonb END) AS chosen(id)
		          WHERE NOT EXISTS (
		                  SELECT 1 FROM app.test_version_options o
		                   WHERE o.id::text = chosen.id
		                     AND o.test_version_question_id = q.id))
		ON CONFLICT (attempt_id, question_id) DO UPDATE
		   SET payload = EXCLUDED.payload,
		       requires_manual = EXCLUDED.requires_manual
		RETURNING question_id::text`,
		in.AttemptID, ids, payloads, versionID)
	if err != nil {
		return 0, nil, fmt.Errorf("attempts: upsert answers: %w", err)
	}
	defer rows.Close()

	landed := make(map[string]struct{}, len(in.Answers))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, nil, fmt.Errorf("attempts: upsert answers: %w", err)
		}
		landed[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return 0, nil, fmt.Errorf("attempts: upsert answers: %w", err)
	}

	var dropped []string
	for _, id := range ids {
		if _, ok := landed[id]; !ok {
			dropped = append(dropped, id)
		}
	}
	return len(landed), dropped, nil
}

// insertEvents appends the batch, ignoring anything already recorded.
//
// [D-01] ON CONFLICT DO NOTHING on (attempt, session, client_seq) is what makes
// a retried flush a no-op instead of a duplicate-key failure. A failed flush
// must never block answering, so the client retries freely and this absorbs it.
//
// A question id that is not on this paper is stored as NULL rather than
// rejected. The column means "what was on screen", it is already nullable for
// the events that have no question, and an event is telemetry -- worth less
// than the answers travelling with it in the same transaction.
func insertEvents(ctx context.Context, q querier, attemptID, sessionID string, events []Event, versionID string) error {
	if len(events) == 0 {
		return nil
	}
	kinds := make([]string, len(events))
	occurred := make([]time.Time, len(events))
	seqs := make([]int32, len(events))
	questions := make([]*string, len(events))
	metas := make([]*string, len(events))
	for i, e := range events {
		kinds[i] = e.Kind
		occurred[i] = e.OccurredAt
		seqs[i] = int32(e.ClientSeq)
		questions[i] = e.QuestionID
		if len(e.Meta) > 0 {
			meta := string(e.Meta)
			metas[i] = &meta
		}
	}

	_, err := q.Exec(ctx, `
		INSERT INTO app.attempt_events
		  (attempt_id, session_id, kind, occurred_at, client_seq, question_id, meta)
		SELECT $1::uuid, $2::uuid, submitted.kind, submitted.occurred_at, submitted.client_seq,
		       q.id, submitted.meta::jsonb
		  FROM unnest($3::text[], $4::timestamptz[], $5::int[], $6::uuid[], $7::text[])
		         AS submitted(kind, occurred_at, client_seq, question_id, meta)
		  LEFT JOIN app.test_version_questions q
		    ON q.id = submitted.question_id
		   AND EXISTS (
		         SELECT 1 FROM app.test_version_sections s
		          WHERE s.id = q.test_version_section_id AND s.test_version_id = $8::uuid)
		ON CONFLICT (attempt_id, session_id, client_seq) DO NOTHING
`,
		attemptID, sessionID, kinds, occurred, seqs, questions, metas, versionID)
	if err != nil {
		return fmt.Errorf("attempts: insert events: %w", err)
	}
	return deriveFocusLoss(ctx, q, attemptID)
}

// deriveFocusLoss recounts the attempt's away episodes from its event log and
// sets the two columns the dashboard, the monitor and the resume payload read.
//
// Recounted rather than incremented, on the same connection as the insert
// that changed the log, so a retried batch that ON CONFLICT swallowed cannot
// count twice. Exactly one event per away episode carries `awayMs` -- the
// client attaches it to the first "returned" signal -- so counting those is
// counting episodes, and only those at or over the assignment's threshold
// (§10.1: a 2-second blur is a notification, not a strike).
//
// The limit is exceeded when the count is OVER it, which is what the intro's
// "quá 2 lần" and the contract's onLimitExceeded both say. `flagged` is
// sticky: the teacher clears it (Phase 4), and a recount must not.
//
// A malformed awayMs is skipped rather than raised. This runs inside the
// answers' transaction, and a bad byte in telemetry must never roll back the
// answer it travelled with (§10.6).
func deriveFocusLoss(ctx context.Context, q querier, attemptID string) error {
	_, err := q.Exec(ctx, `
		UPDATE app.attempts at
		   SET focus_loss_count = counted.n,
		       flagged = at.flagged
		         OR (a.integrity_max_focus_loss > 0
		             AND counted.n > a.integrity_max_focus_loss
		             AND a.integrity_on_limit_exceeded IN ('flag', 'auto_submit'))
		  FROM app.assignments a,
		       LATERAL (
		         SELECT count(*) AS n
		           FROM app.attempt_events e
		          WHERE e.attempt_id = $1::uuid
		            AND CASE WHEN jsonb_typeof(e.meta->'awayMs') = 'number'
		                     THEN (e.meta->>'awayMs')::numeric >= a.integrity_min_away_ms
		                     ELSE false END
		       ) AS counted
		 WHERE at.id = $1::uuid AND a.id = at.assignment_id`, attemptID)
	if err != nil {
		return fmt.Errorf("attempts: derive focus loss: %w", err)
	}
	return nil
}

// Save is the service's side: it owns the clock, and nothing else here needs
// deciding.
func (s *Service) Save(ctx context.Context, in SaveInput) (SaveResult, error) {
	return s.store.Save(ctx, in, s.now())
}

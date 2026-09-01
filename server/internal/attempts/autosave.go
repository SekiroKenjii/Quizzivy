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
	saved, err := upsertAnswers(ctx, tx, in, versionID)
	if err != nil {
		return SaveResult{}, err
	}
	if err := insertEvents(ctx, tx, in, versionID); err != nil {
		return SaveResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return SaveResult{}, fmt.Errorf("attempts: commit save: %w", err)
	}
	return SaveResult{SavedAt: now, Saved: saved}, nil
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
// question on somebody else's paper. Rows that do not survive it are DROPPED
// rather than failing the batch, and that is deliberate: the only ways to send
// an unknown id are a client bug or an attempt at exactly what the join blocks,
// and in both cases refusing the whole batch would throw away the real answers
// sitting beside it. Losing a student's work is the one outcome this feature
// exists to prevent. The count comes back so a caller can notice the gap.
//
// requires_manual is set from the question type rather than left to grading
// (D-19): final_score is VIRTUAL and unindexable, so §7's pendingManual needs a
// real column to filter on, and the type is known here.
func upsertAnswers(ctx context.Context, tx pgx.Tx, in SaveInput, versionID string) (int, error) {
	if len(in.Answers) == 0 {
		return 0, nil
	}
	ids := make([]string, len(in.Answers))
	payloads := make([]string, len(in.Answers))
	for i, a := range in.Answers {
		ids[i] = a.QuestionID
		payloads[i] = string(a.Payload)
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual)
		SELECT $1::uuid, q.id, submitted.payload::jsonb, q.type = 'short_answer'
		  FROM unnest($2::uuid[], $3::text[]) AS submitted(question_id, payload)
		  JOIN app.test_version_questions q ON q.id = submitted.question_id
		  JOIN app.test_version_sections s ON s.id = q.test_version_section_id
		 WHERE s.test_version_id = $4::uuid
		ON CONFLICT (attempt_id, question_id) DO UPDATE
		   SET payload = EXCLUDED.payload,
		       requires_manual = EXCLUDED.requires_manual`,
		in.AttemptID, ids, payloads, versionID)
	if err != nil {
		return 0, fmt.Errorf("attempts: upsert answers: %w", err)
	}
	return int(tag.RowsAffected()), nil
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
func insertEvents(ctx context.Context, tx pgx.Tx, in SaveInput, versionID string) error {
	if len(in.Events) == 0 {
		return nil
	}
	kinds := make([]string, len(in.Events))
	occurred := make([]time.Time, len(in.Events))
	seqs := make([]int32, len(in.Events))
	questions := make([]*string, len(in.Events))
	metas := make([]*string, len(in.Events))
	for i, e := range in.Events {
		kinds[i] = e.Kind
		occurred[i] = e.OccurredAt
		seqs[i] = int32(e.ClientSeq)
		questions[i] = e.QuestionID
		if len(e.Meta) > 0 {
			meta := string(e.Meta)
			metas[i] = &meta
		}
	}

	_, err := tx.Exec(ctx, `
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
		in.AttemptID, in.SessionID, kinds, occurred, seqs, questions, metas, versionID)
	if err != nil {
		return fmt.Errorf("attempts: insert events: %w", err)
	}
	return nil
}

// Save is the service's side: it owns the clock, and nothing else here needs
// deciding.
func (s *Service) Save(ctx context.Context, in SaveInput) (SaveResult, error) {
	return s.store.Save(ctx, in, s.now())
}

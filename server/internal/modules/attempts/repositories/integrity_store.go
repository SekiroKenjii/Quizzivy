package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Timelines struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func NewTimelines(pool *pgxpool.Pool) *Timelines { return &Timelines{pool: pool, now: time.Now} }

// Timeline reads one attempt's log and builds §10.4's view of it.
func (s *Timelines) Timeline(ctx context.Context, attemptID string) (domain.Timeline, error) {
	var (
		startedAt    time.Time
		minAwayMs    int
		audioReplays int
	)
	err := s.pool.QueryRow(ctx, `
		SELECT at.started_at, a.integrity_min_away_ms,
		       coalesce((SELECT sum(greatest(p.plays - q.audio_max_plays, 0))
		                   FROM app.attempt_audio_plays p
		                   JOIN app.test_version_questions q ON q.id = p.question_id
		                  WHERE p.attempt_id = at.id AND q.audio_max_plays IS NOT NULL), 0)
		  FROM app.attempts at
		  JOIN app.assignments a ON a.id = at.assignment_id
		 WHERE at.id = $1::uuid`, attemptID).Scan(&startedAt, &minAwayMs, &audioReplays)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Timeline{}, domain.ErrTimelineNotFound
	}
	if err != nil {
		return domain.Timeline{}, fmt.Errorf("integrity: read attempt: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, kind, occurred_at, received_at, client_seq, session_id::text,
		       question_id::text, meta
		  FROM app.attempt_events
		 WHERE attempt_id = $1::uuid
		 ORDER BY occurred_at, id`, attemptID)
	if err != nil {
		return domain.Timeline{}, fmt.Errorf("integrity: read events: %w", err)
	}
	defer rows.Close()

	var events []domain.IntegrityEvent
	for rows.Next() {
		var e domain.IntegrityEvent
		if err := rows.Scan(&e.ID, &e.Kind, &e.OccurredAt, &e.ReceivedAt, &e.ClientSeq,
			&e.SessionID, &e.QuestionID, &e.Meta); err != nil {
			return domain.Timeline{}, fmt.Errorf("integrity: scan event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return domain.Timeline{}, fmt.Errorf("integrity: read events: %w", err)
	}
	return domain.BuildTimeline(startedAt, minAwayMs, audioReplays, events, s.now()), nil
}

package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"time"

	"github.com/jackc/pgx/v5"
)

// RecordPlay increments the server-authoritative counter and returns the new
// value (§11.4).
const recordPlayQuery = `
	WITH allowed AS (
	  SELECT q.id, q.audio_max_plays
	    FROM app.attempts a
	    JOIN app.test_version_sections s ON s.test_version_id = a.test_version_id
	    JOIN app.test_version_questions q ON q.test_version_section_id = s.id
	   WHERE a.id = $1::uuid AND a.student_id = $2::uuid AND q.id = $3::uuid
	), bumped AS (
	  INSERT INTO app.attempt_audio_plays (attempt_id, question_id, plays, last_played_at)
	  SELECT $1::uuid, allowed.id, 1, $4 FROM allowed
	  ON CONFLICT (attempt_id, question_id) DO UPDATE
	     SET plays = app.attempt_audio_plays.plays + 1,
	         last_played_at = EXCLUDED.last_played_at
	  RETURNING plays
	)
	SELECT bumped.plays, allowed.audio_max_plays FROM bumped, allowed`

func (s *Postgres) RecordPlay(ctx context.Context, attemptID, studentID, questionID string, now time.Time) (domain.Plays, error) {
	var out domain.Plays
	err := s.pool.QueryRow(ctx, recordPlayQuery, attemptID, studentID, questionID, now).
		Scan(&out.Plays, &out.MaxPlays)
	if errors.Is(err, pgx.ErrNoRows) {
		// Not this student's attempt, or not a question on its paper.
		return domain.Plays{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.Plays{}, fmt.Errorf("attempts: record audio play: %w", err)
	}
	return out, nil
}

// AudioPlays is what the payload carries so the client can render remaining
// plays after a reload, having lost whatever it was counting locally.
func (s *Postgres) AudioPlays(ctx context.Context, attemptID string) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT question_id, plays FROM app.attempt_audio_plays
		 WHERE attempt_id = $1::uuid`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("attempts: read audio plays: %w", err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var questionID string
		var plays int
		if err := rows.Scan(&questionID, &plays); err != nil {
			return nil, fmt.Errorf("attempts: scan audio plays: %w", err)
		}
		out[questionID] = plays
	}
	return out, rows.Err()
}

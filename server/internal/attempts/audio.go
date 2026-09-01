package attempts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Plays is what the client renders "còn N lượt nghe" from.
type Plays struct {
	Plays    int
	MaxPlays *int
}

// RecordPlay increments the server-authoritative counter and returns the new
// value (§11.4).
//
// One statement, so ten taps racing each other produce ten plays: concurrent
// upserts serialise on the row and each reads the value it wrote. A
// read-then-write would lose every increment that landed between the two.
//
// It never refuses on count. A play past maxPlays returns 200 with the higher
// number, which is what the teacher sees; blocking would punish bad wifi far
// more often than it would catch anyone, and a student set on replaying can go
// offline, which leaves a gap in the event log instead. The only refusal here
// is the one that is not about counting: whether this attempt and this question
// are the caller's to touch at all.
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

func (s *Store) RecordPlay(ctx context.Context, attemptID, studentID, questionID string, now time.Time) (Plays, error) {
	var out Plays
	err := s.pool.QueryRow(ctx, recordPlayQuery, attemptID, studentID, questionID, now).
		Scan(&out.Plays, &out.MaxPlays)
	if errors.Is(err, pgx.ErrNoRows) {
		// Not this student's attempt, or not a question on its paper. One
		// answer for both, as everywhere else here.
		return Plays{}, ErrForbidden
	}
	if err != nil {
		return Plays{}, fmt.Errorf("attempts: record audio play: %w", err)
	}
	return out, nil
}

// AudioPlays is what the payload carries so the client can render remaining
// plays after a reload, having lost whatever it was counting locally.
func (s *Store) AudioPlays(ctx context.Context, attemptID string) (map[string]int, error) {
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

func (s *Service) RecordPlay(ctx context.Context, attemptID, studentID, questionID string) (Plays, error) {
	return s.store.RecordPlay(ctx, attemptID, studentID, questionID, s.now())
}

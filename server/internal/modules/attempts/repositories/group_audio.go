package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/platform/db"
	"time"

	"github.com/jackc/pgx/v5"
)

// RecordGroupPlay serializes with session changes and atomically records one gesture, count and timeline event.
func (s *Postgres) RecordGroupPlay(ctx context.Context, in domain.GroupPlayInput, now time.Time) (domain.GroupPlays, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.GroupPlays{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	versionID, err := writable(ctx, tx, domain.SaveInput{AttemptID: in.AttemptID, StudentID: in.StudentID, SessionID: in.SessionID}, now)
	if err != nil {
		return domain.GroupPlays{}, err
	}
	out := domain.GroupPlays{PlayID: in.PlayID}
	var groupID string
	err = tx.QueryRow(ctx, `SELECT r.group_id::text,r.max_plays
        FROM app.test_version_group_recordings r
        JOIN app.test_version_groups g ON g.id=r.group_id
        JOIN app.test_version_sections s ON s.id=g.test_version_section_id
        WHERE r.id=$1 AND s.test_version_id=$2
          AND EXISTS (SELECT 1 FROM app.test_version_group_assets ga
            WHERE ga.group_id=r.group_id AND ga.recording_id=r.id AND ga.media_asset_id=r.media_asset_id)`, in.RecordingID, versionID).Scan(&groupID, &out.MaxPlays)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GroupPlays{}, domain.ErrForbidden
	}
	if err != nil {
		return domain.GroupPlays{}, err
	}
	var recordedID string
	err = tx.QueryRow(ctx, `SELECT r.recording_id::text,p.plays FROM app.attempt_group_audio_receipts r
        JOIN app.attempt_group_audio_plays p ON p.attempt_id=r.attempt_id AND p.recording_id=r.recording_id
        WHERE r.attempt_id=$1 AND r.play_id=$2`, in.AttemptID, in.PlayID).Scan(&recordedID, &out.Plays)
	if err == nil {
		if recordedID != in.RecordingID {
			return domain.GroupPlays{}, domain.ErrPlayIDConflict
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.GroupPlays{}, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO app.attempt_group_audio_plays(attempt_id,recording_id,plays,last_played_at)
        VALUES($1,$2,1,$3) ON CONFLICT(attempt_id,recording_id) DO UPDATE
        SET plays=app.attempt_group_audio_plays.plays+1,
            last_played_at=greatest(app.attempt_group_audio_plays.last_played_at,EXCLUDED.last_played_at)
        RETURNING plays`, in.AttemptID, in.RecordingID, now).Scan(&out.Plays)
	if err != nil {
		return domain.GroupPlays{}, err
	}
	if err := appendGroupPlay(ctx, tx, in, out, groupID, now); err != nil {
		return domain.GroupPlays{}, err
	}
	return out, tx.Commit(ctx)
}

func appendGroupPlay(ctx context.Context, tx pgx.Tx, in domain.GroupPlayInput, out domain.GroupPlays, groupID string, now time.Time) error {
	if _, err := tx.Exec(ctx, `INSERT INTO app.attempt_group_audio_receipts(attempt_id,play_id,recording_id,session_id,received_at)
        VALUES($1,$2,$3,$4,$5)`, in.AttemptID, in.PlayID, in.RecordingID, in.SessionID, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO app.attempt_events(attempt_id,session_id,kind,occurred_at,meta)
        VALUES($1,$2,'audio_play',$3,jsonb_build_object('scope','group','groupId',$4::text,
        'recordingId',$5::text,'playId',$6::text,'plays',$7::integer,'maxPlays',$8::integer))`,
		in.AttemptID, in.SessionID, now, groupID, in.RecordingID, in.PlayID, out.Plays, out.MaxPlays)
	return err
}

// GroupAudioPlays returns counters by frozen recording identity, independent of active question or session.
func (s *Postgres) GroupAudioPlays(ctx context.Context, attemptID string) (map[string]int, error) {
	return sharedAudioPlays(ctx, s, attemptID)
}

func (s *Reviews) GroupAudioPlays(ctx context.Context, attemptID string) (map[string]int, error) {
	return sharedAudioPlays(ctx, s, attemptID)
}

func sharedAudioPlays(ctx context.Context, conn db.Querier, attemptID string) (map[string]int, error) {
	rows, err := conn.Query(ctx, `SELECT recording_id::text,plays FROM app.attempt_group_audio_plays WHERE attempt_id=$1`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("attempts: read shared audio plays: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var plays int
		if err := rows.Scan(&id, &plays); err != nil {
			return nil, err
		}
		out[id] = plays
	}
	return out, rows.Err()
}

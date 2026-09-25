package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/platform/db"
)

func (s *Postgres) ReleasedGroupTranscripts(ctx context.Context, versionID string) (map[string]string, error) {
	return groupTranscripts(ctx, s, versionID, false)
}

func (s *Reviews) GroupTranscripts(ctx context.Context, versionID string) (map[string]string, error) {
	return groupTranscripts(ctx, s, versionID, true)
}

func groupTranscripts(ctx context.Context, conn db.Querier, versionID string, teacher bool) (map[string]string, error) {
	rows, err := conn.Query(ctx, `SELECT r.id::text,r.transcript
        FROM app.test_version_group_recordings r
        JOIN app.test_version_groups g ON g.id=r.group_id
        JOIN app.test_version_sections s ON s.id=g.test_version_section_id
        WHERE s.test_version_id=$1 AND r.transcript IS NOT NULL
          AND ($2 OR r.show_transcript_after_submit)`, versionID, teacher)
	if err != nil {
		return nil, fmt.Errorf("review: read shared transcripts: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, transcript string
		if err := rows.Scan(&id, &transcript); err != nil {
			return nil, err
		}
		out[id] = transcript
	}
	return out, rows.Err()
}

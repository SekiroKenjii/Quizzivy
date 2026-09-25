package repositories

import (
	"context"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/db"
	"time"
)

func attachProgress(ctx context.Context, q db.Querier, items []domain.Import) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	positions := make(map[string]int, len(items))
	for i, item := range items {
		ids[i] = item.ID
		positions[item.ID] = i
	}
	rows, err := q.Query(ctx, `SELECT i.id::text, coalesce(d.revision,0), c.test_id::text,
 r.id::text, r.status, r.stage, r.attempt_count, r.max_attempts, r.error_code, r.updated_at
 FROM unnest($1::uuid[]) AS i(id)
 LEFT JOIN app.word_import_drafts d ON d.import_id=i.id
 LEFT JOIN app.word_import_commits c ON c.import_id=i.id
 LEFT JOIN LATERAL (SELECT id,status,stage,attempt_count,max_attempts,error_code,updated_at FROM app.word_import_runs
   WHERE import_id=i.id ORDER BY created_at DESC,id DESC LIMIT 1) r ON true`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var draft int64
		var test, runID, status, stage, code *string
		var attempt, maxAttempts *int
		var updated *time.Time
		if err := rows.Scan(&id, &draft, &test, &runID, &status, &stage, &attempt, &maxAttempts, &code, &updated); err != nil {
			return err
		}
		item := &items[positions[id]]
		item.DraftRevision, item.TestID = draft, test
		if runID != nil {
			item.Run = &domain.RunSummary{ID: *runID, Status: *status, Stage: *stage, Attempt: *attempt, MaxAttempts: *maxAttempts, ErrorCode: code, UpdatedAt: *updated}
		}
	}
	return rows.Err()
}

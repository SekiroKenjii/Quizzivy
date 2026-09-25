package repositories

import (
	"context"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/opt"
)

func recordRunEvent(ctx context.Context, tx pgx.Tx, run domain.Run, kind, code string, actorID *string) error {
	_, err := tx.Exec(ctx, `INSERT INTO app.word_import_run_events(run_id,import_id,kind,stage,claim_token,attempt_count,worker_id,actor_id,error_code) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, run.ID, run.ImportID, kind, run.Stage, run.ClaimToken, run.AttemptCount, run.WorkerID, actorID, opt.String(code))
	return err
}
func (s *Postgres) Events(ctx context.Context, importID, runID string) ([]domain.RunEvent, error) {
	if _, err := s.Run(ctx, importID, runID); err != nil {
		return nil, err
	}
	return db.QueryMany(ctx, s, `SELECT id::text,run_id::text,import_id::text,kind,stage,claim_token,attempt_count,worker_id::text,actor_id::text,error_code,created_at FROM app.word_import_run_events WHERE import_id=$1 AND run_id=$2 ORDER BY id`, []any{importID, runID}, func(row pgx.Rows) (domain.RunEvent, error) {
		var e domain.RunEvent
		err := row.Scan(&e.ID, &e.RunID, &e.ImportID, &e.Kind, &e.Stage, &e.ClaimToken, &e.AttemptCount, &e.WorkerID, &e.ActorID, &e.ErrorCode, &e.CreatedAt)
		return e, err
	})
}

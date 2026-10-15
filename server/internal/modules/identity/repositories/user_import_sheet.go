package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
)

// TakeSheet implements domain.UserImports.
func (u *UserImports) TakeSheet(ctx context.Context, req domain.WriteRequest, id string, now time.Time, open domain.SecretOpener) ([]domain.SheetRow, error) {
	tx, err := u.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("user imports: begin sheet: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	job, err := scanImportJob(tx.QueryRow(ctx, importJobSelect+`
		 WHERE j.id = $2::uuid AND j.created_by = $1::uuid
		   FOR UPDATE OF j`, req.ActorID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUserImportNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user imports: lock the job for its sheet: %w", err)
	}
	if state, ok := job.Sheet(now); !ok || !state.Available {
		return nil, domain.ErrSheetUnavailable
	}

	sheet, err := readSheet(ctx, tx, id, open)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE app.user_import_rows SET secret_ciphertext = NULL
		 WHERE job_id = $1::uuid AND secret_ciphertext IS NOT NULL`, id); err != nil {
		return nil, fmt.Errorf("user imports: erase the sealed passwords: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE app.user_import_jobs SET sheet_downloaded_at = $2 WHERE id = $1::uuid`, id, now); err != nil {
		return nil, fmt.Errorf("user imports: record the download: %w", err)
	}
	diff, err := json.Marshal(struct {
		Rows int `json:"rows"`
	}{len(sheet)})
	if err != nil {
		return nil, fmt.Errorf("user imports: encode audit diff: %w", err)
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "user_import.sheet_downloaded",
		Entity:      entityUserImport,
		EntityID:    &id,
		OccurredAt:  now,
		IP:          opt.String(req.IP),
		UserAgent:   opt.String(req.UserAgent),
		Diff:        diff,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("user imports: commit sheet: %w", err)
	}
	return sheet, nil
}

func readSheet(ctx context.Context, tx pgx.Tx, jobID string, open domain.SecretOpener) ([]domain.SheetRow, error) {
	rows, err := tx.Query(ctx, `
		SELECT r.ordinal, r.full_name, r.email, ro.name, coalesce(c.name, ''), r.secret_ciphertext
		  FROM app.user_import_rows r
		  JOIN app.roles ro ON ro.id = r.role_id
		  LEFT JOIN app.classes c ON c.id = r.class_id
		 WHERE r.job_id = $1::uuid AND r.outcome = 'created' AND r.secret_ciphertext IS NOT NULL
		 ORDER BY r.ordinal`, jobID)
	if err != nil {
		return nil, fmt.Errorf("user imports: read the sheet: %w", err)
	}
	defer rows.Close()
	var sheet []domain.SheetRow
	for rows.Next() {
		var (
			ordinal int
			sealed  []byte
			row     domain.SheetRow
		)
		if err := rows.Scan(&ordinal, &row.FullName, &row.Email, &row.Role, &row.Class, &sealed); err != nil {
			return nil, fmt.Errorf("user imports: read a sheet row: %w", err)
		}
		if row.Password, err = open(ordinal, sealed); err != nil {
			return nil, fmt.Errorf("user imports: open a sealed password: %w", err)
		}
		sheet = append(sheet, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("user imports: read the sheet: %w", err)
	}
	return sheet, nil
}

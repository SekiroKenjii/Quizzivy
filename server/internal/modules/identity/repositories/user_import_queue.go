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
)

const importLeaseSeconds = int64(domain.ImportLease / time.Second)

// Claim implements domain.UserImportQueue.
func (u *UserImports) Claim(ctx context.Context) (domain.ImportJob, error) {
	var (
		out   domain.ImportJob
		found bool
	)
	err := u.InTx(ctx, "user imports: claim", func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			WITH exhausted AS (
			  UPDATE app.user_import_jobs
			     SET status = 'failed', error_code = $1, completed_at = now()
			   WHERE attempts >= $2
			     AND (status = 'queued' OR (status = 'running' AND started_at < now() - $3 * interval '1 second'))
			  RETURNING id)
			UPDATE app.user_import_rows
			   SET secret_ciphertext = NULL
			 WHERE job_id IN (SELECT id FROM exhausted) AND secret_ciphertext IS NOT NULL`,
			domain.ImportFailedInterrupted, domain.ImportMaxAttempts, importLeaseSeconds); err != nil {
			return fmt.Errorf("user imports: fail the interrupted: %w", err)
		}
		claimed, err := scanImportJob(tx.QueryRow(ctx, `
			WITH due AS (
			  SELECT id FROM app.user_import_jobs
			   WHERE status = 'queued'
			      OR (status = 'running' AND started_at < now() - $1 * interval '1 second')
			   ORDER BY created_at
			   LIMIT 1
			     FOR UPDATE SKIP LOCKED)
			UPDATE app.user_import_jobs j
			   SET status = 'running', started_at = now(), attempts = j.attempts + 1
			  FROM due
			 WHERE j.id = due.id
			RETURNING `+importJobColumns, importLeaseSeconds))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("user imports: claim: %w", err)
		}
		out, found = claimed, true
		return nil
	})
	if err != nil {
		return domain.ImportJob{}, err
	}
	if !found {
		return domain.ImportJob{}, domain.ErrNoImportWork
	}
	return out, nil
}

// NextDue implements domain.UserImportQueue.
func (u *UserImports) NextDue(ctx context.Context) (time.Time, bool, error) {
	var due time.Time
	err := u.QueryRow(ctx, `
		SELECT CASE WHEN status = 'queued' THEN now() ELSE started_at + $1 * interval '1 second' END
		  FROM app.user_import_jobs
		 WHERE status IN ('queued', 'running')
		 ORDER BY created_at
		 LIMIT 1`, importLeaseSeconds).Scan(&due)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("user imports: next due: %w", err)
	}
	return due, true, nil
}

// Todo implements domain.UserImportQueue.
func (u *UserImports) Todo(ctx context.Context, jobID string, limit int) ([]domain.ImportTodo, error) {
	rows, err := u.Query(ctx, `
		SELECT r.ordinal, r.full_name, r.email, r.role_id::text, coalesce(r.class_id::text, '')
		  FROM app.user_import_rows r
		 WHERE r.job_id = $1::uuid AND r.outcome = 'pending'
		 ORDER BY r.ordinal
		 LIMIT $2`, jobID, limit)
	if err != nil {
		return nil, fmt.Errorf("user imports: read the rows to do: %w", err)
	}
	defer rows.Close()
	var todo []domain.ImportTodo
	for rows.Next() {
		var t domain.ImportTodo
		if err := rows.Scan(&t.Ordinal, &t.FullName, &t.Email, &t.RoleID, &t.ClassID); err != nil {
			return nil, fmt.Errorf("user imports: read a row to do: %w", err)
		}
		todo = append(todo, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("user imports: read the rows to do: %w", err)
	}
	return todo, nil
}

// Settle implements domain.UserImportQueue.
func (u *UserImports) Settle(ctx context.Context, batch domain.ImportBatch, issue domain.LinkIssue) (domain.ImportProgress, error) {
	if issue != nil && u.links == nil {
		return domain.ImportProgress{}, domain.ErrSetPasswordUnavailable
	}
	tx, err := u.Begin(ctx)
	if err != nil {
		return domain.ImportProgress{}, fmt.Errorf("user imports: begin settle: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var held int
	err = tx.QueryRow(ctx, `
		SELECT 1 FROM app.user_import_jobs
		 WHERE id = $1::uuid AND status = 'running' AND attempts = $2
		   FOR UPDATE`, batch.JobID, batch.Attempt).Scan(&held)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ImportProgress{}, domain.ErrImportLeaseLost
	}
	if err != nil {
		return domain.ImportProgress{}, fmt.Errorf("user imports: lock the job: %w", err)
	}

	settling := settlement{tx: tx, batch: batch, issue: issue, links: u.links, locked: map[string]bool{}, classes: map[string]bool{}}
	for _, row := range batch.Rows {
		if err := settling.row(ctx, row); err != nil {
			return domain.ImportProgress{}, err
		}
	}
	progress, err := settling.finish(ctx)
	if err != nil {
		return domain.ImportProgress{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ImportProgress{}, fmt.Errorf("user imports: commit settle: %w", err)
	}
	for _, after := range settling.after {
		after()
	}
	return progress, nil
}

type settlement struct {
	tx      pgx.Tx
	batch   domain.ImportBatch
	issue   domain.LinkIssue
	links   *SetPasswordLinks
	locked  map[string]bool
	classes map[string]bool
	created int
	skipped int
	after   []func()
}

func (s *settlement) row(ctx context.Context, row domain.ImportCreate) error {
	if !s.locked[row.RoleID] {
		if err := lockRoleAt(ctx, s.tx, row.RoleID, row.RoleRevision); err != nil {
			return err
		}
		s.locked[row.RoleID] = true
	}
	id, inserted, err := s.insert(ctx, row)
	if err != nil {
		return err
	}
	if !inserted {
		if err := s.settle(ctx, row.Ordinal, `UPDATE app.user_import_rows SET outcome = 'skipped_exists'
			 WHERE job_id = $1::uuid AND ordinal = $2 AND outcome = 'pending'`); err != nil {
			return err
		}
		s.skipped++
		return nil
	}
	placed, err := s.enrol(ctx, row, id)
	if err != nil {
		return err
	}
	if err := s.invite(ctx, id); err != nil {
		return err
	}
	if err := s.audit(ctx, row, id); err != nil {
		return err
	}
	tag, err := s.tx.Exec(ctx, `
		UPDATE app.user_import_rows
		   SET outcome = 'created', user_id = $3::uuid, secret_ciphertext = $4,
		       class_id = CASE WHEN $5::bool THEN class_id END
		 WHERE job_id = $1::uuid AND ordinal = $2 AND outcome = 'pending'`,
		s.batch.JobID, row.Ordinal, id, row.Secret, placed)
	if err != nil {
		return fmt.Errorf("user imports: settle a created row: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("user imports: a row to settle was not waiting")
	}
	s.created++
	return nil
}

func (s *settlement) settle(ctx context.Context, ordinal int, statement string) error {
	tag, err := s.tx.Exec(ctx, statement, s.batch.JobID, ordinal)
	if err != nil {
		return fmt.Errorf("user imports: settle a row: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return errors.New("user imports: a row to settle was not waiting")
	}
	return nil
}

func (s *settlement) insert(ctx context.Context, row domain.ImportCreate) (string, bool, error) {
	var hash *string
	if row.Hash != "" {
		hash = &row.Hash
	}
	var id string
	err := s.tx.QueryRow(ctx, `
		INSERT INTO app.users (email, full_name, role_id, password_hash, must_change_password, created_by)
		VALUES ($1, $2, $3::uuid, $4, $5, $6::uuid)
		ON CONFLICT ((lower(email))) DO NOTHING
		RETURNING id::text`,
		row.Email, row.FullName, row.RoleID, hash, hash != nil && !s.batch.Keep, s.batch.By).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("user imports: insert an account: %w", err)
	}
	return id, true, nil
}

func (s *settlement) enrol(ctx context.Context, row domain.ImportCreate, userID string) (bool, error) {
	if row.ClassID == "" {
		return false, nil
	}
	open, known := s.classes[row.ClassID]
	if !known {
		err := lockOpenClass(ctx, s.tx, row.ClassID)
		if err != nil && !errors.Is(err, domain.ErrClassNotFound) {
			return false, err
		}
		open = err == nil
		s.classes[row.ClassID] = open
	}
	if !open {
		return false, nil
	}
	if _, err := s.tx.Exec(ctx, `
		INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
		VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)`, row.ClassID, userID, s.batch.By); err != nil {
		return false, fmt.Errorf("user imports: enrol: %w", err)
	}
	return true, nil
}

func (s *settlement) invite(ctx context.Context, userID string) error {
	if s.issue == nil {
		return nil
	}
	writes, afterCommit := s.links.Writes(s.tx)
	s.after = append(s.after, afterCommit)
	return s.issue(ctx, writes, userID)
}

func (s *settlement) audit(ctx context.Context, row domain.ImportCreate, userID string) error {
	diff, err := json.Marshal(struct {
		Role     string `json:"role"`
		RoleID   string `json:"roleId"`
		Delivery string `json:"delivery"`
		Via      string `json:"via"`
		JobID    string `json:"jobId"`
	}{row.RoleName, row.RoleID, string(s.batch.Delivery), "import", s.batch.JobID})
	if err != nil {
		return fmt.Errorf("user imports: encode audit diff: %w", err)
	}
	return audit.Write(ctx, s.tx, audit.Entry{
		ActorUserID: &s.batch.By,
		Action:      "user.created",
		Entity:      entityUser,
		EntityID:    &userID,
		OccurredAt:  s.batch.Now,
		Diff:        diff,
	})
}

func (s *settlement) finish(ctx context.Context) (domain.ImportProgress, error) {
	progress := domain.ImportProgress{Created: s.created, Skipped: s.skipped}
	err := s.tx.QueryRow(ctx, `
		WITH moved AS (
		  SELECT created_count + $2::int AS created, skipped_count + $3::int AS skipped
		    FROM app.user_import_jobs WHERE id = $1::uuid)
		UPDATE app.user_import_jobs j
		   SET created_count = m.created,
		       skipped_count = m.skipped,
		       status = CASE WHEN m.created + m.skipped = j.total_rows THEN 'done' ELSE j.status END,
		       completed_at = CASE WHEN m.created + m.skipped = j.total_rows THEN $4::timestamptz ELSE j.completed_at END,
		       sheet_expires_at = CASE WHEN j.delivery = 'sheet' AND m.created > 0 AND m.created + m.skipped = j.total_rows
		                               THEN $5::timestamptz ELSE j.sheet_expires_at END
		  FROM moved m
		 WHERE j.id = $1::uuid
		RETURNING j.status = 'done'`,
		s.batch.JobID, s.created, s.skipped, s.batch.Now, s.batch.Now.Add(domain.SheetLifetime)).Scan(&progress.Done)
	if err != nil {
		return domain.ImportProgress{}, fmt.Errorf("user imports: move the counters: %w", err)
	}
	return progress, nil
}

// Fail implements domain.UserImportQueue.
func (u *UserImports) Fail(ctx context.Context, jobID string, attempt int, code string) error {
	return u.InTx(ctx, "user imports: fail", func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE app.user_import_jobs
			   SET status = 'failed', error_code = $3, completed_at = now()
			 WHERE id = $1::uuid AND status = 'running' AND attempts = $2`, jobID, attempt, code)
		if err != nil {
			return fmt.Errorf("user imports: fail the job: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrImportLeaseLost
		}
		if _, err := tx.Exec(ctx, `
			UPDATE app.user_import_rows SET secret_ciphertext = NULL
			 WHERE job_id = $1::uuid AND secret_ciphertext IS NOT NULL`, jobID); err != nil {
			return fmt.Errorf("user imports: erase the sealed passwords: %w", err)
		}
		return nil
	})
}

// Retry implements domain.UserImportQueue.
func (u *UserImports) Retry(ctx context.Context, jobID string, attempt int) error {
	return u.requeue(ctx, `
		UPDATE app.user_import_jobs SET status = 'queued'
		 WHERE id = $1::uuid AND status = 'running' AND attempts = $2`, jobID, attempt)
}

// Release implements domain.UserImportQueue.
func (u *UserImports) Release(ctx context.Context, jobID string, attempt int) error {
	return u.requeue(ctx, `
		UPDATE app.user_import_jobs SET status = 'queued', started_at = NULL, attempts = attempts - 1
		 WHERE id = $1::uuid AND status = 'running' AND attempts = $2`, jobID, attempt)
}

func (u *UserImports) requeue(ctx context.Context, statement, jobID string, attempt int) error {
	tag, err := u.Exec(ctx, statement, jobID, attempt)
	if err != nil {
		return fmt.Errorf("user imports: give the job back: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrImportLeaseLost
	}
	return nil
}

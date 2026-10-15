package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
)

const entityUserImport = "user_import"

// UserImports stores user-import jobs and their rows.
type UserImports struct {
	db.Repository
}

// NewUserImports returns the store over dbx.
func NewUserImports(dbx db.Context) *UserImports {
	return &UserImports{Repository: db.NewRepository(dbx)}
}

const importJobSelect = `
	SELECT j.id::text, j.created_by::text, j.request_id::text, j.filename, j.format, j.default_role_id::text,
	       j.delivery, j.status, j.total_rows, j.created_count, j.skipped_count, j.invalid_count,
	       coalesce(j.error_code, ''), j.sheet_expires_at, j.sheet_downloaded_at,
	       j.created_at, j.started_at, j.completed_at
	  FROM app.user_import_jobs j`

func scanImportJob(row pgx.Row) (domain.ImportJob, error) {
	var j domain.ImportJob
	var format, delivery, status string
	err := row.Scan(&j.ID, &j.CreatedBy, &j.RequestID, &j.Filename, &format, &j.DefaultRoleID,
		&delivery, &status, &j.Total, &j.Created, &j.Skipped, &j.Invalid,
		&j.ErrorCode, &j.SheetExpiresAt, &j.SheetDownloadedAt,
		&j.CreatedAt, &j.StartedAt, &j.CompletedAt)
	if err != nil {
		return domain.ImportJob{}, err
	}
	j.Format, j.Delivery, j.Status = domain.ImportFormat(format), domain.ImportDelivery(delivery), domain.ImportStatus(status)
	return j, nil
}

// ExistingEmails implements domain.UserImports.
func (u *UserImports) ExistingEmails(ctx context.Context, emails []string) (map[string]struct{}, error) {
	found, err := db.QueryMany(ctx, u, `
		SELECT e.address
		  FROM unnest($1::text[]) AS e(address)
		 WHERE EXISTS (SELECT 1 FROM app.users x WHERE lower(x.email) = lower(e.address))`,
		[]any{emails}, func(r pgx.Rows) (string, error) {
			var address string
			return address, r.Scan(&address)
		})
	if err != nil {
		return nil, fmt.Errorf("user imports: existing emails: %w", err)
	}
	out := make(map[string]struct{}, len(found))
	for _, address := range found {
		out[address] = struct{}{}
	}
	return out, nil
}

// ActiveClasses implements domain.UserImports.
func (u *UserImports) ActiveClasses(ctx context.Context) ([]domain.ImportClass, error) {
	classes, err := db.QueryMany(ctx, u, `
		SELECT c.id::text, c.name FROM app.classes c WHERE c.archived_at IS NULL ORDER BY c.name, c.id`,
		nil, func(r pgx.Rows) (domain.ImportClass, error) {
			var c domain.ImportClass
			return c, r.Scan(&c.ID, &c.Name)
		})
	if err != nil {
		return nil, fmt.Errorf("user imports: active classes: %w", err)
	}
	return classes, nil
}

// FindByRequest implements domain.UserImports.
func (u *UserImports) FindByRequest(ctx context.Context, createdBy, requestID string) (domain.ImportJob, bool, error) {
	return findImportByRequest(ctx, u, createdBy, requestID)
}

func findImportByRequest(ctx context.Context, q db.Querier, createdBy, requestID string) (domain.ImportJob, bool, error) {
	job, err := scanImportJob(q.QueryRow(ctx, importJobSelect+`
		 WHERE j.created_by = $1::uuid AND j.request_id = $2::uuid`, createdBy, requestID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ImportJob{}, false, nil
	}
	if err != nil {
		return domain.ImportJob{}, false, fmt.Errorf("user imports: find by request: %w", err)
	}
	return job, true, nil
}

// Find implements domain.UserImports.
func (u *UserImports) Find(ctx context.Context, createdBy, id string) (domain.ImportJob, error) {
	job, err := scanImportJob(u.QueryRow(ctx, importJobSelect+`
		 WHERE j.id = $2::uuid AND j.created_by = $1::uuid`, createdBy, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ImportJob{}, domain.ErrUserImportNotFound
	}
	if err != nil {
		return domain.ImportJob{}, fmt.Errorf("user imports: find: %w", err)
	}
	return job, nil
}

// Create implements domain.UserImports.
func (u *UserImports) Create(ctx context.Context, req domain.WriteRequest, in domain.NewImportJob) (domain.ImportJob, bool, error) {
	skipped := 0
	for _, row := range in.Rows {
		if row.Existing {
			skipped++
		}
	}
	diff, err := json.Marshal(struct {
		Format        string `json:"format"`
		Delivery      string `json:"delivery"`
		DefaultRoleID string `json:"defaultRoleId"`
		Rows          int    `json:"rows"`
		Existing      int    `json:"existing"`
		Invalid       int    `json:"invalid"`
	}{string(in.Format), string(in.Delivery), in.DefaultRoleID, len(in.Rows), skipped, in.Invalid})
	if err != nil {
		return domain.ImportJob{}, false, fmt.Errorf("user imports: encode audit diff: %w", err)
	}

	tx, err := u.Begin(ctx)
	if err != nil {
		return domain.ImportJob{}, false, fmt.Errorf("user imports: begin create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO app.user_import_jobs
		       (created_by, request_id, filename, format, default_role_id, delivery,
		        total_rows, skipped_count, invalid_count, created_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid, $6, $7, $8, $9, $10)
		ON CONFLICT (created_by, request_id) DO NOTHING
		RETURNING id::text`,
		req.ActorID, in.RequestID, in.Filename, string(in.Format), in.DefaultRoleID, string(in.Delivery),
		len(in.Rows), skipped, in.Invalid, in.Now).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return u.replay(ctx, tx, req.ActorID, in.RequestID)
	}
	if err != nil {
		return domain.ImportJob{}, false, fmt.Errorf("user imports: insert job: %w", err)
	}
	if err := insertImportRows(ctx, tx, id, in.Rows); err != nil {
		return domain.ImportJob{}, false, err
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &req.ActorID,
		Action:      "user_import.requested",
		Entity:      entityUserImport,
		EntityID:    &id,
		OccurredAt:  in.Now,
		IP:          opt.String(req.IP),
		UserAgent:   opt.String(req.UserAgent),
		Diff:        diff,
	}); err != nil {
		return domain.ImportJob{}, false, err
	}
	job, err := scanImportJob(tx.QueryRow(ctx, importJobSelect+` WHERE j.id = $1::uuid`, id))
	if err != nil {
		return domain.ImportJob{}, false, fmt.Errorf("user imports: read job: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ImportJob{}, false, fmt.Errorf("user imports: commit create: %w", err)
	}
	return job, false, nil
}

func (u *UserImports) replay(ctx context.Context, tx pgx.Tx, createdBy, requestID string) (domain.ImportJob, bool, error) {
	job, found, err := findImportByRequest(ctx, tx, createdBy, requestID)
	if err != nil {
		return domain.ImportJob{}, false, err
	}
	if !found {
		return domain.ImportJob{}, false, errors.New("user imports: the job of a repeated request vanished")
	}
	return job, true, nil
}

func insertImportRows(ctx context.Context, tx pgx.Tx, jobID string, rows []domain.ImportAccepted) error {
	ordinals := make([]int32, len(rows))
	names := make([]string, len(rows))
	emails := make([]string, len(rows))
	roles := make([]string, len(rows))
	classes := make([]string, len(rows))
	existing := make([]bool, len(rows))
	for i, r := range rows {
		ordinals[i], names[i], emails[i], roles[i], classes[i], existing[i] = int32(r.Ordinal), r.FullName, r.Email, r.RoleID, r.ClassID, r.Existing
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO app.user_import_rows (job_id, ordinal, full_name, email, role_id, class_id, outcome)
		SELECT $1::uuid, t.ordinal::smallint, t.full_name, t.email, t.role_id::uuid, nullif(t.class_id, '')::uuid,
		       CASE WHEN t.existing THEN 'skipped_exists' ELSE 'pending' END
		  FROM unnest($2::int[], $3::text[], $4::text[], $5::text[], $6::text[], $7::boolean[])
		       AS t(ordinal, full_name, email, role_id, class_id, existing)`,
		jobID, ordinals, names, emails, roles, classes, existing)
	if err != nil {
		return fmt.Errorf("user imports: insert rows: %w", err)
	}
	return nil
}

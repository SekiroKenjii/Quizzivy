package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
)

func (p *Postgres) bounded(ctx context.Context, name string, fn func(q db.Querier) error) error {
	return p.InTx(ctx, name, func(tx pgx.Tx) error {
		milliseconds := strconv.FormatInt(domain.StatementTimeout.Milliseconds(), 10)
		if _, err := tx.Exec(ctx, `SELECT set_config('statement_timeout', $1, true), set_config('jit', 'off', true)`, milliseconds); err != nil {
			return fmt.Errorf("%s: settings: %w", name, err)
		}
		return fn(tx)
	})
}

// CountExport counts the entries that match f, reading no more than
// MaxExportRows+1 of them, and reports the highest id among those it read.
func (p *Postgres) CountExport(ctx context.Context, f domain.Filter) (domain.ExportCount, error) {
	where, args := conditions(f, true)
	args = append(args, domain.MaxExportRows+1)
	sql := fmt.Sprintf(`SELECT count(*), coalesce(max(c.id), 0) FROM (SELECT a.id FROM app.audit_log a WHERE %s LIMIT $%d) c`, where, len(args))
	var out domain.ExportCount
	err := p.bounded(ctx, "audit: count export", func(q db.Querier) error {
		return q.QueryRow(ctx, sql, args...).Scan(&out.Rows, &out.Through)
	})
	if err != nil {
		return domain.ExportCount{}, err
	}
	return out, nil
}

// ExportBatch reads the next limit entries of an export, newest first, after
// the cursor, by comparing the pair (occurred_at, id) so the index gives the
// order and no OFFSET is counted.
func (p *Postgres) ExportBatch(ctx context.Context, f domain.Filter, after *domain.Cursor, limit int) ([]domain.Record, error) {
	where, args := conditions(f, true)
	if after != nil {
		args = append(args, after.OccurredAt, after.ID)
		where += fmt.Sprintf(" AND (a.occurred_at, a.id) < ($%d, $%d)", len(args)-1, len(args))
	}
	args = append(args, limit)
	sql := recordsSQL(where, fmt.Sprintf(`LIMIT $%d`, len(args)))
	var records []domain.Record
	err := p.bounded(ctx, "audit: export batch", func(q db.Querier) error {
		var err error
		records, err = db.QueryMany(ctx, q, sql, args, scanRecord)
		return err
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

// RecordExport appends audit.exported: who exported the log, from where, and
// the period, format, filters and number of entries of the file.
func (p *Postgres) RecordExport(ctx context.Context, e domain.Export) error {
	diff, err := json.Marshal(struct {
		Period   domain.Range    `json:"period"`
		Format   string          `json:"format"`
		Rows     int             `json:"rows"`
		Category domain.Category `json:"category,omitempty"`
		Query    string          `json:"q,omitempty"`
	}{Period: e.Period, Format: e.Format, Rows: e.Rows, Category: e.Category, Query: e.Query})
	if err != nil {
		return fmt.Errorf("audit: encode export diff: %w", err)
	}
	return audit.Write(ctx, p, audit.Entry{
		ActorUserID: &e.ActorID,
		Action:      "audit.exported",
		Entity:      "audit_log",
		OccurredAt:  e.At,
		IP:          opt.String(e.IP),
		UserAgent:   opt.String(e.UserAgent),
		Diff:        diff,
	})
}

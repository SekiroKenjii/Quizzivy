package repositories

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/paging"
)

const DefaultLimit = 20

// List returns one page of the imports the filter's scope created, or every
// one under scope.all, newest first; a zero scope lists none.
func (s *Postgres) List(ctx context.Context, in domain.Filter) (domain.List, error) {
	var out domain.List
	page, size, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, 100)
	out.Page = paging.Page{Number: page, Size: size}
	where := ` WHERE ($1='' OR app.immutable_unaccent(lower(i.title)) LIKE app.immutable_unaccent(lower($2)) OR EXISTS (
 SELECT 1 FROM app.word_import_source_set_items si JOIN app.word_import_sources src ON src.id=si.source_id
 WHERE si.import_id=i.id AND si.revision=i.source_revision AND src.format<>'text' AND app.immutable_unaccent(lower(src.filename)) LIKE app.immutable_unaccent(lower($2))))`
	args := []any{in.Search, "%" + db.EscapeLike(in.Search) + "%"}
	if !in.Scope.All {
		args = append(args, opt.String(in.Scope.UserID))
		where += ` AND i.created_by = $3::uuid`
	}
	err := s.InTx(ctx, "list imports", func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY`); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*),
 count(*) FILTER (WHERE i.status IN ('awaiting_sources','queued','processing','committing')),
 count(*) FILTER (WHERE i.status='needs_review'), count(*) FILTER (WHERE i.status='failed'),
 count(*) FILTER (WHERE i.status='committed'), count(*) FILTER (WHERE i.status='cancelled')
 FROM app.word_imports i`+where, args...).Scan(&out.Facets.All, &out.Facets.Processing, &out.Facets.NeedsReview, &out.Facets.Failed, &out.Facets.Committed, &out.Facets.Cancelled); err != nil {
			return err
		}
		args = append(args, in.Status)
		where += fmt.Sprintf(` AND (coalesce(cardinality($%d::text[]),0)=0 OR i.status=ANY($%d::text[]))`, len(args), len(args))
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.word_imports i`+where, args...).Scan(&out.Page.Total); err != nil {
			return err
		}
		rows, err := db.QueryMany(ctx, tx, `SELECT `+importColumns+` FROM app.word_imports i`+where+fmt.Sprintf(` ORDER BY created_at DESC,id DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2), append(args, size, offset), func(r pgx.Rows) (domain.Import, error) { return scanImport(r) })
		if err != nil {
			return err
		}
		out.Items, err = hydrateHistory(ctx, tx, rows)
		if err != nil {
			return err
		}

		return nil
	})
	return out, err
}

func hydrateHistory(ctx context.Context, q db.Querier, items []domain.Import) ([]domain.Import, error) {
	if len(items) == 0 {
		return []domain.Import{}, nil
	}
	ids := make([]string, len(items))
	positions := make(map[string]int, len(items))
	for i, item := range items {
		ids[i] = item.ID
		positions[item.ID] = i
		items[i].Sources = []domain.Source{}
	}
	sources, err := db.QueryMany(ctx, q, `SELECT `+sourceColumns+` FROM app.word_import_sources WHERE id IN (
 SELECT si.source_id FROM app.word_import_source_set_items si JOIN app.word_imports i ON i.id=si.import_id AND i.source_revision=si.revision WHERE i.id=ANY($1::uuid[])) ORDER BY role`, []any{ids}, func(r pgx.Rows) (domain.Source, error) { return scanSource(r) })
	if err != nil {
		return nil, err
	}
	for _, source := range sources {
		index := positions[source.ImportID]
		items[index].Sources = append(items[index].Sources, source)
	}
	rows, err := q.Query(ctx, `SELECT import_id::text,count(*) FROM app.word_import_sources WHERE import_id=ANY($1::uuid[]) AND NOT ready GROUP BY import_id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		items[positions[id]].PendingUploads = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := attachReviewCounts(ctx, q, items, ids, positions); err != nil {
		return nil, err
	}
	return items, attachProgress(ctx, q, items)
}

func attachReviewCounts(ctx context.Context, q db.Querier, items []domain.Import, ids []string, positions map[string]int) error {
	rows, err := q.Query(ctx, `SELECT import_id::text, open_action_count, open_confirm_count FROM app.word_import_drafts WHERE import_id=ANY($1::uuid[])`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var action, confirm *int
		if err := rows.Scan(&id, &action, &confirm); err != nil {
			return err
		}
		if action != nil && confirm != nil {
			items[positions[id]].ReviewCounts = &domain.ReviewCounts{NeedsAction: *action, ToConfirm: *confirm}
		}
	}
	return rows.Err()
}

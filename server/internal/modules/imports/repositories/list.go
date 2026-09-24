package repositories

import (
	"context"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/paging"
)

const DefaultLimit = 20

func (s *Postgres) List(ctx context.Context, in domain.Filter) (domain.List, error) {
	var out domain.List
	page, size, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, 100)
	out.Page = paging.Page{Number: page, Size: size}
	const where = ` WHERE ($1='' OR i.status=$1) AND ($2='' OR app.immutable_unaccent(lower(i.title)) LIKE app.immutable_unaccent(lower($3)) OR EXISTS (
 SELECT 1 FROM app.word_import_source_set_items si JOIN app.word_import_sources src ON src.id=si.source_id
 WHERE si.import_id=i.id AND si.revision=i.source_revision AND app.immutable_unaccent(lower(src.filename)) LIKE app.immutable_unaccent(lower($3))))`
	args := []any{in.Status, in.Search, "%" + db.EscapeLike(in.Search) + "%"}
	err := s.InTx(ctx, "list imports", func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY`); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM app.word_imports i`+where, args...).Scan(&out.Page.Total); err != nil {
			return err
		}
		rows, err := db.QueryMany(ctx, tx, `SELECT `+importColumns+` FROM app.word_imports i`+where+` ORDER BY created_at DESC,id DESC LIMIT $4 OFFSET $5`, append(args, size, offset), func(r pgx.Rows) (domain.Import, error) { return scanImport(r) })
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
	return items, rows.Err()
}

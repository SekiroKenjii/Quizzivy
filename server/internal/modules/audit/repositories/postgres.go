// Package repositories reads the audit log from PostgreSQL.
package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/platform/db"
)

// Postgres reads the audit log from PostgreSQL.
type Postgres struct{ db.Repository }

// NewPostgres reads the log through dbx.
func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

var _ domain.Repository = (*Postgres)(nil)

func recordsSQL(where, tail string) string {
	return `
	SELECT p.id, p.occurred_at, p.action, p.entity, p.entity_id::text, host(p.ip), p.user_agent, p.diff,
	       u.id::text, coalesce(nullif(u.display_name, ''), u.full_name), u.avatar_key
	  FROM (SELECT a.id, a.occurred_at, a.actor_user_id, a.action, a.entity, a.entity_id, a.ip, a.user_agent, a.diff
	          FROM app.audit_log a
	         WHERE ` + where + `
	         ORDER BY a.occurred_at DESC, a.id DESC
	         ` + tail + `) p
	  LEFT JOIN app.users u ON u.id = p.actor_user_id
	 ORDER BY p.occurred_at DESC, p.id DESC`
}

// List reads one page through audit_log_recent_idx: the entries that match f,
// newest first, ties broken by id.
func (p *Postgres) List(ctx context.Context, f domain.Filter, offset, limit int) ([]domain.Record, error) {
	where, args := conditions(f, true)
	args = append(args, limit, offset)
	sql := recordsSQL(where, fmt.Sprintf(`LIMIT $%d OFFSET $%d`, len(args)-1, len(args)))
	var records []domain.Record
	err := p.reading(ctx, "audit: list", f, func(q db.Querier) error {
		var err error
		records, err = db.QueryMany(ctx, q, sql, args, scanRecord)
		return err
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

// Facets counts the entries that match f's search and window once per entity
// and folds the counts into the categories, f's own category left out so that
// choosing a tab never empties the others.
func (p *Postgres) Facets(ctx context.Context, f domain.Filter) (domain.Facets, error) {
	where, args := conditions(f, false)
	perEntity := map[string]int{}
	err := p.reading(ctx, "audit: facets", f, func(q db.Querier) error {
		rows, err := q.Query(ctx, `SELECT a.entity, count(*) FROM app.audit_log a WHERE `+where+` GROUP BY a.entity`, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var (
				entity string
				n      int
			)
			if err := rows.Scan(&entity, &n); err != nil {
				return err
			}
			perEntity[entity] = n
		}
		return rows.Err()
	})
	if err != nil {
		return domain.Facets{}, err
	}
	return domain.FoldFacets(perEntity), nil
}

func (p *Postgres) reading(ctx context.Context, name string, f domain.Filter, fn func(q db.Querier) error) error {
	var err error
	if f.Query == "" {
		err = fn(p)
	} else {
		err = p.bounded(ctx, name, fn)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func scanRecord(rows pgx.Rows) (domain.Record, error) {
	var (
		rec       domain.Record
		actorID   *string
		actorName *string
		avatarKey *string
	)
	if err := rows.Scan(&rec.ID, &rec.OccurredAt, &rec.Action, &rec.EntityType, &rec.EntityID, &rec.IP, &rec.UserAgent, &rec.Diff,
		&actorID, &actorName, &avatarKey); err != nil {
		return domain.Record{}, err
	}
	if actorID != nil {
		rec.Actor = &domain.Actor{ID: *actorID}
		if actorName != nil {
			rec.Actor.Name = *actorName
		}
		if avatarKey != nil {
			rec.Actor.AvatarKey = *avatarKey
		}
	}
	return rec, nil
}

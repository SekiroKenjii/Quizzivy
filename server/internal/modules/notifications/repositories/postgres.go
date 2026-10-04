package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/platform/db"
)

type Postgres struct{ db.Repository }

func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

var _ domain.Repository = (*Postgres)(nil)

const listed = `
	SELECT n.id::text, n.kind, n.params, n.target, n.created_at, n.read_at
	  FROM app.notifications n
	 WHERE n.user_id = $1::uuid`

// List reads one page of the user's own notifications through
// notifications_user_recent_idx: the rows whose id sorts below q.Before,
// newest first. It asks for one row more than the page to learn whether
// older ones exist. The order names the column n.id, never the id::text the
// statement returns, or the index could not give the order.
func (p *Postgres) List(ctx context.Context, q domain.ListQuery) (domain.Page, error) {
	size := q.Size()
	sql, args := listed, []any{q.UserID}
	if q.Before != "" {
		sql += ` AND n.id < $2::uuid`
		args = append(args, q.Before)
	}
	args = append(args, size+1)
	sql += fmt.Sprintf(` ORDER BY n.id DESC LIMIT $%d`, len(args))
	items, err := db.QueryMany(ctx, p, sql, args, scanNotification)
	if err != nil {
		return domain.Page{}, fmt.Errorf("notifications: list: %w", err)
	}
	if len(items) <= size {
		return domain.Page{Items: items}, nil
	}
	return domain.Page{Items: items[:size], NextBefore: items[size-1].ID}, nil
}

func scanNotification(rows pgx.Rows) (domain.Notification, error) {
	var (
		n      domain.Notification
		kind   string
		params []byte
		target []byte
	)
	if err := rows.Scan(&n.ID, &kind, &params, &target, &n.CreatedAt, &n.ReadAt); err != nil {
		return domain.Notification{}, err
	}
	n.Kind = domain.Kind(kind)
	n.Params = json.RawMessage(params)
	if target != nil {
		n.Target = &domain.Target{}
		if err := json.Unmarshal(target, n.Target); err != nil {
			return domain.Notification{}, fmt.Errorf("notifications: target of %s: %w", n.ID, err)
		}
	}
	return n, nil
}

// MarkRead marks those of ids that are the user's own and unread. Every
// other id matches nothing, and read_at keeps its first value.
func (p *Postgres) MarkRead(ctx context.Context, userID string, ids []string) error {
	if _, err := p.Exec(ctx, `
		UPDATE app.notifications
		   SET read_at = now()
		 WHERE user_id = $1::uuid AND id = ANY($2::uuid[]) AND read_at IS NULL`, userID, ids); err != nil {
		return fmt.Errorf("notifications: mark read: %w", err)
	}
	return nil
}

// MarkAllRead marks every unread notification the user has.
func (p *Postgres) MarkAllRead(ctx context.Context, userID string) error {
	if _, err := p.Exec(ctx, `
		UPDATE app.notifications
		   SET read_at = now()
		 WHERE user_id = $1::uuid AND read_at IS NULL`, userID); err != nil {
		return fmt.Errorf("notifications: mark all read: %w", err)
	}
	return nil
}

// Unread counts the user's unread notifications through
// notifications_unread_idx, which holds nothing else.
func (p *Postgres) Unread(ctx context.Context, userID string) (int, error) {
	unread, err := db.Count(ctx, p, `
		SELECT count(*) FROM app.notifications WHERE user_id = $1::uuid AND read_at IS NULL`, userID)
	if err != nil {
		return 0, fmt.Errorf("notifications: unread: %w", err)
	}
	return unread, nil
}

// DeleteBefore deletes the notifications first written before cutoff. It is
// idempotent, so a second run or a second machine removes nothing.
func (p *Postgres) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := p.Exec(ctx, `DELETE FROM app.notifications WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("notifications: prune: %w", err)
	}
	return tag.RowsAffected(), nil
}

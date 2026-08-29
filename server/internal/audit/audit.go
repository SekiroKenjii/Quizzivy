package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Entry is one audited action (§13.4).
type Entry struct {
	// ActorUserID is nil for something the system did on nobody's behalf.
	ActorUserID *string
	// Action is `<entity>.<verb>`, past tense: "class.join_code_rotated".
	Action   string
	Entity   string
	EntityID *string

	OccurredAt time.Time
	IP         *string
	UserAgent  *string
	// Diff is optional jsonb, for actions where "what changed" is the point.
	Diff []byte
}

// Execer is satisfied by both *pgxpool.Pool and pgx.Tx.
//
// Almost every audit row belongs in the same transaction as the thing it
// records -- an audited action that committed without its audit row is
// indistinguishable from one that never happened -- so the transaction is the
// normal argument and the pool is the exception.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

var _ Execer = pgx.Tx(nil)

// Write appends one entry.
func Write(ctx context.Context, db Execer, e Entry) error {
	const q = `
		INSERT INTO app.audit_log
		       (actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	var diff any
	if len(e.Diff) > 0 {
		diff = e.Diff
	}
	if _, err := db.Exec(ctx, q,
		e.ActorUserID, e.Action, e.Entity, e.EntityID, e.OccurredAt,
		e.IP, e.UserAgent, diff); err != nil {
		return fmt.Errorf("write audit entry %s: %w", e.Action, err)
	}
	return nil
}

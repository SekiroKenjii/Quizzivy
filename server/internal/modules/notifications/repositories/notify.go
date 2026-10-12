package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/notifications/domain"
)

const upsert = `
	INSERT INTO app.notifications AS n (user_id, kind, params, target, dedupe_key)
	SELECT $1::uuid, $2::text, $3::text::jsonb, $4::text::jsonb, $5::text
	 WHERE NOT EXISTS (
	       SELECT 1 FROM app.notification_preferences p
	        WHERE p.user_id = $1::uuid AND p.event = $6::text AND NOT p.in_app)
	ON CONFLICT (user_id, dedupe_key) DO UPDATE
	   SET kind    = EXCLUDED.kind,
	       target  = EXCLUDED.target,
	       read_at = NULL,
	       params  = CASE WHEN $7::boolean
	                 THEN EXCLUDED.params || jsonb_strip_nulls(jsonb_build_object(
	                        'count',   COALESCE((n.params->>'count')::int, 0)   + (EXCLUDED.params->>'count')::int,
	                        'toGrade', COALESCE((n.params->>'toGrade')::int, 0) + (EXCLUDED.params->>'toGrade')::int))
	                 ELSE EXCLUDED.params END
	RETURNING n.id::text`

// Upsert writes the notice in one statement, so that concurrent notices for
// one user and key each add to what the others stored and none is lost. The
// insert is skipped while the user's switch for the kind's event is off; a
// kind without a switch is always written. On a conflict the stored row takes
// the notice's kind and target and becomes unread, and its params become the
// notice's under Replace, or under Add the notice's with count and toGrade
// summed with the stored ones where the notice carries them.
func (p *Postgres) Upsert(ctx context.Context, n domain.Notice) (bool, error) {
	params, err := domain.Encode(n.Params)
	if err != nil {
		return false, err
	}
	var target *string
	if n.Target != nil {
		encoded, err := json.Marshal(n.Target)
		if err != nil {
			return false, fmt.Errorf("notifications: target: %w", err)
		}
		target = new(string(encoded))
	}
	var event *string
	if governing, ok := n.Kind.Event(); ok {
		event = new(string(governing))
	}
	var id string
	err = p.QueryRow(ctx, upsert, n.UserID, string(n.Kind), string(params), target, n.DedupeKey, event, n.Merge == domain.Add).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("notifications: upsert: %w", err)
	}
	return true, nil
}

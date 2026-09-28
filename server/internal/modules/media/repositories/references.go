package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/opt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// References lists the published versions whose questions use the asset, by
// title then version. Any at all blocks deletion with a 409 (§8).
func References(ctx context.Context, q db.Querier, assetID string) ([]domain.TestRef, error) {
	byAsset, err := ReferencesFor(ctx, q, []string{assetID})
	if err != nil {
		return nil, err
	}
	return byAsset[assetID], nil
}

// ReferencesFor is References for a whole page of assets in one query, which
// is what the library list needs: a count and a name per row, without a round
// trip per row.
func ReferencesFor(ctx context.Context, q db.Querier, assetIDs []string) (map[string][]domain.TestRef, error) {
	rows, err := q.Query(ctx, `
		WITH version_assets AS (
		 SELECT media_asset_id,test_version_section_id FROM app.test_version_questions WHERE media_asset_id=ANY($1::uuid[])
		 UNION
		 SELECT a.media_asset_id,g.test_version_section_id FROM app.test_version_group_assets a
		 JOIN app.test_version_groups g ON g.id=a.group_id WHERE a.media_asset_id=ANY($1::uuid[])
		 UNION
		 SELECT r.media_asset_id,g.test_version_section_id FROM app.test_version_group_recordings r
		 JOIN app.test_version_groups g ON g.id=r.group_id WHERE r.media_asset_id=ANY($1::uuid[])
		)
		SELECT DISTINCT a.media_asset_id::text,t.id::text,t.title,tv.version
		FROM version_assets a
		JOIN app.test_version_sections tvs ON tvs.id=a.test_version_section_id
		JOIN app.test_versions tv ON tv.id=tvs.test_version_id
		JOIN app.tests t ON t.id=tv.test_id
		ORDER BY a.media_asset_id::text,t.title,tv.version,t.id::text`, assetIDs)
	if err != nil {
		return nil, fmt.Errorf("media: references: %w", err)
	}
	defer rows.Close()
	out := map[string][]domain.TestRef{}
	for rows.Next() {
		var asset string
		var ref domain.TestRef
		if err := rows.Scan(&asset, &ref.ID, &ref.Title, &ref.Version); err != nil {
			return nil, fmt.Errorf("media: references: %w", err)
		}
		out[asset] = append(out[asset], ref)
	}
	return out, rows.Err()
}

// LockForVersionUse takes the row lock that makes the delete check meaningful,
// and must be called by the publish routine before inserting a version question
// that names the asset.
func LockForVersionUse(ctx context.Context, q db.Querier, assetID string) error {
	var deleted bool
	err := q.QueryRow(ctx,
		`SELECT deleted_at IS NOT NULL FROM app.media_assets WHERE id = $1 FOR UPDATE`,
		assetID).Scan(&deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("media: lock for version use: %w", err)
	}
	if deleted {
		return domain.ErrNotFound
	}
	return nil
}

// ReachableByStudent reports whether a student may mint a signed URL for an
// asset, true only when it is bound to a question or group in a version they have an
// attempt on.
func ReachableByStudent(ctx context.Context, q db.Querier, studentID, assetID string) (bool, error) {
	var reachable bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		    FROM app.attempts a
		    JOIN app.test_version_sections s ON s.test_version_id = a.test_version_id
		    JOIN app.test_version_questions q ON q.test_version_section_id = s.id
		   WHERE a.student_id = $1
		     AND q.media_asset_id = $2
		  UNION ALL
		  SELECT 1
		    FROM app.attempts a
		    JOIN app.test_version_sections s ON s.test_version_id = a.test_version_id
		    JOIN app.test_version_groups g ON g.test_version_section_id = s.id
		    JOIN app.test_version_group_assets ga ON ga.group_id = g.id
		   WHERE a.student_id = $1 AND ga.media_asset_id = $2)`, studentID, assetID).Scan(&reachable)
	if err != nil {
		return false, fmt.Errorf("media: student reachability: %w", err)
	}
	return reachable, nil
}

const readableAssets = `
	SELECT a.id::text, a.kind::text
	  FROM app.media_assets a
	 WHERE a.id = ANY($1::uuid[])
	   AND a.deleted_at IS NULL
	   AND ($2::boolean
	    OR a.owner_id = $3::uuid
	    OR EXISTS (SELECT 1 FROM app.questions q
	                WHERE q.media_asset_id = a.id AND q.context_group_id IS NULL
	                  AND q.deleted_at IS NULL AND q.owner_id = $3::uuid)
	    OR EXISTS (SELECT 1
	                 FROM app.question_groups g
	                 LEFT JOIN app.test_sections s ON s.id = g.owner_section_id
	                 LEFT JOIN app.tests t ON t.id = s.test_id AND t.deleted_at IS NULL
	                WHERE g.id IN (SELECT group_id FROM app.group_stimulus_assets WHERE media_asset_id = a.id
	                               UNION ALL
	                               SELECT group_id FROM app.group_recordings WHERE media_asset_id = a.id
	                               UNION ALL
	                               SELECT context_group_id FROM app.questions
	                                WHERE media_asset_id = a.id AND context_group_id IS NOT NULL)
	                  AND CASE WHEN g.owner_section_id IS NULL THEN g.owner_id ELSE t.owner_id END = $3::uuid)
	    OR EXISTS (SELECT 1
	                 FROM app.test_version_sections vs
	                 JOIN app.test_versions tv ON tv.id = vs.test_version_id
	                 JOIN app.tests t ON t.id = tv.test_id
	                WHERE vs.id IN (SELECT test_version_section_id FROM app.test_version_questions WHERE media_asset_id = a.id
	                                UNION ALL
	                                SELECT vg.test_version_section_id FROM app.test_version_group_assets va
	                                  JOIN app.test_version_groups vg ON vg.id = va.group_id
	                                 WHERE va.media_asset_id = a.id
	                                UNION ALL
	                                SELECT vg.test_version_section_id FROM app.test_version_group_recordings vr
	                                  JOIN app.test_version_groups vg ON vg.id = vr.group_id
	                                 WHERE vr.media_asset_id = a.id)
	                  AND t.deleted_at IS NULL AND t.owner_id = $3::uuid))`

// Readable is the one definition of which assets a staff scope may read, and
// so bind or sign: its own, every asset under scope.all, and any asset already
// used by one of its live bank questions, by a group it owns (a section group
// through its live test), or by a published version of one of its live tests.
// It returns the kind of each readable id, keyed by the id's canonical text; an
// id it leaves out is missing, deleted or unreadable, and callers must answer
// the three alike. A zero scope reads nothing. It takes no lock.
func Readable(ctx context.Context, q db.Querier, scope access.Scope, assetIDs []string) (map[string]domain.Kind, error) {
	rows, err := q.Query(ctx, readableAssets, assetIDs, scope.All, opt.String(scope.UserID))
	if err != nil {
		return nil, fmt.Errorf("media: readable assets: %w", err)
	}
	defer rows.Close()
	out := make(map[string]domain.Kind, len(assetIDs))
	for rows.Next() {
		var id, kind string
		if err := rows.Scan(&id, &kind); err != nil {
			return nil, fmt.Errorf("media: scan readable asset: %w", err)
		}
		out[id] = domain.Kind(kind)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("media: readable assets: %w", err)
	}
	return out, nil
}

// RequireReadable is Readable for a binding write inside its transaction: it
// answers ErrNotFound unless the scope may read every one of assetIDs, so an
// unreadable asset is refused exactly as a missing or deleted one is. Callers
// run it after LockForVersionUse has locked each id and before writing any row
// that names one, so a write never authorizes itself.
func RequireReadable(ctx context.Context, q db.Querier, scope access.Scope, assetIDs []string) error {
	readable, err := Readable(ctx, q, scope, assetIDs)
	if err != nil {
		return err
	}
	for _, id := range assetIDs {
		if _, ok := readable[strings.ToLower(id)]; !ok {
			return domain.ErrNotFound
		}
	}
	return nil
}

func (s *Postgres) ReferencesFor(ctx context.Context, assetIDs []string) (map[string][]domain.TestRef, error) {
	return ReferencesFor(ctx, s.Conn(), assetIDs)
}

func (s *Postgres) ReachableByStudent(ctx context.Context, studentID, assetID string) (bool, error) {
	return ReachableByStudent(ctx, s.Conn(), studentID, assetID)
}

func (s *Postgres) LockForVersionUse(ctx context.Context, tx pgx.Tx, assetID string) error {
	return LockForVersionUse(ctx, tx, assetID)
}

func (s *Postgres) Readable(ctx context.Context, scope access.Scope, assetIDs []string) (map[string]domain.Kind, error) {
	return Readable(ctx, s.Conn(), scope, assetIDs)
}

func (s *Postgres) RequireReadable(ctx context.Context, tx pgx.Tx, scope access.Scope, assetIDs []string) error {
	return RequireReadable(ctx, tx, scope, assetIDs)
}

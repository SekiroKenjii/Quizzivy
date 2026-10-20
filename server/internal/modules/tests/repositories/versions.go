package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/opt"

	"github.com/jackc/pgx/v5"
)

const versionColumns = `v.id::text,
		       v.version,
		       v.total_points::text,
		       (SELECT count(*)
		          FROM app.test_version_sections vs
		          JOIN app.test_version_questions vq
		            ON vq.test_version_section_id = vs.id
		         WHERE vs.test_version_id = v.id),
		       (SELECT count(*)
		          FROM app.test_version_sections vs
		          JOIN app.test_version_questions vq
		            ON vq.test_version_section_id = vs.id
		         WHERE vs.test_version_id = v.id
		           AND (vq.media_asset_kind = 'audio' OR EXISTS (
		             SELECT 1 FROM app.test_version_group_members gm
		             JOIN app.test_version_group_recordings gr ON gr.group_id=gm.group_id
		             WHERE gm.question_id=vq.id))),
		       (SELECT count(*)
		          FROM app.test_version_sections vs
		          JOIN app.test_version_questions vq
		            ON vq.test_version_section_id = vs.id
		         WHERE vs.test_version_id = v.id
		           AND vq.type = 'short_answer'),
		       (SELECT coalesce(array_agg(DISTINCT vq.skill ORDER BY vq.skill) FILTER (WHERE vq.skill IS NOT NULL), '{}'::text[])
		          FROM app.test_version_sections vs
		          JOIN app.test_version_questions vq
		            ON vq.test_version_section_id = vs.id
		         WHERE vs.test_version_id = v.id),
		       v.published_at,
		       u.full_name,
		       ` + versionAssignmentCount + `,
		       v.change_note
		  FROM app.test_versions v
		  JOIN app.users u ON u.id = v.published_by`

func scanVersion(row pgx.Row) (domain.Version, error) {
	var v domain.Version
	err := row.Scan(&v.ID, &v.Version, &v.TotalPoints, &v.QuestionCount, &v.AudioCount, &v.ManualCount, &v.Skills,
		&v.PublishedAt, &v.PublishedBy, &v.AssignmentCount, &v.ChangeNote)
	return v, err
}

// ListVersions returns the test's publish history, newest first. A test
// that was never published has none; a test outside scope is
// domain.ErrNotFound, as an unknown id is.
func (s *Postgres) ListVersions(ctx context.Context, scope access.Scope, testID string) ([]domain.Version, error) {
	rows, err := s.Query(ctx, `SELECT `+versionColumns+` JOIN app.tests vt ON vt.id = v.test_id
		WHERE v.test_id = $1 AND vt.deleted_at IS NULL AND ($2::boolean OR vt.owner_id = $3::uuid)
		ORDER BY v.version DESC`,
		testID, scope.All, opt.String(scope.UserID))
	if err != nil {
		return nil, fmt.Errorf("tests: list versions: %w", err)
	}
	defer rows.Close()

	var out []domain.Version
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, fmt.Errorf("tests: scan version: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tests: list versions: %w", err)
	}
	if len(out) == 0 {
		if err := requireVisibleTest(ctx, s, scope, testID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func requireVisibleTest(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, scope access.Scope, testID string) error {
	var visible bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM app.tests
		WHERE id = $1 AND deleted_at IS NULL AND `+scopedTest+`)`,
		testID, scope.All, opt.String(scope.UserID)).Scan(&visible)
	if err != nil {
		return fmt.Errorf("tests: read test: %w", err)
	}
	if !visible {
		return domain.ErrNotFound
	}
	return nil
}

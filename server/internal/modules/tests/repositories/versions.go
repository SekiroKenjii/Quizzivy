package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
)

// ListVersions returns the test's publish history, newest first.
func (s *Postgres) ListVersions(ctx context.Context, testID string) ([]domain.Version, error) {
	rows, err := s.Query(ctx, `
		SELECT v.id::text,
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
		           AND vq.media_asset_kind = 'audio'),
		       (SELECT count(*)
		          FROM app.test_version_sections vs
		          JOIN app.test_version_questions vq
		            ON vq.test_version_section_id = vs.id
		         WHERE vs.test_version_id = v.id
		           AND vq.type = 'short_answer'),
		       v.published_at,
		       u.full_name
		  FROM app.test_versions v
		  JOIN app.users u ON u.id = v.published_by
		 WHERE v.test_id = $1
		 ORDER BY v.version DESC`, testID)
	if err != nil {
		return nil, fmt.Errorf("tests: list versions: %w", err)
	}
	defer rows.Close()

	var out []domain.Version
	for rows.Next() {
		var v domain.Version
		if err := rows.Scan(&v.ID, &v.Version, &v.TotalPoints, &v.QuestionCount, &v.AudioCount, &v.ManualCount,
			&v.PublishedAt, &v.PublishedBy); err != nil {
			return nil, fmt.Errorf("tests: scan version: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tests: list versions: %w", err)
	}
	return out, nil
}

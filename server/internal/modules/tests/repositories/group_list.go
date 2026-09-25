package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/paging"
	"strings"
)

// List returns bank groups with bounded summaries, filtered independently from section-owned copies.
func (s *GroupsPostgres) List(ctx context.Context, in domain.GroupListInput) ([]domain.GroupSummary, paging.Page, error) {
	number, limit, offset := paging.Clamp(in.Page, in.Limit, DefaultLimit, MaxLimit)
	conditions := []string{"g.owner_section_id IS NULL"}
	args := []any{}
	switch in.Status {
	case "all":
	case "archived":
		conditions = append(conditions, "g.archived_at IS NOT NULL")
	default:
		conditions = append(conditions, "g.archived_at IS NULL")
	}
	if term := strings.TrimSpace(in.Query); term != "" {
		args = append(args, db.EscapeLike(term))
		conditions = append(conditions, fmt.Sprintf(`(app.immutable_unaccent(lower(g.title)) LIKE '%%'||app.immutable_unaccent(lower($%[1]d))||'%%' ESCAPE '\'
   OR EXISTS (SELECT 1 FROM app.questions q WHERE q.context_group_id=g.id
    AND app.immutable_unaccent(lower(q.prompt)) LIKE '%%'||app.immutable_unaccent(lower($%[1]d))||'%%' ESCAPE '\'))`, len(args)))
	}
	if in.Tag != "" {
		args = append(args, in.Tag)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (SELECT 1 FROM app.questions q WHERE q.context_group_id=g.id AND $%d=ANY(q.tags))`, len(args)))
	}
	from := ` FROM app.question_groups g WHERE ` + strings.Join(conditions, " AND ")
	page := paging.Page{Number: number, Size: limit}
	if err := s.QueryRow(ctx, `SELECT count(*)`+from, args...).Scan(&page.Total); err != nil {
		return nil, page, err
	}
	args = append(args, limit, offset)
	rows, err := s.Query(ctx, `SELECT g.id::text,g.title,g.revision,g.archived_at,g.updated_at,
  (SELECT count(*) FROM app.questions q WHERE q.context_group_id=g.id),
  (SELECT count(*) FROM app.group_recordings r WHERE r.group_id=g.id),
  coalesce((SELECT sum(q.points) FROM app.questions q WHERE q.context_group_id=g.id),0)::text,
  ARRAY(SELECT DISTINCT unnest(q.tags) AS tag FROM app.questions q WHERE q.context_group_id=g.id ORDER BY tag)`+from+fmt.Sprintf(` ORDER BY g.updated_at DESC,g.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, page, err
	}
	defer rows.Close()
	items := []domain.GroupSummary{}
	for rows.Next() {
		var item domain.GroupSummary
		if err := rows.Scan(&item.ID, &item.Title, &item.Revision, &item.ArchivedAt, &item.UpdatedAt, &item.QuestionCount, &item.RecordingCount, &item.TotalPoints, &item.Tags); err != nil {
			return nil, page, err
		}
		items = append(items, item)
	}
	return items, page, rows.Err()
}

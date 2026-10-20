package repositories

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/platform/db"
)

const userName = `coalesce(nullif(u.display_name, ''), u.full_name)`

type labelSource struct {
	entities []string
	rows     string
}

var labelSources = []labelSource{
	{
		entities: []string{"user", "users", "student", "user_identity", "ai_teacher_credits"},
		rows:     `SELECT u.id, ` + userName + ` AS label FROM app.users u`,
	},
	{
		entities: []string{"class", "class_member"},
		rows:     `SELECT c.id, c.name AS label FROM app.classes c`,
	},
	{
		entities: []string{"class_join_code"},
		rows:     `SELECT j.id, c.name AS label FROM app.class_join_codes j JOIN app.classes c ON c.id = j.class_id`,
	},
	{
		entities: []string{"test"},
		rows:     `SELECT t.id, t.title AS label FROM app.tests t`,
	},
	{
		entities: []string{"test_version"},
		rows:     `SELECT v.id, t.title AS label FROM app.test_versions v JOIN app.tests t ON t.id = v.test_id`,
	},
	{
		entities: []string{"assignment"},
		rows:     `SELECT n.id, t.title AS label FROM app.assignments n JOIN app.tests t ON t.id = n.test_id`,
	},
	{
		entities: []string{"attempt"},
		rows:     `SELECT ap.id, ` + userName + ` AS label FROM app.attempts ap JOIN app.users u ON u.id = ap.student_id`,
	},
	{
		entities: []string{"media_asset"},
		rows:     `SELECT m.id, coalesce(nullif(m.display_name, ''), m.original_filename) AS label FROM app.media_assets m`,
	},
	{
		entities: []string{"word_import"},
		rows:     `SELECT w.id, w.title AS label FROM app.word_imports w`,
	},
	{
		entities: []string{"question_group"},
		rows:     `SELECT g.id, g.title AS label FROM app.question_groups g`,
	},
	{
		entities: []string{"ai_credit_requests"},
		rows:     `SELECT r.id, ` + userName + ` AS label FROM app.ai_credit_requests r JOIN app.users u ON u.id = r.user_id`,
	},
}

// Labels names the rows refs point at, one statement for each kind of row that
// is asked about. A row that is gone, or whose name is empty, is left out.
func (p *Postgres) Labels(ctx context.Context, refs []domain.EntityRef) (map[domain.EntityRef]string, error) {
	out := make(map[domain.EntityRef]string, len(refs))
	for _, source := range labelSources {
		wanted := wantedBy(source, refs)
		if len(wanted) == 0 {
			continue
		}
		ids := make([]string, 0, len(wanted))
		for id := range wanted {
			ids = append(ids, id)
		}
		rows, err := db.QueryMany(ctx, p,
			`SELECT l.id::text, l.label FROM (`+source.rows+`) l WHERE l.id = ANY($1::uuid[]) AND nullif(l.label, '') IS NOT NULL`,
			[]any{ids}, scanLabel)
		if err != nil {
			return nil, fmt.Errorf("audit: labels of %s: %w", source.entities[0], err)
		}
		for _, row := range rows {
			for _, entity := range wanted[row.id] {
				out[domain.EntityRef{Type: entity, ID: row.id}] = row.label
			}
		}
	}
	return out, nil
}

type labelRow struct{ id, label string }

func scanLabel(rows pgx.Rows) (labelRow, error) {
	var row labelRow
	return row, rows.Scan(&row.id, &row.label)
}

func wantedBy(source labelSource, refs []domain.EntityRef) map[string][]string {
	own := make(map[string]bool, len(source.entities))
	for _, entity := range source.entities {
		own[entity] = true
	}
	wanted := map[string][]string{}
	for _, ref := range refs {
		if own[ref.Type] {
			wanted[ref.ID] = append(wanted[ref.ID], ref.Type)
		}
	}
	return wanted
}

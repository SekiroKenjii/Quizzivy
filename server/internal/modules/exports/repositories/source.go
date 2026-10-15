package repositories

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/exports/domain"
)

const statementTimeout = 20 * time.Minute

// Read runs fn over one repeatable-read, read-only snapshot.
func (p *Postgres) Read(ctx context.Context, fn func(domain.Snapshot) error) error {
	return p.InTx(ctx, "exports: read", func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY`); err != nil {
			return fmt.Errorf("exports: open the snapshot: %w", err)
		}
		if _, err := tx.Exec(ctx, `SET LOCAL statement_timeout = `+strconv.FormatInt(statementTimeout.Milliseconds(), 10)); err != nil {
			return fmt.Errorf("exports: bound the snapshot: %w", err)
		}
		return fn(snapshot{tx: tx})
	})
}

type snapshot struct{ tx pgx.Tx }

func (s snapshot) Rows(ctx context.Context, file domain.File, includeAnswers bool, emit func([]string) error) error {
	sql, ok := queries[file.Name]
	if !ok || (file.Name == domain.AnswersFile && !includeAnswers) {
		return fmt.Errorf("exports: no query for %s", file.Name)
	}
	rows, err := s.tx.Query(ctx, sql)
	if err != nil {
		return fmt.Errorf("exports: read %s: %w", file.Name, err)
	}
	defer rows.Close()
	header := file.Header()
	if err := sameColumns(file.Name, rows, header); err != nil {
		return err
	}
	values := make([]*string, len(header))
	targets := make([]any, len(header))
	for i := range values {
		targets[i] = &values[i]
	}
	for rows.Next() {
		if err := rows.Scan(targets...); err != nil {
			return fmt.Errorf("exports: scan %s: %w", file.Name, err)
		}
		if err := emit(cells(values)); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("exports: read %s: %w", file.Name, err)
	}
	return nil
}

func sameColumns(name string, rows pgx.Rows, header []string) error {
	descriptions := rows.FieldDescriptions()
	got := make([]string, len(descriptions))
	for i, d := range descriptions {
		got[i] = d.Name
	}
	if !slices.Equal(got, header) {
		return fmt.Errorf("exports: the query of %s returns the columns %v, not %v", name, got, header)
	}
	return nil
}

func cells(values []*string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		if v != nil {
			out[i] = *v
		}
	}
	return out
}

func at(column string) string {
	return `to_char(` + column + ` AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')`
}

var queries = map[string]string{
	domain.UsersFile: `
		SELECT u.id::text AS id, u.email AS email, u.full_name AS full_name, u.display_name AS display_name,
		       u.phone AS phone, u.role_id::text AS role_id, r.name AS role_name,
		       ` + at("u.disabled_at") + ` AS disabled_at, u.must_change_password::text AS must_change_password,
		       u.locale AS locale, u.time_zone AS time_zone, u.created_by::text AS created_by,
		       ` + at("u.created_at") + ` AS created_at, ` + at("u.last_sign_in_at") + ` AS last_sign_in_at,
		       u.last_sign_in_method AS last_sign_in_method, ` + at("u.last_active_at") + ` AS last_active_at,
		       ` + at("u.password_changed_at") + ` AS password_changed_at
		  FROM app.users u
		  JOIN app.roles r ON r.id = u.role_id
		 ORDER BY u.id`,
	domain.RolesFile: `
		SELECT r.id::text AS id, r.builtin_key AS builtin_key, r.name AS name, r.description AS description,
		       coalesce((SELECT string_agg(rp.permission_key, ';' ORDER BY rp.permission_key)
		                   FROM app.role_permissions rp WHERE rp.role_id = r.id), '') AS permissions,
		       ` + at("r.created_at") + ` AS created_at, ` + at("r.updated_at") + ` AS updated_at
		  FROM app.roles r
		 ORDER BY r.id`,
	domain.ClassesFile: `
		SELECT c.id::text AS id, c.name AS name, c.description AS description, c.teacher_id::text AS teacher_id,
		       t.full_name AS teacher_name, c.self_join_enabled::text AS self_join_enabled,
		       c.schedule_label AS schedule_label, c.room AS room,
		       ` + at("c.created_at") + ` AS created_at, ` + at("c.updated_at") + ` AS updated_at,
		       ` + at("c.archived_at") + ` AS archived_at
		  FROM app.classes c
		  LEFT JOIN app.users t ON t.id = c.teacher_id
		 ORDER BY c.id`,
	domain.ClassMembersFile: `
		SELECT m.class_id::text AS class_id, m.user_id::text AS user_id, m.joined_via::text AS joined_via,
		       ` + at("m.joined_at") + ` AS joined_at, m.added_by::text AS added_by
		  FROM app.class_members m
		 ORDER BY m.class_id, m.user_id`,
	domain.TestsFile: `
		SELECT t.id::text AS id, t.title AS title, t.description AS description, t.status::text AS status,
		       t.current_version::text AS current_version, t.last_published_version::text AS last_published_version,
		       t.owner_id::text AS owner_id, t.created_by::text AS created_by,
		       ` + at("t.created_at") + ` AS created_at, ` + at("t.updated_at") + ` AS updated_at,
		       ` + at("t.deleted_at") + ` AS deleted_at
		  FROM app.tests t
		 ORDER BY t.id`,
	domain.TestVersionsFile: `
		SELECT v.id::text AS id, v.test_id::text AS test_id, v.version::text AS version,
		       v.total_points::text AS total_points, ` + at("v.published_at") + ` AS published_at,
		       v.published_by::text AS published_by, v.delivery_version AS delivery_version,
		       v.change_note AS change_note
		  FROM app.test_versions v
		 ORDER BY v.id`,
	domain.AssignmentsFile: `
		SELECT a.id::text AS id, a.test_id::text AS test_id, a.test_version_id::text AS test_version_id,
		       coalesce((SELECT string_agg(ac.class_id::text, ';' ORDER BY ac.class_id)
		                   FROM app.assignment_classes ac WHERE ac.assignment_id = a.id), '') AS class_ids,
		       coalesce((SELECT string_agg(s.user_id::text, ';' ORDER BY s.user_id)
		                   FROM app.assignment_students s WHERE s.assignment_id = a.id), '') AS student_ids,
		       ` + at("a.opens_at") + ` AS opens_at, ` + at("a.closes_at") + ` AS closes_at,
		       ` + at("a.closed_at") + ` AS closed_at, a.duration_minutes::text AS duration_minutes,
		       a.max_attempts::text AS max_attempts, a.shuffle_questions::text AS shuffle_questions,
		       a.shuffle_options::text AS shuffle_options, a.review_release AS review_release,
		       a.review_show_score::text AS review_show_score,
		       a.review_show_correct_answers::text AS review_show_correct_answers,
		       a.review_show_explanations::text AS review_show_explanations,
		       a.review_show_class_average::text AS review_show_class_average,
		       a.integrity_require_fullscreen::text AS integrity_require_fullscreen,
		       a.integrity_block_copy_paste::text AS integrity_block_copy_paste,
		       a.integrity_max_focus_loss::text AS integrity_max_focus_loss,
		       a.integrity_on_limit_exceeded::text AS integrity_on_limit_exceeded,
		       a.integrity_min_away_ms::text AS integrity_min_away_ms, a.student_note AS student_note,
		       a.created_by::text AS created_by, ` + at("a.created_at") + ` AS created_at,
		       ` + at("a.updated_at") + ` AS updated_at, ` + at("a.published_at") + ` AS published_at
		  FROM app.assignments a
		 ORDER BY a.id`,
	domain.AttemptsFile: `
		SELECT t.id::text AS id, t.assignment_id::text AS assignment_id, t.test_version_id::text AS test_version_id,
		       t.student_id::text AS student_id, t.attempt_no::text AS attempt_no, t.status::text AS status,
		       ` + at("t.started_at") + ` AS started_at, ` + at("t.deadline_at") + ` AS deadline_at,
		       ` + at("t.submitted_at") + ` AS submitted_at, ` + at("t.graded_at") + ` AS graded_at,
		       t.score_earned::text AS score_earned, t.score_total::text AS score_total,
		       t.focus_loss_count::text AS focus_loss_count, t.flagged::text AS flagged,
		       t.void_reason AS void_reason, t.teacher_note AS teacher_note
		  FROM app.attempts t
		 ORDER BY t.id`,
	domain.AnswersFile: `
		SELECT x.attempt_id::text AS attempt_id, x.question_id::text AS question_id, x.payload::text AS answer,
		       (x.payload = '{"removed": true}'::jsonb)::text AS removed,
		       x.requires_manual::text AS requires_manual, x.auto_score::text AS auto_score,
		       x.manual_score::text AS manual_score, x.final_score::text AS final_score,
		       x.grader_comment AS grader_comment, x.graded_by::text AS graded_by,
		       ` + at("x.graded_at") + ` AS graded_at, ` + at("x.updated_at") + ` AS updated_at
		  FROM app.attempt_answers x
		 ORDER BY x.attempt_id, x.question_id`,
}

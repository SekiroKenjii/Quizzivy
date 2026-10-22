//go:build integration

package repositories_test

import (
	"context"
	"testing"

	"quizzivy/internal/modules/exports/domain"
)

func TestAnEssaysBandAndCriteriaAreExportedWithItsAnswer(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	teacher, student := w.admin(), w.user("student", "Học viên "+nonce(t))
	var testID, version, section, essay, assignment, attempt string
	criteria := make([]string, 2)
	scan := func(sql string, dest *string, args ...any) {
		t.Helper()
		if err := w.pool.QueryRow(ctx, sql, args...).Scan(dest); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	scan(`INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Bài luận', 'published', 1, $1::uuid, $1::uuid) RETURNING id::text`, &testID, teacher)
	t.Cleanup(func() {
		for _, q := range []string{
			`DELETE FROM app.attempt_answers WHERE attempt_id IN (SELECT id FROM app.attempts WHERE assignment_id IN (SELECT id FROM app.assignments WHERE test_id = $1::uuid))`,
			`DELETE FROM app.attempts WHERE assignment_id IN (SELECT id FROM app.assignments WHERE test_id = $1::uuid)`,
			`DELETE FROM app.assignments WHERE test_id = $1::uuid`,
			`DELETE FROM app.test_versions WHERE test_id = $1::uuid`,
			`DELETE FROM app.tests WHERE id = $1::uuid`,
		} {
			if _, err := w.pool.Exec(context.Background(), q, testID); err != nil {
				t.Errorf("%s: %v", q, err)
			}
		}
	})
	scan(`INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1::uuid, 1, '9.00', $2::uuid) RETURNING id::text`, &version, testID, teacher)
	scan(`INSERT INTO app.test_version_sections (test_version_id, ordinal, title) VALUES ($1::uuid, 0, 'Viết') RETURNING id::text`, &section, version)
	scan(`INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, points) VALUES ($1::uuid, 0, 'essay', 'Viết', '9.00') RETURNING id::text`, &essay, section)
	for i := range criteria {
		scan(`INSERT INTO app.test_version_rubric_criteria (test_version_question_id, ordinal, label) VALUES ($1::uuid, $2, '=Tiêu chí') RETURNING id::text`, &criteria[i], essay, i)
	}
	scan(`INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, max_attempts, created_by, published_at)
		VALUES ($1::uuid, $2::uuid, now() - interval '2 hours', now() + interval '2 hours', 45, 1, $3::uuid, now()) RETURNING id::text`, &assignment, testID, version, teacher)
	scan(`INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, status, session_id, shuffle_seed,
		  beacon_token_hash, started_at, deadline_at, submitted_at, score_earned, score_total)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, 'submitted', gen_random_uuid(), 1, sha256('b'::bytea),
		        now() - interval '40 minutes', now() + interval '5 minutes', now(), 6.50, 9.00) RETURNING id::text`, &attempt, assignment, version, student)
	w.exec(`INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual, manual_score, band, graded_by, graded_at)
		VALUES ($1::uuid, $2::uuid, '{"type":"text","value":"An essay."}', true, 6.50, 6.5, $3::uuid, now())`, attempt, essay, teacher)
	w.exec(`INSERT INTO app.attempt_answer_criteria (attempt_id, question_id, criterion_id, band)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 7), ($1::uuid, $2::uuid, $4::uuid, 6)`, attempt, essay, criteria[1], criteria[0])

	row := w.read(true, nil).rowWith(domain.AnswersFile, column(t, domain.AnswersFile, "attempt_id"), attempt)
	if row == nil {
		t.Fatal("the essay's answer was not exported")
	}
	if got := row[column(t, domain.AnswersFile, "band")]; got != "6.5" {
		t.Errorf("band %q, want 6.5", got)
	}
	if got, want := row[column(t, domain.AnswersFile, "criteria")], criteria[0]+"=6.0;"+criteria[1]+"=7.0"; got != want {
		t.Errorf("criteria %q, want %q in rubric order", got, want)
	}
}

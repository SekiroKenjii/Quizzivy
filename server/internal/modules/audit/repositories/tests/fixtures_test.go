//go:build integration

package repositories_test

import (
	"fmt"
)

type fixture struct {
	entity string
	id     func(label string) string
}

type fixtures struct {
	w     *world
	owner string
}

func newFixtures(w *world) fixtures {
	return fixtures{w: w, owner: w.user("teacher", "Chủ sở hữu "+nonce(w.t), "")}
}

func (f fixtures) test(title string) string {
	return f.w.insertID(`INSERT INTO app.tests (title, created_by, owner_id) VALUES ($1, $2::uuid, $2::uuid) RETURNING id::text`,
		`DELETE FROM app.tests WHERE id = $1::uuid`, title, f.owner)
}

func (f fixtures) version(testID string) string {
	return f.w.insertID(`INSERT INTO app.test_versions (test_id, version, total_points, published_by) VALUES ($1::uuid, 1, 10, $2::uuid) RETURNING id::text`,
		`DELETE FROM app.test_versions WHERE id = $1::uuid`, testID, f.owner)
}

func (f fixtures) assignment(testID, versionID string) string {
	return f.w.insertID(`
		INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by)
		VALUES ($1::uuid, $2::uuid, now(), now() + interval '1 day', 30, $3::uuid) RETURNING id::text`,
		`DELETE FROM app.assignments WHERE id = $1::uuid`, testID, versionID, f.owner)
}

func (f fixtures) attempt(assignmentID, versionID, studentID string) string {
	return f.w.insertID(`
		INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, session_id, shuffle_seed, beacon_token_hash, deadline_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, uuidv7(), 1, sha256('beacon'::bytea), now() + interval '1 hour') RETURNING id::text`,
		`DELETE FROM app.attempts WHERE id = $1::uuid`, assignmentID, versionID, studentID)
}

func (f fixtures) media(displayName any, filename string) string {
	return f.w.insertID(`
		INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, original_filename, display_name, checksum_sha256, uploaded_by, owner_id)
		VALUES ('image', 'audit-test/' || gen_random_uuid(), 'image/png', 1, $1, $2, sha256('media'::bytea), $3::uuid, $3::uuid) RETURNING id::text`,
		`DELETE FROM app.media_assets WHERE id = $1::uuid`, filename, displayName, f.owner)
}

func (f fixtures) all() map[string]fixture {
	w := f.w
	return map[string]fixture{
		"user":    {"user", func(label string) string { return w.user("teacher", label, "") }},
		"student": {"student", func(label string) string { return w.user("student", label, "") }},
		"credits": {"ai_teacher_credits", func(label string) string { return w.user("teacher", label, "") }},
		"class":   {"class", func(label string) string { return w.class(label, f.owner) }},
		"member":  {"class_member", func(label string) string { return w.class(label, f.owner) }},
		"code": {"class_join_code", func(label string) string {
			class := w.class(label, f.owner)
			return w.insertID(`
				INSERT INTO app.class_join_codes (class_id, code_hash, code_hint, expires_at, created_by)
				VALUES ($1::uuid, sha256($2::bytea), 'AB12', now() + interval '1 day', $3::uuid) RETURNING id::text`,
				`DELETE FROM app.class_join_codes WHERE id = $1::uuid`, class, label, f.owner)
		}},
		"test": {"test", f.test},
		"version": {"test_version", func(label string) string {
			return f.version(f.test(label))
		}},
		"assignment": {"assignment", func(label string) string {
			testID := f.test(label)
			return f.assignment(testID, f.version(testID))
		}},
		"attempt": {"attempt", func(label string) string {
			testID := f.test("Đề của " + label)
			versionID := f.version(testID)
			return f.attempt(f.assignment(testID, versionID), versionID, w.user("student", label, ""))
		}},
		"media":    {"media_asset", func(label string) string { return f.media(label, "ten-tep.png") }},
		"filename": {"media_asset", func(label string) string { return f.media(nil, label) }},
		"import": {"word_import", func(label string) string {
			return w.insertID(`INSERT INTO app.word_imports (created_by, request_id, title) VALUES ($1::uuid, uuidv7(), $2) RETURNING id::text`,
				`DELETE FROM app.word_imports WHERE id = $1::uuid`, f.owner, label)
		}},
		"group": {"question_group", func(label string) string {
			return w.insertID(`INSERT INTO app.question_groups (title, created_by, owner_id) VALUES ($1, $2::uuid, $2::uuid) RETURNING id::text`,
				`DELETE FROM app.question_groups WHERE id = $1::uuid`, label, f.owner)
		}},
		"request": {"ai_credit_requests", func(label string) string {
			user := w.user("teacher", label, "")
			return w.insertID(`
				INSERT INTO app.ai_credit_requests (user_id, month, amount_credits, reason) VALUES ($1::uuid, '2001-03-01', 1000, 'cần thêm') RETURNING id::text`,
				`DELETE FROM app.ai_credit_requests WHERE id = $1::uuid`, user)
		}},
	}
}

func describe(name, entity string) string { return fmt.Sprintf("%s (%s)", name, entity) }

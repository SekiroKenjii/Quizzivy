//go:build integration

package db_test

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
	"golang.org/x/text/unicode/norm"

	"quizzivy/internal/platform/db"
)

const composeMigration = 103

type composedColumn struct {
	table, column, id string
	typed             string
}

type composeWorld struct {
	conn     *sql.DB
	composed []composedColumn
	kept     []composedColumn
	stamped  map[string]string
}

func nfd(s string) string { return norm.NFD.String(s) }

func uniqueDatabase(t *testing.T) *sql.DB {
	t.Helper()
	nonce := make([]byte, 6)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	name := "qv_nfc_" + hex.EncodeToString(nonce)
	admin, err := sql.Open("pgx", db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	t.Cleanup(func() {
		if _, err := admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`); err != nil {
			t.Errorf("dropping %s: %v", name, err)
		}
	})
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	conn, err := sql.Open("pgx", swapDatabase(t, db.TestDSN(t), name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func (w *composeWorld) insert(t *testing.T, query string, args ...any) string {
	t.Helper()
	var id string
	if err := w.conn.QueryRow(query, args...).Scan(&id); err != nil {
		t.Fatalf("seed: %v\n%s", err, query)
	}
	return id
}

func (w *composeWorld) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := w.conn.Exec(query, args...); err != nil {
		t.Fatalf("seed: %v\n%s", err, query)
	}
}

func (w *composeWorld) expectComposed(table, column, id, typed string) {
	w.composed = append(w.composed, composedColumn{table, column, id, typed})
}

func (w *composeWorld) expectKept(table, column, id, typed string) {
	w.kept = append(w.kept, composedColumn{table, column, id, typed})
}

const semanticProse = `{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":%q,"marks":[]}]}]}`

func seedPeople(t *testing.T, w *composeWorld) (teacher, student string) {
	t.Helper()
	teacher = w.insert(t, `INSERT INTO app.users (email, full_name, display_name, role_id, password_hash)
		VALUES ('teacher-nfc@example.com', $1, $2, (SELECT id FROM app.roles WHERE builtin_key = 'admin'), $3) RETURNING id::text`,
		nfd("Nguyễn Văn Á"), nfd("Cô Hạnh"), "argon2id$"+nfd("é"))
	w.expectComposed("users", "full_name", teacher, nfd("Nguyễn Văn Á"))
	w.expectComposed("users", "display_name", teacher, nfd("Cô Hạnh"))
	w.expectKept("users", "password_hash", teacher, "argon2id$"+nfd("é"))
	student = w.insert(t, `INSERT INTO app.users (email, full_name, role_id) VALUES ($1, $2, (SELECT id FROM app.roles WHERE builtin_key = 'student')) RETURNING id::text`,
		nfd("hoc.sinh.é")+"@example.com", nfd("Trần Thị B"))
	w.expectComposed("users", "full_name", student, nfd("Trần Thị B"))
	w.expectKept("users", "email", student, nfd("hoc.sinh.é")+"@example.com")
	return teacher, student
}

func seedMedia(t *testing.T, w *composeWorld, owner string) (image, audio string) {
	t.Helper()
	image = w.insert(t, `INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, original_filename, display_name, checksum_sha256, uploaded_by, owner_id)
		VALUES ('image', $1, 'image/png', 100, $2, $3, decode(repeat('01', 32), 'hex'), $4, $4) RETURNING id::text`,
		"image/"+nfd("khoá")+"-key", nfd("ảnh cái nón.png"), nfd("Ảnh cái nón"), owner)
	w.expectComposed("media_assets", "original_filename", image, nfd("ảnh cái nón.png"))
	w.expectComposed("media_assets", "display_name", image, nfd("Ảnh cái nón"))
	w.expectKept("media_assets", "storage_key", image, "image/"+nfd("khoá")+"-key")
	audio = w.insert(t, `INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, duration_ms, original_filename, checksum_sha256, uploaded_by, owner_id)
		VALUES ('audio', 'audio/nfc-key', 'audio/mpeg', 100, 1000, 'nghe.mp3', decode(repeat('02', 32), 'hex'), $1, $1) RETURNING id::text`, owner)
	return image, audio
}

func seedBank(t *testing.T, w *composeWorld, owner, image, audio string) {
	t.Helper()
	plain := w.insert(t, `INSERT INTO app.questions (type, prompt, explanation, sample_answer, tags, points, created_by, owner_id)
		VALUES ('short_answer', $1, $2, $3, ARRAY[$4::text, $5::text, 'shared'], 1, $6, $6) RETURNING id::text`,
		nfd("Mô tả bức tranh"), nfd("Vì sao như vậy"), nfd("Mẫu trả lời"), nfd("nghé"), "nghé", owner)
	w.expectComposed("questions", "prompt", plain, nfd("Mô tả bức tranh"))
	w.expectComposed("questions", "explanation", plain, nfd("Vì sao như vậy"))
	w.expectComposed("questions", "sample_answer", plain, nfd("Mẫu trả lời"))

	withImage := w.insert(t, `INSERT INTO app.questions (type, prompt, media_asset_id, media_asset_kind, media_alt, points, created_by, owner_id)
		VALUES ('short_answer', 'Mô tả', $1, 'image', $2, 1, $3, $3) RETURNING id::text`, image, nfd("Ảnh cái nón"), owner)
	w.expectComposed("questions", "media_alt", withImage, nfd("Ảnh cái nón"))

	withAudio := w.insert(t, `INSERT INTO app.questions (type, prompt, media_asset_id, media_asset_kind, audio_allow_seek, audio_show_transcript_after, transcript, points, created_by, owner_id)
		VALUES ('short_answer', 'Nghe', $1, 'audio', false, false, $2, 1, $3, $3) RETURNING id::text`, audio, nfd("Lời thoại"), owner)
	w.expectComposed("questions", "transcript", withAudio, nfd("Lời thoại"))

	prompt := fmt.Sprintf(semanticProse, nfd("Đọc kỹ đoạn văn"))
	explanation := fmt.Sprintf(semanticProse, nfd("Giải thích"))
	documented := w.insert(t, `INSERT INTO app.questions (type, prompt, prompt_content, explanation, explanation_content, points, created_by, owner_id)
		VALUES ('short_answer', $1, $2::jsonb, $3, $4::jsonb, 1, $5, $5) RETURNING id::text`,
		nfd("Đọc kỹ đoạn văn"), prompt, nfd("Giải thích"), explanation, owner)
	w.expectKept("questions", "prompt", documented, nfd("Đọc kỹ đoạn văn"))
	w.expectKept("questions", "explanation", documented, nfd("Giải thích"))

	choice := w.insert(t, `INSERT INTO app.questions (type, prompt, points, created_by, owner_id) VALUES ('single_choice', 'Chọn', 1, $1, $1) RETURNING id::text`, owner)
	loose := w.insert(t, `INSERT INTO app.question_options (question_id, ordinal, text, is_correct) VALUES ($1, 0, $2, true) RETURNING id::text`, choice, nfd("Phương án một"))
	w.expectComposed("question_options", "text", loose, nfd("Phương án một"))
	documentedOption := w.insert(t, `INSERT INTO app.question_options (question_id, ordinal, text, content, is_correct) VALUES ($1, 1, $2, $3::jsonb, false) RETURNING id::text`,
		choice, nfd("Phương án hai"), fmt.Sprintf(semanticProse, nfd("Phương án hai")))
	w.expectKept("question_options", "text", documentedOption, nfd("Phương án hai"))
}

func seedAnswers(t *testing.T, w *composeWorld, owner string) {
	t.Helper()
	fill := w.insert(t, `INSERT INTO app.questions (type, prompt, points, created_by, owner_id) VALUES ('fill_blank', 'Thủ đô là {{1}}', 1, $1, $1) RETURNING id::text`, owner)
	blank := w.insert(t, `INSERT INTO app.question_blanks (question_id, ordinal) VALUES ($1, 1) RETURNING id::text`, fill)
	twin := w.insert(t, `INSERT INTO app.question_blank_answers (blank_id, answer) VALUES ($1, $2) RETURNING id::text`, blank, "Hà Nội")
	w.exec(t, `INSERT INTO app.question_blank_answers (blank_id, answer) VALUES ($1, $2)`, blank, nfd("Hà Nội"))
	alone := w.insert(t, `INSERT INTO app.question_blank_answers (blank_id, answer) VALUES ($1, $2) RETURNING id::text`, blank, nfd("Huế"))
	w.exec(t, `INSERT INTO app.question_blank_answers (blank_id, answer) VALUES ($1, $2)`, blank, "ậ")
	w.exec(t, `INSERT INTO app.question_blank_answers (blank_id, answer) VALUES ($1, $2)`, blank, "ậ")
	w.expectComposed("question_blank_answers", "answer", twin, "Hà Nội")
	w.expectComposed("question_blank_answers", "answer", alone, nfd("Huế"))
}

func seedGroup(t *testing.T, w *composeWorld, owner, audio string) {
	t.Helper()
	group := w.insert(t, `INSERT INTO app.question_groups (title, created_by, owner_id) VALUES ($1, $2, $2) RETURNING id::text`, nfd("Nhóm đọc hiểu"), owner)
	w.expectComposed("question_groups", "title", group, nfd("Nhóm đọc hiểu"))
	stimulus := w.insert(t, `INSERT INTO app.group_stimuli (group_id, ordinal, title, content) VALUES ($1, 0, $2, $3::jsonb) RETURNING id::text`,
		group, nfd("Bài đọc"), fmt.Sprintf(semanticProse, nfd("Nội dung bài đọc")))
	w.expectComposed("group_stimuli", "title", stimulus, nfd("Bài đọc"))
	recording := w.insert(t, `INSERT INTO app.group_recordings (group_id, media_asset_id, allow_seek, show_transcript_after_submit, transcript)
		VALUES ($1, $2, false, false, $3) RETURNING id::text`, group, audio, nfd("Lời thoại chung"))
	w.expectComposed("group_recordings", "transcript", recording, nfd("Lời thoại chung"))
}

func seedTeaching(t *testing.T, w *composeWorld, teacher, student string) {
	t.Helper()
	draft := w.insert(t, `INSERT INTO app.tests (title, description, created_by, owner_id) VALUES ($1, $2, $3, $3) RETURNING id::text`,
		nfd("Đề kiểm tra"), nfd("Mô tả đề"), teacher)
	w.expectComposed("tests", "title", draft, nfd("Đề kiểm tra"))
	w.expectComposed("tests", "description", draft, nfd("Mô tả đề"))
	section := w.insert(t, `INSERT INTO app.test_sections (test_id, ordinal, title, instructions) VALUES ($1, 0, $2, $3) RETURNING id::text`,
		draft, nfd("Phần nghe"), nfd("Làm bài cẩn thận"))
	w.expectComposed("test_sections", "title", section, nfd("Phần nghe"))
	w.expectComposed("test_sections", "instructions", section, nfd("Làm bài cẩn thận"))

	class := w.insert(t, `INSERT INTO app.classes (name, description, teacher_id) VALUES ($1, $2, $3) RETURNING id::text`, nfd("Lớp 10A"), nfd("Lớp buổi tối"), teacher)
	w.expectComposed("classes", "name", class, nfd("Lớp 10A"))
	w.expectComposed("classes", "description", class, nfd("Lớp buổi tối"))

	imported := w.insert(t, `INSERT INTO app.word_imports (created_by, request_id, title) VALUES ($1, gen_random_uuid(), $2) RETURNING id::text`, teacher, nfd("Đề thi thử"))
	w.expectComposed("word_imports", "title", imported, nfd("Đề thi thử"))
	source := w.insert(t, `INSERT INTO app.word_import_sources (import_id, upload_id, expected_revision, role, filename, format, bytes, checksum_sha256, storage_key, uploaded_by)
		VALUES ($1, gen_random_uuid(), 1, 'exam', $2, 'docx', 100, decode(repeat('03', 32), 'hex'), $3, $4) RETURNING id::text`,
		imported, nfd("Đề cương.docx"), "imports/"+nfd("khoá"), teacher)
	w.expectComposed("word_import_sources", "filename", source, nfd("Đề cương.docx"))
	w.expectKept("word_import_sources", "storage_key", source, "imports/"+nfd("khoá"))
}

func seedPublished(t *testing.T, w *composeWorld, teacher, student string) {
	t.Helper()
	published := w.insert(t, `INSERT INTO app.tests (title, status, current_version, created_by, owner_id) VALUES ('Đề đã xuất bản', 'published', 1, $1, $1) RETURNING id::text`, teacher)
	version := w.insert(t, `INSERT INTO app.test_versions (test_id, version, total_points, published_by, change_note) VALUES ($1, 1, 10.00, $2, $3) RETURNING id::text`,
		published, teacher, nfd("Sửa đề lần một"))
	w.expectKept("test_versions", "change_note", version, nfd("Sửa đề lần một"))
	section := w.insert(t, `INSERT INTO app.test_version_sections (test_version_id, ordinal, title, instructions) VALUES ($1, 0, $2, $3) RETURNING id::text`,
		version, nfd("Phần một"), nfd("Hướng dẫn"))
	w.expectKept("test_version_sections", "title", section, nfd("Phần một"))
	w.expectKept("test_version_sections", "instructions", section, nfd("Hướng dẫn"))
	frozen := w.insert(t, `INSERT INTO app.test_version_questions (test_version_section_id, ordinal, type, prompt, explanation, sample_answer, points)
		VALUES ($1, 0, 'short_answer', $2, $3, $4, 4.00) RETURNING id::text`, section, nfd("Câu đông cứng"), nfd("Giải thích đông cứng"), nfd("Mẫu đông cứng"))
	w.expectKept("test_version_questions", "prompt", frozen, nfd("Câu đông cứng"))
	w.expectKept("test_version_questions", "explanation", frozen, nfd("Giải thích đông cứng"))
	w.expectKept("test_version_questions", "sample_answer", frozen, nfd("Mẫu đông cứng"))

	assignment := w.insert(t, `INSERT INTO app.assignments (test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by, student_note)
		VALUES ($1, $2, now() - interval '1 day', now() + interval '1 day', 30, $3, $4) RETURNING id::text`, published, version, teacher, nfd("Làm bài cẩn thận"))
	w.expectComposed("assignments", "student_note", assignment, nfd("Làm bài cẩn thận"))
	override := w.insert(t, `INSERT INTO app.assignment_student_overrides (assignment_id, student_id, reason, extra_attempts) VALUES ($1, $2, $3, 1) RETURNING assignment_id::text`,
		assignment, student, nfd("Máy bị hỏng"))
	w.expectComposed("assignment_student_overrides", "reason", override, nfd("Máy bị hỏng"))

	noted := w.insert(t, `INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, session_id, shuffle_seed, beacon_token_hash, deadline_at, teacher_note)
		VALUES ($1, $2, $3, 1, gen_random_uuid(), 1, decode(repeat('04', 32), 'hex'), now() + interval '1 hour', $4) RETURNING id::text`,
		assignment, version, student, nfd("Bài làm tốt"))
	w.expectComposed("attempts", "teacher_note", noted, nfd("Bài làm tốt"))
	voided := w.insert(t, `INSERT INTO app.attempts (assignment_id, test_version_id, student_id, attempt_no, session_id, shuffle_seed, beacon_token_hash, deadline_at, status, void_reason)
		VALUES ($1, $2, $3, 2, gen_random_uuid(), 1, decode(repeat('05', 32), 'hex'), now() + interval '1 hour', 'voided', $4) RETURNING id::text`,
		assignment, version, student, nfd("Sự cố mạng"))
	w.expectComposed("attempts", "void_reason", voided, nfd("Sự cố mạng"))
	answer := w.insert(t, `INSERT INTO app.attempt_answers (attempt_id, question_id, payload, grader_comment) VALUES ($1, $2, $3::jsonb, $4) RETURNING attempt_id::text`,
		noted, frozen, fmt.Sprintf(`{"type":"text","value":%q}`, nfd("Hà Nội")), nfd("Trả lời đúng"))
	w.expectKept("attempt_answers", "grader_comment", answer, nfd("Trả lời đúng"))
}

func (w *composeWorld) value(t *testing.T, c composedColumn) string {
	t.Helper()
	key := "id"
	if c.table == "assignment_student_overrides" || c.table == "attempt_answers" {
		key = "assignment_id"
		if c.table == "attempt_answers" {
			key = "attempt_id"
		}
	}
	var value string
	query := fmt.Sprintf(`SELECT %s::text FROM app.%s WHERE %s = $1::uuid`, c.column, c.table, key)
	if err := w.conn.QueryRow(query, c.id).Scan(&value); err != nil {
		t.Fatalf("%s.%s %s: %v", c.table, c.column, c.id, err)
	}
	return value
}

var triggerTables = map[string]string{
	"tests": "tests_set_updated_at", "questions": "questions_set_updated_at", "question_groups": "question_groups_set_updated_at",
	"classes": "classes_set_updated_at", "assignments": "assignments_set_updated_at",
	"assignment_student_overrides": "assignment_student_overrides_set_updated_at", "users": "users_set_updated_at",
	"word_imports": "word_imports_updated_at",
}

func (w *composeWorld) stamps(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for table := range triggerTables {
		key := "id::text"
		if table == "assignment_student_overrides" {
			key = "assignment_id::text || student_id::text"
		}
		rows, err := w.conn.Query(fmt.Sprintf(`SELECT %s, updated_at::text FROM app.%s ORDER BY 1`, key, table))
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var ctid, at string
			if err := rows.Scan(&ctid, &at); err != nil {
				t.Fatal(err)
			}
			out[table+" "+ctid] = at
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		_ = rows.Close()
	}
	return out
}

func (w *composeWorld) versions(t *testing.T) string {
	t.Helper()
	var out strings.Builder
	for _, table := range []string{"tests", "test_sections", "questions", "question_options", "question_blank_answers", "question_groups", "group_stimuli",
		"group_recordings", "classes", "users", "media_assets", "word_imports", "word_import_sources", "assignments", "assignment_student_overrides",
		"attempts", "attempt_answers", "test_versions", "test_version_sections", "test_version_questions"} {
		rows, err := w.conn.Query(fmt.Sprintf(`SELECT ctid::text, xmin::text FROM app.%s ORDER BY ctid`, table))
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var ctid, xmin string
			if err := rows.Scan(&ctid, &xmin); err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(&out, "%s %s %s\n", table, ctid, xmin)
		}
		_ = rows.Close()
	}
	return out.String()
}

func seedEverything(t *testing.T, w *composeWorld) {
	t.Helper()
	teacher, student := seedPeople(t, w)
	image, audio := seedMedia(t, w, teacher)
	seedBank(t, w, teacher, image, audio)
	seedAnswers(t, w, teacher)
	seedGroup(t, w, teacher, audio)
	seedTeaching(t, w, teacher, student)
	seedPublished(t, w, teacher, student)
}

func TestTheComposeMigrationComposesWhatItListsAndNothingElse(t *testing.T) {
	if os.Getenv("TEST_DESTRUCTIVE") != "1" {
		t.Skip("TEST_DESTRUCTIVE=1 required for an isolated migration database")
	}
	conn := uniqueDatabase(t)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	dir := db.MigrationsDir(t)
	if err := goose.UpTo(conn, dir, composeMigration-1); err != nil {
		t.Fatal(err)
	}
	w := &composeWorld{conn: conn}
	seedEverything(t, w)
	before := w.stamps(t)
	for _, c := range append(append([]composedColumn{}, w.composed...), w.kept...) {
		if got := w.value(t, c); got != c.typed {
			t.Fatalf("the seed for %s.%s was not stored as typed: %q", c.table, c.column, got)
		}
		if c.typed == norm.NFC.String(c.typed) && !strings.Contains(c.typed, "Hà Nội") {
			t.Fatalf("the seed for %s.%s is already composed", c.table, c.column)
		}
	}

	if err := goose.UpTo(conn, dir, composeMigration); err != nil {
		t.Fatal(err)
	}

	for _, c := range w.composed {
		if got, want := w.value(t, c), norm.NFC.String(c.typed); got != want {
			t.Errorf("%s.%s %s = %q, want %q", c.table, c.column, c.id, got, want)
		}
	}
	for _, c := range w.kept {
		if got := w.value(t, c); got != c.typed {
			t.Errorf("%s.%s %s was rewritten: %q, want it byte-identical to %q", c.table, c.column, c.id, got, c.typed)
		}
	}
	assertUnchangedDocuments(t, w)
	assertTags(t, w)
	assertAcceptedAnswers(t, w)
	assertUpdatedAtKept(t, w, before)
	assertTriggersWork(t, w)
}

func assertUnchangedDocuments(t *testing.T, w *composeWorld) {
	t.Helper()
	for _, document := range []struct{ query, want string }{
		{`SELECT prompt_content #>> '{blocks,0,content,0,text}' FROM app.questions WHERE prompt_content IS NOT NULL`, nfd("Đọc kỹ đoạn văn")},
		{`SELECT explanation_content #>> '{blocks,0,content,0,text}' FROM app.questions WHERE explanation_content IS NOT NULL`, nfd("Giải thích")},
		{`SELECT content #>> '{blocks,0,content,0,text}' FROM app.question_options WHERE content IS NOT NULL`, nfd("Phương án hai")},
		{`SELECT content #>> '{blocks,0,content,0,text}' FROM app.group_stimuli`, nfd("Nội dung bài đọc")},
	} {
		var text string
		if err := w.conn.QueryRow(document.query).Scan(&text); err != nil {
			t.Fatal(err)
		}
		if text != document.want {
			t.Errorf("a prose document was rewritten: %q, want %q", text, document.want)
		}
	}
	var payload string
	if err := w.conn.QueryRow(`SELECT payload->>'value' FROM app.attempt_answers`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if payload != nfd("Hà Nội") {
		t.Errorf("a saved answer was rewritten: %q", payload)
	}
}

func assertTags(t *testing.T, w *composeWorld) {
	t.Helper()
	var tags string
	if err := w.conn.QueryRow(`SELECT array_to_string(tags, ',') FROM app.questions WHERE tags <> '{}'`).Scan(&tags); err != nil {
		t.Fatal(err)
	}
	if tags != "nghé,shared" {
		t.Errorf("tags=%q, want the two spellings of nghé to be one, in order, composed", tags)
	}
}

func assertAcceptedAnswers(t *testing.T, w *composeWorld) {
	t.Helper()
	rows, err := w.conn.Query(`SELECT answer FROM app.question_blank_answers ORDER BY answer`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	for rows.Next() {
		var answer string
		if err := rows.Scan(&answer); err != nil {
			t.Fatal(err)
		}
		got = append(got, answer)
	}
	want := []string{norm.NFC.String("Huế"), "Hà Nội", norm.NFC.String("ậ")}
	if len(got) != len(want) {
		t.Fatalf("answers=%q, want one row for each of %q", got, want)
	}
	seen := map[string]bool{}
	for _, answer := range got {
		seen[answer] = true
	}
	for _, answer := range want {
		if !seen[answer] {
			t.Errorf("answers=%q lack %q", got, answer)
		}
	}
}

func assertUpdatedAtKept(t *testing.T, w *composeWorld, before map[string]string) {
	t.Helper()
	after := w.stamps(t)
	for key, at := range before {
		if after[key] != at {
			t.Errorf("%s: updated_at moved from %s to %s", key, at, after[key])
		}
	}
}

func assertTriggersWork(t *testing.T, w *composeWorld) {
	t.Helper()
	for table, trigger := range triggerTables {
		var state string
		if err := w.conn.QueryRow(`SELECT tgenabled::text FROM pg_trigger WHERE tgname = $1 AND tgrelid = ('app.' || $2)::regclass`, trigger, table).Scan(&state); err != nil {
			t.Fatalf("%s on %s: %v", trigger, table, err)
		}
		if state != "O" {
			t.Errorf("%s on %s is %q, want O (enabled)", trigger, table, state)
		}
	}
	for _, change := range []struct{ table, set, key string }{
		{"tests", "title = title || ' '", "id"}, {"questions", "prompt = prompt || ' '", "id"}, {"question_groups", "title = title || ' '", "id"},
		{"classes", "name = name || ' '", "id"}, {"assignments", "student_note = student_note || ' '", "id"},
		{"assignment_student_overrides", "reason = reason || ' '", "assignment_id"}, {"users", "full_name = full_name || ' '", "id"},
		{"word_imports", "title = title || ' '", "id"},
	} {
		var moved bool
		query := fmt.Sprintf(`WITH touched AS (UPDATE app.%[1]s SET %[2]s WHERE %[3]s = (SELECT %[3]s FROM app.%[1]s ORDER BY updated_at LIMIT 1) RETURNING updated_at, OLD.updated_at AS was)
			SELECT bool_and(updated_at > was) FROM touched`, change.table, change.set, change.key)
		if err := w.conn.QueryRow(query).Scan(&moved); err != nil {
			t.Fatalf("%s: %v", change.table, err)
		}
		if !moved {
			t.Errorf("%s: a normal update did not move updated_at, so its trigger no longer fires", change.table)
		}
	}
}

func TestTheComposeMigrationIsIdempotentAndItsDownChangesNothing(t *testing.T) {
	if os.Getenv("TEST_DESTRUCTIVE") != "1" {
		t.Skip("TEST_DESTRUCTIVE=1 required for an isolated migration database")
	}
	conn := uniqueDatabase(t)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	dir := db.MigrationsDir(t)
	if err := goose.UpTo(conn, dir, composeMigration-1); err != nil {
		t.Fatal(err)
	}
	w := &composeWorld{conn: conn}
	seedEverything(t, w)
	schema := schemaSnapshot(t, conn)

	if err := goose.UpTo(conn, dir, composeMigration); err != nil {
		t.Fatal(err)
	}
	once := w.versions(t)
	if err := goose.DownTo(conn, dir, composeMigration-1); err != nil {
		t.Fatal(err)
	}
	if schemaSnapshot(t, conn) != schema {
		t.Error("the Down changed the schema")
	}
	if w.versions(t) != once {
		t.Error("the Down changed a row")
	}
	for _, c := range w.composed {
		if got, want := w.value(t, c), norm.NFC.String(c.typed); got != want {
			t.Errorf("after the Down %s.%s = %q, want it to stay composed (%q)", c.table, c.column, got, want)
		}
	}
	if err := goose.UpTo(conn, dir, composeMigration); err != nil {
		t.Fatal(err)
	}
	if w.versions(t) != once {
		t.Error("running the migration again touched a row; it must select only the rows that change")
	}
}

func TestTheComposeMigrationLeavesAValueThatComposingWouldTakePastItsCheck(t *testing.T) {
	if os.Getenv("TEST_DESTRUCTIVE") != "1" {
		t.Skip("TEST_DESTRUCTIVE=1 required for an isolated migration database")
	}
	conn := uniqueDatabase(t)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	goose.SetLogger(goose.NopLogger())
	dir := db.MigrationsDir(t)
	if err := goose.UpTo(conn, dir, composeMigration-1); err != nil {
		t.Fatal(err)
	}
	w := &composeWorld{conn: conn}
	teacher, _ := seedPeople(t, w)
	title := strings.Repeat("क़", 200)
	long := w.insert(t, `INSERT INTO app.tests (title, created_by, owner_id) VALUES ($1, $2, $2) RETURNING id::text`, title, teacher)
	short := w.insert(t, `INSERT INTO app.tests (title, created_by, owner_id) VALUES ($1, $2, $2) RETURNING id::text`, nfd("Đề kiểm tra"), teacher)

	if err := goose.UpTo(conn, dir, composeMigration); err != nil {
		t.Fatalf("a value at its limit must not stop the migration: %v", err)
	}
	if got := w.value(t, composedColumn{"tests", "title", long, title}); got != title {
		t.Errorf("a title that composing would make 400 characters long was rewritten")
	}
	if got := w.value(t, composedColumn{"tests", "title", short, ""}); got != "Đề kiểm tra" {
		t.Errorf("the neighbouring title was not composed: %q", got)
	}
}

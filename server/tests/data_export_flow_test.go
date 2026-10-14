//go:build e2e

package e2e

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/core/wiring"
	exportsworker "quizzivy/internal/modules/exports/application/worker"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/email"
)

const hostileFormula = `=HYPERLINK("http://x")`

func (w *world) exportRunner() exportsworker.Runner {
	w.t.Helper()
	runner, err := wiring.ExportsWorker(context.Background(), config.ImportWorker{Base: w.cfg}, db.NewContext(w.pool), email.NewOutbox(nil), slog.New(slog.DiscardHandler))
	if err != nil {
		w.t.Fatalf("assemble the export worker: %v", err)
	}
	return runner
}

func (w *world) runExportWorker() {
	w.t.Helper()
	worked, err := w.exportRunner().RunOne(context.Background())
	if err != nil || !worked {
		w.t.Fatalf("the export worker ran %v with error %v", worked, err)
	}
}

func exportErrorCode(body map[string]any) string {
	envelope, _ := body["error"].(map[string]any)
	code, _ := envelope["code"].(string)
	return code
}

func (c *client) sitAnswering(assignmentID string) (attempt, question string) {
	c.w.t.Helper()
	session := c.must(http.StatusOK, http.MethodPost, "/app/assignments/"+assignmentID+"/attempts", nil)
	attempt = id(session["attempt"].(map[string]any))
	first := session["questions"].([]any)[0].(map[string]any)
	question = id(first)
	option := id(first["options"].([]any)[0].(map[string]any))
	c.must(http.StatusOK, http.MethodPatch, "/app/attempts/"+attempt+"/answers", map[string]any{
		"sessionId": session["sessionId"],
		"answers":   map[string]any{question: map[string]any{"type": "choice", "optionIds": []string{option}}},
	})
	c.must(http.StatusOK, http.MethodPost, "/app/attempts/"+attempt+"/submit", map[string]any{"sessionId": session["sessionId"], "reason": "manual"})
	return attempt, question
}

func readZip(t *testing.T, raw []byte) (names []string, files map[string]string) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("the download is not a ZIP: %v", err)
	}
	files = map[string]string{}
	for _, entry := range reader.File {
		in, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(in)
		_ = in.Close()
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, entry.Name)
		files[entry.Name] = string(body)
	}
	return names, files
}

func records(t *testing.T, name, body string) [][]string {
	t.Helper()
	if !strings.HasPrefix(body, "\xEF\xBB\xBF") {
		t.Fatalf("%s does not start with a byte-order mark: %.12q", name, body)
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(body, "\xEF\xBB\xBF"))).ReadAll()
	if err != nil {
		t.Fatalf("%s is not CSV: %v", name, err)
	}
	return rows
}

func find(rows [][]string, column, value string) []string {
	at := -1
	for i, name := range rows[0] {
		if name == column {
			at = i
		}
	}
	if at < 0 {
		return nil
	}
	for _, row := range rows[1:] {
		if row[at] == value {
			return row
		}
	}
	return nil
}

func cell(rows [][]string, row []string, column string) string {
	for i, name := range rows[0] {
		if name == column {
			return row[i]
		}
	}
	return "<no column " + column + ">"
}

func TestAnAdminExportsTheCentresDataAndFetchesItOnce(t *testing.T) {
	w := bootAssistedWithStorage(t, nil)
	adminEmail, adminPassword := w.createStaff("admin")
	admin := w.signedIn(adminEmail, adminPassword)
	rivalEmail, rivalPassword := w.createStaff("admin")
	rival := w.signedIn(rivalEmail, rivalPassword)
	laterEmail, laterPassword := w.createStaff("admin")
	later := w.signedIn(laterEmail, laterPassword)
	sloppyEmail, sloppyPassword := w.createStaff("admin")
	sloppy := w.signedIn(sloppyEmail, sloppyPassword)
	teacherEmail, teacherPassword := w.createStaff("teacher")
	teacher := w.signedIn(teacherEmail, teacherPassword)

	tag := nonce(t)
	classID := teacher.class("Lớp xuất dữ liệu " + tag)
	testID, versionID := teacher.publishedVersion(hostileFormula)
	assignment := teacher.assign(versionID, classID)
	named := w.studentNamed(teacher, classID, hostileFormula)
	plain := w.studentNamed(teacher, classID, "Nguyễn Văn An")
	namedAttempt, question := named.sitAnswering(id(assignment))
	plainAttempt, _ := plain.sitAnswering(id(assignment))
	if _, err := w.pool.Exec(context.Background(), `UPDATE app.attempt_answers SET payload = '{"removed": true}'::jsonb WHERE attempt_id = $1::uuid`, plainAttempt); err != nil {
		t.Fatalf("mark an answer removed: %v", err)
	}
	var adminHash string
	if err := w.pool.QueryRow(context.Background(), `SELECT password_hash FROM app.users WHERE email = $1`, adminEmail).Scan(&adminHash); err != nil || adminHash == "" {
		t.Fatalf("the admin's password hash: %q, %v", adminHash, err)
	}

	wakesBefore := fixtures.wakeCalls.Load()
	if status, _ := teacher.call(http.MethodPost, "/admin/data-exports", map[string]any{"includeAnswers": true}); status != http.StatusForbidden {
		t.Errorf("a teacher's request answered %d, want 403", status)
	}
	if status, _ := named.call(http.MethodPost, "/admin/data-exports", map[string]any{"includeAnswers": true}); status != http.StatusForbidden {
		t.Errorf("a student's request answered %d, want 403", status)
	}
	if status, _ := w.browser().call(http.MethodPost, "/admin/data-exports", map[string]any{"includeAnswers": true}); status != http.StatusUnauthorized {
		t.Errorf("an anonymous request answered %d, want 401", status)
	}
	if status, _ := sloppy.call(http.MethodPost, "/admin/data-exports", map[string]any{}); status != http.StatusBadRequest {
		t.Errorf("a request without includeAnswers answered %d, want 400", status)
	}

	queued := admin.must(http.StatusAccepted, http.MethodPost, "/admin/data-exports", map[string]any{"includeAnswers": true})
	exportID := id(queued)
	if queued["status"] != "queued" || queued["includeAnswers"] != true || queued["bytes"] != nil || queued["downloadedAt"] != nil {
		t.Errorf("the queued export reads %v", queued)
	}
	woken := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline) && !woken; time.Sleep(20 * time.Millisecond) {
		woken = fixtures.wakeCalls.Load() > wakesBefore
	}
	if !woken {
		t.Error("the worker was not woken")
	}
	if status, _ := admin.call(http.MethodPost, "/admin/data-exports", map[string]any{"includeAnswers": true}); status != http.StatusTooManyRequests {
		t.Errorf("a second request in the hour by the same admin answered %d, want 429", status)
	}
	busy := rival.must(http.StatusConflict, http.MethodPost, "/admin/data-exports", map[string]any{"includeAnswers": false})
	if exportErrorCode(busy) != "EXPORT_IN_PROGRESS" {
		t.Errorf("another admin's request while one is queued reads %v", busy)
	}

	if got := admin.must(http.StatusOK, http.MethodGet, "/admin/data-exports/"+exportID, nil); got["status"] != "queued" {
		t.Errorf("the requester reads %v", got)
	}
	for who, c := range map[string]*client{"another admin": rival, "a teacher": teacher} {
		status, _ := c.call(http.MethodGet, "/admin/data-exports/"+exportID, nil)
		want := http.StatusNotFound
		if who == "a teacher" {
			want = http.StatusForbidden
		}
		if status != want {
			t.Errorf("%s reading the export answered %d, want %d", who, status, want)
		}
	}
	if status, body := rival.call(http.MethodPost, "/admin/data-exports/"+exportID+"/download", nil); status != http.StatusNotFound {
		t.Errorf("another admin's download answered %d, want 404: %v", status, body)
	}
	if status, body := admin.call(http.MethodPost, "/admin/data-exports/"+exportID+"/download", nil); status != http.StatusGone || exportErrorCode(body) != "EXPORT_UNAVAILABLE" {
		t.Errorf("a download before the file exists answered %d: %v", status, body)
	}

	w.runExportWorker()

	ready := admin.must(http.StatusOK, http.MethodGet, "/admin/data-exports/"+exportID, nil)
	if ready["status"] != "ready" || ready["bytes"].(float64) <= 0 || ready["completedAt"] == nil || ready["downloadedAt"] != nil {
		t.Fatalf("the finished export reads %v", ready)
	}
	expires, err := time.Parse(time.RFC3339, ready["expiresAt"].(string))
	if err != nil || time.Until(expires) < 23*time.Hour || time.Until(expires) > 25*time.Hour {
		t.Errorf("the export expires at %v (%v), want about a day from now", ready["expiresAt"], err)
	}

	var template, to, params string
	err = w.pool.QueryRow(context.Background(), `SELECT template, to_address, params::text FROM app.email_outbox WHERE dedupe_key = $1`, "data_export_ready:"+exportID).Scan(&template, &to, &params)
	if err != nil {
		t.Fatalf("the requester was not told: %v", err)
	}
	var notice map[string]string
	if err := json.Unmarshal([]byte(params), &notice); err != nil {
		t.Fatal(err)
	}
	if template != "data_export_ready" || to != adminEmail || notice["exportId"] != exportID || len(notice) != 2 {
		t.Errorf("notice = %s to %s with %v", template, to, notice)
	}

	download := admin.must(http.StatusOK, http.MethodPost, "/admin/data-exports/"+exportID+"/download", nil)
	if download["expiresInSeconds"] != float64(60) {
		t.Errorf("the download reads %v", download)
	}
	address := download["url"].(string)
	resp, err := http.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("the presigned address answered %d: %v", resp.StatusCode, err)
	}
	if !regexp.MustCompile(`^attachment; filename="?quizzivy-export-\d{8}\.zip"?$`).MatchString(resp.Header.Get("Content-Disposition")) {
		t.Errorf("the file is offered as %q", resp.Header.Get("Content-Disposition"))
	}
	if int64(len(raw)) != int64(ready["bytes"].(float64)) {
		t.Errorf("the file is %d bytes, the export says %v", len(raw), ready["bytes"])
	}

	names, files := readZip(t, raw)
	wantNames := []string{"README.txt", "users.csv", "roles.csv", "classes.csv", "class_members.csv", "tests.csv", "test_versions.csv", "assignments.csv", "attempts.csv", "answers.csv"}
	if strings.Join(names, ",") != strings.Join(wantNames, ",") {
		t.Fatalf("the ZIP holds %v, want %v", names, wantNames)
	}
	if !strings.HasPrefix(files["README.txt"], "\xEF\xBB\xBFBẢN XUẤT DỮ LIỆU QUIZZIVY") || !strings.Contains(files["README.txt"], "QUIZZIVY DATA EXPORT") {
		t.Errorf("the README is not Vietnamese then English: %.60q", files["README.txt"])
	}
	for _, secret := range []string{adminHash, "argon2id", "beacon", "session_id"} {
		for name, body := range files {
			if strings.Contains(body, secret) && name != "README.txt" {
				t.Errorf("%s carries %q", name, secret)
			}
		}
	}

	users := records(t, "users.csv", files["users.csv"])
	if got := find(users, "email", adminEmail); got == nil || cell(users, got, "role_name") == "" {
		t.Errorf("the requester is not in users.csv: %v", got)
	}
	escaped := find(users, "full_name", "'"+hostileFormula)
	if escaped == nil {
		t.Fatalf("users.csv holds no escaped copy of %s", hostileFormula)
	}
	if find(users, "full_name", hostileFormula) != nil {
		t.Error("a name that begins with = reached users.csv as a formula")
	}
	tests := records(t, "tests.csv", files["tests.csv"])
	mine := find(tests, "id", testID)
	if mine == nil || cell(tests, mine, "title") != "'"+hostileFormula || cell(tests, mine, "status") != "published" {
		t.Errorf("tests.csv row = %v", mine)
	}
	classes := records(t, "classes.csv", files["classes.csv"])
	if row := find(classes, "id", classID); row == nil || cell(classes, row, "name") != "Lớp xuất dữ liệu "+tag {
		t.Errorf("classes.csv row = %v", row)
	}
	members := records(t, "class_members.csv", files["class_members.csv"])
	if find(members, "class_id", classID) == nil {
		t.Error("class_members.csv lacks the class's students")
	}
	versions := records(t, "test_versions.csv", files["test_versions.csv"])
	if row := find(versions, "id", versionID); row == nil || cell(versions, row, "test_id") != testID {
		t.Errorf("test_versions.csv row = %v", row)
	}
	assignments := records(t, "assignments.csv", files["assignments.csv"])
	if row := find(assignments, "id", id(assignment)); row == nil || cell(assignments, row, "class_ids") != classID || cell(assignments, row, "max_attempts") != "1" {
		t.Errorf("assignments.csv row = %v", row)
	}
	attempts := records(t, "attempts.csv", files["attempts.csv"])
	if row := find(attempts, "id", namedAttempt); row == nil || cell(attempts, row, "assignment_id") != id(assignment) ||
		cell(attempts, row, "status") == "in_progress" || cell(attempts, row, "attempt_no") != "1" {
		t.Errorf("attempts.csv row = %v", row)
	}
	answers := records(t, "answers.csv", files["answers.csv"])
	given := find(answers, "attempt_id", namedAttempt)
	if given == nil || cell(answers, given, "question_id") != question || cell(answers, given, "removed") != "false" || !strings.Contains(cell(answers, given, "answer"), `"optionIds"`) {
		t.Errorf("answers.csv row = %v", given)
	}
	gone := find(answers, "attempt_id", plainAttempt)
	if gone == nil || cell(answers, gone, "removed") != "true" {
		t.Errorf("a removed answer reads %v", gone)
	}

	if status, body := admin.call(http.MethodPost, "/admin/data-exports/"+exportID+"/download", nil); status != http.StatusGone || exportErrorCode(body) != "EXPORT_UNAVAILABLE" {
		t.Errorf("the second download answered %d: %v", status, body)
	}
	after := admin.must(http.StatusOK, http.MethodGet, "/admin/data-exports/"+exportID, nil)
	if after["downloadedAt"] == nil {
		t.Errorf("the export does not say it was fetched: %v", after)
	}
	for _, action := range []string{"data_export.requested", "data_export.downloaded"} {
		var n int
		if err := w.pool.QueryRow(context.Background(),
			`SELECT count(*) FROM app.audit_log WHERE action = $1 AND entity_id = $2::uuid AND actor_user_id = (SELECT id FROM app.users WHERE email = $3)`,
			action, exportID, adminEmail).Scan(&n); err != nil || n != 1 {
			t.Errorf("%s was audited %d times (%v), want once", action, n, err)
		}
	}

	next := later.must(http.StatusAccepted, http.MethodPost, "/admin/data-exports", map[string]any{"includeAnswers": false})
	w.runExportWorker()
	lean := later.must(http.StatusOK, http.MethodGet, "/admin/data-exports/"+id(next), nil)
	if lean["status"] != "ready" || lean["includeAnswers"] != false {
		t.Fatalf("the second export reads %v", lean)
	}
	again := later.must(http.StatusOK, http.MethodPost, "/admin/data-exports/"+id(next)+"/download", nil)
	resp, err = http.Get(again["url"].(string))
	if err != nil {
		t.Fatal(err)
	}
	raw, err = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("the second download answered %d: %v", resp.StatusCode, err)
	}
	names, files = readZip(t, raw)
	if strings.Join(names, ",") != strings.Join(wantNames[:len(wantNames)-1], ",") || strings.Contains(files["README.txt"], "answers.csv") {
		t.Errorf("an export without answers holds %v", names)
	}
}

func TestAWorkerThatCannotStoreTheFileFailsTheExportWithACodeAndTellsNobody(t *testing.T) {
	w := bootAssistedWithStorage(t, nil)
	email, password := w.createStaff("admin")
	admin := w.signedIn(email, password)
	queued := admin.must(http.StatusAccepted, http.MethodPost, "/admin/data-exports", map[string]any{"includeAnswers": false})

	broken := w.cfg
	broken.S3Endpoint = "http://127.0.0.1:1"
	runner, err := wiring.ExportsWorker(context.Background(), config.ImportWorker{Base: broken}, db.NewContext(w.pool), nil, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	if worked, err := runner.RunOne(context.Background()); err != nil || !worked {
		t.Fatalf("RunOne = %v, %v", worked, err)
	}
	got := admin.must(http.StatusOK, http.MethodGet, "/admin/data-exports/"+id(queued), nil)
	if got["status"] != "failed" || got["bytes"] != nil || got["expiresAt"] != nil {
		t.Errorf("the failed export reads %v", got)
	}
	if status, body := admin.call(http.MethodPost, "/admin/data-exports/"+id(queued)+"/download", nil); status != http.StatusGone {
		t.Errorf("a failed export's download answered %d: %v", status, body)
	}
	var code string
	if err := w.pool.QueryRow(context.Background(), `SELECT error_code FROM app.data_exports WHERE id = $1::uuid`, id(queued)).Scan(&code); err != nil || code != "STORAGE_UNAVAILABLE" {
		t.Errorf("error code = %q, %v", code, err)
	}
	var mails int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.email_outbox WHERE dedupe_key = $1`, "data_export_ready:"+id(queued)).Scan(&mails); err != nil || mails != 0 {
		t.Errorf("%d notices for a failed export (%v)", mails, err)
	}
}

func TestWithoutStorageTheExportAnswersNotImplemented(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	admin := w.signedIn(email, password)
	if status, body := admin.call(http.MethodPost, "/admin/data-exports", map[string]any{"includeAnswers": false}); status != http.StatusNotImplemented {
		t.Errorf("a request without a worker answered %d: %v", status, body)
	}
}

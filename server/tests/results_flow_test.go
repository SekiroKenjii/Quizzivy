//go:build e2e

package e2e

import (
	"context"
	"encoding/csv"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func (c *client) download(path string) (int, http.Header, []byte) {
	c.w.t.Helper()
	req, err := http.NewRequest(http.MethodGet, c.w.server.URL+path, nil)
	if err != nil {
		c.w.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		c.w.t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.w.t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, raw
}

func (w *world) studentNamed(teacher *client, classID, fullName string) *client {
	w.t.Helper()
	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "student-" + nonce(w.t) + "@example.com", "fullName": fullName, "classIds": []string{classID},
	})
	student := w.browser()
	student.loginSettled(created["user"].(map[string]any)["email"].(string), created["temporaryPassword"].(string))
	return student
}

func TestATeacherExportsAnalysesAndDuplicatesAnAssignment(t *testing.T) {
	w := boot(t)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	otherEmail, otherPassword := w.createStaff("teacher")
	other := w.signedIn(otherEmail, otherPassword)

	tag := nonce(t)
	classID := teacher.class("Lớp xuất " + tag)
	_, versionID := teacher.publishedVersion("Phát âm " + tag)
	assignment := teacher.assign(versionID, classID)
	hostile := `=HYPERLINK("http://evil.example","bấm")`
	sitting := w.studentNamed(teacher, classID, hostile)
	w.studentNamed(teacher, classID, "Nguyễn Văn An")
	sitting.sit(id(assignment))

	status, header, raw := teacher.download("/teacher/assignments/results.csv?ids=" + id(assignment))
	if status != http.StatusOK {
		t.Fatalf("the export answered %d: %s", status, raw)
	}
	if !strings.HasPrefix(header.Get("Content-Type"), "text/csv") || header.Get("Cache-Control") != "private, no-store" ||
		!regexp.MustCompile(`^attachment; filename="results-\d{8}\.csv"$`).MatchString(header.Get("Content-Disposition")) {
		t.Errorf("the export's headers read %v", header)
	}
	if !strings.HasPrefix(string(raw), "\xEF\xBB\xBF") {
		t.Fatalf("the file starts %q, want a byte-order mark", raw[:min(len(raw), 8)])
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(raw), "\xEF\xBB\xBF")))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 || len(records[0]) != 12 {
		t.Fatalf("the file holds %d records of %d columns, want a header and two students of 12: %q", len(records), len(records[0]), records)
	}
	byName := map[string][]string{}
	for _, row := range records[1:] {
		byName[row[3]] = row
	}
	escaped := byName["'"+hostile]
	if escaped == nil {
		t.Fatalf("the hostile name is not escaped in %q", records)
	}
	if escaped[2] != "Lớp xuất "+tag || escaped[5] != "Đã nộp" && escaped[5] != "Đã chấm" || escaped[7] != "1" {
		t.Errorf("the student who sat the paper reads %q", escaped)
	}
	if idle := byName["Nguyễn Văn An"]; idle == nil || idle[5] != "Chưa làm" || idle[6] != "" || idle[9] != "" {
		t.Errorf("the student who did not start reads %q", idle)
	}

	foreign := other.class("Lớp khác " + tag)
	_, foreignVersion := other.publishedVersion("Đề khác " + tag)
	theirs := other.assign(foreignVersion, foreign)
	var answers []string
	for _, ids := range []string{uuid.NewString(), id(theirs)} {
		got, _, body := teacher.download("/teacher/assignments/results.csv?ids=" + id(assignment) + "&ids=" + ids)
		answers = append(answers, strings.TrimSpace(string(body)))
		if got != http.StatusNotFound {
			t.Errorf("an export naming %s answered %d, want 404 for the whole request: %s", ids, got, body)
		}
	}
	if regexp.MustCompile(`[0-9a-f-]{36}`).ReplaceAllString(answers[0], "<id>") != regexp.MustCompile(`[0-9a-f-]{36}`).ReplaceAllString(answers[1], "<id>") {
		t.Errorf("another teacher's assignment answers %s where a missing one answers %s", answers[1], answers[0])
	}

	many := url.Values{}
	for range 51 {
		many.Add("ids", uuid.NewString())
	}
	for name, query := range map[string]string{
		"none":            "",
		"fifty-one":       many.Encode(),
		"a repeated id":   "ids=" + id(assignment) + "&ids=" + id(assignment),
		"not an id":       "ids=abc",
		"one good, a bad": "ids=" + id(assignment) + "&ids=nope",
	} {
		if got, _, body := teacher.download("/teacher/assignments/results.csv?" + query); got != http.StatusBadRequest {
			t.Errorf("an export with %s answered %d, want 400: %s", name, got, body)
		}
	}

	analysis := teacher.must(http.StatusOK, http.MethodGet, "/teacher/assignments/"+id(assignment)+"/item-analysis", nil)
	items, _ := analysis["items"].([]any)
	if analysis["handedIn"] != float64(1) || len(items) != 1 {
		t.Fatalf("the analysis reads %v, want one paper and one question", analysis)
	}
	item := items[0].(map[string]any)
	if item["number"] != float64(1) || item["type"] != "single_choice" || item["answered"] != float64(0) || item["correctRate"] != float64(0) ||
		!strings.Contains(item["promptExcerpt"].(string), "Which word is a noun?") {
		t.Errorf("the question reads %v, want number 1, unanswered, rate 0", item)
	}
	if status, _ := other.call(http.MethodGet, "/teacher/assignments/"+id(assignment)+"/item-analysis", nil); status != http.StatusNotFound {
		t.Errorf("another teacher's analysis answered %d, want 404", status)
	}

	second := teacher.class("Lớp hai " + tag)
	copied := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/assignments/"+id(assignment)+"/duplicate", map[string]any{"classIds": []string{second}})
	if copied["status"] != "draft" || copied["publishedAt"] != nil || copied["testVersionId"] != versionID || copied["questionCount"] != float64(1) {
		t.Errorf("the copy reads %v, want a draft of the same version with its one question", copied)
	}
	targets := copied["targets"].(map[string]any)
	if classes, _ := targets["classes"].([]any); len(classes) != 1 || classes[0].(map[string]any)["id"] != second {
		t.Errorf("the copy targets %v, want only the class named", targets)
	}
	if status, body := teacher.call(http.MethodPost, "/teacher/assignments/"+id(assignment)+"/duplicate", map[string]any{"classIds": []string{foreign}}); status != http.StatusBadRequest {
		t.Errorf("a copy into another teacher's class answered %d, want 400: %v", status, body)
	}
	if status, _ := other.call(http.MethodPost, "/teacher/assignments/"+id(assignment)+"/duplicate", map[string]any{"classIds": []string{}}); status != http.StatusNotFound {
		t.Errorf("another teacher copying the assignment answered %d, want 404", status)
	}

	listed := teacher.must(http.StatusOK, http.MethodGet, "/teacher/assignments?q="+url.QueryEscape("PHAT AM "+tag)+"&classId="+classID+"&classId="+second, nil)
	rows, _ := listed["items"].([]any)
	found := map[string]bool{}
	for _, row := range rows {
		found[id(row.(map[string]any))] = true
		if row.(map[string]any)["questionCount"] != float64(1) {
			t.Errorf("the list says %v questions for a version with one", row.(map[string]any)["questionCount"])
		}
	}
	if len(rows) != 2 || !found[id(assignment)] || !found[id(copied)] {
		t.Errorf("a search for the title within both classes lists %d rows %v, want the assignment and its copy", len(rows), found)
	}
	only := teacher.must(http.StatusOK, http.MethodGet, "/teacher/assignments?q="+url.QueryEscape("lop hai")+"&classId="+classID+"&classId="+second, nil)
	if got, _ := only["items"].([]any); len(got) != 1 || id(got[0].(map[string]any)) != id(copied) {
		t.Errorf("a search for the second class's name lists %v, want only the copy", only["items"])
	}
}

func exportedRows(t *testing.T, teacher *client, assignmentID string) map[string][]string {
	t.Helper()
	status, _, raw := teacher.download("/teacher/assignments/results.csv?ids=" + assignmentID)
	if status != http.StatusOK {
		t.Fatalf("the export answered %d: %s", status, raw)
	}
	records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(raw), "\xEF\xBB\xBF"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string][]string{}
	for _, record := range records[1:] {
		rows[record[3]] = record
	}
	if len(rows) != len(records)-1 {
		t.Fatalf("the file names a student twice: %q", records)
	}
	return rows
}

func (w *world) adminWhoTakesTestsJoins(teacher *client, classID string) *client {
	w.t.Helper()
	granted := false
	w.t.Cleanup(func() {
		if !granted {
			return
		}
		if _, err := w.pool.Exec(context.Background(), `
			DELETE FROM app.role_permissions
			 WHERE permission_key = 'learning.take_tests'
			   AND role_id = (SELECT r.id FROM app.roles r WHERE r.builtin_key = 'admin')`); err != nil {
			w.t.Errorf("removing the Admin role's Take tests grant: %v", err)
		}
	})
	grant, err := w.pool.Exec(context.Background(), `
		INSERT INTO app.role_permissions (role_id, permission_key)
		SELECT r.id, 'learning.take_tests' FROM app.roles r WHERE r.builtin_key = 'admin'
		ON CONFLICT (role_id, permission_key) DO NOTHING`)
	if err != nil {
		w.t.Fatal(err)
	}
	granted = grant.RowsAffected() > 0
	adminEmail, adminPassword := w.createStaff("admin")
	taker := w.signedIn(adminEmail, adminPassword)
	code := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/classes/"+classID+"/join-code", map[string]any{})["code"].(string)
	taker.must(http.StatusOK, http.MethodPost, "/app/classes/join", map[string]any{"joinCode": code})
	var member bool
	if err := w.pool.QueryRow(context.Background(), `
		SELECT EXISTS (SELECT 1 FROM app.class_members m JOIN app.users u ON u.id = m.user_id
		                WHERE m.class_id = $1::uuid AND u.email = $2)`, classID, adminEmail).Scan(&member); err != nil || !member {
		w.t.Fatalf("the Admin who takes tests is not a member of the class: %v", err)
	}
	return taker
}

func TestTheResultsExportLeavesOutStaffWhoOnlyJoinedAClassAndKeepsThoseWhoSat(t *testing.T) {
	w := boot(t)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	tag := nonce(t)
	classID := teacher.class("Lớp có quản trị " + tag)
	_, versionID := teacher.publishedVersion("Đề có quản trị " + tag)
	assignment := teacher.assign(versionID, classID)
	pupil := w.studentNamed(teacher, classID, "Trần Thị Bình")
	pupil.sit(id(assignment))
	taker := w.adminWhoTakesTestsJoins(teacher, classID)

	const staffName = "Cô Thương"
	rows := exportedRows(t, teacher, id(assignment))
	if _, listed := rows[staffName]; listed || len(rows) != 1 || rows["Trần Thị Bình"] == nil {
		t.Errorf("an Admin who joined the class and has not sat the paper is in the export: %q", rows)
	}

	taker.sit(id(assignment))
	rows = exportedRows(t, teacher, id(assignment))
	sat := rows[staffName]
	if sat == nil || len(rows) != 2 || sat[5] == "Chưa làm" || sat[1] == "" {
		t.Errorf("an Admin who sat the paper reads %q among %q, want their result kept beside the student's", sat, rows)
	}
}

func monitorRows(t *testing.T, teacher *client, assignmentID string) map[string]string {
	t.Helper()
	monitor := teacher.must(http.StatusOK, http.MethodGet, "/teacher/assignments/"+assignmentID+"/attempts", nil)
	states := map[string]string{}
	for _, row := range monitor["rows"].([]any) {
		listed := row.(map[string]any)
		name := listed["fullName"].(string)
		if _, again := states[name]; again {
			t.Fatalf("the monitor names %s twice: %v", name, monitor["rows"])
		}
		states[name] = listed["state"].(string)
	}
	return states
}

func TestTheMonitorListsStaffWhoOnlyJoinedAClassOnlyOnceTheySatAsTheExportDoes(t *testing.T) {
	w := boot(t)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	tag := nonce(t)
	classID := teacher.class("Lớp giám sát " + tag)
	_, versionID := teacher.publishedVersion("Đề giám sát " + tag)
	assignment := teacher.assign(versionID, classID)
	w.studentNamed(teacher, classID, "Trần Thị Bình")
	taker := w.adminWhoTakesTestsJoins(teacher, classID)

	const staffName = "Cô Thương"
	states := monitorRows(t, teacher, id(assignment))
	if _, listed := states[staffName]; listed || len(states) != 1 || states["Trần Thị Bình"] != "not_started" {
		t.Errorf("the monitor lists %v, want only the student who has not started and not the Admin who joined the class", states)
	}
	exported := exportedRows(t, teacher, id(assignment))
	if len(exported) != len(states) {
		t.Errorf("the monitor lists %v where the export lists %q", states, exported)
	}

	taker.sit(id(assignment))
	states = monitorRows(t, teacher, id(assignment))
	if state, listed := states[staffName]; !listed || state == "not_started" || len(states) != 2 {
		t.Errorf("after the Admin sat the paper the monitor lists %v, want their paper beside the student", states)
	}
	exported = exportedRows(t, teacher, id(assignment))
	if len(exported) != len(states) || exported[staffName] == nil {
		t.Errorf("the monitor lists %v where the export lists %q", states, exported)
	}
}

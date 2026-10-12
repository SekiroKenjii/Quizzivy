//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAnIdNamedTwiceInAnySpellingIsRefusedWhereverTheContractSaysTheListIsUnique(t *testing.T) {
	w := boot(t)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)

	tag := nonce(t)
	classID := teacher.class("Lớp lặp " + tag)
	spare := teacher.class("Lớp dự phòng " + tag)
	_, versionID := teacher.publishedVersion("Đề lặp " + tag)
	assignment := teacher.assign(versionID, classID)
	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "lap-" + nonce(t) + "@example.com", "fullName": "Lê Văn Lặp", "classIds": []string{classID},
	})
	studentID := id(created["user"].(map[string]any))

	refused := func(label string, status int, body map[string]any, field string) {
		t.Helper()
		envelope, _ := body["error"].(map[string]any)
		if code, _ := envelope["code"].(string); status != http.StatusBadRequest || code != "VALIDATION_FAILED" {
			t.Errorf("%s answered %d %v, want 400 VALIDATION_FAILED as a repeated id does", label, status, body)
			return
		}
		details, _ := envelope["details"].(map[string]any)
		if _, named := details[field]; !named {
			t.Errorf("%s: details %v, want %s named", label, details, field)
		}
	}
	upper := strings.ToUpper
	assignmentsBefore := teacher.must(http.StatusOK, http.MethodGet, "/teacher/assignments", nil)["total"]

	t.Run("the results export", func(t *testing.T) {
		status, _, raw := teacher.download("/teacher/assignments/results.csv?ids=" + id(assignment) + "&ids=" + upper(id(assignment)))
		body := map[string]any{}
		_ = json.Unmarshal(raw, &body)
		refused("an export naming one assignment in two spellings", status, body, "ids")
		if got, _, file := teacher.download("/teacher/assignments/results.csv?ids=" + id(assignment)); got != http.StatusOK {
			t.Errorf("the export naming the assignment once answered %d: %s", got, file)
		}
	})

	t.Run("duplicating an assignment", func(t *testing.T) {
		status, body := teacher.call(http.MethodPost, "/teacher/assignments/"+id(assignment)+"/duplicate",
			map[string]any{"classIds": []string{spare, upper(spare)}})
		refused("a copy into one class in two spellings", status, body, "classIds")
		if after := teacher.must(http.StatusOK, http.MethodGet, "/teacher/assignments", nil)["total"]; after != assignmentsBefore {
			t.Errorf("a refused copy left %v assignments where there were %v", after, assignmentsBefore)
		}
		copied := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/assignments/"+id(assignment)+"/duplicate",
			map[string]any{"classIds": []string{spare}})
		if copied["status"] != "draft" {
			t.Errorf("the copy into the class named once reads %v", copied)
		}
	})

	t.Run("student overrides", func(t *testing.T) {
		path := "/teacher/assignments/" + id(assignment) + "/student-overrides"
		status, body := teacher.call(http.MethodPut, path, map[string]any{
			"studentIds": []string{studentID, upper(studentID)}, "extraAttempts": 1, "reason": "Máy hỏng",
		})
		refused("an override naming one student in two spellings", status, body, "studentIds")
		if listed := teacher.must(http.StatusOK, http.MethodGet, path, nil); len(listed["items"].([]any)) != 0 {
			t.Errorf("a refused override left %v", listed["items"])
		}
		set := teacher.must(http.StatusOK, http.MethodPut, path, map[string]any{
			"studentIds": []string{studentID}, "extraAttempts": 1, "reason": "Máy hỏng",
		})
		if items, _ := set["items"].([]any); len(items) != 1 {
			t.Errorf("the override naming the student once answered %v", set)
		}
	})

	t.Run("the assignments list", func(t *testing.T) {
		query := "classId=" + url.QueryEscape(classID) + "&classId=" + url.QueryEscape(upper(classID))
		status, body := teacher.call(http.MethodGet, "/teacher/assignments?"+query, nil)
		refused("a list naming one class in two spellings", status, body, "classId")
		if listed := teacher.must(http.StatusOK, http.MethodGet, "/teacher/assignments?classId="+classID, nil); listed["total"] == float64(0) {
			t.Errorf("the list naming the class once is empty: %v", listed)
		}
	})
}

//go:build e2e

package e2e

import (
	"net/http"
	"testing"
	"time"
)

// publishedVersion authors one single-choice question, puts it on a test and publishes it.
func (c *client) publishedVersion(title string) (testID, versionID string) {
	c.w.t.Helper()
	question := c.must(http.StatusCreated, http.MethodPost, "/admin/questions", map[string]any{
		"type":   "single_choice",
		"prompt": "Which word is a noun?",
		"points": 1,
		"options": []map[string]any{
			{"text": "quickly", "isCorrect": false},
			{"text": "teacher", "isCorrect": true},
		},
	})
	test := c.must(http.StatusCreated, http.MethodPost, "/admin/tests", map[string]any{"title": title})
	c.must(http.StatusOK, http.MethodPatch, "/admin/tests/"+id(test), map[string]any{
		"expectedUpdatedAt": test["updatedAt"],
		"sections":          []map[string]any{{"title": "Phần 1", "questionIds": []string{id(question)}}},
	})
	version := c.must(http.StatusCreated, http.MethodPost, "/admin/tests/"+id(test)+"/publish", nil)
	return id(test), id(version)
}

func (c *client) class(name string) string {
	c.w.t.Helper()
	return id(c.must(http.StatusCreated, http.MethodPost, "/admin/classes", map[string]any{"name": name}))
}

func (c *client) assign(versionID, classID string) map[string]any {
	c.w.t.Helper()
	now := time.Now()
	return c.must(http.StatusCreated, http.MethodPost, "/admin/assignments", map[string]any{
		"testVersionId":   versionID,
		"targets":         map[string]any{"classIds": []string{classID}, "studentIds": []string{}},
		"window":          map[string]any{"opensAt": rfc3339(now.Add(-time.Minute)), "closesAt": rfc3339(now.Add(2 * time.Hour))},
		"durationMinutes": 30,
		"maxAttempts":     1,
		"review":          map[string]any{"showScore": true, "showCorrectAnswers": false, "showExplanations": false},
		"integrity": map[string]any{
			"requireFullscreen": false, "blockCopyPaste": true, "maxFocusLoss": 0, "onLimitExceeded": "flag", "minAwayMs": 3000,
		},
	})
}

func TestATeacherAuthorsPublishesAndAssignsATest(t *testing.T) {
	w := boot(t)
	email, password := w.teacher()
	teacher := w.browser()
	teacher.login(email, password)

	testID, versionID := teacher.publishedVersion("Unit 5 — nouns " + nonce(t))
	got := teacher.must(http.StatusOK, http.MethodGet, "/admin/tests/"+testID, nil)
	if got["status"] != "published" || got["currentVersion"].(float64) != 1 {
		t.Fatalf("after publishing the test reads %v", got)
	}

	classID := teacher.class("Lớp E2E " + nonce(t))
	assignment := teacher.assign(versionID, classID)
	if assignment["status"] != "open" {
		t.Fatalf("a window that opened a minute ago is %v, want open", assignment["status"])
	}

	listed := teacher.must(http.StatusOK, http.MethodGet, "/admin/assignments?status=open&classId="+classID, nil)
	items, _ := listed["items"].([]any)
	found := false
	for _, item := range items {
		if item.(map[string]any)["id"] == assignment["id"] {
			found = true
		}
	}
	if !found {
		t.Fatalf("the new assignment is missing from the class's open list: %v", listed)
	}
}

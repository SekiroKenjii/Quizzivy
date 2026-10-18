//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type ownedRows struct {
	Classes, Tests, Questions, Groups, Media, Assignments int
}

func (w *world) ownedBy(userID string) ownedRows {
	w.t.Helper()
	var o ownedRows
	err := w.pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM app.classes WHERE teacher_id = $1::uuid)::int,
		       (SELECT count(*) FROM app.tests WHERE owner_id = $1::uuid)::int,
		       (SELECT count(*) FROM app.questions WHERE owner_id = $1::uuid)::int,
		       (SELECT count(*) FROM app.question_groups WHERE owner_id = $1::uuid)::int,
		       (SELECT count(*) FROM app.media_assets WHERE owner_id = $1::uuid)::int,
		       (SELECT count(*) FROM app.assignments WHERE created_by = $1::uuid)::int`, userID).
		Scan(&o.Classes, &o.Tests, &o.Questions, &o.Groups, &o.Media, &o.Assignments)
	if err != nil {
		w.t.Fatal(err)
	}
	return o
}

func (w *world) liveOwnedBy(userID string) map[string]float64 {
	w.t.Helper()
	var o ownedRows
	err := w.pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM app.classes WHERE teacher_id = $1::uuid)::int,
		       (SELECT count(*) FROM app.tests WHERE owner_id = $1::uuid AND deleted_at IS NULL)::int,
		       (SELECT count(*) FROM app.questions WHERE owner_id = $1::uuid AND deleted_at IS NULL)::int,
		       (SELECT count(*) FROM app.question_groups WHERE owner_id = $1::uuid AND archived_at IS NULL)::int,
		       (SELECT count(*) FROM app.media_assets WHERE owner_id = $1::uuid AND deleted_at IS NULL)::int,
		       (SELECT count(*) FROM app.assignments WHERE created_by = $1::uuid)::int`, userID).
		Scan(&o.Classes, &o.Tests, &o.Questions, &o.Groups, &o.Media, &o.Assignments)
	if err != nil {
		w.t.Fatal(err)
	}
	return map[string]float64{
		"classes": float64(o.Classes), "tests": float64(o.Tests), "questions": float64(o.Questions),
		"questionGroups": float64(o.Groups), "media": float64(o.Media), "assignments": float64(o.Assignments),
	}
}

func lists(t *testing.T, c *client, path, id string) bool {
	t.Helper()
	body := c.must(http.StatusOK, http.MethodGet, path, nil)
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(string(raw), id)
}

func TestADisabledTeachersContentMovesToAnotherTeacherWhoListsAndEditsItWhileStudentsKeepTheirWork(t *testing.T) {
	w := bootWithStorage(t)
	a := w.teacherWorld("chuyen")
	adminEmail, adminPassword := w.createStaff("admin")
	admin := w.signedIn(adminEmail, adminPassword)
	heirEmail, heirPassword := w.createStaff("teacher")
	heir := w.signedIn(heirEmail, heirPassword)
	heirID := w.userID(heirEmail)
	thirdEmail, thirdPassword := w.createStaff("teacher")
	third := w.signedIn(thirdEmail, thirdPassword)
	path := "/admin/users/" + a.userID + "/transfer-content"
	var attemptsBefore int
	if err := w.pool.QueryRow(context.Background(), `
		SELECT count(*)::int FROM app.attempts t JOIN app.assignments s ON s.id = t.assignment_id WHERE s.created_by = $1::uuid`, a.userID).Scan(&attemptsBefore); err != nil {
		t.Fatal(err)
	}

	if status, body := admin.call(http.MethodPost, path, map[string]any{"toUserId": heirID}); status != http.StatusConflict || errorCode(body) != "USER_NOT_DISABLED" {
		t.Fatalf("moving an active account's content: %d %v, want 409 USER_NOT_DISABLED", status, body)
	}
	admin.must(http.StatusOK, http.MethodPost, "/admin/users/"+a.userID+"/disable", nil)
	for label, to := range map[string]string{
		"the account itself": a.userID, "a student": a.ids["student"], "nobody": "01935000-0000-7000-8000-00000000fffe",
	} {
		if status, body := admin.call(http.MethodPost, path, map[string]any{"toUserId": to}); status != http.StatusUnprocessableEntity || errorCode(body) != "INVALID_RECIPIENT" {
			t.Errorf("moving to %s: %d %v, want 422 INVALID_RECIPIENT", label, status, body)
		}
	}
	if status, _ := heir.call(http.MethodPost, path, map[string]any{"toUserId": heirID}); status != http.StatusForbidden {
		t.Errorf("a teacher moving content answered %d, want 403", status)
	}

	owned, live := w.ownedBy(a.userID), w.liveOwnedBy(a.userID)
	var unfinished float64
	if err := w.pool.QueryRow(context.Background(), `
		SELECT count(*)::float8 FROM app.word_imports WHERE created_by = $1::uuid AND status NOT IN ('committed', 'cancelled')`, a.userID).Scan(&unfinished); err != nil {
		t.Fatal(err)
	}
	result := admin.must(http.StatusOK, http.MethodPost, path, map[string]any{"toUserId": heirID, "notify": true})

	moved, _ := result["moved"].(map[string]any)
	for key, n := range live {
		if moved[key] != n {
			t.Errorf("moved.%s = %v, want %v: %v", key, moved[key], n, result)
		}
	}
	if left, _ := result["leftBehind"].(map[string]any); left["imports"] != unfinished || unfinished < 1 {
		t.Errorf("leftBehind = %v, want the %v unfinished imports", left, unfinished)
	}
	if after := w.ownedBy(a.userID); after != (ownedRows{}) {
		t.Errorf("the disabled account still owns %+v", after)
	}
	if got := w.ownedBy(heirID); got != owned {
		t.Errorf("the recipient owns %+v, want everything the account owned, deleted rows too: %+v", got, owned)
	}
	var stillMadeBy int
	if err := w.pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM app.tests WHERE created_by = $1::uuid) + (SELECT count(*) FROM app.questions WHERE created_by = $1::uuid)
		     + (SELECT count(*) FROM app.media_assets WHERE uploaded_by = $1::uuid)`, a.userID).Scan(&stillMadeBy); err != nil {
		t.Fatal(err)
	}
	if stillMadeBy == 0 {
		t.Error("the transfer rewrote who made the rows; provenance must stay")
	}
	var audited string
	if err := w.pool.QueryRow(context.Background(), `
		SELECT diff->>'to' FROM app.audit_log WHERE action = 'user.content_transferred' AND entity_id = $1::uuid`, a.userID).Scan(&audited); err != nil || audited != heirID {
		t.Errorf("audit row names %q (%v), want the recipient", audited, err)
	}

	for p, id := range map[string]string{
		"/teacher/tests": a.ids["test"], "/teacher/questions": a.ids["question"], "/teacher/question-groups": a.ids["question-group"],
		"/teacher/media": a.ids["media"], "/teacher/classes": a.ids["class"], "/teacher/assignments": a.ids["assignment"],
	} {
		if !lists(t, heir, p, id) {
			t.Errorf("the recipient's %s does not hold %s", p, id)
		}
		if lists(t, third, p, id) {
			t.Errorf("a third teacher's %s holds %s", p, id)
		}
	}
	test := heir.must(http.StatusOK, http.MethodGet, "/teacher/tests/"+a.ids["test"], nil)
	heir.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+a.ids["test"], map[string]any{
		"expectedUpdatedAt": test["updatedAt"], "title": "Đề đã nhận " + nonce(t),
	})
	heir.must(http.StatusOK, http.MethodGet, "/teacher/attempts/"+a.ids["attempt"], nil)
	for _, p := range []string{"/teacher/tests/" + a.ids["test"], "/teacher/classes/" + a.ids["class"], "/teacher/attempts/" + a.ids["attempt"]} {
		if status, _ := third.call(http.MethodGet, p, nil); status != http.StatusNotFound {
			t.Errorf("a third teacher reading %s: %d, want 404", p, status)
		}
	}

	a.student.c.must(http.StatusOK, http.MethodGet, "/app/assignments/"+a.student.ids["assignment"], nil)
	var attemptsAfter int
	if err := w.pool.QueryRow(context.Background(), `
		SELECT count(*)::int FROM app.attempts t JOIN app.assignments s ON s.id = t.assignment_id WHERE s.created_by = $1::uuid`, heirID).Scan(&attemptsAfter); err != nil {
		t.Fatal(err)
	}
	if attemptsAfter != attemptsBefore || attemptsBefore == 0 {
		t.Errorf("the moved assignments hold %d attempts, want the %d they held", attemptsAfter, attemptsBefore)
	}

	if !lists(t, heir, "/me/notifications", "content.transferred") {
		t.Error("the recipient's bell holds no content.transferred notification")
	}
	if status, body := admin.call(http.MethodPost, path, map[string]any{"toUserId": w.userID(adminEmail)}); status != http.StatusOK {
		t.Errorf("moving to an Admin, who holds the teacher workspace: %d %v, want 200", status, body)
	}
}

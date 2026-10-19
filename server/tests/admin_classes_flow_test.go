//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

func TestAnAdminGivesAClassToAnotherTeacherWhoThenTeachesItAndItsAssignments(t *testing.T) {
	w := bootWithStorage(t)
	a := w.teacherWorld("doi-giao-vien")
	adminEmail, adminPassword := w.createStaff("admin")
	admin := w.signedIn(adminEmail, adminPassword)
	heirEmail, heirPassword := w.createStaff("teacher")
	heir := w.signedIn(heirEmail, heirPassword)
	heirID := w.userID(heirEmail)
	classID := a.ids["class"]
	change := "/admin/classes/" + classID + "/teacher"

	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/admin/classes"}, {http.MethodPost, "/admin/classes"}, {http.MethodPut, change},
	} {
		body := map[string]any{"name": "Lớp", "teacherId": heirID}
		if status, _ := heir.call(c.method, c.path, body); status != http.StatusForbidden {
			t.Errorf("a teacher's %s %s: %d, want 403", c.method, c.path, status)
		}
	}
	if !lists(t, admin, "/admin/classes?limit=50", classID) {
		t.Fatal("the Admin's list does not hold the teacher's class")
	}
	if status, body := admin.call(http.MethodPut, change, map[string]any{"teacherId": a.student.userID}); status != http.StatusUnprocessableEntity || errorCode(body) != "INVALID_RECIPIENT" {
		t.Errorf("giving the class to a student: %d %v, want 422 INVALID_RECIPIENT", status, body)
	}

	moved := admin.must(http.StatusOK, http.MethodPut, change, map[string]any{"teacherId": heirID, "notify": true})
	if moved["id"] != classID {
		t.Fatalf("the change answered %v", moved)
	}
	if !lists(t, heir, "/teacher/classes", classID) || !lists(t, heir, "/teacher/assignments", a.ids["assignment"]) {
		t.Error("the new teacher does not list the class and its assignment")
	}
	if status, _ := a.c.call(http.MethodGet, "/teacher/classes/"+classID, nil); status != http.StatusNotFound {
		t.Errorf("the previous teacher reading the class: %d, want 404", status)
	}
	if !lists(t, heir, "/me/notifications", "class.teacher_changed") || !lists(t, a.c, "/me/notifications", "class.teacher_changed") {
		t.Error("a teacher's bell holds no class.teacher_changed item")
	}

	created := admin.must(http.StatusCreated, http.MethodPost, "/admin/classes", map[string]any{
		"name": "Lớp do quản trị tạo " + nonce(t), "teacherId": heirID, "scheduleLabel": "Thứ 7 · 9:00",
	})
	if created["joinCode"] == nil || created["selfJoinEnabled"] != true {
		t.Errorf("the class made for the teacher is %v, want self-join on with a code", created)
	}
	if !lists(t, heir, "/teacher/classes", created["id"].(string)) {
		t.Error("the teacher does not list the class made for them")
	}
}

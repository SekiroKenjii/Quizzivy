//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"quizzivy/internal/platform/config"
)

type staffRig struct {
	w        *world
	admin    *client
	adminID  string
	teacher  *client
	person   string
	email    string
	password string
}

func (w *world) userID(email string) string {
	w.t.Helper()
	var id string
	if err := w.pool.QueryRow(context.Background(), `SELECT id::text FROM app.users WHERE lower(email) = lower($1)`, email).Scan(&id); err != nil {
		w.t.Fatal(err)
	}
	return id
}

func (w *world) roleID(builtin string) string {
	w.t.Helper()
	var id string
	if err := w.pool.QueryRow(context.Background(), `SELECT id::text FROM app.roles WHERE builtin_key = $1`, builtin).Scan(&id); err != nil {
		w.t.Fatal(err)
	}
	return id
}

func newStaffRig(t *testing.T, configure ...func(*config.Config)) *staffRig {
	t.Helper()
	w := boot(t, configure...)
	adminEmail, adminPassword := w.createStaff("admin")
	r := &staffRig{w: w, admin: w.browser(), teacher: w.browser()}
	r.admin.login(adminEmail, adminPassword)
	r.adminID = w.userID(adminEmail)
	r.email, r.password = w.createStaff("teacher")
	r.person = w.userID(r.email)
	r.teacher.login(r.email, r.password)
	r.teacher.must(http.StatusOK, http.MethodGet, "/teacher/dashboard", nil)
	return r
}

func (r *staffRig) act(status int, action string, payload any) map[string]any {
	r.w.t.Helper()
	return r.admin.must(status, http.MethodPost, "/admin/users/"+r.person+"/"+action, payload)
}

func (r *staffRig) signedOut(t *testing.T, why string) {
	t.Helper()
	if status, body := r.teacher.call(http.MethodGet, "/teacher/dashboard", nil); status != http.StatusUnauthorized {
		t.Errorf("after %s the next request answered %d %v, want 401", why, status, body)
	}
	if status, _ := r.teacher.call(http.MethodPost, "/auth/refresh", nil); status != http.StatusUnauthorized {
		t.Errorf("after %s a refresh answered %d, want 401: the session must not be renewable", why, status)
	}
}

func TestDisablingAnAccountEndsItsSessionsAtOnceAndEnablingLetsItBackIn(t *testing.T) {
	r := newStaffRig(t)

	disabled := r.act(http.StatusOK, "disable", nil)
	if disabled["status"] != "disabled" {
		t.Errorf("the row after a disable says %v, want disabled", disabled["status"])
	}
	r.signedOut(t, "a disable")
	if status, _ := r.w.browser().call(http.MethodPost, "/auth/login", map[string]any{"email": r.email, "password": r.password}); status == http.StatusOK {
		t.Error("a disabled account signed in")
	}
	if status, _ := r.admin.call(http.MethodPost, "/admin/users/"+r.person+"/reset-password", map[string]any{"delivery": "temporary_password"}); status != http.StatusConflict {
		t.Errorf("resetting a disabled account answered %d, want 409", status)
	}

	enabled := r.act(http.StatusOK, "enable", nil)
	if enabled["status"] == "disabled" {
		t.Errorf("the row after an enable says %v", enabled["status"])
	}
	back := r.w.browser()
	back.login(r.email, r.password)
	back.must(http.StatusOK, http.MethodGet, "/teacher/dashboard", nil)
	r.signedOut(t, "an enable of the account whose sessions the disable had ended")
}

func TestSigningAnAccountOutEndsItsSessionsAndLeavesItEnabled(t *testing.T) {
	r := newStaffRig(t)

	r.act(http.StatusNoContent, "sign-out", nil)

	r.signedOut(t, "a sign-out")
	again := r.w.browser()
	again.login(r.email, r.password)
	again.must(http.StatusOK, http.MethodGet, "/teacher/dashboard", nil)

	r.admin.must(http.StatusNoContent, http.MethodPost, "/admin/users/"+r.adminID+"/sign-out", nil)
	if status, _ := r.admin.call(http.MethodGet, "/admin/users", nil); status != http.StatusUnauthorized {
		t.Errorf("an Admin who signed themselves out was answered %d, want 401", status)
	}
}

func TestAnAdminMayNotDisableOrResetOrDemoteThemselvesAndAnUnknownAccountIsNotFound(t *testing.T) {
	r := newStaffRig(t)
	self := "/admin/users/" + r.adminID
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, self + "/disable", nil},
		{http.MethodPost, self + "/reset-password", map[string]any{"delivery": "temporary_password"}},
		{http.MethodPut, self + "/role", map[string]any{"roleId": r.w.roleID("teacher")}},
	} {
		status, body := r.admin.call(c.method, c.path, c.body)
		failure, _ := body["error"].(map[string]any)
		if status != http.StatusUnprocessableEntity || failure["code"] != "CANNOT_ACT_ON_SELF" {
			t.Errorf("%s %s: %d %v, want 422 CANNOT_ACT_ON_SELF", c.method, c.path, status, body)
		}
	}
	missing := "/admin/users/01935000-0000-7000-8000-00000000fffe"
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, missing + "/disable"}, {http.MethodPost, missing + "/enable"}, {http.MethodPost, missing + "/sign-out"},
	} {
		if status, _ := r.admin.call(c.method, c.path, nil); status != http.StatusNotFound {
			t.Errorf("%s %s: %d, want 404", c.method, c.path, status)
		}
	}
	if status, _ := r.teacher.call(http.MethodPost, "/admin/users/"+r.person+"/disable", nil); status != http.StatusForbidden {
		t.Errorf("a Teacher disabling an account answered %d, want 403", status)
	}
}

func TestChangingARoleTakesEffectAtOnceAndAnOwnerMustTransferFirst(t *testing.T) {
	r := newStaffRig(t)
	student := r.w.roleID("student")
	if _, err := r.w.pool.Exec(context.Background(), `INSERT INTO app.classes (name, teacher_id) VALUES ('Lớp giữ', $1::uuid)`, r.person); err != nil {
		t.Fatal(err)
	}

	status, body := r.admin.call(http.MethodPut, "/admin/users/"+r.person+"/role", map[string]any{"roleId": student})
	failure, _ := body["error"].(map[string]any)
	details, _ := failure["details"].(map[string]any)
	if status != http.StatusConflict || failure["code"] != "USER_OWNS_CONTENT" || details["classes"] != 1.0 {
		t.Fatalf("an owner moved to Student: %d %v, want 409 USER_OWNS_CONTENT with one class", status, body)
	}
	r.teacher.must(http.StatusOK, http.MethodGet, "/teacher/dashboard", nil)

	if _, err := r.w.pool.Exec(context.Background(), `DELETE FROM app.classes WHERE teacher_id = $1::uuid`, r.person); err != nil {
		t.Fatal(err)
	}
	row := r.admin.must(http.StatusOK, http.MethodPut, "/admin/users/"+r.person+"/role", map[string]any{"roleId": student})
	if role, _ := row["role"].(map[string]any); role["builtinKey"] != "student" {
		t.Errorf("the row after the change names the role %v", row["role"])
	}
	r.signedOut(t, "a role change")
	again := r.w.browser()
	again.login(r.email, r.password)
	if status, _ := again.call(http.MethodGet, "/teacher/dashboard", nil); status != http.StatusForbidden {
		t.Errorf("a Student asking for the teacher dashboard answered %d, want 403: the new role must apply on the next request", status)
	}
}

func TestATemporaryPasswordMustBeChangedBeforeAnythingElseWorks(t *testing.T) {
	r := newStaffRig(t)

	reset := r.act(http.StatusOK, "reset-password", map[string]any{"delivery": "temporary_password"})
	temporary, _ := reset["temporaryPassword"].(string)
	if temporary == "" {
		t.Fatalf("no temporary password came back: %v", reset)
	}
	r.signedOut(t, "a reset")
	if status, _ := r.w.browser().call(http.MethodPost, "/auth/login", map[string]any{"email": r.email, "password": r.password}); status == http.StatusOK {
		t.Error("the old password still signs in after a reset")
	}

	person := r.w.browser()
	person.login(r.email, temporary)
	status, body := person.call(http.MethodGet, "/teacher/dashboard", nil)
	failure, _ := body["error"].(map[string]any)
	if status != http.StatusForbidden || failure["code"] != "PASSWORD_CHANGE_REQUIRED" {
		t.Fatalf("a teacher operation on a temporary password: %d %v, want 403 PASSWORD_CHANGE_REQUIRED", status, body)
	}
	me := person.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	if me["mustChangePassword"] != true {
		t.Errorf("/auth/me says mustChangePassword = %v", me["mustChangePassword"])
	}
	person.must(http.StatusOK, http.MethodPost, "/auth/refresh", nil)
	person.must(http.StatusNoContent, http.MethodPost, "/auth/logout", nil)

	again := r.w.browser()
	again.login(r.email, temporary)
	again.must(http.StatusNoContent, http.MethodPost, "/auth/change-password", map[string]any{"newPassword": "mat-khau-moi-dai-" + nonce(t)})
	again.must(http.StatusOK, http.MethodGet, "/teacher/dashboard", nil)
}

func TestALinkResetClearsThePasswordAtOnceAndQueuesOneMessage(t *testing.T) {
	r := newStaffRig(t, func(c *config.Config) { c.SecretsKey = []byte(importSecretsKey) })

	r.act(http.StatusOK, "reset-password", map[string]any{"delivery": "email_link"})

	r.signedOut(t, "a link reset")
	if status, _ := r.w.browser().call(http.MethodPost, "/auth/login", map[string]any{"email": r.email, "password": r.password}); status == http.StatusOK {
		t.Error("the old password still signs in after a link reset: it must stop working at once")
	}
	var hasPassword bool
	var mails, links int
	if err := r.w.pool.QueryRow(context.Background(), `
		SELECT u.password_hash IS NOT NULL,
		       (SELECT count(*) FROM app.email_outbox o WHERE o.to_user_id = u.id AND o.template = 'password_reset_link'),
		       (SELECT count(*) FROM app.set_password_tokens l WHERE l.user_id = u.id AND l.purpose = 'reset' AND l.used_at IS NULL AND l.revoked_at IS NULL)
		  FROM app.users u WHERE u.id = $1::uuid`, r.person).Scan(&hasPassword, &mails, &links); err != nil {
		t.Fatal(err)
	}
	if hasPassword || mails != 1 || links != 1 {
		t.Errorf("password kept %v, %d reset messages, %d live reset links, want none, one and one", hasPassword, mails, links)
	}
}

func TestABulkDisableAndABulkResetDoWhatTheyCanAndNameWhatTheyCould(t *testing.T) {
	r := newStaffRig(t)
	otherEmail, _ := r.w.createStaff("teacher")
	other := r.w.userID(otherEmail)
	missing := "01935000-0000-7000-8000-00000000fffe"

	reset := r.admin.must(http.StatusOK, http.MethodPost, "/admin/users/reset-passwords", map[string]any{
		"userIds": []string{r.person, missing, r.adminID, other}, "delivery": "temporary_password",
	})
	var resetIDs []string
	for _, item := range reset["items"].([]any) {
		entry := item.(map[string]any)
		if entry["temporaryPassword"] == "" || entry["temporaryPassword"] == nil || entry["email"] == nil || entry["fullName"] == nil {
			t.Errorf("an item lacks its password, email or name: %v", entry)
		}
		resetIDs = append(resetIDs, entry["userId"].(string))
	}
	if !slices.Equal(resetIDs, []string{r.person, other}) {
		t.Errorf("reset %v, want the two teachers in the order named", resetIDs)
	}
	failed := map[string]string{}
	for _, item := range reset["failed"].([]any) {
		entry := item.(map[string]any)
		failed[entry["userId"].(string)] = entry["code"].(string)
	}
	if failed[missing] != "NOT_FOUND" || failed[r.adminID] != "CANNOT_ACT_ON_SELF" || len(failed) != 2 {
		t.Errorf("failed = %v, want the unknown account and the caller", failed)
	}

	disabled := r.admin.must(http.StatusOK, http.MethodPost, "/admin/users/disable", map[string]any{"userIds": []string{other, missing, r.adminID, r.person}})
	var done []string
	for _, id := range disabled["disabled"].([]any) {
		done = append(done, id.(string))
	}
	if !slices.Equal(done, []string{other, r.person}) {
		t.Errorf("disabled %v, want the two teachers", done)
	}
	if got := disabled["failed"].([]any); len(got) != 2 {
		t.Errorf("failed = %v, want two", got)
	}

	if status, body := r.admin.call(http.MethodPost, "/admin/users/disable", map[string]any{"userIds": []string{other, other}}); status != http.StatusBadRequest {
		t.Errorf("a repeated id answered %d %v, want 400", status, body)
	}
	if status, _ := r.admin.call(http.MethodPost, "/admin/users/disable", map[string]any{"userIds": []string{}}); status != http.StatusBadRequest {
		t.Errorf("an empty list answered %d, want 400 from the contract", status)
	}
}

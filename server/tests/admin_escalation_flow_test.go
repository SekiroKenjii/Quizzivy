//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"
)

type escalationRig struct {
	w        *world
	admin    *client
	adminID  string
	lead     *client
	leadID   string
	leadRole string
	empty    string
}

func refusalCode(body map[string]any) string {
	failure, _ := body["error"].(map[string]any)
	code, _ := failure["code"].(string)
	return code
}

func (w *world) createWithRole(roleID string) (email, password string) {
	w.t.Helper()
	email, password = w.createStaff("teacher")
	if _, err := w.pool.Exec(context.Background(), `UPDATE app.users SET role_id = $2::uuid WHERE lower(email) = lower($1)`, email, roleID); err != nil {
		w.t.Fatal(err)
	}
	return email, password
}

func (w *world) dropRoleAfter(roleID string) {
	w.t.Helper()
	w.t.Cleanup(func() {
		for _, stmt := range []string{
			`DELETE FROM app.audit_log WHERE entity_id = $1::uuid OR entity_id IN (SELECT id FROM app.users WHERE role_id = $1::uuid)`,
			`DELETE FROM app.users WHERE role_id = $1::uuid`,
			`DELETE FROM app.role_permissions WHERE role_id = $1::uuid`,
			`DELETE FROM app.roles WHERE id = $1::uuid`,
		} {
			if _, err := w.pool.Exec(context.Background(), stmt, roleID); err != nil {
				w.t.Errorf("cleanup %q: %v", stmt, err)
			}
		}
	})
}

func (r *escalationRig) newRole(name string, grants ...string) string {
	r.w.t.Helper()
	created := r.admin.must(http.StatusCreated, http.MethodPost, "/admin/roles", map[string]any{"name": name + " " + nonce(r.w.t)})
	roleID := id(created)
	r.w.dropRoleAfter(roleID)
	if len(grants) == 0 {
		return roleID
	}
	changes := make([]map[string]any, 0, len(grants))
	for _, key := range grants {
		changes = append(changes, map[string]any{"roleId": roleID, "key": key, "granted": true})
	}
	r.admin.must(http.StatusOK, http.MethodPut, "/admin/roles/permissions", map[string]any{
		"changes":           changes,
		"expectedRevisions": []map[string]any{{"roleId": roleID, "revision": created["revision"]}},
	})
	return roleID
}

func (r *escalationRig) role(roleID string) map[string]any {
	r.w.t.Helper()
	matrix := r.admin.must(http.StatusOK, http.MethodGet, "/admin/roles", nil)
	for _, item := range matrix["roles"].([]any) {
		if role := item.(map[string]any); role["id"] == roleID {
			return role
		}
	}
	r.w.t.Fatalf("the matrix holds no role %s", roleID)
	return nil
}

func newEscalationRig(t *testing.T) *escalationRig {
	t.Helper()
	w := bootForUserImports(t)
	adminEmail, adminPassword := w.createStaff("admin")
	r := &escalationRig{w: w, admin: w.signedIn(adminEmail, adminPassword), adminID: w.userID(adminEmail)}
	r.leadRole = r.newRole("Tổ trưởng thử", "people.users.manage", "people.roles.manage")
	r.empty = r.newRole("Vai trò trống thử")
	leadEmail, leadPassword := w.createWithRole(r.leadRole)
	r.leadID = w.userID(leadEmail)
	r.lead = w.signedIn(leadEmail, leadPassword)
	r.lead.must(http.StatusOK, http.MethodGet, "/admin/users", nil)
	return r
}

func TestAManagerOfUsersAndRolesCannotReachPowerItLacks(t *testing.T) {
	r := newEscalationRig(t)
	adminRole := r.w.roleID("admin")
	otherEmail, _ := r.w.createStaff("admin")
	otherAdmin := r.w.userID(otherEmail)
	leadBefore, adminBefore := r.role(r.leadRole), r.role(adminRole)
	domain := "leo-quyen-" + nonce(t) + ".example.test"

	for _, c := range []struct {
		name, method, path string
		body               any
		status             int
		code               string
	}{
		{"creating an Admin", http.MethodPost, "/admin/users",
			map[string]any{"fullName": "Quản trị mới", "email": "admin@" + domain, "roleId": adminRole, "delivery": "temporary_password"},
			http.StatusForbidden, "TARGET_ROLE_NOT_ALLOWED"},
		{"resetting an Admin's password", http.MethodPost, "/admin/users/" + otherAdmin + "/reset-password",
			map[string]any{"delivery": "temporary_password"}, http.StatusForbidden, "TARGET_ROLE_NOT_ALLOWED"},
		{"disabling an Admin", http.MethodPost, "/admin/users/" + otherAdmin + "/disable", nil,
			http.StatusForbidden, "TARGET_ROLE_NOT_ALLOWED"},
		{"granting itself system.settings.write", http.MethodPut, "/admin/roles/permissions",
			map[string]any{
				"changes":           []map[string]any{{"roleId": r.leadRole, "key": "system.settings.write", "granted": true}},
				"expectedRevisions": []map[string]any{{"roleId": r.leadRole, "revision": leadBefore["revision"]}},
			}, http.StatusUnprocessableEntity, "CANNOT_ACT_ON_SELF"},
		{"granting another role a key it lacks", http.MethodPut, "/admin/roles/permissions",
			map[string]any{
				"changes":           []map[string]any{{"roleId": r.empty, "key": "system.settings.write", "granted": true}},
				"expectedRevisions": []map[string]any{{"roleId": r.empty, "revision": r.role(r.empty)["revision"]}},
			}, http.StatusForbidden, "TARGET_ROLE_NOT_ALLOWED"},
		{"copying the Admin role", http.MethodPost, "/admin/roles",
			map[string]any{"name": "Bản sao quản trị " + nonce(t), "copyFromRoleId": adminRole},
			http.StatusForbidden, "TARGET_ROLE_NOT_ALLOWED"},
		{"changing its own role", http.MethodPut, "/admin/users/" + r.leadID + "/role",
			map[string]any{"roleId": r.empty}, http.StatusUnprocessableEntity, "CANNOT_ACT_ON_SELF"},
	} {
		status, body := r.lead.call(c.method, c.path, c.body)
		if status != c.status || refusalCode(body) != c.code {
			t.Errorf("%s: %d %v, want %d %s", c.name, status, body, c.status, c.code)
		}
	}

	file := []byte("full_name,email,role\nQuản trị nhập,nhap@" + domain + ",admin\n")
	preview := r.lead.send(http.MethodPost, "/admin/user-imports/preview", ptr(importForm(t, file, importFields("sheet"))))
	issues, _ := preview.json["invalid"].([]any)
	if preview.status != http.StatusOK || len(issues) != 1 || issues[0].(map[string]any)["code"] != "ROLE_NOT_ALLOWED" {
		t.Errorf("previewing a row with role admin: %s, want the row refused with ROLE_NOT_ALLOWED", answer(preview))
	}
	created := r.lead.send(http.MethodPost, "/admin/user-imports", ptr(importForm(t, file, importFields("sheet", "requestId", uuid.NewString()))))
	if code, _ := importError(created); created.status != http.StatusUnprocessableEntity || code != "USER_IMPORT_NOTHING_TO_CREATE" {
		t.Errorf("importing a row with role admin: %s, want 422 USER_IMPORT_NOTHING_TO_CREATE", answer(created))
	}

	if n := r.w.accountsAt(domain); n != 0 {
		t.Errorf("%d accounts exist at %s", n, domain)
	}
	leadAfter, adminAfter := r.role(r.leadRole), r.role(adminRole)
	if leadAfter["revision"] != leadBefore["revision"] || !slices.Equal(keys(leadAfter), keys(leadBefore)) {
		t.Errorf("the caller's role moved from %v to %v", keys(leadBefore), keys(leadAfter))
	}
	if adminAfter["revision"] != adminBefore["revision"] {
		t.Errorf("the Admin role's revision moved from %v to %v", adminBefore["revision"], adminAfter["revision"])
	}
	if empty := r.role(r.empty); len(keys(empty)) != 0 {
		t.Errorf("the empty role now holds %v", keys(empty))
	}
	var copies int
	if err := r.w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.roles WHERE name LIKE 'Bản sao quản trị %'`).Scan(&copies); err != nil {
		t.Fatal(err)
	}
	if copies != 0 {
		t.Errorf("%d copies of the Admin role exist", copies)
	}
	var otherRole string
	var disabled bool
	if err := r.w.pool.QueryRow(context.Background(), `
		SELECT r.builtin_key, u.disabled_at IS NOT NULL FROM app.users u JOIN app.roles r ON r.id = u.role_id WHERE u.id = $1::uuid`, otherAdmin).Scan(&otherRole, &disabled); err != nil {
		t.Fatal(err)
	}
	if otherRole != "admin" || disabled {
		t.Errorf("the other Admin is now %s, disabled %v", otherRole, disabled)
	}
	if _, err := r.w.pool.Exec(context.Background(), `DELETE FROM app.users WHERE id = $1::uuid`, otherAdmin); err != nil {
		t.Errorf("cleanup of the other Admin: %v", err)
	}
}

func keys(role map[string]any) []string {
	var out []string
	for _, key := range role["permissions"].([]any) {
		out = append(out, key.(string))
	}
	slices.Sort(out)
	return out
}

func TestAnAdminCannotRemoveTheLastAdminWayOrLockEveryoneOut(t *testing.T) {
	r := newEscalationRig(t)
	self := "/admin/users/" + r.adminID
	for _, c := range []struct {
		name, method, path string
		body               any
	}{
		{"disabling themselves", http.MethodPost, self + "/disable", nil},
		{"demoting themselves", http.MethodPut, self + "/role", map[string]any{"roleId": r.w.roleID("teacher")}},
	} {
		status, body := r.admin.call(c.method, c.path, c.body)
		if status != http.StatusUnprocessableEntity || refusalCode(body) != "CANNOT_ACT_ON_SELF" {
			t.Errorf("%s: %d %v, want 422 CANNOT_ACT_ON_SELF", c.name, status, body)
		}
	}
	if status, body := r.admin.call(http.MethodDelete, self, nil); status != http.StatusNotFound {
		t.Errorf("deleting themselves: %d %v, want 404: a delete reaches only a disabled student", status, body)
	}
	bulk := r.admin.must(http.StatusOK, http.MethodPost, "/admin/users/disable", map[string]any{"userIds": []string{r.adminID}})
	if failed, _ := bulk["failed"].([]any); len(failed) != 1 {
		t.Errorf("a bulk disable naming only the caller answered %v, want the caller among the failures", bulk)
	}
	r.admin.must(http.StatusOK, http.MethodGet, "/admin/users", nil)

	settings := r.admin.must(http.StatusOK, http.MethodGet, "/admin/settings", nil)
	signIn := settings["signIn"].(map[string]any)
	locked := map[string]any{}
	for k, v := range signIn {
		locked[k] = v
	}
	locked["password"], locked["google"] = false, true
	locked["allowedGoogleDomains"] = []string{"khong-ai-" + nonce(t) + ".example.test"}
	status, body := r.admin.call(http.MethodPut, "/admin/settings", map[string]any{
		"organization":     settings["organization"],
		"signIn":           locked,
		"retention":        settings["retention"],
		"expectedRevision": settings["revision"],
	})
	if status != http.StatusUnprocessableEntity || refusalCode(body) != "SIGN_IN_POLICY_LOCKOUT" {
		t.Errorf("saving a policy that leaves the Admin no way in: %d %v, want 422 SIGN_IN_POLICY_LOCKOUT", status, body)
	}
	if after := r.admin.must(http.StatusOK, http.MethodGet, "/admin/settings", nil); after["revision"] != settings["revision"] {
		t.Errorf("the refused policy moved the revision from %v to %v", settings["revision"], after["revision"])
	}
}

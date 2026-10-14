//go:build e2e

package e2e

import (
	"context"
	"encoding/csv"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func (w *world) auditRow(actorEmail, action, entity string) {
	w.t.Helper()
	var id int64
	err := w.pool.QueryRow(context.Background(), `
		INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id, ip, user_agent, diff)
		SELECT u.id, $2, $3, gen_random_uuid(), '203.0.113.5'::inet, 'Mozilla/5.0 (X11; Linux x86_64) Firefox/120.0',
		       '{"name": {"old": "9A", "new": "9B"}, "code": "SECRET1234"}'::jsonb
		  FROM app.users u WHERE lower(u.email) = lower($1)
		RETURNING id`, actorEmail, action, entity).Scan(&id)
	if err != nil {
		w.t.Fatalf("audit row: %v", err)
	}
	w.t.Cleanup(func() {
		if _, err := w.pool.Exec(context.Background(), `DELETE FROM app.audit_log WHERE id = $1`, id); err != nil {
			w.t.Errorf("removing audit row %d: %v", id, err)
		}
	})
}

func TestOnlyAnAdminReadsTheAuditLogAndItsExportIsOnTheRecord(t *testing.T) {
	w := boot(t)
	adminEmail, adminPassword := w.createStaff("admin")
	teacherEmail, teacherPassword := w.createStaff("teacher")
	assistantEmail, assistantPassword := w.createStaff("assistant")
	token := strings.SplitN(strings.SplitN(teacherEmail, "@", 2)[0], "-", 2)[1]
	t.Cleanup(func() {
		if _, err := w.pool.Exec(context.Background(),
			`DELETE FROM app.audit_log WHERE actor_user_id IN (SELECT id FROM app.users WHERE lower(email) = lower($1))`, adminEmail); err != nil {
			t.Errorf("removing the export entries: %v", err)
		}
	})
	w.auditRow(teacherEmail, "class.created", "class")
	list := "/admin/audit-log?q=" + url.QueryEscape(token)

	if status, _, _ := w.browser().download(list); status != http.StatusUnauthorized {
		t.Errorf("without a token: %d, want 401", status)
	}
	for name, credentials := range map[string][2]string{"teacher": {teacherEmail, teacherPassword}, "assistant": {assistantEmail, assistantPassword}} {
		browser := w.browser()
		browser.login(credentials[0], credentials[1])
		for _, path := range []string{list, "/admin/audit-log/export"} {
			if status, _, _ := browser.download(path); status != http.StatusForbidden {
				t.Errorf("%s on %s: %d, want 403", name, path, status)
			}
		}
	}

	admin := w.browser()
	admin.login(adminEmail, adminPassword)
	page := admin.must(http.StatusOK, http.MethodGet, list, nil)
	items := page["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("%d entries for the teacher's search, want 1: %v", len(items), page)
	}
	entry := items[0].(map[string]any)
	if entry["action"] != "class.created" || entry["category"] != "classes" || entry["actor"].(map[string]any)["name"] != "Cô Thương" {
		t.Errorf("entry %v", entry)
	}
	if device := entry["device"]; device != "Linux · Firefox" {
		t.Errorf("device %v", device)
	}
	facets := page["facets"].(map[string]any)
	if facets["all"] != float64(1) || facets["classes"] != float64(1) || facets["tests"] != float64(0) {
		t.Errorf("facets %v", facets)
	}
	if status, body := admin.call(http.MethodGet, "/admin/audit-log?range=custom", nil); status != http.StatusBadRequest || body["error"].(map[string]any)["code"] != "VALIDATION_FAILED" {
		t.Errorf("a custom range with no days: %d %v", status, body)
	}

	status, header, file := admin.download("/admin/audit-log/export?format=csv&period=7d&q=" + url.QueryEscape(token))
	if status != http.StatusOK || !strings.HasPrefix(header.Get("Content-Disposition"), "attachment; filename=\"audit-log-") || header.Get("Cache-Control") != "no-store" {
		t.Fatalf("export: %d %v", status, header)
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(file), "\xEF\xBB\xBF"))).ReadAll()
	if err != nil || len(rows) != 2 {
		t.Fatalf("the file has %d rows (%v), want its header and the one entry: %s", len(rows), err, file)
	}
	if strings.Contains(string(file), "SECRET1234") || !strings.Contains(string(file), "••••-1234") {
		t.Errorf("the file carries a join code unhinted: %s", file)
	}

	adminToken := strings.SplitN(strings.SplitN(adminEmail, "@", 2)[0], "-", 2)[1]
	recorded := admin.must(http.StatusOK, http.MethodGet, "/admin/audit-log?q="+url.QueryEscape(adminToken), nil)
	entries := recorded["items"].([]any)
	if len(entries) != 1 || entries[0].(map[string]any)["action"] != "audit.exported" {
		t.Fatalf("the export is not on the record: %v", recorded)
	}
	if facetsOf := recorded["facets"].(map[string]any); facetsOf["all"] != float64(1) || facetsOf["accounts"] != float64(0) {
		t.Errorf("an export is listed under All only: %v", facetsOf)
	}
}

//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"quizzivy/internal/platform/config"
)

var adminReadings = []string{"/admin/overview", "/admin/summary", "/admin/usage"}

func walk(body map[string]any, path ...string) any {
	var at any = body
	for _, key := range path {
		m, ok := at.(map[string]any)
		if !ok {
			return nil
		}
		at = m[key]
	}
	return at
}

func figure(t *testing.T, body map[string]any, path ...string) int {
	t.Helper()
	n, ok := walk(body, path...).(float64)
	if !ok {
		t.Fatalf("%v is not a number in %v", path, body)
	}
	return int(n)
}

func attentionRow(body map[string]any, kind string) (count int, detail map[string]any) {
	rows, _ := body["attention"].([]any)
	for _, row := range rows {
		r := row.(map[string]any)
		if r["kind"] == kind {
			detail, _ = r["detail"].(map[string]any)
			return int(r["count"].(float64)), detail
		}
	}
	return 0, nil
}

func planned(c *config.Config) {
	c.Plan = config.Plan{Name: "Gói thử", SeatLimit: 1_000_000, TeacherLimit: 100_000, MediaBytes: 1 << 40, ImportBytes: 1 << 40}
}

func TestTheAdminReadingsAreOpenedByTheKeysACallerHolds(t *testing.T) {
	w := boot(t, planned)

	anonymous := w.browser()
	for _, path := range adminReadings {
		if status, _ := anonymous.call(http.MethodGet, path, nil); status != http.StatusUnauthorized {
			t.Errorf("%s without a token: %d, want 401", path, status)
		}
	}
	teacher := w.signedIn(w.createStaff("teacher"))
	assistant := w.signedIn(w.createStaff("assistant"))
	for name, who := range map[string]*client{"a teacher": teacher, "an assistant": assistant} {
		for _, path := range adminReadings {
			if status, _ := who.call(http.MethodGet, path, nil); status != http.StatusForbidden {
				t.Errorf("%s on %s: %d, want 403", name, path, status)
			}
		}
	}

	sections := []string{"people", "classes", "dailyActive", "recentChanges", "storage"}
	cases := []struct {
		name  string
		key   string
		opens []string
		usage int
	}{
		{"a custom role holding only the audit key", "system.audit.read", []string{"recentChanges"}, http.StatusForbidden},
		{"a manager of people", "people.users.manage", []string{"people", "dailyActive"}, http.StatusForbidden},
		{"the owner of the settings", "system.settings.write", []string{"storage"}, http.StatusOK},
		{"a custom role holding only the AI credits key", "ai.credits.manage", nil, http.StatusForbidden},
	}
	for _, c := range cases {
		who := w.signedIn(w.staffHolding(c.key))
		overview := who.must(http.StatusOK, http.MethodGet, "/admin/overview", nil)
		for _, section := range sections {
			opened := false
			for _, o := range c.opens {
				opened = opened || o == section
			}
			value, present := overview[section]
			if !present {
				t.Errorf("%s: %s is missing, want the key with a null", c.name, section)
			} else if (value != nil) != opened {
				t.Errorf("%s: %s = %v, opened = %v", c.name, section, value, opened)
			}
		}
		summary := who.must(http.StatusOK, http.MethodGet, "/admin/summary", nil)
		if _, isNumber := summary["mustChangePassword"].(float64); isNumber != (c.key == "people.users.manage") {
			t.Errorf("%s: mustChangePassword = %v", c.name, summary["mustChangePassword"])
		}
		if status, _ := who.call(http.MethodGet, "/admin/usage", nil); status != c.usage {
			t.Errorf("%s on /admin/usage: %d, want %d", c.name, status, c.usage)
		}
	}
}

func TestAnAccountHoldingOnlyTheCreditsKeySignsInToTheAdminConsole(t *testing.T) {
	w := boot(t, planned)
	who := w.signedIn(w.staffHolding("ai.credits.manage"))
	me := who.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	var workspaces []string
	for _, workspace := range me["workspaces"].([]any) {
		workspaces = append(workspaces, workspace.(string))
	}
	if !slices.Equal(workspaces, []string{"admin"}) {
		t.Errorf("workspaces %v, want [admin]", workspaces)
	}
	for _, path := range []string{"/admin/overview", "/admin/summary"} {
		who.must(http.StatusOK, http.MethodGet, path, nil)
	}
}

func TestTheAdminOverviewCountsWhatTheCentreHolds(t *testing.T) {
	w := boot(t, planned)
	ctx := context.Background()
	admin := w.signedIn(w.createStaff("admin"))
	_ = w.signedIn(w.createStaff("teacher"))

	overview := func() map[string]any { return admin.must(http.StatusOK, http.MethodGet, "/admin/overview", nil) }
	before, usageBefore := overview(), admin.must(http.StatusOK, http.MethodGet, "/admin/usage", nil)
	summaryBefore := admin.must(http.StatusOK, http.MethodGet, "/admin/summary", nil)

	days := walk(before, "dailyActive", "days").([]any)
	if len(days) != 14 {
		t.Fatalf("the chart has %d days, want 14", len(days))
	}
	oldest := days[0].(map[string]any)["date"].(string)

	account := func(builtin, name string, disabled, mustChange bool) string {
		t.Helper()
		var id string
		err := w.pool.QueryRow(ctx, `
			INSERT INTO app.users (email, full_name, role_id, password_hash, must_change_password, disabled_at)
			SELECT $1, $2, r.id, CASE WHEN $4 THEN 'hash' END, $4, CASE WHEN $5 THEN now() END
			  FROM app.roles r WHERE r.builtin_key = $3 RETURNING id::text`,
			builtin+"-"+nonce(t)+"@example.com", name, builtin, mustChange, disabled).Scan(&id)
		if err != nil {
			t.Fatalf("%s %s: %v", builtin, name, err)
		}
		return id
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := w.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	student := account("student", "Học viên chưa vào", false, false)
	account("student", "Học viên mật khẩu tạm", false, true)
	staff := account("teacher", "Cô chưa vào", false, false)
	gone := account("teacher", "Thầy đã khóa", true, false)
	account("admin", "Quản trị phụ", false, false)

	orphan := "Lớp mồ côi " + nonce(t)
	exec(`INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2)`, orphan, gone)
	exec(`INSERT INTO app.user_activity_days (user_id, day) VALUES ($1, $3::date), ($2, $3::date)`, student, staff, oldest)
	exec(`INSERT INTO app.media_assets (kind, display_name, original_filename, mime_type, bytes, checksum_sha256, storage_key, uploaded_by, owner_id)
		VALUES ('image', 'Ảnh', 'a.png', 'image/png', 1234, sha256($1::bytea), $1, $2, $2)`, "media/"+nonce(t), gone)
	var imported string
	if err := w.pool.QueryRow(ctx, `INSERT INTO app.word_imports (created_by, request_id, title) VALUES ($1, gen_random_uuid(), 'Đề nhập') RETURNING id::text`, gone).Scan(&imported); err != nil {
		t.Fatal(err)
	}
	exec(`INSERT INTO app.word_import_sources (import_id, upload_id, expected_revision, role, filename, format, bytes, checksum_sha256, storage_key, uploaded_by)
		VALUES ($1, gen_random_uuid(), 1, 'exam', 'de.docx', 'docx', 4321, sha256($2::bytea), $2, $3)`, imported, "imports/"+nonce(t), gone)

	after, usageAfter := overview(), admin.must(http.StatusOK, http.MethodGet, "/admin/usage", nil)
	summaryAfter := admin.must(http.StatusOK, http.MethodGet, "/admin/summary", nil)

	moved := func(name string, got, want int) {
		t.Helper()
		if got != want {
			t.Errorf("%s moved by %d, want %d", name, got, want)
		}
	}
	people := func(body map[string]any, key string) int { return figure(t, body, "people", key) }
	moved("people.total", people(after, "total")-people(before, "total"), 5)
	moved("people.students", people(after, "students")-people(before, "students"), 2)
	moved("people.teachers", people(after, "teachers")-people(before, "teachers"), 2)
	moved("people.teachersDisabled", people(after, "teachersDisabled")-people(before, "teachersDisabled"), 1)
	moved("people.studentsNewThisMonth", people(after, "studentsNewThisMonth")-people(before, "studentsNewThisMonth"), 2)
	moved("classes.active", figure(t, after, "classes", "active")-figure(t, before, "classes", "active"), 1)

	count := func(body map[string]any, kind string) int { n, _ := attentionRow(body, kind); return n }
	moved("must_change_password", count(after, "must_change_password")-count(before, "must_change_password"), 1)
	moved("never_signed_in", count(after, "never_signed_in")-count(before, "never_signed_in"), 3)
	moved("disabled_teacher_classes", count(after, "disabled_teacher_classes")-count(before, "disabled_teacher_classes"), 1)
	if n, detail := attentionRow(after, "disabled_teacher_classes"); n == 0 || detail["className"] != orphan || detail["teacherName"] != "Thầy đã khóa" {
		t.Errorf("disabled teacher row = %d %v, want the newest class %q with its teacher", n, detail, orphan)
	}
	if n, _ := attentionRow(after, "storage"); n != 0 {
		t.Errorf("storage row appeared under limits of a terabyte: %d", n)
	}
	moved("summary.mustChangePassword", figure(t, summaryAfter, "mustChangePassword")-figure(t, summaryBefore, "mustChangePassword"), 1)

	first := func(body map[string]any, key string) int {
		t.Helper()
		row := walk(body, "dailyActive", "days").([]any)[0].(map[string]any)
		if row["date"] != oldest {
			t.Fatalf("the chart began on %v, want %v", row["date"], oldest)
		}
		return int(row[key].(float64))
	}
	moved("students active on the oldest day", first(after, "students")-first(before, "students"), 1)
	moved("staff active on the oldest day", first(after, "staff")-first(before, "staff"), 1)

	for _, section := range []string{"seats", "teachers"} {
		moved("usage."+section, figure(t, usageAfter, section, "used")-figure(t, usageBefore, section, "used"), map[string]int{"seats": 4, "teachers": 2}[section])
	}
	moved("usage.media.bytes", figure(t, usageAfter, "media", "bytes")-figure(t, usageBefore, "media", "bytes"), 1234)
	moved("usage.imports.bytes", figure(t, usageAfter, "imports", "bytes")-figure(t, usageBefore, "imports", "bytes"), 4321)
	moved("usage.imports.count", figure(t, usageAfter, "imports", "count")-figure(t, usageBefore, "imports", "count"), 1)
	if usageAfter["aiCredits"] != nil {
		t.Errorf("aiCredits = %v, want null while the assistant is off", usageAfter["aiCredits"])
	}
	if walk(usageAfter, "plan", "name") != "Gói thử" || figure(t, usageAfter, "seats", "limit") != 1_000_000 {
		t.Errorf("plan and limits = %v %v", usageAfter["plan"], usageAfter["seats"])
	}
	if walk(usageAfter, "plan", "renewsOn") != nil {
		t.Errorf("renewsOn = %v, want null: none is stated", walk(usageAfter, "plan", "renewsOn"))
	}
}

func TestTheCreditRequestRowAndTheUsageQuoteTheAIModuleOnlyToWhoManagesCredits(t *testing.T) {
	w := bootAssisted(t, newAssistant())
	admin := w.signedIn(w.createStaff("admin"))
	teacher := w.signedIn(w.createStaff("teacher"))
	settings := w.signedIn(w.staffHolding("system.settings.write"))

	overview := func(c *client) map[string]any { return c.must(http.StatusOK, http.MethodGet, "/admin/overview", nil) }
	waiting := func(c *client) int { n, _ := attentionRow(overview(c), "ai_credit_requests"); return n }
	before := waiting(admin)

	request := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/ai/credit-requests", map[string]any{"amount": 5000, "reason": "Chuẩn bị kiểm tra giữa kỳ"})
	if got := waiting(admin); got != before+1 {
		t.Fatalf("credit-request rows = %d, want %d: a teacher is waiting", got, before+1)
	}
	if _, detail := attentionRow(overview(admin), "ai_credit_requests"); detail == nil || detail["teacherName"] == nil || detail["allowance"] == nil || detail["used"] == nil {
		t.Errorf("the row quotes %v, want the oldest request's teacher and standing", detail)
	}
	if got := waiting(settings); got != 0 {
		t.Errorf("a holder of system.settings.write without ai.credits.manage sees %d credit-request rows, want none", got)
	}

	console := admin.must(http.StatusOK, http.MethodGet, "/admin/ai/credits", nil)
	usage := admin.must(http.StatusOK, http.MethodGet, "/admin/usage", nil)
	if figure(t, usage, "aiCredits", "used") != figure(t, console, "pool", "used") || figure(t, usage, "aiCredits", "pool") != figure(t, console, "pool", "total") {
		t.Errorf("usage quotes %v, the credits page says %v", usage["aiCredits"], console["pool"])
	}
	if quoted := settings.must(http.StatusOK, http.MethodGet, "/admin/usage", nil)["aiCredits"]; quoted != nil {
		t.Errorf("usage without ai.credits.manage quotes %v, want null", quoted)
	}
	if quoted := settings.must(http.StatusOK, http.MethodGet, "/admin/overview", nil)["storage"].(map[string]any)["aiCredits"]; quoted != nil {
		t.Errorf("the overview's storage without ai.credits.manage quotes %v, want null", quoted)
	}

	admin.must(http.StatusOK, http.MethodPost, "/admin/ai/credit-requests/"+id(request)+"/approve", nil)
	if got := waiting(admin); got != before {
		t.Errorf("credit-request rows after the approval = %d, want %d", got, before)
	}
}

func TestACredentialShapedKeyIsAbsentFromTheOverviewsRecentChanges(t *testing.T) {
	w := boot(t, planned)
	auditOnly := w.signedIn(w.staffHolding("system.audit.read"))
	teacherEmail, _ := w.createStaff("teacher")
	id := w.auditRowWithDiff(teacherEmail, "class.created", "class", `{
		"resetToken": "S3CRET-1", "secretKey": "S3CRET-2", "Authorization": "Bearer S3CRET-3", "sessionToken": "S3CRET-4",
		"nested": {"tokenHash": "S3CRET-5", "kept": "visible-nested"}, "list": [{"Set-Cookie": "S3CRET-6", "kept": "visible-listed"}],
		"note": "visible-note"}`)

	overview := auditOnly.must(http.StatusOK, http.MethodGet, "/admin/overview", nil)
	entries, _ := overview["recentChanges"].([]any)
	var shown string
	for _, e := range entries {
		if entry := e.(map[string]any); entry["id"] == float64(id) {
			raw, err := json.Marshal(entry)
			if err != nil {
				t.Fatal(err)
			}
			shown = string(raw)
		}
	}
	if shown == "" {
		t.Fatalf("entry %d is not among the %d recent changes: %v", id, len(entries), entries)
	}
	if strings.Contains(shown, "S3CRET") {
		t.Errorf("a credential-shaped key reached the overview of a caller holding only the audit key: %s", shown)
	}
	for _, kept := range []string{"visible-note", "visible-nested", "visible-listed"} {
		if !strings.Contains(shown, kept) {
			t.Errorf("%q is missing from the entry, which hid more than the credential-shaped keys: %s", kept, shown)
		}
	}
}

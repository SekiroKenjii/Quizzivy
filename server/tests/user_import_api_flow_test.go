//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"quizzivy/internal/platform/config"
)

const importSecretsKey = "0123456789abcdef0123456789abcdef"

func bootForUserImports(t *testing.T) *world {
	t.Helper()
	return bootAssistedWithStorage(t, nil, func(c *config.Config) { c.SecretsKey = []byte(importSecretsKey) })
}

func importForm(t *testing.T, file []byte, fields map[string]string) payload {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for name, value := range fields {
		if err := form.WriteField(name, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := form.CreateFormFile("file", "ds.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(file); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	return payload{contentType: form.FormDataContentType(), data: body.Bytes()}
}

func importFields(delivery string, extra ...string) map[string]string {
	fields := map[string]string{"defaultRole": "student", "delivery": delivery}
	for i := 0; i+1 < len(extra); i += 2 {
		fields[extra[i]] = extra[i+1]
	}
	return fields
}

func importError(s sent) (code, message string) {
	envelope, _ := s.json["error"].(map[string]any)
	code, _ = envelope["code"].(string)
	message, _ = envelope["message"].(string)
	return code, message
}

func (w *world) accountsAt(domain string) int {
	w.t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.users WHERE email LIKE '%@' || $1`, domain).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

func TestAnImportIsPreviewedThenStoredOnceAndOnlyItsOwnerReadsIt(t *testing.T) {
	w := bootForUserImports(t)
	email, password := w.teacher()
	admin := w.signedIn(email, password)
	otherEmail, otherPassword := w.teacher()
	other := w.signedIn(otherEmail, otherPassword)
	domain := "nhap-" + nonce(t) + ".example.test"
	taken := "co-san@" + domain
	if _, err := w.pool.Exec(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Đã có', (SELECT id FROM app.roles WHERE builtin_key = 'student'))`, taken); err != nil {
		t.Fatal(err)
	}
	file := []byte(strings.Join([]string{
		"full_name,email,role",
		"Nguyễn Văn An,an@" + domain + ",",
		"Trần Thị Bình,binh@" + domain + ",teacher",
		"Lê Văn Cường," + strings.ToUpper(taken) + ",",
		"Phạm Thị Dung,không-phải-email,",
	}, "\n") + "\n")
	before := w.accountsAt(domain)

	preview := admin.send(http.MethodPost, "/admin/user-imports/preview", ptr(importForm(t, file, importFields("email_link"))))
	if preview.status != http.StatusOK {
		t.Fatalf("preview: %s", answer(preview))
	}
	if preview.json["rows"] != 4.0 || preview.json["newAccounts"] != 2.0 || preview.json["invalidCount"] != 1.0 {
		t.Errorf("preview = %s", preview.body)
	}
	if existing := preview.json["existingEmails"].(map[string]any); existing["count"] != 1.0 {
		t.Errorf("existing = %v, want the one address the file spells in capitals", existing)
	}
	if issues := preview.json["invalid"].([]any); len(issues) != 1 || issues[0].(map[string]any)["code"] != "INVALID" || issues[0].(map[string]any)["row"] != 5.0 {
		t.Errorf("issues = %v", issues)
	}
	if w.accountsAt(domain) != before {
		t.Error("a preview created accounts")
	}

	requestID := uuid.NewString()
	created := admin.send(http.MethodPost, "/admin/user-imports", ptr(importForm(t, file, importFields("email_link", "requestId", requestID))))
	if created.status != http.StatusAccepted || created.json["status"] != "queued" || created.json["total"] != 3.0 || created.json["skipped"] != 1.0 || created.json["invalid"] != 1.0 {
		t.Fatalf("create: %s", answer(created))
	}
	jobID := id(created.json)
	again := admin.send(http.MethodPost, "/admin/user-imports", ptr(importForm(t, []byte("full_name,email\n"), importFields("sheet", "requestId", requestID))))
	if again.status != http.StatusAccepted || id(again.json) != jobID {
		t.Errorf("the repeated request: %s, want the first job back", answer(again))
	}
	if w.accountsAt(domain) != before {
		t.Error("storing a job created accounts: the worker does that")
	}

	read := admin.send(http.MethodGet, "/admin/user-imports/"+jobID, nil)
	if read.status != http.StatusOK || read.json["processed"] != 1.0 || read.json["delivery"] != "email_link" {
		t.Errorf("read: %s", answer(read))
	}
	if _, has := read.json["sheet"]; has && read.json["sheet"] != nil {
		t.Errorf("an email job reports a sheet: %s", read.body)
	}
	if foreign := other.send(http.MethodGet, "/admin/user-imports/"+jobID, nil); foreign.status != http.StatusNotFound {
		t.Errorf("another Admin read it: %s", answer(foreign))
	}
	if unknown := admin.send(http.MethodGet, "/admin/user-imports/"+uuid.NewString(), nil); unknown.status != http.StatusNotFound {
		t.Errorf("an unknown id: %s", answer(unknown))
	}

	var action, entity string
	var diff []byte
	if err := w.pool.QueryRow(context.Background(), `
		SELECT action, entity, diff FROM app.audit_log WHERE entity_id = $1::uuid`, jobID).Scan(&action, &entity, &diff); err != nil {
		t.Fatal(err)
	}
	if action != "user_import.requested" || entity != "user_import" || strings.Contains(string(diff), domain) || strings.Contains(string(diff), "Nguyễn") {
		t.Errorf("audit = %s on %s with %s", action, entity, diff)
	}
}

func TestOnlyAStaffMemberWhoManagesUsersMeetsTheImportOperations(t *testing.T) {
	w := bootForUserImports(t)
	file := []byte("full_name,email\nNguyễn Văn An,an@" + nonce(t) + ".example.test\n")
	form := ptr(importForm(t, file, importFields("email_link", "requestId", uuid.NewString())))

	anonymous := w.browser()
	for _, call := range [][2]string{{http.MethodPost, "/admin/user-imports/preview"}, {http.MethodPost, "/admin/user-imports"}, {http.MethodGet, "/admin/user-imports/" + uuid.NewString()}} {
		body := form
		if call[0] == http.MethodGet {
			body = nil
		}
		if got := anonymous.send(call[0], call[1], body); got.status != http.StatusUnauthorized {
			t.Errorf("%s %s without a token answered %d", call[0], call[1], got.status)
		}
		for _, role := range []string{"teacher", "assistant"} {
			email, password := w.createStaff(role)
			if got := w.signedIn(email, password).send(call[0], call[1], body); got.status != http.StatusForbidden {
				t.Errorf("%s %s as a %s answered %d: %s", call[0], call[1], role, got.status, got.body)
			}
		}
	}
}

func TestAFileTheReaderRefusesIsAnsweredWithItsOwnCodeAndNeverEchoed(t *testing.T) {
	w := bootForUserImports(t)
	email, password := w.teacher()
	admin := w.signedIn(email, password)
	secret := "kin-mat-" + nonce(t)
	notUTF8 := []byte("full_name,email\nT\xe9n," + secret + "@example.test\n")
	notZip := append([]byte("PK\x03\x04"), []byte(strings.Repeat(secret, 8))...)
	noEmail := []byte("full_name,ho_ten\nAn," + secret + "\n")
	var rows strings.Builder
	rows.WriteString("full_name,email\n")
	for i := range 2001 {
		fmt.Fprintf(&rows, "Học sinh %d,hs%d@example.test\n", i, i)
	}

	for name, tc := range map[string]struct {
		file   []byte
		status int
		code   string
		create bool
	}{
		"a CSV in another encoding": {notUTF8, http.StatusUnprocessableEntity, "CSV_NOT_UTF8", true},
		"bytes that are not a book": {notZip, http.StatusUnprocessableEntity, "USER_IMPORT_FILE_UNREADABLE", true},
		"no header":                 {nil, http.StatusUnprocessableEntity, "USER_IMPORT_FILE_EMPTY", false},
		"a header without email":    {noEmail, http.StatusUnprocessableEntity, "USER_IMPORT_COLUMNS", true},
		"2,001 rows":                {[]byte(rows.String()), http.StatusUnprocessableEntity, "USER_IMPORT_TOO_MANY_ROWS", false},
		"a file with nothing new":   {[]byte("full_name,email\n"), http.StatusUnprocessableEntity, "USER_IMPORT_NOTHING_TO_CREATE", true},
	} {
		paths := []string{"/admin/user-imports/preview"}
		if tc.create {
			paths = append(paths, "/admin/user-imports")
		}
		for _, path := range paths {
			fields := importFields("email_link", "requestId", uuid.NewString())
			got := admin.send(http.MethodPost, path, ptr(importForm(t, tc.file, fields)))
			code, message := importError(got)
			if path == "/admin/user-imports/preview" && tc.code == "USER_IMPORT_NOTHING_TO_CREATE" {
				if got.status != http.StatusOK {
					t.Errorf("%s: preview answered %d, want the counts", name, got.status)
				}
				continue
			}
			if got.status != tc.status || code != tc.code || message == "" {
				t.Errorf("%s %s: %d %s %q, want %d %s", name, path, got.status, code, message, tc.status, tc.code)
			}
			if strings.Contains(string(got.body), secret) {
				t.Errorf("%s %s echoed the file: %s", name, path, got.body)
			}
		}
	}
	var jobs int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.user_import_jobs WHERE created_by = (SELECT id FROM app.users WHERE email = $1)`, email).Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if jobs != 0 {
		t.Errorf("refused files left %d job(s)", jobs)
	}
}

func TestAnUploadOverTheBudgetIsRefusedWithTheWayOutAndStoresNothing(t *testing.T) {
	w := bootForUserImports(t)
	email, password := w.teacher()
	admin := w.signedIn(email, password)
	big := bytes.Repeat([]byte("a,b\n"), (5<<20)/4+4096)

	for _, path := range []string{"/admin/user-imports/preview", "/admin/user-imports"} {
		got := admin.send(http.MethodPost, path, ptr(importForm(t, big, importFields("email_link", "requestId", uuid.NewString()))))
		code, message := importError(got)
		if got.status != http.StatusRequestEntityTooLarge || !strings.Contains(message, "CSV") {
			t.Errorf("%s: %d %s %q, want 413 naming CSV", path, got.status, code, message)
		}
	}
}

func TestADeliveryTheServerCannotSealAnswers501(t *testing.T) {
	w := bootWithStorage(t)
	email, password := w.teacher()
	admin := w.signedIn(email, password)
	file := []byte("full_name,email\nNguyễn Văn An,an@" + nonce(t) + ".example.test\n")

	for _, delivery := range []string{"email_link", "sheet"} {
		for _, path := range []string{"/admin/user-imports/preview", "/admin/user-imports"} {
			got := admin.send(http.MethodPost, path, ptr(importForm(t, file, importFields(delivery, "requestId", uuid.NewString()))))
			if got.status != http.StatusNotImplemented {
				t.Errorf("%s by %s without SECRETS_KEY answered %d: %s", path, delivery, got.status, got.body)
			}
		}
	}
}

func TestPreviewsAreLimitedToTenAMinutePerAdmin(t *testing.T) {
	w := bootForUserImports(t)
	email, password := w.teacher()
	admin := w.signedIn(email, password)
	otherEmail, otherPassword := w.teacher()
	other := w.signedIn(otherEmail, otherPassword)
	form := func() *payload {
		return ptr(importForm(t, []byte("full_name,email\nNguyễn Văn An,an@"+nonce(t)+".example.test\n"), importFields("email_link")))
	}

	for i := range 10 {
		if got := admin.send(http.MethodPost, "/admin/user-imports/preview", form()); got.status != http.StatusOK {
			t.Fatalf("preview %d answered %d: %s", i+1, got.status, got.body)
		}
	}
	limited := admin.send(http.MethodPost, "/admin/user-imports/preview", form())
	if limited.status != http.StatusTooManyRequests {
		t.Errorf("the eleventh preview answered %d: %s", limited.status, limited.body)
	}
	if got := other.send(http.MethodPost, "/admin/user-imports/preview", form()); got.status != http.StatusOK {
		t.Errorf("another Admin behind the same address was limited too: %d", got.status)
	}
}

func ptr[T any](v T) *T { return &v }

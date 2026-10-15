//go:build e2e

package e2e

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"quizzivy/internal/core/wiring"
	identityworker "quizzivy/internal/modules/identity/application/worker"
	identitydomain "quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/tabular"
)

func (w *world) userImportRunner() identityworker.UserImports {
	return w.userImportRunnerLogging(slog.New(slog.DiscardHandler))
}

func (w *world) userImportRunnerLogging(logger *slog.Logger) identityworker.UserImports {
	w.t.Helper()
	mail, err := wiring.BuildMail(w.cfg, db.NewContext(w.pool), logger)
	if err != nil {
		w.t.Fatalf("assemble the mail: %v", err)
	}
	runner, err := wiring.UserImportsWorker(context.Background(), config.ImportWorker{Base: w.cfg}, db.NewContext(w.pool), mail.Outbox, logger)
	if err != nil {
		w.t.Fatalf("assemble the user import worker: %v", err)
	}
	return runner
}

func (w *world) jobState(jobID string) (status string, attempts int) {
	w.t.Helper()
	if err := w.pool.QueryRow(context.Background(), `SELECT status, attempts FROM app.user_import_jobs WHERE id = $1::uuid`, jobID).Scan(&status, &attempts); err != nil {
		w.t.Fatal(err)
	}
	return status, attempts
}

func (w *world) runUserImports(runner identityworker.UserImports, jobID string) string {
	w.t.Helper()
	for range 200 {
		if status, _ := w.jobState(jobID); status == "done" || status == "failed" {
			return status
		}
		worked, err := runner.RunOne(context.Background())
		if err != nil {
			w.t.Logf("a run ended with %v", err)
			continue
		}
		if !worked {
			status, _ := w.jobState(jobID)
			w.t.Fatalf("the queue is empty but the job is %s", status)
		}
	}
	w.t.Fatal("the worker did not reach the job in 200 runs")
	return ""
}

func (c *client) fetch(method, path string) (int, http.Header, []byte) {
	c.w.t.Helper()
	req, err := http.NewRequest(method, c.w.server.URL+path, nil)
	if err != nil {
		c.w.t.Fatal(err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.w.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.w.t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, raw
}

func (w *world) existingAccount(email string) {
	w.t.Helper()
	if _, err := w.pool.Exec(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Đã có', (SELECT id FROM app.roles WHERE builtin_key = 'student'))`, email); err != nil {
		w.t.Fatal(err)
	}
}

func (w *world) queuedMails(addresses ...string) int {
	w.t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.email_outbox WHERE to_address = ANY($1::text[])`, addresses).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

func (w *world) sealedPasswords(jobID string) int {
	w.t.Helper()
	var n int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.user_import_rows WHERE job_id = $1::uuid AND secret_ciphertext IS NOT NULL`, jobID).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

func csvLines(lines ...string) []byte { return []byte(strings.Join(lines, "\n") + "\n") }

func TestASheetImportCreatesTheAccountsAndHandsTheirPasswordsOutOnce(t *testing.T) {
	w := bootForUserImports(t)
	email, password := w.teacher()
	admin := w.signedIn(email, password)
	domain := "bang-" + nonce(t) + ".example.test"
	taken := "co-san@" + domain
	w.existingAccount(taken)
	file := csvLines(
		"full_name,email,role",
		"Nguyễn Văn An,an@"+domain+",",
		`"=HYPERLINK(""http://x"")",ten-doc-hai@`+domain+",",
		"Trần Thị Bình,binh@"+domain+",teacher",
		"Có Sẵn,"+strings.ToUpper(taken)+",",
	)
	sheetFields := importFields("sheet", "requestId", uuid.NewString())

	preview := admin.send(http.MethodPost, "/admin/user-imports/preview", ptr(importForm(t, file, importFields("sheet"))))
	if preview.status != http.StatusOK || preview.json["rows"] != 4.0 || preview.json["newAccounts"] != 3.0 {
		t.Fatalf("preview: %s", answer(preview))
	}
	created := admin.send(http.MethodPost, "/admin/user-imports", ptr(importForm(t, file, sheetFields)))
	if created.status != http.StatusAccepted || created.json["total"] != 4.0 || created.json["delivery"] != "sheet" {
		t.Fatalf("create: %s", answer(created))
	}
	jobID := id(created.json)
	sheetPath := "/admin/user-imports/" + jobID + "/sheet"

	if early := admin.send(http.MethodPost, sheetPath, nil); early.status != http.StatusGone {
		t.Errorf("a sheet asked for before the job ran answered %d, want 410: %s", early.status, early.body)
	} else if code, _ := importError(early); code != "SHEET_UNAVAILABLE" {
		t.Errorf("code = %q, want SHEET_UNAVAILABLE", code)
	}
	if w.accountsAt(domain) != 1 {
		t.Fatal("storing the job created accounts")
	}

	logs := &lockedBuffer{}
	runner := w.userImportRunnerLogging(slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	if status := w.runUserImports(runner, jobID); status != "done" {
		t.Fatalf("the job ended %s", status)
	}

	read := admin.send(http.MethodGet, "/admin/user-imports/"+jobID, nil)
	sheet, _ := read.json["sheet"].(map[string]any)
	if read.status != http.StatusOK || read.json["status"] != "done" || read.json["created"] != 3.0 || read.json["skipped"] != 1.0 || read.json["processed"] != 4.0 || read.json["errorCode"] != nil {
		t.Fatalf("read: %s", answer(read))
	}
	if sheet["available"] != true || sheet["expiresAt"] == nil || sheet["downloadedAt"] != nil {
		t.Errorf("sheet = %v, want available with an expiry and no download", sheet)
	}
	if got := w.accountsAt(domain); got != 4 {
		t.Errorf("%d account(s) at the domain, want the existing one and 3 new", got)
	}
	if w.sealedPasswords(jobID) != 3 {
		t.Errorf("%d sealed password(s) held, want 3", w.sealedPasswords(jobID))
	}

	otherEmail, otherPassword := w.teacher()
	if got, _, _ := w.signedIn(otherEmail, otherPassword).fetch(http.MethodPost, sheetPath); got != http.StatusNotFound {
		t.Errorf("another Admin asked for the sheet and got %d, want 404", got)
	}
	for _, role := range []string{"teacher", "assistant"} {
		staffEmail, staffPassword := w.createStaff(role)
		if got, _, _ := w.signedIn(staffEmail, staffPassword).fetch(http.MethodPost, sheetPath); got != http.StatusForbidden {
			t.Errorf("a %s asked for the sheet and got %d, want 403", role, got)
		}
	}
	if got, _, _ := w.browser().fetch(http.MethodPost, sheetPath); got != http.StatusUnauthorized {
		t.Errorf("an anonymous caller asked for the sheet and got %d, want 401", got)
	}
	if w.sealedPasswords(jobID) != 3 {
		t.Fatal("a caller who may not have the sheet used it up")
	}

	status, header, body := admin.fetch(http.MethodPost, sheetPath)
	if status != http.StatusOK {
		t.Fatalf("the requester's download answered %d: %s", status, body)
	}
	if header.Get("Cache-Control") != "no-store" || !strings.HasPrefix(header.Get("Content-Type"), "text/csv") {
		t.Errorf("headers = %v, want text/csv and no-store", header)
	}
	if !regexp.MustCompile(`^attachment; filename="new-accounts-\d{8}\.csv"$`).MatchString(header.Get("Content-Disposition")) {
		t.Errorf("Content-Disposition = %q", header.Get("Content-Disposition"))
	}
	rows := records(t, "the sheet", string(body))
	if len(rows) != 4 || strings.Join(rows[0], ",") != "full_name,email,role,class,temporary_password" {
		t.Fatalf("sheet = %v, want the header and 3 accounts", rows)
	}
	hostile := find(rows, "email", "ten-doc-hai@"+domain)
	if hostile == nil || hostile[0] != "'"+hostileFormula {
		t.Errorf("the formula row = %q, want the name escaped with a leading quote", hostile)
	}
	if find(rows, "email", taken) != nil {
		t.Error("the sheet lists an account that already existed")
	}
	if teacherRow := find(rows, "email", "binh@"+domain); teacherRow == nil || teacherRow[2] == "" {
		t.Errorf("the teacher row = %q, want a role name", teacherRow)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		given := row[4]
		if given == "" || seen[given] {
			t.Errorf("password %q of %s is empty or repeated", given, row[1])
		}
		seen[given] = true
		for column, cell := range map[string]string{"email": row[1], "temporary_password": row[4]} {
			if tabular.EscapeCell(cell) != cell {
				t.Errorf("the %s cell %q of %s would be changed by the escape: the sheet must hand out what signs in", column, cell, row[1])
			}
		}
		student := w.browser()
		if got, _ := student.call(http.MethodPost, "/auth/login", map[string]any{"email": row[1], "password": given}); got != http.StatusOK {
			t.Errorf("%s cannot sign in with the sheet's password: %d", row[1], got)
		}
	}

	again := admin.send(http.MethodPost, sheetPath, nil)
	if code, _ := importError(again); again.status != http.StatusGone || code != "SHEET_UNAVAILABLE" {
		t.Errorf("the second download answered %d %s, want 410 SHEET_UNAVAILABLE", again.status, code)
	}
	if strings.Contains(string(again.body), rows[1][4]) {
		t.Error("the refusal repeats a password")
	}
	after := admin.send(http.MethodGet, "/admin/user-imports/"+jobID, nil)
	if sheet, _ := after.json["sheet"].(map[string]any); sheet["available"] != false || sheet["downloadedAt"] == nil {
		t.Errorf("sheet after the download = %v, want unavailable and dated", sheet)
	}
	if w.sealedPasswords(jobID) != 0 {
		t.Errorf("%d sealed password(s) outlive the download", w.sealedPasswords(jobID))
	}
	for _, shown := range [][]byte{read.body, after.body, again.body} {
		for given := range seen {
			if strings.Contains(string(shown), given) {
				t.Errorf("a response other than the sheet carries the password %q", given)
			}
		}
	}

	if text := logs.String(); !strings.Contains(text, `"msg":"user import"`) {
		t.Errorf("the worker logged nothing of its run: %q", text)
	} else {
		for given := range seen {
			if strings.Contains(text, given) {
				t.Errorf("the worker's log carries the password %q", given)
			}
		}
		for _, row := range rows[1:] {
			if strings.Contains(text, row[1]) {
				t.Errorf("the worker's log carries the address %s", row[1])
			}
		}
	}

	var downloads int
	var diffs string
	if err := w.pool.QueryRow(context.Background(), `
		SELECT count(*), coalesce(string_agg(diff::text, ' '), '') FROM app.audit_log
		 WHERE entity_id = $1::uuid AND action = 'user_import.sheet_downloaded'`, jobID).Scan(&downloads, &diffs); err != nil {
		t.Fatal(err)
	}
	if downloads != 1 {
		t.Errorf("%d download(s) audited, want exactly the one that gave the file", downloads)
	}
	for given := range seen {
		if strings.Contains(diffs, given) {
			t.Errorf("the audit entry carries the password %q", given)
		}
	}
}

func TestAnEmailImportQueuesOneInvitationForEachCreatedAccountAndNoSheet(t *testing.T) {
	w := bootForUserImports(t)
	email, password := w.teacher()
	admin := w.signedIn(email, password)
	domain := "thu-" + nonce(t) + ".example.test"
	taken := "co-san@" + domain
	w.existingAccount(taken)
	created := []string{"mot@" + domain, "hai@" + domain, "ba@" + domain}
	file := csvLines(
		"full_name,email",
		"Học viên Một,"+created[0],
		"Học viên Hai,"+created[1],
		"Học viên Ba,"+created[2],
		"Học viên Cũ,"+taken,
		"Không Thư,không-phải-thư",
	)
	requestID := uuid.NewString()

	preview := admin.send(http.MethodPost, "/admin/user-imports/preview", ptr(importForm(t, file, importFields("email_link"))))
	job := admin.send(http.MethodPost, "/admin/user-imports", ptr(importForm(t, file, importFields("email_link", "requestId", requestID))))
	if preview.status != http.StatusOK || job.status != http.StatusAccepted {
		t.Fatalf("preview %d, create %d: %s", preview.status, job.status, job.body)
	}
	if job.json["total"].(float64) != preview.json["rows"].(float64)-preview.json["invalidCount"].(float64) ||
		job.json["invalid"] != preview.json["invalidCount"] || job.json["skipped"] != 1.0 {
		t.Errorf("preview %s and job %s disagree", preview.body, job.body)
	}
	jobID := id(job.json)
	if repeated := admin.send(http.MethodPost, "/admin/user-imports", ptr(importForm(t, file, importFields("email_link", "requestId", requestID)))); id(repeated.json) != jobID {
		t.Errorf("the repeated request made another job: %s", repeated.body)
	}

	runner := w.userImportRunner()
	if status := w.runUserImports(runner, jobID); status != "done" {
		t.Fatalf("the job ended %s", status)
	}
	if got := w.queuedMails(created...); got != 3 {
		t.Errorf("%d invitation(s) queued, want one for each created account", got)
	}
	if got := w.queuedMails(taken); got != 0 {
		t.Errorf("the account that existed was sent %d invitation(s)", got)
	}
	var withPassword int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.users WHERE email = ANY($1::text[]) AND password_hash IS NOT NULL`, created).Scan(&withPassword); err != nil || withPassword != 0 {
		t.Errorf("%d invited account(s) hold a password (%v), want none until they set one", withPassword, err)
	}
	read := admin.send(http.MethodGet, "/admin/user-imports/"+jobID, nil)
	if read.json["status"] != "done" || read.json["created"] != 3.0 || read.json["skipped"] != 1.0 || read.json["sheet"] != nil {
		t.Errorf("read: %s", answer(read))
	}
	if got := admin.send(http.MethodPost, "/admin/user-imports/"+jobID+"/sheet", nil); got.status != http.StatusGone {
		t.Errorf("an email job gave a sheet: %d %s", got.status, got.body)
	}

	if worked, err := runner.RunOne(context.Background()); err != nil {
		t.Fatal(err)
	} else if worked {
		if status, _ := w.jobState(jobID); status != "done" {
			t.Errorf("a finished job was claimed again and is now %s", status)
		}
	}
	if got := w.queuedMails(created...); got != 3 {
		t.Errorf("%d invitation(s) queued after another run, want still 3", got)
	}
}

func TestAJobThatWasInterruptedResumesWithoutMakingAnAccountTwice(t *testing.T) {
	w := bootForUserImports(t)
	email, password := w.teacher()
	admin := w.signedIn(email, password)
	domain := "tiep-" + nonce(t) + ".example.test"
	const rows = 60
	lines := []string{"full_name,email"}
	for i := range rows {
		lines = append(lines, fmt.Sprintf("Học viên %02d,hv%02d@%s", i, i, domain))
	}
	job := admin.send(http.MethodPost, "/admin/user-imports", ptr(importForm(t, csvLines(lines...), importFields("sheet", "requestId", uuid.NewString()))))
	if job.status != http.StatusAccepted {
		t.Fatalf("create: %s", answer(job))
	}
	jobID := id(job.json)

	hash, err := identitydomain.Passwords.Hash(context.Background(), "mat-khau-chung")
	if err != nil {
		t.Fatal(err)
	}
	var made atomic.Int32
	var failAfter atomic.Int32
	failAfter.Store(25)
	runner := w.userImportRunner()
	runner.Passwords = func(context.Context) (string, string, error) {
		n := made.Add(1)
		if n > failAfter.Load() {
			return "", "", errors.New("the worker died here")
		}
		return fmt.Sprintf("tam-%03d-%s", n, nonce(t)), hash, nil
	}

	var interrupted error
	for range 200 {
		_, interrupted = runner.RunOne(context.Background())
		if _, attempts := w.jobState(jobID); attempts > 0 {
			break
		}
	}
	status, attempts := w.jobState(jobID)
	if interrupted == nil || status != "queued" || attempts != 1 {
		t.Fatalf("after the interrupted run: %v, job %s after %d attempt(s), want an error and the job queued after 1", interrupted, status, attempts)
	}
	if got := w.accountsAt(domain); got != 25 {
		t.Fatalf("%d account(s) after the first batch, want 25", got)
	}

	failAfter.Store(1 << 20)
	if status := w.runUserImports(runner, jobID); status != "done" {
		t.Fatalf("the resumed job ended %s", status)
	}
	if got := w.accountsAt(domain); got != rows {
		t.Errorf("%d account(s), want %d with none made twice", got, rows)
	}
	var total, distinct int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*), count(DISTINCT lower(email)) FROM app.users WHERE email LIKE '%@' || $1`, domain).Scan(&total, &distinct); err != nil || total != distinct {
		t.Errorf("%d account(s), %d distinct addresses (%v)", total, distinct, err)
	}
	read := admin.send(http.MethodGet, "/admin/user-imports/"+jobID, nil)
	if read.json["status"] != "done" || read.json["created"] != float64(rows) || read.json["skipped"] != 0.0 {
		t.Errorf("read: %s", answer(read))
	}
	if _, attempts := w.jobState(jobID); attempts != 2 {
		t.Errorf("%d attempt(s), want the interrupted run and the one that finished", attempts)
	}
	downloaded, _, body := admin.fetch(http.MethodPost, "/admin/user-imports/"+jobID+"/sheet")
	if downloaded != http.StatusOK {
		t.Fatalf("sheet: %d %s", downloaded, body)
	}
	sheet := records(t, "the sheet", string(body))
	passwords := map[string]bool{}
	for _, row := range sheet[1:] {
		passwords[row[4]] = true
	}
	if len(sheet) != rows+1 || len(passwords) != rows {
		t.Errorf("sheet holds %d account(s) with %d distinct password(s), want %d of each", len(sheet)-1, len(passwords), rows)
	}
}

func TestAnImporterWhoIsNotAnAdminMayNotGiveWhatTheirRoleDoesNotHold(t *testing.T) {
	w := bootForUserImports(t)
	email, password := w.staffHolding("people.users.manage")
	importer := w.signedIn(email, password)
	domain := "han-" + nonce(t) + ".example.test"
	file := csvLines(
		"full_name,email,role",
		"Học viên Hợp lệ,hop-le@"+domain+",",
		"Quản trị Giả,quan-tri@"+domain+",admin",
	)

	preview := importer.send(http.MethodPost, "/admin/user-imports/preview", ptr(importForm(t, file, importFields("email_link"))))
	if preview.status != http.StatusOK || preview.json["rows"] != 2.0 || preview.json["newAccounts"] != 1.0 || preview.json["invalidCount"] != 1.0 {
		t.Fatalf("preview: %s", answer(preview))
	}
	issue := preview.json["invalid"].([]any)[0].(map[string]any)
	if issue["row"] != 3.0 || (issue["code"] != "ROLE_NOT_ALLOWED" && issue["code"] != "ROLE_NOT_GIVABLE") {
		t.Errorf("issue = %v, want the admin row refused for its role", issue)
	}

	job := importer.send(http.MethodPost, "/admin/user-imports", ptr(importForm(t, file, importFields("email_link", "requestId", uuid.NewString()))))
	if job.status != http.StatusAccepted || job.json["total"] != 1.0 || job.json["invalid"] != 1.0 {
		t.Fatalf("create: %s", answer(job))
	}
	if status := w.runUserImports(w.userImportRunner(), id(job.json)); status != "done" {
		t.Fatalf("the job ended %s", status)
	}
	if got := w.accountsAt(domain); got != 1 {
		t.Errorf("%d account(s) created, want only the student", got)
	}
	var admins int
	if err := w.pool.QueryRow(context.Background(), `
		SELECT count(*) FROM app.users u JOIN app.roles r ON r.id = u.role_id
		 WHERE u.email LIKE '%@' || $1 AND r.builtin_key = 'admin'`, domain).Scan(&admins); err != nil || admins != 0 {
		t.Errorf("%d Admin(s) made by an importer who is not one (%v)", admins, err)
	}
}

func TestAJobWhoseImporterLostTheRightIsFailedAndCreatesNothing(t *testing.T) {
	w := bootForUserImports(t)
	email, password := w.staffHolding("people.users.manage")
	importer := w.signedIn(email, password)
	domain := "mat-" + nonce(t) + ".example.test"
	job := importer.send(http.MethodPost, "/admin/user-imports", ptr(importForm(t, csvLines("full_name,email", "Học viên,hv@"+domain), importFields("sheet", "requestId", uuid.NewString()))))
	if job.status != http.StatusAccepted {
		t.Fatalf("create: %s", answer(job))
	}
	jobID := id(job.json)
	if _, err := w.pool.Exec(context.Background(), `
		UPDATE app.users SET disabled_at = now() WHERE email = $1`, email); err != nil {
		t.Fatal(err)
	}

	if status := w.runUserImports(w.userImportRunner(), jobID); status != "failed" {
		t.Fatalf("the job ended %s, want failed", status)
	}
	if got := w.accountsAt(domain); got != 0 {
		t.Errorf("%d account(s) created for a disabled importer", got)
	}
	var code string
	if err := w.pool.QueryRow(context.Background(), `SELECT error_code FROM app.user_import_jobs WHERE id = $1::uuid`, jobID).Scan(&code); err != nil || code != "IMPORTER_NOT_ALLOWED" {
		t.Errorf("error code = %q (%v), want IMPORTER_NOT_ALLOWED", code, err)
	}
}

func TestTheSheetIsLimitedToFiveARequestPerMinutePerAdmin(t *testing.T) {
	w := bootForUserImports(t)
	email, password := w.teacher()
	admin := w.signedIn(email, password)
	otherEmail, otherPassword := w.teacher()
	other := w.signedIn(otherEmail, otherPassword)
	path := func() string { return "/admin/user-imports/" + uuid.NewString() + "/sheet" }

	for i := range 5 {
		if got, _, body := admin.fetch(http.MethodPost, path()); got != http.StatusNotFound {
			t.Fatalf("request %d answered %d: %s", i+1, got, body)
		}
	}
	if got, _, _ := admin.fetch(http.MethodPost, path()); got != http.StatusTooManyRequests {
		t.Errorf("the sixth request answered %d, want 429", got)
	}
	if got, _, _ := other.fetch(http.MethodPost, path()); got != http.StatusNotFound {
		t.Errorf("another Admin behind the same address was limited too: %d", got)
	}
}

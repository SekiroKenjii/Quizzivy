package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	identityhttp "quizzivy/internal/modules/identity/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/tabular"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

type importSpy struct {
	preview command.PreviewUserImport
	create  command.CreateUserImport
	get     query.GetUserImport
	sheet   command.DownloadUserImportSheet
	calls   int
}

type importOutcome struct {
	preview command.UserImportPreview
	create  command.CreateUserImportResult
	get     query.GetUserImportResult
	sheet   []domain.SheetRow
	err     error
}

func importApp(spy *importSpy, out importOutcome) *application.Application {
	return &application.Application{
		Commands: application.Commands{
			PreviewUserImport: cqrs.HandlerFunc[command.PreviewUserImport, command.UserImportPreview](func(_ context.Context, cmd command.PreviewUserImport) (command.UserImportPreview, error) {
				spy.preview, spy.calls = cmd, spy.calls+1
				return out.preview, out.err
			}),
			CreateUserImport: cqrs.HandlerFunc[command.CreateUserImport, command.CreateUserImportResult](func(_ context.Context, cmd command.CreateUserImport) (command.CreateUserImportResult, error) {
				spy.create, spy.calls = cmd, spy.calls+1
				return out.create, out.err
			}),
			DownloadUserImportSheet: cqrs.HandlerFunc[command.DownloadUserImportSheet, []domain.SheetRow](func(_ context.Context, cmd command.DownloadUserImportSheet) ([]domain.SheetRow, error) {
				spy.sheet, spy.calls = cmd, spy.calls+1
				return out.sheet, out.err
			}),
		},
		Queries: application.Queries{
			GetUserImport: cqrs.HandlerFunc[query.GetUserImport, query.GetUserImportResult](func(_ context.Context, q query.GetUserImport) (query.GetUserImportResult, error) {
				spy.get, spy.calls = q, spy.calls+1
				return out.get, out.err
			}),
		},
	}
}

type importFile struct {
	file      []byte
	filename  string
	fields    map[string]string
	extraFile bool
}

func goodUpload() importFile {
	return importFile{
		file: []byte("full_name,email\nNguyễn Văn An,an@example.test\n"), filename: "danh-sach.csv",
		fields: map[string]string{"defaultRole": "student", "delivery": "email_link", "requestId": uuid.NewString()},
	}
}

func (u importFile) reader(t *testing.T) *multipart.Reader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range u.fields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatal(err)
		}
	}
	files := 1
	if u.extraFile {
		files = 2
	}
	if u.file != nil {
		for range files {
			part, err := writer.CreateFormFile("file", u.filename)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := part.Write(u.file); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return multipart.NewReader(&body, writer.Boundary())
}

func importCall(t *testing.T, principal access.Principal, accept string, call func(context.Context, http.ResponseWriter) error) served {
	t.Helper()
	var result served
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { result.err = call(r.Context(), w) })
	verify := func(string) (httpx.Principal, error) {
		return httpx.Principal{UserID: principal.UserID, Access: principal}, nil
	}
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(httpx.RequireAuth(nil, verify)(inner))
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	request.Header.Set("User-Agent", "imports-http-test")
	if accept != "" {
		request.Header.Set("Accept-Language", accept)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	result.status, result.header, result.body = recorder.Code, recorder.Header(), recorder.Body.Bytes()
	return result
}

func (u importFile) previewing(t *testing.T, h identityhttp.Identity, accept string) served {
	t.Helper()
	return importCall(t, anAdmin(), accept, func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.PreviewUserImport(ctx, openapi.PreviewUserImportRequestObject{Body: u.reader(t)})
		return render(w, openapi.PreviewUserImportResponseObject.VisitPreviewUserImportResponse, response, err)
	})
}

func (u importFile) creating(t *testing.T, h identityhttp.Identity, accept string) served {
	t.Helper()
	return importCall(t, anAdmin(), accept, func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.CreateUserImport(ctx, openapi.CreateUserImportRequestObject{Body: u.reader(t)})
		return render(w, openapi.CreateUserImportResponseObject.VisitCreateUserImportResponse, response, err)
	})
}

func (s served) errorCode(t *testing.T) (openapi.ErrorCode, string) {
	t.Helper()
	var body struct {
		Error struct {
			Code    openapi.ErrorCode `json:"code"`
			Message string            `json:"message"`
			Details map[string]any    `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(s.body, &body); err != nil {
		t.Fatalf("not an error envelope: %s", s.body)
	}
	return body.Error.Code, body.Error.Message
}

func aJob() domain.ImportJob {
	return domain.ImportJob{
		ID: uuid.NewString(), CreatedBy: uuid.NewString(), RequestID: uuid.NewString(), Filename: "danh-sach.csv",
		Format: domain.ImportCSV, DefaultRoleID: uuid.NewString(), Delivery: domain.ImportEmailLink, Status: domain.ImportQueued,
		Total: 3, Skipped: 1, Invalid: 2, CreatedAt: time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC),
	}
}

func TestPreviewPassesTheCallerTheFileAndTheChoicesOn(t *testing.T) {
	spy := &importSpy{}
	h := identityhttp.NewIdentity(importApp(spy, importOutcome{}), 0, false, nil)
	up := goodUpload()
	up.fields["delivery"] = "sheet"
	up.fields["defaultRole"] = "teacher"

	got := up.previewing(t, h, "")
	if got.err != nil || got.status != http.StatusOK {
		t.Fatalf("answered %d, %v: %s", got.status, got.err, got.body)
	}
	cmd := spy.preview
	if !cmd.Request.All || cmd.Request.IP != "203.0.113.9" || cmd.Request.UserAgent != "imports-http-test" || !cmd.Request.Grants.Has(access.PeopleUsersManage) {
		t.Errorf("request = %+v, want the Admin, scope.all, the request's origin and the caller's grants", cmd.Request)
	}
	if !bytes.Equal(cmd.File, up.file) || cmd.DefaultRole != domain.DefaultTeacher || cmd.Delivery != domain.ImportSheet {
		t.Errorf("command = %+v, want the file and the two choices", cmd)
	}
}

func TestAPreviewAnswersTheCountsTheIssuesAndTheBudget(t *testing.T) {
	class := uuid.NewString()
	h := identityhttp.NewIdentity(importApp(&importSpy{}, importOutcome{preview: command.UserImportPreview{
		Rows: 5, NewAccounts: 2, ExistingCount: 1, ExistingSample: []string{"CO-SAN@example.test"},
		Issues:      []domain.ImportIssue{{Line: 4, Field: domain.FieldEmail, Code: domain.CodeInvalid}, {Line: 5, Field: domain.FieldRole, Code: domain.CodeRoleNotGivable}},
		InvalidRows: 2, Classes: []domain.ImportClass{{ID: class, Name: "Lớp 10A"}},
		Budget: &model.EmailAllowance{Remaining: 7, Limit: 100},
	}}), 0, false, nil)

	got := goodUpload().previewing(t, h, "")
	if got.status != http.StatusOK {
		t.Fatalf("answered %d: %s", got.status, got.body)
	}
	var body map[string]any
	if err := json.Unmarshal(got.body, &body); err != nil {
		t.Fatal(err)
	}
	want := `{"classes":[{"id":"` + class + `","name":"Lớp 10A"}],"emailBudget":{"limit":100,"remainingToday":7},` +
		`"existingEmails":{"count":1,"sample":["CO-SAN@example.test"]},"invalid":[{"code":"INVALID","field":"email","row":4},` +
		`{"code":"ROLE_NOT_GIVABLE","field":"role","row":5}],"invalidCount":2,"newAccounts":2,"rows":5}`
	if canonical(t, got.body) != canonical(t, []byte(want)) {
		t.Errorf("body = %s\nwant %s", got.body, want)
	}
}

func canonical(t *testing.T, raw []byte) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("not JSON: %s", raw)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestAPreviewWithNoBudgetAnswersNull(t *testing.T) {
	h := identityhttp.NewIdentity(importApp(&importSpy{}, importOutcome{preview: command.UserImportPreview{Rows: 1, NewAccounts: 1}}), 0, false, nil)
	got := goodUpload().previewing(t, h, "")
	var body map[string]json.RawMessage
	if err := json.Unmarshal(got.body, &body); err != nil {
		t.Fatal(err)
	}
	if string(body["emailBudget"]) != "null" {
		t.Errorf("emailBudget = %s, want null", body["emailBudget"])
	}
	for _, key := range []string{"invalid", "classes"} {
		if string(body[key]) != "[]" {
			t.Errorf("%s = %s, want []", key, body[key])
		}
	}
}

func TestCreatePassesTheRequestIDTheNameAndTheFileOnAndAnswers202(t *testing.T) {
	spy := &importSpy{}
	job := aJob()
	h := identityhttp.NewIdentity(importApp(spy, importOutcome{create: command.CreateUserImportResult{Job: job}}), 0, false, nil)
	up := goodUpload()

	got := up.creating(t, h, "")
	if got.err != nil || got.status != http.StatusAccepted {
		t.Fatalf("answered %d, %v: %s", got.status, got.err, got.body)
	}
	cmd := spy.create
	if cmd.RequestID != up.fields["requestId"] || cmd.Filename != "danh-sach.csv" || !bytes.Equal(cmd.File, up.file) || cmd.DefaultRole != domain.DefaultStudent || cmd.Delivery != domain.ImportEmailLink {
		t.Errorf("command = %+v", cmd)
	}
	var body struct {
		ID        string `json:"id"`
		Status    string `json:"status"`
		Total     int    `json:"total"`
		Processed int    `json:"processed"`
		Skipped   int    `json:"skipped"`
		Invalid   int    `json:"invalid"`
		Delivery  string `json:"delivery"`
		Sheet     any    `json:"sheet"`
	}
	if err := json.Unmarshal(got.body, &body); err != nil {
		t.Fatal(err)
	}
	if body.ID != job.ID || body.Status != "queued" || body.Total != 3 || body.Processed != 1 || body.Skipped != 1 || body.Invalid != 2 || body.Delivery != "email_link" {
		t.Errorf("body = %+v", body)
	}
	if strings.Contains(string(got.body), "danh-sach") || strings.Contains(string(got.body), job.CreatedBy) {
		t.Errorf("the job carries the file's name or its owner: %s", got.body)
	}
}

func TestAJobDeliveredBySheetReportsTheSheetAndAnEmailJobReportsNone(t *testing.T) {
	expires := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	sheetJob := aJob()
	sheetJob.Delivery, sheetJob.Status, sheetJob.Created, sheetJob.Skipped = domain.ImportSheet, domain.ImportDone, 3, 0
	sheetJob.SheetExpiresAt = &expires
	state := domain.ImportSheetState{Available: true, ExpiresAt: &expires}

	h := identityhttp.NewIdentity(importApp(&importSpy{}, importOutcome{get: query.GetUserImportResult{Job: sheetJob, Sheet: &state}}), 0, false, nil)
	got := importCall(t, anAdmin(), "", func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.GetUserImport(ctx, openapi.GetUserImportRequestObject{Id: uuid.MustParse(sheetJob.ID)})
		return render(w, openapi.GetUserImportResponseObject.VisitGetUserImportResponse, response, err)
	})
	if got.status != http.StatusOK {
		t.Fatalf("answered %d: %s", got.status, got.body)
	}
	var body struct {
		Sheet *struct {
			Available    bool       `json:"available"`
			ExpiresAt    *time.Time `json:"expiresAt"`
			DownloadedAt *time.Time `json:"downloadedAt"`
		} `json:"sheet"`
	}
	if err := json.Unmarshal(got.body, &body); err != nil {
		t.Fatal(err)
	}
	if body.Sheet == nil || !body.Sheet.Available || body.Sheet.ExpiresAt == nil || !body.Sheet.ExpiresAt.Equal(expires) || body.Sheet.DownloadedAt != nil {
		t.Errorf("sheet = %+v, want it available until %s", body.Sheet, expires)
	}

	mail := identityhttp.NewIdentity(importApp(&importSpy{}, importOutcome{get: query.GetUserImportResult{Job: aJob()}}), 0, false, nil)
	got = importCall(t, anAdmin(), "", func(ctx context.Context, w http.ResponseWriter) error {
		response, err := mail.GetUserImport(ctx, openapi.GetUserImportRequestObject{Id: uuid.New()})
		return render(w, openapi.GetUserImportResponseObject.VisitGetUserImportResponse, response, err)
	})
	if strings.Contains(string(got.body), `"sheet":{`) {
		t.Errorf("an email job reports a sheet: %s", got.body)
	}
}

func downloadingTheSheet(t *testing.T, caller access.Principal, h identityhttp.Identity, id uuid.UUID, accept string) served {
	t.Helper()
	return importCall(t, caller, accept, func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.DownloadUserImportSheet(ctx, openapi.DownloadUserImportSheetRequestObject{Id: id})
		return render(w, openapi.DownloadUserImportSheetResponseObject.VisitDownloadUserImportSheetResponse, response, err)
	})
}

func TestTheSheetIsACSVAttachmentOfTheAccountsAndPasswordsThatNothingCaches(t *testing.T) {
	spy := &importSpy{}
	rows := []domain.SheetRow{
		{FullName: "Nguyễn Văn An", Email: "an@example.test", Role: "Học viên", Class: "Lớp 10A1", Password: "cam-trai-nui-42"},
		{FullName: `=HYPERLINK("http://x")`, Email: "x@example.test", Role: "Giáo viên", Password: "bien-xanh-mua-07"},
	}
	h := identityhttp.NewIdentity(importApp(spy, importOutcome{sheet: rows}), 0, false, nil)
	id := uuid.New()
	admin := anAdmin()

	got := downloadingTheSheet(t, admin, h, id, "")

	if got.status != http.StatusOK {
		t.Fatalf("answered %d: %s", got.status, got.body)
	}
	if got.header.Get("Cache-Control") != "no-store" || !strings.HasPrefix(got.header.Get("Content-Type"), "text/csv") {
		t.Errorf("headers = %v, want text/csv and no-store", got.header)
	}
	if disposition := got.header.Get("Content-Disposition"); !regexp.MustCompile(`^attachment; filename="new-accounts-\d{8}\.csv"$`).MatchString(disposition) {
		t.Errorf("Content-Disposition = %q, want an attachment named new-accounts-YYYYMMDD.csv", disposition)
	}
	want := "\xEF\xBB\xBF" + "full_name,email,role,class,temporary_password\r\n" +
		"Nguyễn Văn An,an@example.test,Học viên,Lớp 10A1,cam-trai-nui-42\r\n" +
		"\"'=HYPERLINK(\"\"http://x\"\")\",x@example.test,Giáo viên,,bien-xanh-mua-07\r\n"
	if string(got.body) != want {
		t.Errorf("body = %q, want %q", got.body, want)
	}
	if spy.sheet.ID != id.String() || spy.sheet.Request.ActorID != admin.UserID || spy.calls != 1 {
		t.Errorf("the command got %+v after %d call(s), want the caller and the job id once", spy.sheet, spy.calls)
	}
}

func TestASheetThatCannotBeHandedOutAnswers404Or410InTheCallersLanguage(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
		code   openapi.ErrorCode
		vi, en string
	}{
		{domain.ErrUserImportNotFound, http.StatusNotFound, openapi.NOTFOUND, "Không tìm thấy lần nhập người dùng này.", "This user import was not found."},
		{domain.ErrSheetUnavailable, http.StatusGone, openapi.SHEETUNAVAILABLE,
			"Không còn bảng mật khẩu để tải: lần nhập này chưa xong, gửi qua email, không tạo tài khoản nào, đã được tải một lần hoặc đã quá 24 giờ.",
			"There is no sheet to download: this import is not finished, was sent by email, created no account, was already downloaded, or finished more than 24 hours ago."},
	} {
		h := identityhttp.NewIdentity(importApp(&importSpy{}, importOutcome{err: c.err}), 0, false, nil)
		for accept, want := range map[string]string{"vi": c.vi, "en": c.en} {
			got := downloadingTheSheet(t, anAdmin(), h, uuid.New(), accept)
			code, message := got.errorCode(t)
			if got.status != c.status || code != c.code || message != want {
				t.Errorf("%v in %s: %d %s %q, want %d %s %q", c.err, accept, got.status, code, message, c.status, c.code, want)
			}
			if got.header.Get("Content-Disposition") != "" {
				t.Errorf("%v: a refusal carries a file name", c.err)
			}
		}
	}
}

func TestASheetOnAServerThatCannotOpenItAnswers501(t *testing.T) {
	failing := identityhttp.NewIdentity(importApp(&importSpy{}, importOutcome{err: domain.ErrImportSheetsOff}), 0, false, nil)
	if got := downloadingTheSheet(t, anAdmin(), failing, uuid.New(), ""); !errors.Is(got.err, httpx.ErrNotImplemented) {
		t.Errorf("a server that cannot open a sheet: %v (status %d), want httpx.ErrNotImplemented", got.err, got.status)
	}
	none := identityhttp.NewIdentity(&application.Application{}, 0, false, nil)
	if got := downloadingTheSheet(t, anAdmin(), none, uuid.New(), ""); !errors.Is(got.err, httpx.ErrNotImplemented) {
		t.Errorf("without an import application: %v (status %d), want httpx.ErrNotImplemented", got.err, got.status)
	}
}

func TestAnUnknownOrForeignImportAnswers404InTheCallersLanguage(t *testing.T) {
	h := identityhttp.NewIdentity(importApp(&importSpy{}, importOutcome{err: domain.ErrUserImportNotFound}), 0, false, nil)
	for accept, want := range map[string]string{"vi": "Không tìm thấy lần nhập người dùng này.", "en": "This user import was not found."} {
		got := importCall(t, anAdmin(), accept, func(ctx context.Context, w http.ResponseWriter) error {
			response, err := h.GetUserImport(ctx, openapi.GetUserImportRequestObject{Id: uuid.New()})
			return render(w, openapi.GetUserImportResponseObject.VisitGetUserImportResponse, response, err)
		})
		code, message := got.errorCode(t)
		if got.status != http.StatusNotFound || code != openapi.NOTFOUND || message != want {
			t.Errorf("%s: %d %s %q, want 404 NOT_FOUND %q", accept, got.status, code, message, want)
		}
	}
}

func TestAFormThatIsNotTheExpectedOneIsRefusedByNamingTheField(t *testing.T) {
	for name, tc := range map[string]struct {
		edit  func(*importFile)
		field string
		both  bool
	}{
		"no file":                  {func(u *importFile) { u.file = nil }, "file", true},
		"two files":                {func(u *importFile) { u.extraFile = true }, "file", true},
		"no default role":          {func(u *importFile) { delete(u.fields, "defaultRole") }, "defaultRole", true},
		"an Admin as default role": {func(u *importFile) { u.fields["defaultRole"] = "admin" }, "defaultRole", true},
		"a role in capitals":       {func(u *importFile) { u.fields["defaultRole"] = "Student" }, "defaultRole", true},
		"no delivery":              {func(u *importFile) { delete(u.fields, "delivery") }, "delivery", true},
		"a made-up delivery":       {func(u *importFile) { u.fields["delivery"] = "post" }, "delivery", true},
		"no request id":            {func(u *importFile) { delete(u.fields, "requestId") }, "requestId", false},
		"a request id that is not": {func(u *importFile) { u.fields["requestId"] = "123" }, "requestId", false},
	} {
		spy := &importSpy{}
		h := identityhttp.NewIdentity(importApp(spy, importOutcome{}), 0, false, nil)
		up := goodUpload()
		tc.edit(&up)

		got := up.creating(t, h, "")
		assertBadForm(t, name+" (create)", got, tc.field)
		preview := up.previewing(t, h, "")
		if tc.both {
			assertBadForm(t, name+" (preview)", preview, tc.field)
		} else if preview.status != http.StatusOK {
			t.Errorf("%s: preview answered %d: the request id is the create's alone", name, preview.status)
		}
		if tc.both && spy.calls != 0 {
			t.Errorf("%s: the command ran %d time(s) on a refused form", name, spy.calls)
		}
	}
}

func assertBadForm(t *testing.T, name string, got served, field string) {
	t.Helper()
	code, _ := got.errorCode(t)
	if got.status != http.StatusBadRequest || code != openapi.VALIDATIONFAILED {
		t.Errorf("%s: answered %d %s, want 400 VALIDATION_FAILED: %s", name, got.status, code, got.body)
		return
	}
	var body struct {
		Error struct {
			Details map[string]string `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(got.body, &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body.Error.Details[field]; !ok || len(body.Error.Details) != 1 {
		t.Errorf("%s: details = %v, want only %s", name, body.Error.Details, field)
	}
}

var theReadersRefusals = []struct {
	err    error
	status int
	code   openapi.ErrorCode
	way    string
}{
	{tabular.ErrTooLarge, http.StatusRequestEntityTooLarge, openapi.USERIMPORTFILETOOLARGE, "CSV"},
	{tabular.ErrNotUTF8, http.StatusUnprocessableEntity, openapi.CSVNOTUTF8, "UTF-8"},
	{tabular.ErrMacros, http.StatusUnprocessableEntity, openapi.XLSXMACROS, "macro"},
	{tabular.ErrEncrypted, http.StatusUnprocessableEntity, openapi.XLSXENCRYPTED, "mật khẩu"},
	{tabular.ErrEmpty, http.StatusUnprocessableEntity, openapi.USERIMPORTFILEEMPTY, "full_name"},
	{tabular.ErrTooManyRows, http.StatusUnprocessableEntity, openapi.USERIMPORTTOOMANYROWS, "2.000"},
	{tabular.ErrTooManyColumns, http.StatusUnprocessableEntity, openapi.USERIMPORTFILEUNREADABLE, "full_name"},
	{tabular.ErrCellTooLong, http.StatusUnprocessableEntity, openapi.USERIMPORTFILEUNREADABLE, "1.000"},
	{tabular.ErrTooManyEntries, http.StatusUnprocessableEntity, openapi.USERIMPORTFILEUNREADABLE, "CSV"},
	{tabular.ErrExpandedTooLarge, http.StatusUnprocessableEntity, openapi.USERIMPORTFILEUNREADABLE, "CSV"},
	{tabular.ErrSheetTooLarge, http.StatusUnprocessableEntity, openapi.USERIMPORTFILEUNREADABLE, "bảng tính mới"},
	{tabular.ErrPartTooLarge, http.StatusUnprocessableEntity, openapi.USERIMPORTFILEUNREADABLE, "bảng tính mới"},
	{tabular.ErrTooManySheets, http.StatusUnprocessableEntity, openapi.USERIMPORTFILEUNREADABLE, "CSV"},
	{tabular.ErrNotWorksheet, http.StatusUnprocessableEntity, openapi.USERIMPORTFILEUNREADABLE, "trang tính đầu tiên"},
	{tabular.ErrLayout, http.StatusUnprocessableEntity, openapi.USERIMPORTFILEUNREADABLE, "LibreOffice"},
	{tabular.ErrFormat, http.StatusUnprocessableEntity, openapi.USERIMPORTFILEUNREADABLE, "CSV"},
}

func TestEveryErrorOfTheReaderAnswersWithItsOwnCodeAndWordsInBothLanguages(t *testing.T) {
	for _, c := range theReadersRefusals {
		spy := &importSpy{}
		leak := fmt.Sprintf("%v: row 3 reads kín-mật@example.test", c.err)
		h := identityhttp.NewIdentity(importApp(spy, importOutcome{err: fmt.Errorf("%w; %s", c.err, leak)}), 0, false, nil)
		vi := goodUpload().creating(t, h, "vi")
		en := goodUpload().previewing(t, h, "en")

		viCode, viMessage := vi.errorCode(t)
		enCode, enMessage := en.errorCode(t)
		if vi.status != c.status || en.status != c.status || viCode != c.code || enCode != c.code {
			t.Errorf("%v: create %d %s, preview %d %s, want %d %s", c.err, vi.status, viCode, en.status, enCode, c.status, c.code)
		}
		if viMessage == enMessage || viMessage == "" || enMessage == "" {
			t.Errorf("%v: vi %q and en %q, want two different non-empty messages", c.err, viMessage, enMessage)
		}
		if !strings.Contains(viMessage, c.way) && !strings.Contains(enMessage, c.way) {
			t.Errorf("%v: neither message names the way out %q: %q / %q", c.err, c.way, viMessage, enMessage)
		}
		for _, body := range []string{string(vi.body), string(en.body)} {
			if strings.Contains(body, "kín-mật") || strings.Contains(body, c.err.Error()) || strings.Contains(body, "row 3") {
				t.Errorf("%v: the answer carries text of the reader's error: %s", c.err, body)
			}
		}
	}
}

func TestEachErrorOfTheReaderHasWordsOfItsOwn(t *testing.T) {
	seen := map[string]error{}
	h := func(err error) identityhttp.Identity {
		return identityhttp.NewIdentity(importApp(&importSpy{}, importOutcome{err: err}), 0, false, nil)
	}
	for _, c := range theReadersRefusals {
		_, message := goodUpload().creating(t, h(c.err), "en").errorCode(t)
		if other, dup := seen[message]; dup {
			t.Errorf("%v and %v share the message %q", c.err, other, message)
		}
		seen[message] = c.err
	}
}

func TestAnUnrecognisedErrorIsAFaultAndShowsNothingOfIt(t *testing.T) {
	h := identityhttp.NewIdentity(importApp(&importSpy{}, importOutcome{err: errors.New("pq: kín-mật@example.test")}), 0, false, nil)
	got := goodUpload().creating(t, h, "")
	if got.err == nil || len(got.body) != 0 {
		t.Errorf("answered %d, %v: %s, want the error handed on and nothing written", got.status, got.err, got.body)
	}
}

func TestAHeaderThatCannotBeUsedNamesTheColumnsToFix(t *testing.T) {
	for _, c := range []struct {
		problem    domain.ImportHeaderProblem
		vi, en     string
		notContain string
	}{
		{domain.ImportHeaderProblem{Missing: []domain.ImportField{domain.FieldEmail}}, "thiếu cột bắt buộc: email", "is missing the required column: email", "lặp"},
		{domain.ImportHeaderProblem{Missing: []domain.ImportField{domain.FieldFullName, domain.FieldEmail}}, "thiếu cột bắt buộc: full_name, email", "is missing the required column: full_name, email", "lặp"},
		{domain.ImportHeaderProblem{Repeated: []domain.ImportField{domain.FieldEmail}}, "có cột bị lặp: email", "repeats a column: email", "thiếu"},
	} {
		err := fmt.Errorf("read: %w", &domain.ImportHeaderError{Problem: c.problem})
		h := identityhttp.NewIdentity(importApp(&importSpy{}, importOutcome{err: err}), 0, false, nil)
		for accept, want := range map[string]string{"vi": c.vi, "en": c.en} {
			got := goodUpload().previewing(t, h, accept)
			code, message := got.errorCode(t)
			if got.status != http.StatusUnprocessableEntity || code != openapi.USERIMPORTCOLUMNS || !strings.Contains(message, want) {
				t.Errorf("%s %+v: %d %s %q, want 422 USER_IMPORT_COLUMNS naming %q", accept, c.problem, got.status, code, message, want)
			}
			if strings.Contains(message, c.notContain) {
				t.Errorf("%s: %q mentions what is not wrong", accept, message)
			}
		}
	}
}

func TestTheRulesOfTheRequestHaveTheirStatusAndCode(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
		code   openapi.ErrorCode
	}{
		{accessdomain.ErrTargetRoleNotAllowed, http.StatusForbidden, openapi.TARGETROLENOTALLOWED},
		{domain.ErrImportNothingToCreate, http.StatusUnprocessableEntity, openapi.USERIMPORTNOTHINGTOCREATE},
		{domain.ErrPasswordSignInDisabled, http.StatusUnprocessableEntity, openapi.PASSWORDSIGNINDISABLED},
	} {
		h := identityhttp.NewIdentity(importApp(&importSpy{}, importOutcome{err: c.err}), 0, false, nil)
		for name, got := range map[string]served{"create": goodUpload().creating(t, h, ""), "preview": goodUpload().previewing(t, h, "")} {
			code, message := got.errorCode(t)
			if got.status != c.status || code != c.code || message == "" {
				t.Errorf("%s on %v: %d %s %q, want %d %s", name, c.err, got.status, code, message, c.status, c.code)
			}
		}
	}
}

func TestAServerThatCannotSendALinkOrKeepASheetAnswers501(t *testing.T) {
	for _, err := range []error{domain.ErrSetPasswordUnavailable, domain.ErrImportSheetsOff} {
		h := identityhttp.NewIdentity(importApp(&importSpy{}, importOutcome{err: err}), 0, false, nil)
		for name, got := range map[string]served{"create": goodUpload().creating(t, h, ""), "preview": goodUpload().previewing(t, h, "")} {
			if !errors.Is(got.err, httpx.ErrNotImplemented) {
				t.Errorf("%s on %v: got %v (status %d), want httpx.ErrNotImplemented", name, err, got.err, got.status)
			}
		}
	}
}

func TestWithoutAnImportApplicationEveryOperationAnswers501(t *testing.T) {
	h := identityhttp.NewIdentity(&application.Application{}, 0, false, nil)
	if got := goodUpload().creating(t, h, ""); !errors.Is(got.err, httpx.ErrNotImplemented) {
		t.Errorf("create: %v", got.err)
	}
	if got := goodUpload().previewing(t, h, ""); !errors.Is(got.err, httpx.ErrNotImplemented) {
		t.Errorf("preview: %v", got.err)
	}
	got := importCall(t, anAdmin(), "", func(ctx context.Context, w http.ResponseWriter) error {
		response, err := h.GetUserImport(ctx, openapi.GetUserImportRequestObject{Id: uuid.New()})
		return render(w, openapi.GetUserImportResponseObject.VisitGetUserImportResponse, response, err)
	})
	if !errors.Is(got.err, httpx.ErrNotImplemented) {
		t.Errorf("get: %v", got.err)
	}
}

type cutOff struct {
	head []byte
	read bool
}

func (c *cutOff) Read(p []byte) (int, error) {
	if c.read {
		return 0, &http.MaxBytesError{Limit: 5<<20 + 16<<10}
	}
	c.read = true
	return copy(p, c.head), nil
}

func TestABodyOverItsBudgetAnswers413WithTheWayOutInBothLanguages(t *testing.T) {
	for accept, want := range map[string]string{"vi": "CSV UTF-8", "en": "CSV UTF-8"} {
		h := identityhttp.NewIdentity(importApp(&importSpy{}, importOutcome{}), 0, false, nil)
		boundary := "xyz"
		head := []byte("--" + boundary + "\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.csv\"\r\n\r\nfull_name,email\n")
		for name, call := range map[string]func(context.Context, http.ResponseWriter) error{
			"preview": func(ctx context.Context, w http.ResponseWriter) error {
				response, err := h.PreviewUserImport(ctx, openapi.PreviewUserImportRequestObject{Body: multipart.NewReader(&cutOff{head: head}, boundary)})
				return render(w, openapi.PreviewUserImportResponseObject.VisitPreviewUserImportResponse, response, err)
			},
			"create": func(ctx context.Context, w http.ResponseWriter) error {
				response, err := h.CreateUserImport(ctx, openapi.CreateUserImportRequestObject{Body: multipart.NewReader(&cutOff{head: head}, boundary)})
				return render(w, openapi.CreateUserImportResponseObject.VisitCreateUserImportResponse, response, err)
			},
		} {
			got := importCall(t, anAdmin(), accept, call)
			code, message := got.errorCode(t)
			if got.status != http.StatusRequestEntityTooLarge || code != openapi.USERIMPORTFILETOOLARGE || !strings.Contains(message, want) {
				t.Errorf("%s %s: %d %s %q, want 413 USER_IMPORT_FILE_TOO_LARGE naming %q", name, accept, got.status, code, message, want)
			}
		}
	}
}

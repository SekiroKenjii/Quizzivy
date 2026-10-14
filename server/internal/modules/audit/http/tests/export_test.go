package http_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/audit/domain"
	audithttp "quizzivy/internal/modules/audit/http"
)

func exporting(transport audithttp.Audit, params openapi.ExportAuditLogParams) serving {
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.ExportAuditLog(ctx, openapi.ExportAuditLogRequestObject{Params: params})
		if err != nil {
			return err
		}
		return response.VisitExportAuditLogResponse(w)
	}
}

func csvOf(t *testing.T, body []byte) [][]string {
	t.Helper()
	if !bytes.HasPrefix(body, []byte("\xEF\xBB\xBF")) {
		t.Fatalf("the file does not start with a byte-order mark: %q", body[:min(10, len(body))])
	}
	if bytes.Contains(bytes.ReplaceAll(body, []byte("\r\n"), nil), []byte("\n")) {
		t.Error("a line ends in a bare LF, want CRLF throughout")
	}
	rows, err := csv.NewReader(bytes.NewReader(body[3:])).ReadAll()
	if err != nil {
		t.Fatalf("not CSV: %v", err)
	}
	return rows
}

func someEntries(n int) *log {
	l := &log{}
	for i := range n {
		r := record(int64(i+1), march(10+i%4, 9, i%60), "class.created", "class")
		r.Actor = &domain.Actor{ID: teacherID, Name: "Cô Lan"}
		l.records = append(l.records, r)
	}
	return l
}

func TestACsvIsAnAttachmentWithABomCrlfAndTheViewersLanguage(t *testing.T) {
	l := someEntries(3)
	classID := "0195c4b0-0000-7000-8000-0000000000c1"
	l.records[2].EntityID = &classID
	l.records[2].UserAgent = ptr("Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0 Safari/537.36")
	l.records[2].Diff = []byte(`{"name": {"old": "A", "new": "B"}, "note": "x"}`)
	l.labels = map[domain.EntityRef]string{{Type: "class", ID: classID}: "Lớp 9A"}
	transport := l.transport()

	for language, want := range map[string]struct{ time, system, action string }{
		"vi": {"Thời gian (Asia/Ho_Chi_Minh)", "Hệ thống", "Đã tạo lớp"},
		"en": {"Time (Asia/Ho_Chi_Minh)", "System", "Created a class"},
	} {
		rec, aborted := answered(t, language, exporting(transport, openapi.ExportAuditLogParams{Period: ptr(openapi.ExportAuditLogParamsPeriodN30d)}))
		if aborted || rec.Code != http.StatusOK {
			t.Fatalf("%s: %d aborted %v: %s", language, rec.Code, aborted, rec.Body)
		}
		if got := rec.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
			t.Errorf("Content-Type %q", got)
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control %q", got)
		}
		if got := rec.Header().Get("Content-Disposition"); got != `attachment; filename="audit-log-20010315.csv"` {
			t.Errorf("Content-Disposition %q", got)
		}
		rows := csvOf(t, rec.Body.Bytes())
		if len(rows) != 4 || rows[0][1] != want.time {
			t.Fatalf("%s: %d rows, header %v", language, len(rows), rows[0])
		}
		if rows[1][4] != want.action || rows[1][5] != "class.created" || rows[1][2] != "Cô Lan" {
			t.Errorf("%s: row %v", language, rows[1])
		}
		newest := rows[1]
		if newest[0] != "3" || newest[8] != "Lớp 9A" || newest[11] != "Windows · Chrome" || newest[12] != "name: A -> B" || newest[13] != `{"note":"x"}` {
			t.Errorf("%s: the newest row %v", language, newest)
		}
	}
	l.records[0].Actor = nil
	rec, _ := answered(t, "en", exporting(transport, openapi.ExportAuditLogParams{}))
	if rows := csvOf(t, rec.Body.Bytes()); rows[3][2] != "System" {
		t.Errorf("an entry with no actor reads %q", rows[3][2])
	}
}

func TestACellThatASpreadsheetWouldRunIsEscaped(t *testing.T) {
	l := someEntries(1)
	l.records[0].Actor.Name = `=HYPERLINK("http://evil.example","click")`
	id := "0195c4b0-0000-7000-8000-0000000000c2"
	l.records[0].EntityID = &id
	l.labels = map[domain.EntityRef]string{{Type: "class", ID: id}: "+cmd|' /C calc'!A0"}

	rec, _ := answered(t, "en", exporting(l.transport(), openapi.ExportAuditLogParams{}))

	rows := csvOf(t, rec.Body.Bytes())
	if rows[1][2] != `'=HYPERLINK("http://evil.example","click")` {
		t.Errorf("the actor cell is %q", rows[1][2])
	}
	if rows[1][8] != "'+cmd|' /C calc'!A0" {
		t.Errorf("the entity cell is %q", rows[1][8])
	}
}

func TestAJsonFileIsAStreamedArrayOfTheEntriesTheListReturns(t *testing.T) {
	l := someEntries(3)
	l.records[0].Diff = []byte(`{"password": "hunter2", "code": "ABCDEFGHJK", "status": {"old": "a", "new": "b"}}`)
	transport := l.transport()

	rec, aborted := answered(t, "en", exporting(transport, openapi.ExportAuditLogParams{Format: ptr(openapi.ExportAuditLogParamsFormat("json"))}))

	if aborted || rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("%d %s aborted %v", rec.Code, rec.Header().Get("Content-Type"), aborted)
	}
	if rec.Header().Get("Content-Disposition") != `attachment; filename="audit-log-20010315.json"` || rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("headers %v", rec.Header())
	}
	var entries []openapi.AuditEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("%v: %s", err, rec.Body)
	}
	if len(entries) != 3 || entries[0].Id < entries[1].Id && entries[0].OccurredAt.Before(entries[1].OccurredAt) {
		t.Errorf("entries %+v", entries)
	}
	if strings.Contains(rec.Body.String(), "hunter2") || strings.Contains(rec.Body.String(), "ABCDEFGHJK") {
		t.Errorf("a secret is in the file: %s", rec.Body)
	}
	listed, _ := answered(t, "en", listing(transport, openapi.ListAuditLogParams{}))
	var page struct {
		Items []openapi.AuditEntry `json:"items"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(entries)
	b, _ := json.Marshal(page.Items)
	if !bytes.Equal(a, b) {
		t.Errorf("the file and the list differ:\n%s\n%s", a, b)
	}
}

func TestAnEmptyJsonFileIsAnEmptyArrayAndAnEmptyCsvIsItsHeader(t *testing.T) {
	l := &log{}
	rec, _ := answered(t, "en", exporting(l.transport(), openapi.ExportAuditLogParams{Format: ptr(openapi.ExportAuditLogParamsFormat("json"))}))
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("json %q", rec.Body)
	}
	rec, _ = answered(t, "en", exporting(l.transport(), openapi.ExportAuditLogParams{}))
	if rows := csvOf(t, rec.Body.Bytes()); len(rows) != 1 {
		t.Errorf("%d rows, want the header alone", len(rows))
	}
}

func TestAFileLargerThanOneBatchIsWholeAndInOrder(t *testing.T) {
	n := domain.ExportBatchSize*2 + 345
	l := someEntries(n)
	transport := l.transport()

	rec, aborted := answered(t, "en", exporting(transport, openapi.ExportAuditLogParams{Period: ptr(openapi.ExportAuditLogParamsPeriodYear)}))

	if aborted {
		t.Fatal("aborted")
	}
	rows := csvOf(t, rec.Body.Bytes())
	if len(rows) != n+1 {
		t.Fatalf("%d rows, want %d", len(rows), n+1)
	}
	seen := map[string]bool{}
	var previous time.Time
	for i, row := range rows[1:] {
		if seen[row[0]] {
			t.Fatalf("entry %s twice", row[0])
		}
		seen[row[0]] = true
		at, err := time.Parse(time.RFC3339, row[1])
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 && at.After(previous) {
			t.Fatalf("row %d is newer than the one before it", i)
		}
		previous = at
	}
	if l.batches != 3 {
		t.Errorf("%d batches read, want 3", l.batches)
	}
}

func TestTheExportIsAuditedBeforeItsFirstBatchIsRead(t *testing.T) {
	l := someEntries(5)
	transport := l.transport()

	answered(t, "en", exporting(transport, openapi.ExportAuditLogParams{
		Period: ptr(openapi.ExportAuditLogParamsPeriodN30d), Format: ptr(openapi.ExportAuditLogParamsFormat("json")),
		Category: ptr(openapi.AuditCategoryClasses), Q: ptr("lan"),
	}))

	if len(l.exports) != 1 {
		t.Fatalf("%d exports recorded", len(l.exports))
	}
	e := l.exports[0]
	if e.ActorID != viewer || e.Period != domain.Month || e.Format != "json" || e.Rows != 5 || e.Category != domain.Classes || e.Query != "lan" {
		t.Errorf("export %+v", e)
	}
	if e.IP != "203.0.113.9" || !strings.Contains(e.UserAgent, "Firefox") || !e.At.Equal(march(15, 12, 0)) {
		t.Errorf("from %q %q at %v", e.IP, e.UserAgent, e.At)
	}
	if len(l.batchSawAudit) == 0 || !l.batchSawAudit[0] {
		t.Error("a batch was read before the export was recorded")
	}
}

func TestTheDefaultFileIsTheWeeksCsv(t *testing.T) {
	l := someEntries(2)
	l.records[0].OccurredAt = march(8, 9, 0)
	l.records[1].OccurredAt = march(9, 9, 0)

	rec, _ := answered(t, "en", exporting(l.transport(), openapi.ExportAuditLogParams{}))

	if len(csvOf(t, rec.Body.Bytes())) != 2 {
		t.Errorf("the default period did not stop at the 9th: %s", rec.Body)
	}
	if l.exports[0].Period != domain.Week || l.exports[0].Format != "csv" {
		t.Errorf("recorded %+v", l.exports[0])
	}
}

func TestMoreThanTheCapIsRefusedBeforeAnythingIsSentOrRecorded(t *testing.T) {
	l := someEntries(1)
	over := domain.MaxExportRows + 1
	l.countRows = &over

	rec, aborted := answered(t, "vi", exporting(l.transport(), openapi.ExportAuditLogParams{}))

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%v: %s", err, rec.Body)
	}
	if aborted || rec.Code != http.StatusUnprocessableEntity || body.Error.Code != "EXPORT_TOO_LARGE" || !strings.Contains(body.Error.Message, "100000") {
		t.Errorf("%d %+v", rec.Code, body.Error)
	}
	if rec.Header().Get("Content-Disposition") != "" {
		t.Error("the refusal is an attachment")
	}
	if len(l.exports) != 0 || l.batches != 0 {
		t.Errorf("recorded %d exports and read %d batches", len(l.exports), l.batches)
	}
}

func TestTheCapItselfIsAllowed(t *testing.T) {
	l := someEntries(1)
	atCap := domain.MaxExportRows
	l.countRows = &atCap
	rec, _ := answered(t, "en", exporting(l.transport(), openapi.ExportAuditLogParams{}))
	if rec.Code != http.StatusOK || len(l.exports) != 1 || l.exports[0].Rows != domain.MaxExportRows {
		t.Errorf("%d, exports %+v", rec.Code, l.exports)
	}
}

func TestAFailureAfterTheFirstByteEndsTheConnectionAndNotTheFileCleanly(t *testing.T) {
	l := someEntries(domain.ExportBatchSize + 10)
	l.failBatchFrom = 2

	rec, aborted := answered(t, "en", exporting(l.transport(), openapi.ExportAuditLogParams{}))

	if !aborted {
		t.Errorf("the handler returned normally after %d bytes; it must abort the response so the client sees an incomplete download", rec.Body.Len())
	}
	if strings.Contains(rec.Body.String(), "something went wrong") || strings.Contains(strings.ToLower(rec.Body.String()), `"error"`) {
		t.Error("an error body was written into the file")
	}
}

func TestAFailureBeforeTheFirstByteIsAnOrdinaryErrorAndRecordsNothing(t *testing.T) {
	l := someEntries(3)
	l.countErr = errors.New("the database went away")

	var err error
	answered(t, "en", func(ctx context.Context, _ http.ResponseWriter) error {
		_, err = l.transport().ExportAuditLog(ctx, openapi.ExportAuditLogRequestObject{})
		return nil
	})

	if err == nil {
		t.Fatal("the export began although the entries could not be counted")
	}
	if len(l.exports) != 0 || l.batches != 0 {
		t.Errorf("recorded %d exports and read %d batches", len(l.exports), l.batches)
	}
}

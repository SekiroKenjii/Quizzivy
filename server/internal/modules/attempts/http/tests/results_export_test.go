package http_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"
)

const byteOrderMark = "\xEF\xBB\xBF"

func exportTransport(export domain.ResultsExport, err error) attemptshttp.Attempts {
	app := &application.Application{Queries: application.Queries{
		ResultsExport: cqrs.HandlerFunc[query.ResultsExport, domain.ResultsExport](func(context.Context, query.ResultsExport) (domain.ResultsExport, error) {
			return export, err
		}),
	}}
	return attemptshttp.NewAttempts(app, nil, nil, nil)
}

func exportedIn(t *testing.T, acceptLanguage string, transport attemptshttp.Attempts) openapi.ExportResultsCsvResponseObject {
	t.Helper()
	var response openapi.ExportResultsCsvResponseObject
	var served error
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			response, served = transport.ExportResultsCsv(r.Context(), openapi.ExportResultsCsvRequestObject{
				Params: openapi.ExportResultsCsvParams{Ids: []openapi.Uuid{uuid.New()}},
			})
		}))
	request := httptest.NewRequest(http.MethodGet, "/teacher/assignments/results.csv", nil)
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if served != nil {
		t.Fatal(served)
	}
	return response
}

func fileOf(t *testing.T, response openapi.ExportResultsCsvResponseObject) (openapi.ExportResultsCsv200TextcsvResponse, string) {
	t.Helper()
	file, ok := response.(openapi.ExportResultsCsv200TextcsvResponse)
	if !ok {
		t.Fatalf("response = %T, want the file", response)
	}
	raw, err := io.ReadAll(file.Body)
	if err != nil {
		t.Fatal(err)
	}
	return file, string(raw)
}

func recordsOf(t *testing.T, raw string) [][]string {
	t.Helper()
	if !strings.HasPrefix(raw, byteOrderMark) {
		t.Fatalf("the file starts %q, want a UTF-8 byte-order mark", raw[:min(len(raw), 8)])
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(raw, byteOrderMark)))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func points(v float64) *float64 { return &v }

func sampleExport() domain.ResultsExport {
	submitted := time.Date(2026, 10, 10, 20, 30, 0, 0, time.UTC)
	focus := 2
	return domain.ResultsExport{Zone: "Asia/Ho_Chi_Minh", Rows: []domain.ResultRow{
		{
			AssignmentTitle: "Kiểm tra Unit 3", Version: 2, Classes: "6A; 6B", StudentName: "Nguyễn Văn An", Email: "an@example.com",
			State: "graded", Earned: points(7.5), Total: points(10), SubmittedAt: &submitted, FocusLoss: &focus, Flagged: true,
		},
		{AssignmentTitle: "Kiểm tra Unit 3", Version: 2, Classes: "6A", StudentName: "Trần Thị Bé", Email: "be@example.com", State: "not_started"},
	}}
}

func TestTheExportIsAMarkedCRLFFileWithTheContractsHeaders(t *testing.T) {
	file, raw := fileOf(t, exportedIn(t, "", exportTransport(sampleExport(), nil)))

	if !strings.HasPrefix(raw, byteOrderMark) {
		t.Fatalf("the file starts %q, want a UTF-8 byte-order mark", raw[:min(len(raw), 8)])
	}
	if strings.Count(raw, "\n") != strings.Count(raw, "\r\n") {
		t.Errorf("the file has a line end that is not CRLF: %q", raw)
	}
	if file.ContentLength != int64(len(raw)) {
		t.Errorf("Content-Length %d, the file is %d bytes", file.ContentLength, len(raw))
	}
	if file.Headers.CacheControl != "private, no-store" {
		t.Errorf("Cache-Control = %q, want private, no-store", file.Headers.CacheControl)
	}
	if !regexp.MustCompile(`^attachment; filename="results-\d{8}\.csv"$`).MatchString(file.Headers.ContentDisposition) {
		t.Errorf("Content-Disposition = %q, want an attachment named results-YYYYMMDD.csv", file.Headers.ContentDisposition)
	}

	want := [][]string{
		{"Bài giao", "Phiên bản", "Lớp", "Học viên", "Email", "Trạng thái", "Điểm", "Điểm tối đa", "Phần trăm", "Nộp bài (Asia/Ho_Chi_Minh)", "Số lần rời trang", "Gắn cờ"},
		{"Kiểm tra Unit 3", "2", "6A; 6B", "Nguyễn Văn An", "an@example.com", "Đã chấm", "7.5", "10", "75.0", "2026-10-11 03:30", "2", "Có"},
		{"Kiểm tra Unit 3", "2", "6A", "Trần Thị Bé", "be@example.com", "Chưa làm", "", "", "", "", "", "Không"},
	}
	if got := recordsOf(t, raw); !equalRecords(got, want) {
		t.Errorf("the file reads\n%v\nwant\n%v", got, want)
	}
}

func TestTheExportSpeaksEnglishToAReaderWhoPrefersIt(t *testing.T) {
	_, raw := fileOf(t, exportedIn(t, "en", exportTransport(sampleExport(), nil)))
	want := [][]string{
		{"Assignment", "Version", "Class", "Student", "Email", "Status", "Score", "Max", "Percent", "Submitted (Asia/Ho_Chi_Minh)", "Focus lost", "Flagged"},
		{"Kiểm tra Unit 3", "2", "6A; 6B", "Nguyễn Văn An", "an@example.com", "Graded", "7.5", "10", "75.0", "2026-10-11 03:30", "2", "Yes"},
		{"Kiểm tra Unit 3", "2", "6A", "Trần Thị Bé", "be@example.com", "Not started", "", "", "", "", "", "No"},
	}
	if got := recordsOf(t, raw); !equalRecords(got, want) {
		t.Errorf("the file reads\n%v\nwant\n%v", got, want)
	}
}

func TestEveryCellOfTheExportIsEscapedAgainstFormulas(t *testing.T) {
	export := sampleExport()
	export.Rows = []domain.ResultRow{
		{AssignmentTitle: `=HYPERLINK("http://evil.example","x")`, Version: 1, Classes: "+1", StudentName: "-2+3", Email: "@SUM(A1)", State: "not_started"},
		{AssignmentTitle: "  =1+1", Version: 1, Classes: "\t=1", StudentName: "＝1+1", Email: "ok@example.com", State: "not_started"},
	}
	_, raw := fileOf(t, exportedIn(t, "", exportTransport(export, nil)))

	want := [][]string{
		{`'=HYPERLINK("http://evil.example","x")`, "1", "'+1", "'-2+3", "'@SUM(A1)", "Chưa làm", "", "", "", "", "", "Không"},
		{"'  =1+1", "1", "'\t=1", "'＝1+1", "ok@example.com", "Chưa làm", "", "", "", "", "", "Không"},
	}
	if got := recordsOf(t, raw)[1:]; !equalRecords(got, want) {
		t.Errorf("the rows read\n%q\nwant\n%q", got, want)
	}
}

func TestTimesAreWrittenInTheCallersOwnZone(t *testing.T) {
	for zone, want := range map[string][2]string{
		"Asia/Ho_Chi_Minh":  {"Nộp bài (Asia/Ho_Chi_Minh)", "2026-10-11 03:30"},
		"America/New_York":  {"Nộp bài (America/New_York)", "2026-10-10 16:30"},
		"Mars/Olympus_Mons": {"Nộp bài (Asia/Ho_Chi_Minh)", "2026-10-11 03:30"},
		"":                  {"Nộp bài (Asia/Ho_Chi_Minh)", "2026-10-11 03:30"},
		"Local":             {"Nộp bài (Asia/Ho_Chi_Minh)", "2026-10-11 03:30"},
	} {
		export := sampleExport()
		export.Zone = zone
		_, raw := fileOf(t, exportedIn(t, "", exportTransport(export, nil)))
		records := recordsOf(t, raw)
		if records[0][9] != want[0] || records[1][9] != want[1] {
			t.Errorf("zone %q: header %q and time %q, want %q and %q", zone, records[0][9], records[1][9], want[0], want[1])
		}
	}
}

func TestAScoreStaysEmptyUntilEveryMarkIsIn(t *testing.T) {
	export := sampleExport()
	export.Rows[0].Earned, export.Rows[0].Total = nil, nil
	export.Rows[0].State = "submitted"
	_, raw := fileOf(t, exportedIn(t, "", exportTransport(export, nil)))
	row := recordsOf(t, raw)[1]
	if row[5] != "Đã nộp" || row[6] != "" || row[7] != "" || row[8] != "" {
		t.Errorf("a submitted paper with marks outstanding reads status %q, score %q, max %q, percent %q", row[5], row[6], row[7], row[8])
	}
}

func TestAnUnreachedAssignmentIs404AndTooManyRowsIs422(t *testing.T) {
	missing := exportedIn(t, "", exportTransport(domain.ResultsExport{}, domain.ErrNotFound))
	if _, ok := missing.(openapi.ExportResultsCsv404JSONResponse); !ok {
		t.Errorf("an assignment the caller does not reach answered %T, want the 404", missing)
	}

	large := exportedIn(t, "", exportTransport(domain.ResultsExport{}, domain.ErrExportTooLarge))
	refused, ok := large.(openapi.ExportResultsCsv422JSONResponse)
	if !ok {
		t.Fatalf("an export over the cap answered %T, want the 422", large)
	}
	raw, err := json.Marshal(refused)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Error struct {
			Code    string         `json:"code"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.Error.Code != "VALIDATION_FAILED" || wire.Error.Details["ids"] == nil {
		t.Errorf("the refusal reads %s, want VALIDATION_FAILED naming ids", raw)
	}

	request := openapi.ExportResultsCsvRequestObject{Params: openapi.ExportResultsCsvParams{Ids: []openapi.Uuid{uuid.New()}}}
	if _, err := exportTransport(domain.ResultsExport{}, errors.New("database is down")).ExportResultsCsv(context.Background(), request); err == nil {
		t.Error("a failure that is neither answered 404 nor 422 was swallowed")
	}
}

func TestTheItemAnalysisKeepsAnUnmarkedQuestionsRateAsNull(t *testing.T) {
	rate := 0.25
	app := &application.Application{Queries: application.Queries{
		ItemAnalysis: cqrs.HandlerFunc[query.ItemAnalysis, domain.ItemAnalysis](func(context.Context, query.ItemAnalysis) (domain.ItemAnalysis, error) {
			return domain.ItemAnalysis{HandedIn: 4, Items: []domain.AnalysisItem{
				{QuestionID: uuid.NewString(), Number: 2, Type: "single_choice", PromptExcerpt: "Hà Nội?", Answered: 3, CorrectRate: &rate},
				{QuestionID: uuid.NewString(), Number: 1, Type: "short_answer", PromptExcerpt: "Viết 2 câu", Answered: 4},
			}}, nil
		}),
	}}
	response, err := attemptshttp.NewAttempts(app, nil, nil, nil).GetItemAnalysis(context.Background(), openapi.GetItemAnalysisRequestObject{Id: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		HandedIn int              `json:"handedIn"`
		Items    []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.HandedIn != 4 || len(wire.Items) != 2 {
		t.Fatalf("the analysis reads %s", raw)
	}
	if wire.Items[0]["correctRate"] != 0.25 || wire.Items[0]["number"] != float64(2) || wire.Items[0]["type"] != "single_choice" {
		t.Errorf("the first item reads %v", wire.Items[0])
	}
	if rate, present := wire.Items[1]["correctRate"]; !present || rate != nil {
		t.Errorf("the second item reads correctRate %v (present %v), want an explicit null", rate, present)
	}
}

func equalRecords(a, b [][]string) bool {
	return slices.EqualFunc(a, b, func(x, y []string) bool { return slices.Equal(x, y) })
}

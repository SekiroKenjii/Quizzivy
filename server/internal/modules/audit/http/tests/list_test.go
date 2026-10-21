package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/audit/domain"
	audithttp "quizzivy/internal/modules/audit/http"
)

func listing(transport audithttp.Audit, params openapi.ListAuditLogParams) serving {
	return func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.ListAuditLog(ctx, openapi.ListAuditLogRequestObject{Params: params})
		if err != nil {
			return err
		}
		return response.VisitListAuditLogResponse(w)
	}
}

func dateOf(year int, month time.Month, day int) *openapi_types.Date {
	return &openapi_types.Date{Time: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

type page struct {
	Items []struct {
		ID         int64     `json:"id"`
		OccurredAt time.Time `json:"occurredAt"`
		Actor      *struct {
			ID        string  `json:"id"`
			Name      string  `json:"name"`
			AvatarURL *string `json:"avatarUrl"`
		} `json:"actor"`
		Action   string  `json:"action"`
		Category *string `json:"category"`
		Entity   struct {
			Type  string  `json:"type"`
			ID    *string `json:"id"`
			Label *string `json:"label"`
		} `json:"entity"`
		IP      *string `json:"ip"`
		Device  *string `json:"device"`
		Changes []struct {
			Field string `json:"field"`
			From  any    `json:"from"`
			To    any    `json:"to"`
		} `json:"changes"`
		Details map[string]any `json:"details"`
	} `json:"items"`
	Facets   map[string]int `json:"facets"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Total    int            `json:"total"`
}

func TestTheListAnswersEntriesFacetsAndPage(t *testing.T) {
	classID := "0195c4b0-0000-7000-8000-0000000000c1"
	l := &log{labels: map[domain.EntityRef]string{{Type: "class", ID: classID}: "Lớp 9A"}}
	byTeacher := record(3, march(12, 9, 0), "class.created", "class")
	byTeacher.Actor = &domain.Actor{ID: teacherID, Name: "Cô Lan", AvatarKey: "avatars/lan.png"}
	byTeacher.EntityID = &classID
	byTeacher.IP = ptr("203.0.113.7")
	byTeacher.UserAgent = ptr("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15")
	byTeacher.Diff = []byte(`{"name": {"old": null, "new": "Lớp 9A"}, "source": "form"}`)
	bySystem := record(2, march(11, 9, 0), "maintenance.scheduled", "maintenance_window")
	l.records = []domain.Record{record(1, march(10, 9, 0), "test.created", "test"), bySystem, byTeacher}

	rec, _ := answered(t, "vi", listing(l.transport(), openapi.ListAuditLogParams{}))

	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("%d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	var body page
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("%v: %s", err, rec.Body)
	}
	if body.Page != 1 || body.PageSize != 10 || body.Total != 3 || len(body.Items) != 3 {
		t.Errorf("page %d size %d total %d with %d items", body.Page, body.PageSize, body.Total, len(body.Items))
	}
	if body.Facets["all"] != 3 || body.Facets["classes"] != 1 || body.Facets["tests"] != 1 || len(body.Facets) != 7 {
		t.Errorf("facets %v", body.Facets)
	}

	teacher := body.Items[0]
	if teacher.ID != 3 || teacher.Actor == nil || teacher.Actor.Name != "Cô Lan" || teacher.Actor.AvatarURL == nil || *teacher.Actor.AvatarURL != "https://photos.example/avatars/lan.png" {
		t.Errorf("entry %+v", teacher)
	}
	if teacher.Category == nil || *teacher.Category != "classes" || teacher.Entity.Label == nil || *teacher.Entity.Label != "Lớp 9A" || *teacher.Entity.ID != classID {
		t.Errorf("category %v entity %+v", teacher.Category, teacher.Entity)
	}
	if teacher.Device == nil || *teacher.Device != "Mac · Safari" || teacher.IP == nil || *teacher.IP != "203.0.113.7" {
		t.Errorf("device %v ip %v", teacher.Device, teacher.IP)
	}
	if len(teacher.Changes) != 1 || teacher.Changes[0].Field != "name" || teacher.Changes[0].From != nil || teacher.Changes[0].To != "Lớp 9A" || teacher.Details["source"] != "form" {
		t.Errorf("changes %+v details %v", teacher.Changes, teacher.Details)
	}

	system := body.Items[1]
	if system.Actor != nil || system.Category != nil || system.Entity.ID != nil || system.Entity.Label != nil || system.IP != nil || system.Device != nil || system.Details != nil {
		t.Errorf("the system's entry shows %+v: its actor, category, entity id, label, address, device and details are all null", system)
	}
	if system.Changes == nil || len(system.Changes) != 0 {
		t.Error("changes must be an empty list")
	}
}

func TestTheListTheNullsAreWrittenOutNotLeftOut(t *testing.T) {
	l := &log{records: []domain.Record{record(1, march(10, 9, 0), "maintenance.scheduled", "maintenance_window")}}
	rec, _ := answered(t, "en", listing(l.transport(), openapi.ListAuditLogParams{}))

	var raw struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"actor", "category", "ip", "device", "details"} {
		if string(raw.Items[0][key]) != "null" {
			t.Errorf("%s is %s, want null", key, raw.Items[0][key])
		}
	}
	if string(raw.Items[0]["changes"]) != "[]" {
		t.Errorf("changes is %s, want []", raw.Items[0]["changes"])
	}
}

func TestAJoinCodeIsHintedAndASecretIsLeftOutOfTheAnswer(t *testing.T) {
	stored := record(1, march(10, 9, 0), "class.join_code_issued", "class_join_code")
	stored.Diff = []byte(`{"code": "ABCD-EFGH-WXYZ", "joinCode": {"old": "AAAAAAAA", "new": "BBBBBBBB"}, "password": "hunter2", "temporaryPassword": "Tmp-1", "token": "t", "secret": "s", "uses": 3}`)
	l := &log{records: []domain.Record{stored}}

	rec, _ := answered(t, "en", listing(l.transport(), openapi.ListAuditLogParams{}))

	for _, leaked := range []string{"ABCD-EFGH", "AAAAAAAA", "BBBBBBBB", "hunter2", "Tmp-1", `"password"`, `"temporaryPassword"`, `"token"`, `"secret"`} {
		if strings.Contains(rec.Body.String(), leaked) {
			t.Errorf("%s is in the answer: %s", leaked, rec.Body)
		}
	}
	var body page
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	item := body.Items[0]
	if item.Details["code"] != "••••-WXYZ" || item.Details["uses"] != float64(3) {
		t.Errorf("details %v", item.Details)
	}
	if len(item.Changes) != 1 || item.Changes[0].From != "••••-AAAA" || item.Changes[0].To != "••••-BBBB" {
		t.Errorf("changes %+v", item.Changes)
	}
}

func TestTheListFiltersByRangeCategoryAndPage(t *testing.T) {
	l := &log{}
	for i := range 25 {
		l.records = append(l.records, record(int64(i+1), march(1+i, 9, 0), "class.created", "class"))
	}
	l.records = append(l.records, record(100, march(14, 10, 0), "test.created", "test"))
	params := openapi.ListAuditLogParams{
		Category: ptr(openapi.AuditCategoryClasses),
		Range:    ptr(openapi.ListAuditLogParamsRangeCustom), From: dateOf(2001, time.March, 5), To: dateOf(2001, time.March, 14),
		Page: ptr(2), Limit: ptr(3),
	}

	rec, _ := answered(t, "en", listing(l.transport(), params))

	var body page
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 10 || body.Page != 2 || body.PageSize != 3 || len(body.Items) != 3 {
		t.Errorf("total %d page %d size %d items %d", body.Total, body.Page, body.PageSize, len(body.Items))
	}
	if body.Facets["all"] != 11 || body.Facets["classes"] != 10 || body.Facets["tests"] != 1 {
		t.Errorf("facets %v: the selected category must not change them", body.Facets)
	}
	if body.Items[0].ID != 11 {
		t.Errorf("first item %d, want the 11th entry: page 2 of a newest-first list of days 14 to 5, three to a page", body.Items[0].ID)
	}
}

func TestARefusedRangeNamesTheFieldInTheViewersLanguage(t *testing.T) {
	l := &log{}
	cases := []struct {
		name   string
		params openapi.ListAuditLogParams
		field  string
		vi     string
		en     string
	}{
		{"from without custom", openapi.ListAuditLogParams{From: dateOf(2001, time.March, 1)}, "from",
			"Chỉ dùng ngày này khi chọn khoảng thời gian tuỳ chọn.", "Dates are used only with the custom range."},
		{"custom without to", openapi.ListAuditLogParams{Range: ptr(openapi.ListAuditLogParamsRangeCustom), From: dateOf(2001, time.March, 1)}, "to",
			"Khoảng tuỳ chọn cần cả ngày bắt đầu và ngày kết thúc.", "A custom range needs both a start date and an end date."},
		{"to before from", openapi.ListAuditLogParams{Range: ptr(openapi.ListAuditLogParamsRangeCustom), From: dateOf(2001, time.March, 9), To: dateOf(2001, time.March, 1)}, "to",
			"Ngày kết thúc không được trước ngày bắt đầu.", "The end date cannot be before the start date."},
	}
	for _, c := range cases {
		for language, want := range map[string]string{"vi": c.vi, "en": c.en} {
			rec, _ := answered(t, language, listing(l.transport(), c.params))
			var body struct {
				Error struct {
					Code    string            `json:"code"`
					Details map[string]string `json:"details"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if rec.Code != http.StatusBadRequest || body.Error.Code != "VALIDATION_FAILED" || body.Error.Details[c.field] != want {
				t.Errorf("%s in %s: %d %+v, want 400 VALIDATION_FAILED with %s: %q", c.name, language, rec.Code, body.Error, c.field, want)
			}
		}
	}
}

func TestWithoutAnApplicationTheListIsNotImplemented(t *testing.T) {
	transport := audithttp.Audit{}
	_, err := transport.ListAuditLog(context.Background(), openapi.ListAuditLogRequestObject{})
	if err == nil {
		t.Error("a transport with no application answered")
	}
	if _, err := transport.ExportAuditLog(context.Background(), openapi.ExportAuditLogRequestObject{}); err == nil {
		t.Error("a transport with no application exported")
	}
}

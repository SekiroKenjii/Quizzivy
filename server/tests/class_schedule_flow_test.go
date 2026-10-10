//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	identitydomain "quizzivy/internal/modules/identity/domain"
)

type logSink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *logSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *logSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

type classRig struct {
	t      *testing.T
	w      *world
	logs   *logSink
	a, b   *client
	admin  *client
	reader *client
	aID    string
	rID    string
}

func (r *classRig) me(c *client) string {
	r.t.Helper()
	return c.must(http.StatusOK, http.MethodGet, "/auth/me", nil)["id"].(string)
}

func (r *classRig) readerOfClasses() *client {
	r.t.Helper()
	ctx := context.Background()
	var role string
	if err := r.w.pool.QueryRow(ctx, `INSERT INTO app.roles (name, icon, color) VALUES ($1, 'user', 'gray') RETURNING id::text`, "Chỉ xem "+nonce(r.t)).Scan(&role); err != nil {
		r.t.Fatal(err)
	}
	if _, err := r.w.pool.Exec(ctx, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1::uuid, 'people.students.read')`, role); err != nil {
		r.t.Fatal(err)
	}
	password := "doc-lop-" + nonce(r.t)
	hash, err := identitydomain.Passwords.Hash(ctx, password)
	if err != nil {
		r.t.Fatal(err)
	}
	email := "doc-lop-" + nonce(r.t) + "@example.com"
	if _, err := r.w.pool.Exec(ctx, `INSERT INTO app.users (email, full_name, role_id, password_hash) VALUES ($1, 'Người xem lớp', $2::uuid, $3)`, email, role, hash); err != nil {
		r.t.Fatal(err)
	}
	return r.w.signedIn(email, password)
}

func newClassRig(t *testing.T, logged bool) *classRig {
	t.Helper()
	r := &classRig{t: t, logs: &logSink{}}
	if logged {
		r.w = bootLogging(t, slog.New(slog.NewTextHandler(r.logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	} else {
		r.w = bootWithStorage(t)
	}
	staff := func(builtin string) *client {
		email, password := r.w.createStaff(builtin)
		return r.w.signedIn(email, password)
	}
	r.a, r.b, r.admin = staff("teacher"), staff("teacher"), staff("admin")
	r.aID = r.me(r.a)
	return r
}

func items(body map[string]any) []map[string]any {
	raw, _ := body["items"].([]any)
	out := make([]map[string]any, len(raw))
	for i, v := range raw {
		out[i] = v.(map[string]any)
	}
	return out
}

func byID(rows []map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, row := range rows {
		out[row["id"].(string)] = row
	}
	return out
}

func codeOf(row map[string]any) any {
	joinCode, _ := row["joinCode"].(map[string]any)
	if joinCode == nil {
		return nil
	}
	return joinCode["code"]
}

func (r *classRig) listClasses(c *client, query string) (int, http.Header, map[string]any) {
	r.t.Helper()
	return c.callWith(nil, http.MethodGet, "/teacher/classes?"+query, nil)
}

func (r *classRig) newClass(c *client, extra map[string]any) map[string]any {
	r.t.Helper()
	body := map[string]any{"name": "Lớp " + nonce(r.t)}
	for k, v := range extra {
		body[k] = v
	}
	return c.must(http.StatusCreated, http.MethodPost, "/teacher/classes", body)
}

func (r *classRig) issue(c *client, classID string, days int) map[string]any {
	r.t.Helper()
	return c.must(http.StatusCreated, http.MethodPost, "/teacher/classes/"+classID+"/join-code", map[string]any{"expiresInDays": days})
}

func TestTheClassCardsShowEveryCodeFromOneCallAndTheListNeverShowsMoreThanReadingByIdWould(t *testing.T) {
	r := newClassRig(t, true)
	r.reader = r.readerOfClasses()
	r.rID = r.me(r.reader)

	sealed := id(r.newClass(r.a, map[string]any{"scheduleLabel": "Thứ 3, 5 · 18:00", "room": "A1"}))
	quiet := id(r.newClass(r.a, nil))
	transferred := id(r.newClass(r.a, nil))
	codes := map[string]string{sealed: r.issue(r.a, sealed, 30)["code"].(string), transferred: r.issue(r.a, transferred, 30)["code"].(string)}
	if _, err := r.w.pool.Exec(context.Background(), `UPDATE app.classes SET teacher_id = $2::uuid WHERE id = $1::uuid`, transferred, r.rID); err != nil {
		t.Fatal(err)
	}

	status, header, body := r.listClasses(r.a, "withCodes=true&status=all&limit=100")
	if status != http.StatusOK || !strings.Contains(header.Get("Cache-Control"), "no-store") {
		t.Fatalf("the class cards' call answered %d with Cache-Control %q", status, header.Get("Cache-Control"))
	}
	cards := byID(items(body))
	if cards[sealed]["scheduleLabel"] != "Thứ 3, 5 · 18:00" || cards[sealed]["room"] != "A1" || codeOf(cards[sealed]) != codes[sealed] {
		t.Errorf("the sealed class's card: schedule %v room %v code %v, want its schedule, room and code %s", cards[sealed]["scheduleLabel"], cards[sealed]["room"], codeOf(cards[sealed]), codes[sealed])
	}
	if cards[quiet]["joinCode"] != nil || cards[quiet]["averageScore"] != nil {
		t.Errorf("a class with no code or grade shows joinCode %v and average %v, want both null", cards[quiet]["joinCode"], cards[quiet]["averageScore"])
	}
	if _, ok := cards[quiet]["averageScore"]; !ok {
		t.Error("a teacher's class row lacks averageScore altogether, want it null")
	}

	status, header, body = r.listClasses(r.a, "status=all&limit=100")
	if status != http.StatusOK || !strings.Contains(header.Get("Cache-Control"), "no-store") {
		t.Fatalf("a picker's call answered %d with Cache-Control %q: the answer is never cached, codes or not", status, header.Get("Cache-Control"))
	}
	for classID, card := range byID(items(body)) {
		if codeOf(card) != nil {
			t.Errorf("a call that did not ask for codes opened the code of class %s", classID)
		}
	}

	callers := map[string]*client{"the teacher": r.a, "an Admin": r.admin, "a reader who teaches one class": r.reader, "another teacher": r.b}
	for who, c := range callers {
		_, _, listed := r.listClasses(c, "withCodes=true&status=all&limit=100")
		rows := byID(items(listed))
		for _, classID := range []string{sealed, quiet, transferred} {
			status, _, read := c.callWith(nil, http.MethodGet, "/teacher/classes/"+classID+"/join-code", nil)
			row, present := rows[classID]
			switch status {
			case http.StatusOK:
				if !present || codeOf(row) != read["code"] {
					t.Errorf("%s: reading class %s answers the code %v, and the list shows %v", who, classID, read["code"], codeOf(row))
				}
			case http.StatusForbidden:
				if present && codeOf(row) != nil {
					t.Errorf("%s: reading class %s is refused, and the list shows its code %v", who, classID, codeOf(row))
				}
			case http.StatusNotFound:
				if present && row["joinCode"] != nil {
					t.Errorf("%s: class %s has nothing to read for this caller, and the list shows %v", who, classID, row["joinCode"])
				}
			default:
				t.Errorf("%s: reading class %s answered %d", who, classID, status)
			}
		}
		if who == "another teacher" {
			for _, code := range codes {
				if raw, _ := json.Marshal(listed); strings.Contains(string(raw), code) {
					t.Errorf("another teacher's list names a code that is not theirs: %s", raw)
				}
			}
			if len(rows) != 0 {
				t.Errorf("another teacher lists %d classes, want none of the first teacher's", len(rows))
			}
		}
	}
	if _, _, listed := r.listClasses(r.reader, "withCodes=true&limit=100"); len(items(listed)) != 1 || codeOf(items(listed)[0]) != nil {
		t.Errorf("the reader lists %v, want the one class they teach, its code null", listed)
	}

	logs := r.logs.String()
	for _, code := range codes {
		if strings.Contains(logs, code) || strings.Contains(logs, strings.ReplaceAll(code, "-", "")) {
			t.Errorf("the log holds a join code")
		}
	}
	if !strings.Contains(logs, "/teacher/classes") {
		t.Error("the log never mentions the list, so a search of it proves nothing")
	}
}

func TestTheClassListIsLimitedPerUserAndRefusedCallersSpendNothing(t *testing.T) {
	r := newClassRig(t, false)
	student := r.a.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "han-muc-" + nonce(t) + "@example.com", "fullName": "Học viên", "classIds": []string{},
	})
	learner := r.w.signedIn(student["user"].(map[string]any)["email"].(string), student["temporaryPassword"].(string))

	for range 3 {
		if status, _, _ := r.listClasses(r.w.browser(), "limit=1"); status != http.StatusUnauthorized {
			t.Fatalf("an anonymous caller answered %d, want 401", status)
		}
		if status, _, _ := r.listClasses(learner, "limit=1"); status != http.StatusForbidden {
			t.Fatalf("a student answered %d, want 403", status)
		}
	}
	for i := range 60 {
		if status, _, body := r.listClasses(r.a, "limit=1"); status != http.StatusOK {
			t.Fatalf("call %d of the teacher's 60 a minute answered %d %v after refused callers had spent nothing", i+1, status, body)
		}
	}
	status, header, body := r.listClasses(r.a, "limit=1")
	if status != http.StatusTooManyRequests || r.a.errorCode(body) != "RATE_LIMITED" || header.Get("Retry-After") == "" {
		t.Errorf("the 61st call in a minute answered %d %v with Retry-After %q, want 429 RATE_LIMITED", status, body, header.Get("Retry-After"))
	}
	if status, _, _ := r.listClasses(r.b, "limit=1"); status != http.StatusOK {
		t.Errorf("another teacher at the same address answered %d: the budget is per user", status)
	}
}

func TestAClassIsCreatedAndEditedWithItsScheduleAndRoom(t *testing.T) {
	r := newClassRig(t, false)
	created := r.newClass(r.a, map[string]any{"scheduleLabel": "  Thứ 2, 4 · 19:00 ", "room": "   "})
	classID := id(created)
	if created["scheduleLabel"] != "Thứ 2, 4 · 19:00" || created["room"] != nil {
		t.Fatalf("created with schedule %v and room %v, want the trimmed label and no room", created["scheduleLabel"], created["room"])
	}
	patch := func(body map[string]any) map[string]any {
		return r.a.must(http.StatusOK, http.MethodPatch, "/teacher/classes/"+classID, body)
	}
	if got := patch(map[string]any{"name": "Đổi tên"}); got["scheduleLabel"] != "Thứ 2, 4 · 19:00" || got["name"] != "Đổi tên" {
		t.Errorf("renaming changed the schedule: %v", got["scheduleLabel"])
	}
	if got := patch(map[string]any{"room": " B2.01 "}); got["room"] != "B2.01" || got["scheduleLabel"] == nil {
		t.Errorf("setting the room gave room %v and schedule %v", got["room"], got["scheduleLabel"])
	}
	if got := patch(map[string]any{"scheduleLabel": nil}); got["scheduleLabel"] != nil || got["room"] != "B2.01" {
		t.Errorf("null gave schedule %v and room %v, want the schedule cleared and the room kept", got["scheduleLabel"], got["room"])
	}
	if got := patch(map[string]any{"room": "  "}); got["room"] != nil {
		t.Errorf("a blank room left %v, want it cleared", got["room"])
	}
	if got := patch(map[string]any{}); got["averageScore"] != nil || got["name"] != "Đổi tên" {
		t.Errorf("an edit that changes nothing answered %v, want the class as it was with no average", got)
	}

	long := func(n int) string { return strings.Repeat("a", n) }
	for name, c := range map[string]struct {
		method string
		path   string
		body   map[string]any
	}{
		"a 121-character label on create": {http.MethodPost, "/teacher/classes", map[string]any{"name": "Lớp", "scheduleLabel": long(121)}},
		"a 61-character room on create":   {http.MethodPost, "/teacher/classes", map[string]any{"name": "Lớp", "room": long(61)}},
		"a 121-character label on edit":   {http.MethodPatch, "/teacher/classes/" + classID, map[string]any{"scheduleLabel": long(121)}},
		"a 61-character room on edit":     {http.MethodPatch, "/teacher/classes/" + classID, map[string]any{"room": long(61)}},
		"a number for the label":          {http.MethodPatch, "/teacher/classes/" + classID, map[string]any{"scheduleLabel": 7}},
		"a number for the room":           {http.MethodPost, "/teacher/classes", map[string]any{"name": "Lớp", "room": 7}},
	} {
		if status, _ := r.a.call(c.method, c.path, c.body); status != http.StatusBadRequest {
			t.Errorf("%s answered %d, want 400", name, status)
		}
	}
	if status, _ := r.a.call(http.MethodPost, "/teacher/classes", map[string]any{"name": "Lớp", "scheduleLabel": long(120), "room": long(60)}); status != http.StatusCreated {
		t.Errorf("a 120-character label and a 60-character room answered %d, want 201", status)
	}
}

func (r *classRig) setZone(userID, zone string) {
	r.t.Helper()
	if _, err := r.w.pool.Exec(context.Background(), `UPDATE app.users SET time_zone = $2 WHERE id = $1::uuid`, userID, zone); err != nil {
		r.t.Fatal(err)
	}
}

func (r *classRig) expiresAt(c *client, classID string, days int) time.Time {
	r.t.Helper()
	rotated := r.issue(c, classID, days)
	expires, err := time.Parse(time.RFC3339, rotated["expiresAt"].(string))
	if err != nil {
		r.t.Fatal(err)
	}
	read := c.must(http.StatusOK, http.MethodGet, "/teacher/classes/"+classID+"/join-code", nil)
	if again, _ := time.Parse(time.RFC3339, read["expiresAt"].(string)); !again.Equal(expires) {
		r.t.Errorf("rotating answered %s and reading the code %s", expires, again)
	}
	return expires
}

func (r *classRig) wantEndOfDay(label string, expires time.Time, zone string, days int, before, after time.Time) {
	r.t.Helper()
	loc, err := time.LoadLocation(zone)
	if err != nil {
		r.t.Fatal(err)
	}
	local := expires.In(loc)
	if local.Hour() != 23 || local.Minute() != 59 || local.Second() != 59 {
		r.t.Errorf("%s: expires at %s, want 23:59:59 in %s", label, local, zone)
	}
	day := func(at time.Time) string {
		y, m, d := at.In(loc).Date()
		return time.Date(y, m, d+days, 0, 0, 0, 0, loc).Format(time.DateOnly)
	}
	if got := local.Format(time.DateOnly); got != day(before) && got != day(after) {
		r.t.Errorf("%s: expires on %s, want %d days after today in %s, which is %s", label, got, days, zone, day(before))
	}
}

func TestACodeExpiresAtTheEndOfTheDayInTheIssuersProfileZone(t *testing.T) {
	r := newClassRig(t, false)
	adminID := r.me(r.admin)
	r.setZone(r.aID, "Asia/Tokyo")
	r.setZone(adminID, "America/New_York")
	teachersClass, otherClass := id(r.newClass(r.a, nil)), id(r.newClass(r.b, nil))
	bID := r.me(r.b)

	before := time.Now()
	tokyo := r.expiresAt(r.a, teachersClass, 7)
	r.wantEndOfDay("a teacher whose profile zone is Tokyo", tokyo, "Asia/Tokyo", 7, before, time.Now())

	before = time.Now()
	saigon := r.expiresAt(r.b, otherClass, 7)
	r.wantEndOfDay("a teacher with no profile zone", saigon, "Asia/Ho_Chi_Minh", 7, before, time.Now())

	r.setZone(bID, "Mars/Olympus_Mons")
	before = time.Now()
	unknown := r.expiresAt(r.b, otherClass, 30)
	r.wantEndOfDay("a teacher whose stored zone is not one", unknown, "Asia/Ho_Chi_Minh", 30, before, time.Now())

	before = time.Now()
	admin := r.expiresAt(r.admin, teachersClass, 90)
	r.wantEndOfDay("an Admin rotating a teacher's class, in the Admin's own zone", admin, "America/New_York", 90, before, time.Now())
}

func TestAStudentSeesWhenAndWhereAClassMeetsAndNothingElse(t *testing.T) {
	r := newClassRig(t, false)
	if got := r.a.send(http.MethodPut, "/me/avatar", new(filePayload(t, "chan-dung.png", avatarPNG(t)))); got.status != http.StatusOK {
		t.Fatalf("the teacher's photo: %d %s", got.status, got.body)
	}
	meeting := r.newClass(r.a, map[string]any{"scheduleLabel": "Thứ 7 · 08:00", "room": "C3"})
	plain := r.newClass(r.a, nil)
	elsewhere := r.newClass(r.b, nil)
	code := r.issue(r.a, id(meeting), 30)["code"].(string)

	made := r.a.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "lop-cua-em-" + nonce(t) + "@example.com", "fullName": "Học viên", "classIds": []string{id(plain), id(meeting)},
	})
	learner := r.w.signedIn(made["user"].(map[string]any)["email"].(string), made["temporaryPassword"].(string))
	status, body := learner.call(http.MethodGet, "/app/classes", nil)
	if status != http.StatusOK {
		t.Fatalf("the student's classes: %d %v", status, body)
	}
	want := []string{"description", "id", "joinedAt", "name", "room", "scheduleLabel", "teacherAvatarUrl", "teacherName"}
	shown := byID(items(body))
	if len(shown) != 2 {
		t.Fatalf("the student sees %d classes, want 2", len(shown))
	}
	for classID, row := range shown {
		keys := make([]string, 0, len(row))
		for k := range row {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		if !slices.Equal(keys, want) {
			t.Errorf("class %s carries %v, want exactly %v", classID, keys, want)
		}
		photo, _ := row["teacherAvatarUrl"].(string)
		if photo == "" || !strings.Contains(photo, "X-Amz-Signature") {
			t.Errorf("class %s: the teacher's photo is %v, want a signed URL", classID, row["teacherAvatarUrl"])
		}
	}
	if row := shown[id(meeting)]; row["scheduleLabel"] != "Thứ 7 · 08:00" || row["room"] != "C3" {
		t.Errorf("the meeting class shows schedule %v and room %v", row["scheduleLabel"], row["room"])
	}
	if row := shown[id(plain)]; row["scheduleLabel"] != nil || row["room"] != nil {
		t.Errorf("a class with neither shows schedule %v and room %v, want null", row["scheduleLabel"], row["room"])
	}
	if _, listed := shown[id(elsewhere)]; listed {
		t.Error("the student sees a class they are not in")
	}

	newcomer := r.a.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "moi-vao-" + nonce(t) + "@example.com", "fullName": "Học viên mới", "classIds": []string{},
	})
	joiner := r.w.signedIn(newcomer["user"].(map[string]any)["email"].(string), newcomer["temporaryPassword"].(string))
	status, joined := joiner.call(http.MethodPost, "/app/classes/join", map[string]any{"joinCode": code})
	if status != http.StatusOK {
		t.Fatalf("joining by code: %d %v", status, joined)
	}
	if joined["room"] != "C3" || joined["scheduleLabel"] != "Thứ 7 · 08:00" {
		t.Errorf("the class joined shows schedule %v and room %v", joined["scheduleLabel"], joined["room"])
	}
	for _, banned := range []string{"averageScore", "joinCode"} {
		if _, has := joined[banned]; has {
			t.Errorf("the class a student joins carries %s", banned)
		}
	}

	otherStudent := r.b.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "lop-khac-" + nonce(t) + "@example.com", "fullName": "Học viên lớp khác", "classIds": []string{id(elsewhere)},
	})
	stranger := r.w.signedIn(otherStudent["user"].(map[string]any)["email"].(string), otherStudent["temporaryPassword"].(string))
	_, theirs := stranger.call(http.MethodGet, "/app/classes", nil)
	for _, row := range items(theirs) {
		if row["teacherAvatarUrl"] != nil {
			t.Errorf("a teacher without a photo shows %v", row["teacherAvatarUrl"])
		}
	}
	raw, _ := json.Marshal(theirs)
	if strings.Contains(string(raw), id(meeting)) || strings.Contains(string(raw), code) {
		t.Errorf("another teacher's student sees the first teacher's class: %s", raw)
	}
}

//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

const bulkResetPath = "/teacher/students/reset-passwords"

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type resetRig struct {
	t     *testing.T
	w     *world
	logs  *lockedBuffer
	a, b  *client
	admin *client
	aID   string
	bID   string
	admID string
	rootB string
	rootA string
	issue []string
}

type enrolled struct {
	id       string
	email    string
	password string
}

func newResetRig(t *testing.T) *resetRig {
	t.Helper()
	logs := &lockedBuffer{}
	w := bootLogging(t, slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	r := &resetRig{t: t, w: w, logs: logs}
	r.a, r.aID = r.staff("teacher")
	r.b, r.bID = r.staff("teacher")
	r.admin, r.admID = r.staff("admin")
	r.rootA = r.class(r.a)
	r.rootB = r.class(r.b)
	return r
}

func (r *resetRig) staff(builtin string) (*client, string) {
	r.t.Helper()
	email, password := r.w.createStaff(builtin)
	c := r.w.signedIn(email, password)
	return c, c.must(http.StatusOK, http.MethodGet, "/auth/me", nil)["id"].(string)
}

func (r *resetRig) class(c *client) string {
	r.t.Helper()
	return id(c.must(http.StatusCreated, http.MethodPost, "/teacher/classes", map[string]any{"name": "Lớp " + nonce(r.t)}))
}

func (r *resetRig) student(c *client, classes ...string) enrolled {
	r.t.Helper()
	created := c.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": "dat-lai-" + nonce(r.t) + "@example.com", "fullName": "Học viên đặt lại", "classIds": classes,
	})
	user := created["user"].(map[string]any)
	return enrolled{id: user["id"].(string), email: user["email"].(string), password: created["temporaryPassword"].(string)}
}

func (r *resetRig) reset(c *client, ids ...string) (int, http.Header, map[string]any) {
	r.t.Helper()
	return c.callWith(nil, http.MethodPost, bulkResetPath, map[string]any{"studentIds": ids})
}

func (r *resetRig) mustReset(c *client, ids ...string) map[string]any {
	r.t.Helper()
	status, _, body := r.reset(c, ids...)
	if status != http.StatusOK {
		r.t.Fatalf("resetting %v: %d %v", ids, status, body)
	}
	for _, item := range objects(body["items"]) {
		r.issue = append(r.issue, item["temporaryPassword"].(string))
	}
	return body
}

func objects(v any) []map[string]any {
	raw, _ := v.([]any)
	out := make([]map[string]any, len(raw))
	for i, item := range raw {
		out[i] = item.(map[string]any)
	}
	return out
}

func idsOf(items []map[string]any, field string) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item[field].(string)
	}
	return out
}

func codesOf(items []map[string]any) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item["studentId"].(string) + ":" + item["code"].(string)
	}
	return out
}

func (r *resetRig) account(userID string) string {
	r.t.Helper()
	var state string
	err := r.w.pool.QueryRow(context.Background(), `
		SELECT concat_ws('|', coalesce(u.password_hash, ''), u.must_change_password::text, u.session_epoch::text, coalesce(u.disabled_at::text, ''),
		       (SELECT count(*) FROM app.refresh_tokens t WHERE t.user_id = u.id AND t.revoked_at IS NULL)::text,
		       (SELECT count(*) FROM app.audit_log a WHERE a.entity_id = u.id AND a.action = 'student.password_reset')::text)
		  FROM app.users u WHERE u.id = $1::uuid`, userID).Scan(&state)
	if err != nil {
		r.t.Fatal(err)
	}
	return state
}

func (r *resetRig) nothingKeptThePasswords() {
	r.t.Helper()
	logs := r.logs.String()
	if !strings.Contains(logs, "reset-passwords") {
		r.t.Fatalf("the log never mentions the bulk reset, so a search of it proves nothing:\n%.2000s", logs)
	}
	var audits string
	if err := r.w.pool.QueryRow(context.Background(), `SELECT coalesce(string_agg(to_jsonb(a)::text, ' '), '') FROM app.audit_log a`).Scan(&audits); err != nil {
		r.t.Fatal(err)
	}
	var plain int
	if err := r.w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.users WHERE password_hash = ANY($1::text[])`, r.issue).Scan(&plain); err != nil {
		r.t.Fatal(err)
	}
	if plain != 0 {
		r.t.Errorf("%d accounts store a temporary password as their hash", plain)
	}
	for _, password := range r.issue {
		if strings.Contains(logs, password) {
			r.t.Errorf("the log, at debug level, holds the temporary password %q", password)
		}
		if strings.Contains(audits, password) {
			r.t.Errorf("an audit row holds the temporary password %q", password)
		}
	}
	if len(r.issue) == 0 {
		r.t.Fatal("no temporary password was issued, so the search proves nothing")
	}
}

func TestABulkResetEndsEachStudentsSessionsAndAnswersOnceWithoutCaching(t *testing.T) {
	r := newResetRig(t)
	first, second, third := r.student(r.a, r.rootA), r.student(r.a, r.rootA), r.student(r.a, r.rootA)
	device := r.w.signedIn(first.email, first.password)
	other := r.w.signedIn(second.email, second.password)
	for _, c := range []*client{device, other} {
		c.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	}

	status, header, body := r.reset(r.a, first.id, second.id, third.id)

	if status != http.StatusOK {
		t.Fatalf("the bulk reset answered %d %v", status, body)
	}
	if cache := header.Get("Cache-Control"); !strings.Contains(cache, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store: the answer holds three passwords", cache)
	}
	items := objects(body["items"])
	if got, want := idsOf(items, "studentId"), []string{first.id, second.id, third.id}; !slices.Equal(got, want) {
		t.Fatalf("items %v, want %v in the order asked", got, want)
	}
	if failed := objects(body["failed"]); len(failed) != 0 {
		t.Errorf("failed %v, want none", failed)
	}
	seen := map[string]bool{}
	for _, item := range items {
		password := item["temporaryPassword"].(string)
		r.issue = append(r.issue, password)
		if password == "" || seen[password] || item["fullName"] != "Học viên đặt lại" || item["email"] == "" {
			t.Errorf("item %v: want its own password, the student's name and address", item)
		}
		seen[password] = true
	}

	if got, _ := device.call(http.MethodGet, "/auth/me", nil); got != http.StatusUnauthorized {
		t.Errorf("the reset student's old access token answered %d on its next request, want 401", got)
	}
	if got, _ := other.call(http.MethodGet, "/auth/me", nil); got != http.StatusUnauthorized {
		t.Errorf("the second reset student's old access token answered %d on its next request, want 401", got)
	}
	if got, _ := device.call(http.MethodPost, "/auth/refresh", nil); got != http.StatusUnauthorized {
		t.Errorf("the reset student's old refresh session answered %d, want 401", got)
	}
	if got, _ := r.w.browser().call(http.MethodPost, "/auth/login", map[string]any{"email": first.email, "password": first.password}); got != http.StatusUnauthorized {
		t.Errorf("the old password signed in with %d, want 401", got)
	}
	reissued := items[0]["temporaryPassword"].(string)
	session := r.w.browser()
	signedIn := session.login(first.email, reissued)
	if user, _ := signedIn["user"].(map[string]any); user["mustChangePassword"] != true {
		t.Errorf("signing in with the new password shows %v, want a forced change", user)
	}
	session.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	r.nothingKeptThePasswords()
}

func TestABulkResetRejectsWhatTheContractForbids(t *testing.T) {
	r := newResetRig(t)
	student := r.student(r.a, r.rootA)
	tooMany := make([]string, 41)
	for i := range tooMany {
		tooMany[i] = uuid.NewString()
	}
	for label, c := range map[string]struct {
		body map[string]any
	}{
		"no ids":             {map[string]any{"studentIds": []string{}}},
		"forty-one ids":      {map[string]any{"studentIds": tooMany}},
		"a repeated id":      {map[string]any{"studentIds": []string{student.id, student.id}}},
		"an id that is none": {map[string]any{"studentIds": []string{"not-an-id"}}},
		"another property":   {map[string]any{"studentIds": []string{student.id}, "all": true}},
		"no property":        {map[string]any{}},
	} {
		t.Run(label, func(t *testing.T) {
			teacher, _ := r.staff("teacher")
			status, _, body := teacher.callWith(nil, http.MethodPost, bulkResetPath, c.body)
			if status != http.StatusBadRequest {
				t.Errorf("answered %d %v, want 400", status, body)
			}
		})
	}
	if state := r.account(student.id); !strings.HasSuffix(state, "|0") {
		t.Errorf("a refused request reset the student: %s", state)
	}
}

func TestTheBulkResetIsLimitedPerUserAndRefusedCallersSpendNothing(t *testing.T) {
	r := newResetRig(t)
	mine := r.student(r.a, r.rootA)
	learner := r.w.signedIn(mine.email, mine.password)

	for range 3 {
		if got, _, _ := r.reset(r.w.browser(), mine.id); got != http.StatusUnauthorized {
			t.Fatalf("an anonymous caller answered %d, want 401", got)
		}
		if got, _, _ := r.reset(learner, mine.id); got != http.StatusForbidden {
			t.Fatalf("a student answered %d, want 403", got)
		}
	}
	for i := range 2 {
		if status, _, body := r.reset(r.a, mine.id); status != http.StatusOK {
			t.Fatalf("call %d of the teacher's budget answered %d %v after refused callers had spent nothing", i+1, status, body)
		}
	}
	status, header, body := r.reset(r.a, mine.id)
	if status != http.StatusTooManyRequests || r.a.errorCode(body) != "RATE_LIMITED" || header.Get("Retry-After") == "" {
		t.Errorf("the third call in a minute answered %d %v with Retry-After %q, want 429 RATE_LIMITED", status, body, header.Get("Retry-After"))
	}
	if status, _, body := r.reset(r.b, uuid.NewString()); status != http.StatusOK {
		t.Errorf("another teacher at the same address answered %d %v: the budget is per user, not per address", status, body)
	}
}

func TestABulkResetOfAnotherTeachersAccountsChangesNothingAndAnswersAsMissingOnes(t *testing.T) {
	r := newResetRig(t)
	mineOne, mineTwo := r.student(r.a, r.rootA), r.student(r.a, r.rootA)
	theirs := r.student(r.b, r.rootB)
	disabled := r.student(r.a, r.rootA)
	if _, err := r.w.pool.Exec(context.Background(), `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, disabled.id); err != nil {
		t.Fatal(err)
	}
	theirSession := r.w.signedIn(theirs.email, theirs.password)
	absent := uuid.NewString()
	untouched := []string{theirs.id, disabled.id, r.bID, r.admID}
	before := map[string]string{}
	for _, id := range untouched {
		before[id] = r.account(id)
	}

	body := r.mustReset(r.a, mineOne.id, theirs.id, absent, r.bID, r.admID, disabled.id, mineTwo.id)

	if got, want := idsOf(objects(body["items"]), "studentId"), []string{mineOne.id, mineTwo.id}; !slices.Equal(got, want) {
		t.Fatalf("reset %v, want only the caller's two students, though a stranger's id sat between them", got)
	}
	want := []string{theirs.id + ":NOT_FOUND", absent + ":NOT_FOUND", r.bID + ":NOT_FOUND", r.admID + ":NOT_FOUND", disabled.id + ":NOT_FOUND"}
	if got := codesOf(objects(body["failed"])); !slices.Equal(got, want) {
		t.Errorf("failed %v, want %v: another teacher's student, a teacher, an Admin and a disabled account all answer as a missing id", got, want)
	}
	raw := body["failed"].([]any)
	if a, b := raw[0].(map[string]any), raw[1].(map[string]any); len(a) != len(b) || a["code"] != b["code"] {
		t.Errorf("another teacher's student answered %v where a missing one answered %v", a, b)
	}
	for _, id := range untouched {
		if after := r.account(id); after != before[id] {
			t.Errorf("the refused reset changed %s: %q became %q", id, before[id], after)
		}
	}
	theirSession.must(http.StatusOK, http.MethodGet, "/auth/me", nil)
	r.w.signedIn(theirs.email, theirs.password)
	r.nothingKeptThePasswords()
}

func TestASharedStudentIsRefusedToATeacherAndResetForAnAdmin(t *testing.T) {
	r := newResetRig(t)
	shared := r.student(r.a, r.rootA)
	if _, err := r.w.pool.Exec(context.Background(), `INSERT INTO app.class_members (class_id, user_id, joined_via, added_by) VALUES ($1::uuid, $2::uuid, 'admin', $3::uuid)`, r.rootB, shared.id, r.bID); err != nil {
		t.Fatal(err)
	}
	learner := r.w.signedIn(shared.email, shared.password)
	before := r.account(shared.id)

	for name, teacher := range map[string]*client{"A": r.a, "B": r.b} {
		body := r.mustReset(teacher, shared.id)
		if got, want := codesOf(objects(body["failed"])), []string{shared.id + ":STUDENT_SHARED"}; !slices.Equal(got, want) || len(objects(body["items"])) != 0 {
			t.Errorf("teacher %s resetting the shared student: %v and items %v, want %v", name, got, body["items"], want)
		}
	}
	if status, _, body := r.a.callWith(nil, http.MethodPost, "/teacher/students/"+shared.id+"/reset-password", nil); status == http.StatusOK {
		t.Errorf("the single reset of a shared student by a teacher answered 200 %v, where the bulk reset refuses it", body)
	}
	if after := r.account(shared.id); after != before {
		t.Errorf("the refusals changed the shared student: %q became %q", before, after)
	}
	learner.must(http.StatusOK, http.MethodGet, "/auth/me", nil)

	body := r.mustReset(r.admin, shared.id)
	if got := idsOf(objects(body["items"]), "studentId"); !slices.Equal(got, []string{shared.id}) {
		t.Fatalf("the Admin reset %v (failed %v), want the shared student", got, body["failed"])
	}
	if got, _ := learner.call(http.MethodGet, "/auth/me", nil); got != http.StatusUnauthorized {
		t.Errorf("the shared student's old token answered %d after the Admin's reset, want 401", got)
	}
	r.nothingKeptThePasswords()
}

func TestListingStudentsTakesSeveralClassesAndThePasswordFlag(t *testing.T) {
	r := newResetRig(t)
	second, third := r.class(r.a), r.class(r.a)
	inFirst, inSecond := r.student(r.a, r.rootA), r.student(r.a, second)
	inBoth, inThird := r.student(r.a, r.rootA, second), r.student(r.a, third)
	changed := r.w.signedIn(inFirst.email, inFirst.password)
	changed.must(http.StatusNoContent, http.MethodPost, "/auth/change-password", map[string]any{"newPassword": "mat-khau-moi-" + nonce(t)})
	r.mustReset(r.a, inSecond.id)

	list := func(status int, query url.Values) map[string]any {
		t.Helper()
		got, body := r.a.call(http.MethodGet, "/teacher/students?"+query.Encode(), nil)
		if got != status {
			t.Fatalf("GET %s: %d %v, want %d", query.Encode(), got, body, status)
		}
		return body
	}
	ids := func(body map[string]any) []string {
		out := idsOf(objects(body["items"]), "id")
		slices.Sort(out)
		return out
	}
	sorted := func(values ...string) []string {
		slices.Sort(values)
		return values
	}

	either := list(http.StatusOK, url.Values{"classId": {r.rootA, second}})
	if got, want := ids(either), sorted(inFirst.id, inSecond.id, inBoth.id); !slices.Equal(got, want) || either["total"] != float64(3) {
		t.Errorf("two classes list %v with total %v, want %v: a student in both counted once, the third class left out", got, either["total"], want)
	}
	if one := list(http.StatusOK, url.Values{"classId": {second}}); !slices.Equal(ids(one), sorted(inSecond.id, inBoth.id)) {
		t.Errorf("one class lists %v", ids(one))
	}
	if foreign := list(http.StatusOK, url.Values{"classId": {r.rootB}}); len(ids(foreign)) != 0 {
		t.Errorf("another teacher's class lists %v", ids(foreign))
	}
	if waiting := list(http.StatusOK, url.Values{"classId": {r.rootA, second, third}, "mustChangePassword": {"true"}}); !slices.Equal(ids(waiting), sorted(inSecond.id, inBoth.id, inThird.id)) {
		t.Errorf("students who must change their password: %v, want %v", ids(waiting), sorted(inSecond.id, inBoth.id, inThird.id))
	}
	if done := list(http.StatusOK, url.Values{"mustChangePassword": {"false"}}); !slices.Equal(ids(done), []string{inFirst.id}) {
		t.Errorf("students who have changed theirs: %v, want %v", ids(done), []string{inFirst.id})
	}
	list(http.StatusBadRequest, url.Values{"classId": {"not-an-id"}})
	list(http.StatusBadRequest, url.Values{"mustChangePassword": {"maybe"}})
	many := make([]string, 51)
	for i := range many {
		many[i] = uuid.NewString()
	}
	list(http.StatusBadRequest, url.Values{"classId": many})
}

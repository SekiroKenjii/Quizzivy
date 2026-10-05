//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func (x *iso) view(op scopedOp, c isoCase, viewer *party, bare bool) string {
	x.t.Helper()
	method, path, body := x.build(op, c, viewer, "", "", bare)
	got := viewer.c.send(method, path, body)
	if got.status != http.StatusOK {
		x.t.Fatalf("%s as its owner: %d %s", op.id, got.status, got.body)
	}
	return signature.ReplaceAllString(string(got.body), "")
}

var signature = regexp.MustCompile(`[?&]X-Amz-[^"]*`)

func (x *iso) views(ops []scopedOp) map[string]string {
	out := map[string]string{}
	for _, op := range ops {
		c := x.cases[op.id]
		if op.method != http.MethodGet || op.listing == "" && op.id != "getDashboard" && !strings.HasPrefix(op.id, "listMy") {
			continue
		}
		viewer := x.b
		if c.student {
			viewer = x.b.student
		}
		out[op.id] = x.view(op, c, viewer, true)
		if slices.ContainsFunc(op.slots, func(s resourceSlot) bool { return s.In == "query" && op.optional[s.Name] }) {
			out[op.id+" filtered"] = x.view(op, c, viewer, false)
		}
	}
	if got := x.b.c.send(http.MethodGet, "/teacher/students/"+x.shared, nil); got.status == http.StatusOK {
		out["getStudent shared"] = string(got.body)
	} else {
		x.t.Fatalf("B reading the shared student: %s", answer(got))
	}
	return out
}

func (x *iso) commitReviewWith(asset string) sent {
	x.t.Helper()
	imported := x.w.reviewedImport(x.b.c, x.b.userID, "commit-"+asset[:8])
	origins := map[string]any{"type": "teacher_entered", "prompt": "teacher_entered", "options": "teacher_entered", "answer": "teacher_entered", "points": "teacher_entered"}
	question := map[string]any{
		"id": "q1", "label": "1", "type": "single_choice", "prompt": textDoc("Bức tranh vẽ gì?"),
		"options": []any{
			map[string]any{"id": "a", "label": "A", "content": textDoc("một lớp học")},
			map[string]any{"id": "b", "label": "B", "content": textDoc("một khu chợ")},
		},
		"blanks": []any{}, "answer": map[string]any{"state": "known", "optionIds": []any{"a"}, "evidence": []any{}},
		"points": "1", "origins": origins, "source": []any{},
	}
	stimulus := map[string]any{"format": "semantic_v1", "blocks": []any{map[string]any{"type": "image", "assetId": asset, "alt": "Hình minh hoạ"}}}
	saved := x.b.c.must(http.StatusOK, http.MethodPut, "/teacher/imports/"+imported+"/review", map[string]any{
		"expectedRevision": 1, "title": "Đề có hình", "acknowledged": []any{},
		"sections": []any{map[string]any{"id": "s1", "title": "Phần 1", "origin": "teacher_entered", "source": []any{}, "items": []any{
			map[string]any{"group": map[string]any{"id": "g1", "stimulus": stimulus, "gaps": []any{}, "source": []any{}, "questions": []any{question}}},
		}}},
	})
	return x.b.c.send(http.MethodPost, "/teacher/imports/"+imported+"/commit", new(jsonPayload(x.t, map[string]any{
		"requestId": uuid.NewString(), "draftRevision": saved["revision"],
	})))
}

func (x *iso) beaconTo(attempt string) sent {
	x.t.Helper()
	raw, err := json.Marshal(map[string]any{
		"beaconToken": x.b.student.beacon, "sessionId": x.b.student.session,
		"events": []any{map[string]any{"kind": "tab_hidden", "occurredAt": rfc3339(time.Now()), "clientSeq": 1}},
	})
	if err != nil {
		x.t.Fatal(err)
	}
	return x.w.browser().send(http.MethodPost, "/app/attempts/"+attempt+"/events", &payload{contentType: "text/plain;charset=UTF-8", data: raw})
}

func (x *iso) groupUnit(group string) sent {
	x.t.Helper()
	b := x.b
	test := b.c.must(http.StatusOK, http.MethodGet, "/teacher/tests/"+b.id("listening-test"), nil)
	return b.c.send(http.MethodPatch, "/teacher/tests/"+b.id("listening-test"), new(jsonPayload(x.t, map[string]any{
		"expectedUpdatedAt": test["updatedAt"], "outlineFormat": "group_v1",
		"sections": []any{map[string]any{
			"id": b.id("listening-section"), "title": "Phần nghe", "questionIds": []any{},
			"units": []any{map[string]any{"kind": "group", "id": group}},
		}},
	})))
}

func TestAnotherTeachersIdsAnswerAsMissingOnes(t *testing.T) {
	w := bootWithStorage(t)
	x := &iso{t: t, w: w, cases: isolationCases()}
	ops := scopedOperations(t)

	var missing, unknown []string
	named := map[string]bool{}
	for _, op := range ops {
		named[op.id] = true
		if _, ok := x.cases[op.id]; !ok {
			missing = append(missing, op.id)
		}
	}
	for id := range x.cases {
		if !named[id] {
			unknown = append(unknown, id)
		}
	}
	sort.Strings(missing)
	sort.Strings(unknown)
	if len(missing) > 0 || len(unknown) > 0 {
		t.Fatalf("the isolation table lacks %v and names operations the contract does not have %v", missing, unknown)
	}

	x.b = w.teacherWorld("B")
	var shared *client
	shared, x.shared = w.sharedStudent(x.b)
	before := x.views(ops)
	x.a = w.teacherWorld("A")
	t.Cleanup(func() {
		if _, err := w.pool.Exec(context.Background(), `
			UPDATE app.word_imports SET status = 'cancelled'
			 WHERE created_by = ANY($1::uuid[]) AND status NOT IN ('committed', 'cancelled')`, []string{x.a.userID, x.b.userID}); err != nil {
			t.Errorf("closing the suite's imports: %v", err)
		}
	})
	if joined := shared.send(http.MethodPost, "/app/classes/join", new(jsonPayload(t, map[string]any{"joinCode": x.a.code}))); !success(joined) {
		t.Fatalf("B's student joining another teacher's class: %s", answer(joined))
	}
	sitting := shared.must(http.StatusOK, http.MethodPost, "/app/assignments/"+x.a.id("assignment")+"/attempts", nil)
	shared.must(http.StatusOK, http.MethodPost, "/app/attempts/"+id(sitting["attempt"].(map[string]any))+"/submit",
		map[string]any{"sessionId": sitting["sessionId"], "reason": "manual"})
	email, password := w.createStaff("admin")
	x.admin = w.signedIn(email, password)
	snapshot, footprint := w.snapshotOf(x.a), w.rowsNaming(x.a.all(), "word_import_drafts")

	for id, view := range x.views(ops) {
		if leaked := mentions([]byte(view), x.a); len(leaked) > 0 {
			t.Errorf("%s shows B another teacher's %v", id, leaked)
		}
		if view != before[id] {
			t.Errorf("%s changed for B when another teacher added their own work:\nbefore %s\nafter  %s", id, before[id], view)
		}
	}

	theirs, absent, mine := x.beaconTo(x.a.student.id("attempt")), x.beaconTo(uuid.NewString()), x.beaconTo(x.b.student.id("attempt"))
	if success(absent) || answer(theirs) != answer(absent) {
		t.Errorf("a beacon to another teacher's attempt answered\n  %s\nwhere a missing attempt answers\n  %s", answer(theirs), answer(absent))
	}
	if !success(mine) {
		t.Errorf("a beacon to the student's own attempt: %s", answer(mine))
	}

	for _, op := range ops {
		x.substitute(op, x.cases[op.id])
	}

	theirs, absent, mine = x.commitReviewWith(x.a.id("media")), x.commitReviewWith(uuid.NewString()), x.commitReviewWith(x.b.id("media"))
	if success(absent) || answer(theirs) != answer(absent) {
		t.Errorf("committing a review that holds another teacher's image answered\n  %s\nwhere a missing image answers\n  %s", answer(theirs), answer(absent))
	}
	if !success(mine) {
		t.Errorf("committing a review that holds the teacher's own image: %s", answer(mine))
	}

	theirs, absent, mine = x.groupUnit(x.a.id("listening-group")), x.groupUnit(uuid.NewString()), x.groupUnit(x.b.id("listening-group"))
	if success(absent) || answer(theirs) != answer(absent) || answer(x.groupUnit(x.a.id("question-group"))) != answer(absent) {
		t.Errorf("a group unit naming another teacher's group answered\n  %s\nwhere a missing group answers\n  %s", answer(theirs), answer(absent))
	}
	if !success(mine) {
		t.Errorf("a group unit naming the test's own group: %s", answer(mine))
	}

	for _, row := range w.crossReferences(x.a.all(), append(x.b.all(), x.made...), "word_import_drafts") {
		t.Errorf("a row ties another teacher's work to B's: %s", row)
	}
	if after := w.rowsNaming(x.a.all(), "word_import_drafts"); !slices.Equal(after, footprint) {
		t.Errorf("rows naming another teacher's work changed while B was refused:\nbefore %v\nafter  %v", footprint, after)
	}

	if after := w.snapshotOf(x.a); after != snapshot {
		t.Errorf("another teacher's rows changed while B was refused: %s became %s", snapshot, after)
	}
	for kind, path := range map[string]string{
		"test": "/teacher/tests/", "class": "/teacher/classes/", "assignment": "/teacher/assignments/",
		"attempt": "/teacher/attempts/", "question": "/teacher/questions/", "question-group": "/teacher/question-groups/",
		"import": "/teacher/imports/", "student": "/teacher/students/",
	} {
		if got := x.admin.send(http.MethodGet, path+x.a.id(kind), nil); got.status != http.StatusOK {
			t.Errorf("the Admin opening the teacher's %s: %d %s", kind, got.status, got.body)
		}
	}
	x.adminListsOwnRows()
	x.adminsOwnAssignmentNamesEveryTarget()
}

func (x *iso) adminListsOwnRows() {
	x.t.Helper()
	for path, counts := range map[string][]string{
		"/teacher/tests":                      {"total", "facets.all"},
		"/teacher/questions":                  {"total", "facets.all", "bankTotal"},
		"/teacher/question-groups?status=all": {"total"},
		"/teacher/media":                      {"total", "totalBytes", "facets.all", "facets.unused", "usage.audioBytes", "usage.imageBytes"},
		"/teacher/imports":                    {"total"},
		"/teacher/assignments":                {"total", "facets.all"},
	} {
		got := x.admin.send(http.MethodGet, path, nil)
		if got.status != http.StatusOK {
			x.t.Errorf("the Admin listing %s: %s", path, answer(got))
			continue
		}
		if leaked := append(mentions(got.body, x.a), mentions(got.body, x.b)...); len(leaked) > 0 {
			x.t.Errorf("the Admin's %s shows a teacher's %v", path, leaked)
		}
		for _, field := range counts {
			var value any = got.json
			for _, key := range strings.Split(field, ".") {
				object, _ := value.(map[string]any)
				value = object[key]
			}
			if n, ok := value.(float64); !ok || n != 0 {
				x.t.Errorf("the Admin's %s counts %s = %v, want 0: the Admin owns nothing here", path, field, value)
			}
		}
	}
}

func (x *iso) adminsOwnAssignmentNamesEveryTarget() {
	x.t.Helper()
	class := x.a.id("class")
	created := x.admin.assign(x.a.id("test-version"), class)
	opened := x.admin.must(http.StatusOK, http.MethodGet, "/teacher/assignments/"+id(created), nil)
	targets, _ := opened["targets"].(map[string]any)
	if classes, _ := targets["classes"].([]any); len(classes) != 1 || opened["targetCount"] == float64(0) {
		x.t.Fatalf("the Admin's assignment on the teacher's class opens with targets %v and counts %v", opened["targets"], opened["targetCount"])
	}
	for label, path := range map[string]string{
		"list":                         "/teacher/assignments",
		"list for the teacher's class": "/teacher/assignments?classId=" + class,
	} {
		got := x.admin.must(http.StatusOK, http.MethodGet, path, nil)
		items, _ := got["items"].([]any)
		facets, _ := got["facets"].(map[string]any)
		if len(items) != 1 || got["total"] != float64(1) || facets["all"] != float64(1) {
			x.t.Errorf("the Admin's %s holds %d rows, total %v and facets %v, want the Admin's one assignment", label, len(items), got["total"], facets)
			continue
		}
		row := items[0].(map[string]any)
		if id(row) != id(created) || !reflect.DeepEqual(row["targets"], opened["targets"]) || row["targetCount"] != opened["targetCount"] {
			x.t.Errorf("the Admin's %s names %v and counts %v, want what the assignment opened by id names and counts: %v and %v",
				label, row["targets"], row["targetCount"], opened["targets"], opened["targetCount"])
		}
	}
}

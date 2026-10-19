//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func firstSectionOf(t *testing.T, teacher *client, test string) string {
	t.Helper()
	got := teacher.must(http.StatusOK, http.MethodGet, "/teacher/tests/"+test, nil)
	return got["sections"].([]any)[0].(map[string]any)["id"].(string)
}

func TestStandaloneGenerateRefusesABadShapeBeforeAnyCredit(t *testing.T) { runStandaloneGenerate(t, 0) }

func TestStandaloneGenerateReadsOnlyTheCallersSectionAndNoKey(t *testing.T) {
	runStandaloneGenerate(t, 1)
}

func runStandaloneGenerate(t *testing.T, part int) {
	provider := newAssistant()
	w := bootAssisted(t, provider)
	aEmail, aPassword := w.createStaff("teacher")
	bEmail, bPassword := w.createStaff("teacher")
	a, b := w.signedIn(aEmail, aPassword), w.signedIn(bEmail, bPassword)
	aTest := w.harbourTest(a, true)
	bTest := w.harbourTest(b, true)
	aSection, bSection := firstSectionOf(t, a, aTest), firstSectionOf(t, b, bTest)
	base := func(extra map[string]any) map[string]any {
		body := map[string]any{"types": []string{"short_answer"}, "count": 3, "level": "b1", "explanations": true, "language": "en"}
		for k, v := range extra {
			body[k] = v
		}
		return body
	}
	used := func() float64 {
		return a.must(http.StatusOK, http.MethodGet, "/teacher/ai/usage", nil)["used"].(float64)
	}
	reserved := func() int {
		var n int
		if err := w.pool.QueryRow(t.Context(), `SELECT coalesce(sum(reserved_credits),0) FROM app.ai_teacher_credits`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	other := uuid.NewString()
	refusals := []struct {
		name   string
		body   map[string]any
		status int
		field  string
	}{
		{"passage source 39 chars", base(map[string]any{"source": strings.Repeat("x", 39)}), 400, "source"},
		{"passage source 2 chars (schema)", base(map[string]any{"source": "ab"}), 400, ""},
		{"standalone no source no test", base(map[string]any{"standalone": true}), 400, "source"},
		{"standalone topic 2 chars", base(map[string]any{"standalone": true, "source": "ab"}), 400, ""},
		{"standalone whitespace topic", base(map[string]any{"standalone": true, "source": "  a  "}), 400, "source"},
		{"standalone test without section", base(map[string]any{"standalone": true, "testId": aTest}), 400, "sectionId"},
		{"standalone section without test", base(map[string]any{"standalone": true, "sectionId": aSection}), 400, "testId"},
		{"section without standalone", base(map[string]any{"testId": aTest, "sectionId": aSection}), 400, "sectionId"},
		{"standalone with stimulus", base(map[string]any{"standalone": true, "testId": aTest, "sectionId": aSection, "stimulusId": other}), 400, "stimulusId"},
		{"another teacher's test and section", base(map[string]any{"standalone": true, "testId": bTest, "sectionId": bSection}), 404, ""},
		{"my test, another teacher's section", base(map[string]any{"standalone": true, "testId": aTest, "sectionId": bSection}), 404, ""},
		{"another teacher's test, my section", base(map[string]any{"standalone": true, "testId": bTest, "sectionId": aSection}), 404, ""},
		{"section the test does not hold", base(map[string]any{"standalone": true, "testId": aTest, "sectionId": other}), 404, ""},
	}
	if part == 0 {
		refusals = refusals[:8]
	} else {
		refusals = refusals[8:]
	}
	for _, c := range refusals {
		before, usedBefore := len(provider.sent()), used()
		status, body := a.call(http.MethodPost, "/teacher/ai/questions/generate", c.body)
		if status != c.status {
			t.Errorf("%s: status %d, want %d: %v", c.name, status, c.status, body)
		}
		if c.field != "" {
			errBody, _ := body["error"].(map[string]any)
			details, _ := errBody["details"].(map[string]any)
			if _, ok := details[c.field]; !ok {
				t.Errorf("%s: details %v, want field %q", c.name, details, c.field)
			}
		}
		if n := len(provider.sent()); n != before || used() != usedBefore || reserved() != 0 {
			t.Errorf("%s: the provider was called or credits moved (calls %d->%d, used %v->%v, reserved %d)", c.name, before, n, usedBefore, used(), reserved())
		}
	}

	if part == 0 {
		old := a.must(http.StatusOK, http.MethodPost, "/teacher/ai/questions/generate", base(map[string]any{"source": strings.Repeat("x", 40)}))
		if _, ok := old["items"]; !ok {
			t.Errorf("an old-shaped request answered %v", old)
		}
		return
	}
	got := a.must(http.StatusOK, http.MethodPost, "/teacher/ai/questions/generate", base(map[string]any{"standalone": true, "testId": aTest, "sectionId": aSection}))
	if items, _ := got["items"].([]any); len(items) != 1 {
		t.Fatalf("a fitted run kept %v", got)
	}
	sent := provider.sent()
	read := textOf(sent[len(sent)-1])
	for _, want := range []string{"Title: Phần 1", loneQuestion, firstMember, secondMember, "never refers to a passage"} {
		if !strings.Contains(read, want) {
			t.Errorf("the section run was not shown %q:\n%s", want, read)
		}
	}
	for _, leak := range []string{harbourPassage, "In 1902", "In 1920", "A teacher.", "sampleAnswer", "isCorrect", aEmail, aTest, aSection} {
		if strings.Contains(read, leak) {
			t.Errorf("the section run carries %q:\n%s", leak, read)
		}
	}
}

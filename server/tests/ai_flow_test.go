//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"

	aiports "quizzivy/internal/modules/ai/application/ports"
	identitydomain "quizzivy/internal/modules/identity/domain"
)

var cleanupBody = map[string]any{"checks": []string{"duplicates", "tags"}}

const forbiddenCopy = "Bạn không có quyền thao tác trên tài khoản này."

func (w *world) staffHolding(keys ...string) (email, password string) {
	w.t.Helper()
	ctx := context.Background()
	var role string
	if err := w.pool.QueryRow(ctx, `INSERT INTO app.roles (name, icon, color) VALUES ($1, 'user', 'gray') RETURNING id::text`, "Vai trò thử "+nonce(w.t)).Scan(&role); err != nil {
		w.t.Fatal(err)
	}
	for _, key := range keys {
		if _, err := w.pool.Exec(ctx, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1::uuid, $2)`, role, key); err != nil {
			w.t.Fatalf("grant %s: %v", key, err)
		}
	}
	password = "vai-tro-" + nonce(w.t)
	hash, err := identitydomain.Passwords.Hash(ctx, password)
	if err != nil {
		w.t.Fatal(err)
	}
	email = "vai-tro-" + nonce(w.t) + "@example.com"
	if _, err := w.pool.Exec(ctx, `INSERT INTO app.users (email, full_name, role_id, password_hash) VALUES ($1, 'Người giữ vai trò', $2::uuid, $3)`, email, role, hash); err != nil {
		w.t.Fatal(err)
	}
	return email, password
}

type sitting struct {
	teacher    *client
	student    *client
	test       string
	assignment string
	attempt    string
	short      string
	choice     string
	shortBank  string
	choiceBank string
	className  string
	name       string
	email      string
	studentID  string
	classID    string
	answer     string
}

func (w *world) assistedSitting(teacher *client) sitting {
	w.t.Helper()
	s := sitting{teacher: teacher, className: "Lớp Bí Mật " + nonce(w.t), name: "Nguyễn Kín Đáo " + nonce(w.t), answer: "The person is a teacher who helps."}
	short := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "short_answer", "prompt": "Name the person who teaches a class.", "points": 1, "sampleAnswer": "A teacher.",
	})
	choice := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
		"type": "single_choice", "prompt": "Which word is a noun?", "points": 1,
		"options": []map[string]any{{"text": "quickly", "isCorrect": false}, {"text": "teacher", "isCorrect": true}},
	})
	s.shortBank, s.choiceBank = id(short), id(choice)
	test := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests", map[string]any{"title": "Đề có trợ lý " + nonce(w.t)})
	s.test = id(test)
	teacher.must(http.StatusOK, http.MethodPatch, "/teacher/tests/"+s.test, map[string]any{
		"expectedUpdatedAt": test["updatedAt"],
		"sections":          []map[string]any{{"title": "Phần 1", "questionIds": []string{s.shortBank, s.choiceBank}}},
	})
	version := id(teacher.must(http.StatusCreated, http.MethodPost, "/teacher/tests/"+s.test+"/publish", nil))
	s.classID = teacher.class(s.className)
	s.email = "kin-dao-" + nonce(w.t) + "@example.com"
	created := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/students", map[string]any{
		"email": s.email, "fullName": s.name, "classIds": []string{s.classID},
	})
	s.studentID = created["user"].(map[string]any)["id"].(string)
	s.student = w.signedIn(s.email, created["temporaryPassword"].(string))
	s.assignment = id(teacher.assign(version, s.classID))
	session := s.student.must(http.StatusOK, http.MethodPost, "/app/assignments/"+s.assignment+"/attempts", nil)
	s.attempt = id(session["attempt"].(map[string]any))
	for _, q := range session["questions"].([]any) {
		question := q.(map[string]any)
		switch question["type"] {
		case "short_answer":
			s.short = id(question)
		case "single_choice":
			s.choice = id(question)
		}
	}
	if s.short == "" || s.choice == "" {
		w.t.Fatalf("the paper has no short answer or no choice question: %v", session["questions"])
	}
	s.student.must(http.StatusOK, http.MethodPatch, "/app/attempts/"+s.attempt+"/answers", map[string]any{
		"sessionId": session["sessionId"],
		"answers":   map[string]any{s.short: map[string]any{"type": "text", "value": s.answer}},
	})
	s.student.must(http.StatusOK, http.MethodPost, "/app/attempts/"+s.attempt+"/submit", map[string]any{"sessionId": session["sessionId"], "reason": "manual"})
	return s
}

func usageOf(t *testing.T, result map[string]any) (credits, left float64) {
	t.Helper()
	usage, ok := result["usage"].(map[string]any)
	if !ok {
		t.Fatalf("the answer carries no usage: %v", result)
	}
	return usage["credits"].(float64), usage["left"].(float64)
}

func TestEveryAssistantOperationRunsThroughTheWholeStackMetersWhatItSpendsAndSendsNoStudentIdentifier(t *testing.T) {
	provider := newAssistant()
	w := bootAssisted(t, provider)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	teacherID := teacher.me(t)
	s := w.assistedSitting(teacher)

	passage := strings.Repeat("Parks give a town room to breathe, to play and to meet. ", 3)
	calls := []struct {
		name   string
		method string
		path   string
		body   any
		field  string
	}{
		{"generate", http.MethodPost, "/teacher/ai/questions/generate", map[string]any{
			"source": passage, "types": []string{"short_answer"}, "count": 3, "level": "b1", "explanations": true, "language": "en",
		}, "items"},
		{"check", http.MethodPost, "/teacher/ai/tests/" + s.test + "/check", map[string]any{"checks": []string{"key"}}, "findings"},
		{"variant", http.MethodPost, "/teacher/ai/tests/" + s.test + "/variant", map[string]any{"changes": []string{"reword"}, "difficulty": "same"}, "changes"},
		{"explanation", http.MethodPost, "/teacher/ai/questions/" + s.choiceBank + "/explanation", map[string]any{"language": "en"}, "text"},
		{"distractors", http.MethodPost, "/teacher/ai/questions/" + s.choiceBank + "/distractors", nil, "items"},
		{"score", http.MethodPost, "/teacher/ai/attempts/" + s.attempt + "/answers/" + s.short + "/score-suggestion", nil, "score"},
		{"insights", http.MethodPost, "/teacher/ai/assignments/" + s.assignment + "/insights", nil, "items"},
		{"cleanup", http.MethodPost, "/teacher/ai/bank/cleanup", map[string]any{"checks": []string{"duplicates", "tags"}}, "duplicates"},
	}

	before := teacher.must(http.StatusOK, http.MethodGet, "/teacher/ai/usage", nil)
	if before["used"] != float64(0) || before["paused"] != false {
		t.Fatalf("a new teacher's month: %v", before)
	}
	var charged float64
	lastLeft := before["left"].(float64)
	for _, call := range calls {
		got := teacher.must(http.StatusOK, call.method, call.path, call.body)
		if _, ok := got[call.field]; !ok {
			t.Errorf("%s: the answer has no %q: %v", call.name, call.field, got)
		}
		credits, left := usageOf(t, got)
		if credits <= 0 || left != lastLeft-credits {
			t.Errorf("%s charged %v and left %v, want a positive charge off the %v that remained", call.name, credits, left, lastLeft)
		}
		charged += credits
		lastLeft = left
	}
	if sent := provider.sent(); len(sent) != len(calls) {
		t.Fatalf("the provider was asked %d times for %d operations", len(sent), len(calls))
	}

	after := teacher.must(http.StatusOK, http.MethodGet, "/teacher/ai/usage", nil)
	if after["used"] != charged || after["left"] != lastLeft {
		t.Errorf("the month reads used %v left %v, want the %v the calls were charged and the %v they left", after["used"], after["left"], charged, lastLeft)
	}

	var reserved int
	if err := w.pool.QueryRow(context.Background(), `SELECT reserved_credits FROM app.ai_teacher_credits WHERE user_id = $1::uuid`, teacherID).Scan(&reserved); err != nil || reserved != 0 {
		t.Errorf("credits still held after every call settled: %d, %v", reserved, err)
	}

	identifying := []string{s.name, s.email, s.studentID, s.attempt, s.assignment, s.classID, s.className, teacherID, email}
	for i, request := range provider.sent() {
		sent := textOf(request)
		for _, secret := range identifying {
			if strings.Contains(sent, secret) {
				t.Errorf("%s: the prompt carries %q, which identifies a person or a record", calls[i].name, secret)
			}
		}
	}
	score := textOf(provider.sent()[5])
	if !strings.Contains(score, s.answer) || !strings.Contains(score, "A teacher.") {
		t.Errorf("the score prompt lacks the answer it is asked about or its key:\n%s", score)
	}
}

func TestTheAssistantAnswersWhatHappenedWhenTheProviderFailsOrTheAllowanceRunsOut(t *testing.T) {
	provider := newAssistant()
	w := bootAssisted(t, provider)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	adminEmail, adminPassword := w.createStaff("admin")
	admin := w.signedIn(adminEmail, adminPassword)
	teacherID := teacher.me(t)
	s := w.assistedSitting(teacher)
	check := func() (int, http.Header, map[string]any) {
		return teacher.callWith(nil, http.MethodPost, "/teacher/ai/tests/"+s.test+"/check", map[string]any{"checks": []string{"key"}})
	}

	provider.failWith(aiports.ErrUnavailable)
	status, header, body := check()
	if status != http.StatusServiceUnavailable || teacher.errorCode(body) != "AI_UNAVAILABLE" || header.Get("Retry-After") != "30" {
		t.Fatalf("a provider that is down answered %d %v with Retry-After %q", status, body, header.Get("Retry-After"))
	}
	if used := teacher.must(http.StatusOK, http.MethodGet, "/teacher/ai/usage", nil)["used"]; used != float64(0) {
		t.Errorf("a failed call was charged %v", used)
	}
	provider.failWith(aiports.ErrRejected)
	status, header, body = check()
	if status != http.StatusServiceUnavailable || teacher.errorCode(body) != "AI_UNAVAILABLE" || header.Get("Retry-After") != "30" {
		t.Fatalf("a request the provider rejected answered %d %v with Retry-After %q, want the same 503 as a provider that is down", status, body, header.Get("Retry-After"))
	}
	provider.failWith(nil)

	admin.must(http.StatusOK, http.MethodPut, "/admin/ai/teachers/"+teacherID+"/allowance", map[string]any{"custom": true, "credits": 1000})
	provider.usage = aiports.Usage{InputTokens: 400_000, OutputTokens: 40_000}
	for i := range 3 {
		if status, _, body := check(); status != http.StatusOK {
			t.Fatalf("call %d of the allowance: %d %v", i+1, status, body)
		}
	}
	status, _, body = check()
	if status != http.StatusPaymentRequired || teacher.errorCode(body) != "AI_CREDITS_EXHAUSTED" {
		t.Fatalf("a call past the allowance answered %d %v", status, body)
	}
	if scope := body["error"].(map[string]any)["details"].(map[string]any)["scope"]; scope != "teacher" {
		t.Errorf("the refusal names scope %v, want teacher", scope)
	}
	asked := len(provider.sent())
	if asked != 5 {
		t.Errorf("the provider was asked %d times, want the two failed calls and the three that fit", asked)
	}

	admin.must(http.StatusOK, http.MethodPut, "/admin/ai/teachers/"+teacherID+"/allowance", map[string]any{"paused": true})
	status, _, body = check()
	if status != http.StatusForbidden || teacher.errorCode(body) != "AI_PAUSED" {
		t.Fatalf("a paused teacher's call answered %d %v", status, body)
	}
	if len(provider.sent()) != asked {
		t.Error("the provider was asked on behalf of a paused teacher")
	}
}

func TestTheBankCleanupReadsOnlyTheCallersOwnQuestions(t *testing.T) {
	provider := newAssistant()
	w := bootAssisted(t, provider)
	post := func(c *client, text string) {
		c.must(http.StatusCreated, http.MethodPost, "/teacher/questions", map[string]any{
			"type": "short_answer", "prompt": text, "points": 1, "sampleAnswer": "Mẫu.",
		})
	}
	var teachers [2]*client
	for i := range teachers {
		email, password := w.createStaff("teacher")
		teachers[i] = w.signedIn(email, password)
	}
	marker := "dau-vet-" + nonce(t)
	post(teachers[0], "Câu của cô thứ nhất "+marker)
	post(teachers[0], "Câu khác của cô thứ nhất "+marker)
	post(teachers[1], "Câu của cô thứ hai "+marker+"-hai")

	got := teachers[1].must(http.StatusOK, http.MethodPost, "/teacher/ai/bank/cleanup", cleanupBody)
	if got["scanned"] != float64(1) {
		t.Errorf("the second teacher's cleanup scanned %v questions, want their one", got["scanned"])
	}
	sent := textOf(provider.sent()[0])
	if !strings.Contains(sent, marker+"-hai") || strings.Contains(sent, "thứ nhất") {
		t.Errorf("the cleanup prompt names another teacher's questions or lacks the caller's own:\n%s", sent)
	}

	adminEmail, adminPassword := w.createStaff("admin")
	admin := w.signedIn(adminEmail, adminPassword)
	status, body := admin.call(http.MethodPost, "/teacher/ai/bank/cleanup", cleanupBody)
	if status == http.StatusOK || len(provider.sent()) != 1 {
		t.Errorf("the Admin, who owns no questions, was answered %d %v and the provider asked %d times", status, body, len(provider.sent()))
	}
}

func TestACustomRoleHoldingOnlyCreditsManageCannotPauseOrDecideForSomeoneWhoHoldsMore(t *testing.T) {
	provider := newAssistant()
	w := bootAssisted(t, provider)
	adminEmail, adminPassword := w.createStaff("admin")
	admin := w.signedIn(adminEmail, adminPassword)
	adminID := admin.me(t)
	teacherEmail, teacherPassword := w.createStaff("teacher")
	teacher := w.signedIn(teacherEmail, teacherPassword)
	teacherID := teacher.me(t)

	managerEmail, managerPassword := w.staffHolding("ai.credits.manage")
	manager := w.signedIn(managerEmail, managerPassword)

	paused := map[string]any{"paused": true}
	for name, target := range map[string]string{"an Admin": adminID, "a Teacher": teacherID} {
		status, body := manager.call(http.MethodPut, "/admin/ai/teachers/"+target+"/allowance", paused)
		if status != http.StatusForbidden || manager.errorCode(body) != "FORBIDDEN" {
			t.Errorf("pausing %s answered %d %v, want 403 FORBIDDEN", name, status, body)
			continue
		}
		if message := body["error"].(map[string]any)["message"]; message != forbiddenCopy {
			t.Errorf("pausing %s said %q, want the words a refused user-admin operation uses: %q", name, message, forbiddenCopy)
		}
	}
	custom := map[string]any{"custom": true, "credits": 5000}
	if status, body := manager.call(http.MethodPut, "/admin/ai/teachers/"+teacherID+"/allowance", custom); status != http.StatusForbidden {
		t.Errorf("setting a Teacher's allowance answered %d %v", status, body)
	}

	request := teacher.must(http.StatusCreated, http.MethodPost, "/teacher/ai/credit-requests", map[string]any{"amount": 5000, "reason": "Chuẩn bị kiểm tra giữa kỳ"})
	for _, verb := range []string{"approve", "decline"} {
		status, body := manager.call(http.MethodPost, "/admin/ai/credit-requests/"+id(request)+"/"+verb, nil)
		if status != http.StatusForbidden || manager.errorCode(body) != "FORBIDDEN" {
			t.Errorf("%s by a role that holds less than the Teacher answered %d %v", verb, status, body)
		}
	}

	var changes, decided int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.audit_log WHERE action = 'ai.allowance.updated' AND entity_id = ANY($1::uuid[])`, []string{adminID, teacherID}).Scan(&changes); err != nil {
		t.Fatal(err)
	}
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.ai_credit_requests WHERE id = $1::uuid AND status <> 'pending'`, id(request)).Scan(&decided); err != nil {
		t.Fatal(err)
	}
	if changes != 0 || decided != 0 {
		t.Errorf("refused changes left %d allowance audit rows and %d decided requests", changes, decided)
	}

	approved := admin.must(http.StatusOK, http.MethodPost, "/admin/ai/credit-requests/"+id(request)+"/approve", nil)
	if approved["request"].(map[string]any)["status"] != "approved" {
		t.Errorf("the Admin's approval: %v", approved)
	}

	peerEmail, peerPassword := w.staffHolding("ai.credits.manage", "ai.use", "content.tests.write")
	peer := w.signedIn(peerEmail, peerPassword)
	peerID := peer.me(t)
	juniorEmail, juniorPassword := w.staffHolding("ai.use", "learning.take_tests")
	junior := w.signedIn(juniorEmail, juniorPassword)
	juniorID := junior.me(t)

	got := peer.must(http.StatusOK, http.MethodPut, "/admin/ai/teachers/"+juniorID+"/allowance", paused)
	if got["paused"] != true {
		t.Errorf("a role that holds every key of the target but learning.take_tests could not pause them: %v", got)
	}
	got = peer.must(http.StatusOK, http.MethodPut, "/admin/ai/teachers/"+juniorID+"/allowance", map[string]any{"paused": false})
	if got["paused"] != false {
		t.Errorf("resuming: %v", got)
	}
	if status, body := junior.call(http.MethodPut, "/admin/ai/teachers/"+peerID+"/allowance", paused); status != http.StatusForbidden {
		t.Errorf("a role without the permission answered %d %v", status, body)
	}
}

func TestEveryAssistantOperationAnswersNotImplementedWhileNoProviderIsConfigured(t *testing.T) {
	w := boot(t)
	email, password := w.createStaff("teacher")
	teacher := w.signedIn(email, password)
	adminEmail, adminPassword := w.createStaff("admin")
	admin := w.signedIn(adminEmail, adminPassword)
	teacherID := teacher.me(t)

	for name, call := range map[string]struct {
		as           *client
		method, path string
		body         any
	}{
		"usage":        {teacher, http.MethodGet, "/teacher/ai/usage", nil},
		"cleanup":      {teacher, http.MethodPost, "/teacher/ai/bank/cleanup", cleanupBody},
		"request":      {teacher, http.MethodPost, "/teacher/ai/credit-requests", map[string]any{"amount": 5000, "reason": "Cần thêm"}},
		"console":      {admin, http.MethodGet, "/admin/ai/credits", nil},
		"allowance":    {admin, http.MethodPut, "/admin/ai/teachers/" + teacherID + "/allowance", map[string]any{"paused": true}},
		"rules":        {admin, http.MethodPatch, "/admin/ai/rules", map[string]any{"warnPercent": 70}},
		"addedCredits": {admin, http.MethodPost, "/admin/ai/credits/additions", map[string]any{"credits": 5000}},
	} {
		status, body := call.as.call(call.method, call.path, call.body)
		if status != http.StatusNotImplemented || call.as.errorCode(body) != "AI_OFF" {
			t.Errorf("%s answered %d %v, want 501 AI_OFF", name, status, body)
		}
	}
	status := w.browser()
	if got := status.must(http.StatusOK, http.MethodGet, "/public/status", nil)["ai"]; got != false {
		t.Errorf("the public status says ai %v while no provider is configured", got)
	}
}

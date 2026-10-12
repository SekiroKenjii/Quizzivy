package router_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"quizzivy/internal/core/router"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"testing"
)

func TestGradingQueueServesTheAuthorizedDefault(t *testing.T) {
	issuer := testIssuer(t)
	called := false
	app := &application.Application{Queries: application.Queries{
		GradingQueue: cqrs.HandlerFunc[query.GradingQueue, domain.GradingQueue](func(_ context.Context, q query.GradingQueue) (domain.GradingQueue, error) {
			called = true
			if q.Mode != "student" || q.Scope.UserID != teacherUser || q.Scope.All {
				t.Fatalf("default/scope lost: %+v", q)
			}
			return domain.GradingQueue{Groups: []domain.GradingQueueGroup{}, Items: []domain.GradingQueueItem{}}, nil
		})}}
	h, err := router.New(router.Deps{Principals: rolePrincipals(), DB: fakeDB{}, Tokens: issuer, Modules: router.Modules{Attempts: attemptshttp.NewAttempts(app, nil, nil, nil)}}, slog.New(slog.NewTextHandler(io.Discard, nil)), []string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatal(err)
	}
	response := sendAs(t, h, issuer, http.MethodGet, "/teacher/grading/queue", teacherUser, "")
	if response.Code != http.StatusOK {
		t.Fatalf("queue status=%d want200: %s", response.Code, response.Body.String())
	}
	var body struct {
		Groups           []any `json:"groups"`
		Items            []any `json:"items"`
		AnswersRemaining int   `json:"answersRemaining"`
		StudentsWaiting  int   `json:"studentsWaiting"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !called || body.Groups == nil || body.Items == nil || body.AnswersRemaining != 0 || body.StudentsWaiting != 0 {
		t.Fatalf("empty queue contract/call: called=%t %+v", called, body)
	}
}

func TestGradingQueueRouteValidatesAndRequiresGradingPermission(t *testing.T) {
	issuer := testIssuer(t)
	principals := rolePrincipals()
	denied := builtinPrincipal(teacherUser, access.BuiltinTeacher)
	denied.Permissions = denied.Permissions.Without(access.TeachingGrading)
	seen := []query.GradingQueue{}
	app := &application.Application{Queries: application.Queries{GradingQueue: cqrs.HandlerFunc[query.GradingQueue, domain.GradingQueue](func(_ context.Context, q query.GradingQueue) (domain.GradingQueue, error) {
		seen = append(seen, q)
		if q.AssignmentID != nil {
			return domain.GradingQueue{}, domain.ErrPaperNotFound
		}
		return domain.GradingQueue{}, nil
	})}}
	h, err := router.New(router.Deps{Principals: principals, DB: fakeDB{}, Tokens: issuer, Modules: router.Modules{Attempts: attemptshttp.NewAttempts(app, nil, nil, nil)}}, slog.New(slog.NewTextHandler(io.Discard, nil)), []string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, path, user string
		want             int
	}{
		{"anonymous", "/teacher/grading/queue", "", 401}, {"student", "/teacher/grading/queue", studentUser, 403},
		{"bad mode", "/teacher/grading/queue?mode=unknown", teacherUser, 400}, {"bad assignment UUID", "/teacher/grading/queue?assignmentId=bad", teacherUser, 400}, {"bad student UUID", "/teacher/grading/queue?studentId=bad", teacherUser, 400},
		{"question mode", "/teacher/grading/queue?mode=question", teacherUser, 200}, {"all scope", "/teacher/grading/queue", adminUser, 200},
		{"unknown reference", "/teacher/grading/queue?assignmentId=01935000-0000-7000-8000-000000000000", teacherUser, 404},
	} {
		t.Run(c.name, func(t *testing.T) {
			before := len(seen)
			got := sendAs(t, h, issuer, http.MethodGet, c.path, c.user, "")
			if got.Code != c.want {
				t.Fatalf("status=%d want%d: %s", got.Code, c.want, got.Body.String())
			}
			if c.want == 400 || c.want == 401 || c.want == 403 {
				if len(seen) != before {
					t.Fatal("refused request reached query")
				}
			}
		})
	}
	if len(seen) != 3 || seen[0].Mode != "question" || seen[1].Mode != "student" || !seen[1].Scope.All {
		t.Fatalf("authorized query/scope=%+v", seen)
	}
	principals.set(denied)
	before := len(seen)
	got := sendAs(t, h, issuer, http.MethodGet, "/teacher/grading/queue", teacherUser, "")
	if got.Code != 403 || len(seen) != before {
		t.Fatalf("removed permission=%d querycalls=%d", got.Code, len(seen))
	}
}

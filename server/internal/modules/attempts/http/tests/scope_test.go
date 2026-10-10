package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	identityquery "quizzivy/internal/modules/identity/application/query"
	identitydomain "quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

type reachResolver map[string]access.Principal

func (r reachResolver) Resolve(_ context.Context, userID string) (access.Principal, error) {
	return r[userID], nil
}

func reachContext(t *testing.T, principal access.Principal) context.Context {
	t.Helper()
	var ctx context.Context
	verify := func(string) (httpx.Principal, error) { return httpx.Principal{UserID: principal.UserID}, nil }
	gate := httpx.RequirePermission(map[string]access.Requirement{"GET /teacher/attempts": access.AnyOf(access.WorkspaceTeacher)},
		reachResolver{principal.UserID: principal})
	h := httpx.RequireAuth(nil, verify)(gate(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { ctx = r.Context() })))
	req := httptest.NewRequest(http.MethodGet, "/teacher/attempts", nil)
	req.Pattern = "GET /teacher/attempts"
	req.Header.Set("Authorization", "Bearer token")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ctx == nil {
		t.Fatal("the gate did not pass the request")
	}
	return ctx
}

func TestEveryTeacherAttemptOperationCarriesTheCallersScope(t *testing.T) {
	for name, principal := range map[string]access.Principal{
		"a Teacher": {UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingGrading, access.TeachingAttemptsIntervene)},
		"an Admin":  {UserID: uuid.NewString(), Permissions: access.NewSet(access.All()...)},
	} {
		t.Run(name, func(t *testing.T) {
			want := access.Scope{UserID: principal.UserID, All: principal.Permissions.Has(access.ScopeAll)}
			scopes := map[string][]access.Scope{}
			seen := func(op string, scope access.Scope) { scopes[op] = append(scopes[op], scope) }
			intervene := func(op string, req domain.Request) { seen(op, req.Scope()) }
			paper := domain.Review{Attempt: domain.Attempt{ID: uuid.NewString(), StudentID: uuid.NewString(), Status: domain.Submitted}}
			app := &application.Application{
				Queries: application.Queries{
					Monitor: cqrs.HandlerFunc[query.Monitor, domain.Monitor](func(_ context.Context, q query.Monitor) (domain.Monitor, error) {
						seen("monitor", q.Scope)
						return domain.Monitor{}, domain.ErrNotFound
					}),
					Review: cqrs.HandlerFunc[query.Review, domain.Review](func(_ context.Context, q query.Review) (domain.Review, error) {
						seen("review", q.Scope)
						return paper, nil
					}),
					Timeline: cqrs.HandlerFunc[query.Timeline, domain.Timeline](func(_ context.Context, q query.Timeline) (domain.Timeline, error) {
						seen("timeline", q.Scope)
						return domain.Timeline{}, nil
					}),
					AnswersForQuestion: cqrs.HandlerFunc[query.AnswersForQuestion, domain.ByQuestion](func(_ context.Context, q query.AnswersForQuestion) (domain.ByQuestion, error) {
						seen("byQuestion", q.Scope)
						return domain.ByQuestion{}, domain.ErrPaperNotFound
					}),
				},
				Commands: application.Commands{
					ExpireDue: cqrs.HandlerFunc[command.ExpireDue, cqrs.Nothing](func(_ context.Context, c command.ExpireDue) (cqrs.Nothing, error) {
						seen("expire", c.Scope)
						return cqrs.Nothing{}, nil
					}),
					SetNote: cqrs.HandlerFunc[command.SetNote, cqrs.Nothing](func(_ context.Context, c command.SetNote) (cqrs.Nothing, error) {
						seen("note", c.Scope)
						return cqrs.Nothing{}, nil
					}),
					Grade: cqrs.HandlerFunc[command.Grade, domain.Score](func(_ context.Context, c command.Grade) (domain.Score, error) {
						seen("grade", c.Scope)
						return domain.Score{}, domain.ErrPaperNotFound
					}),
					Finish: cqrs.HandlerFunc[command.Finish, domain.Attempt](func(_ context.Context, c command.Finish) (domain.Attempt, error) {
						seen("finish", c.Scope)
						return domain.Attempt{}, domain.ErrPaperNotFound
					}),
					Extend: cqrs.HandlerFunc[command.Extend, domain.Attempt](func(_ context.Context, c command.Extend) (domain.Attempt, error) {
						intervene("extend", c.Request)
						return domain.Attempt{}, domain.ErrNotFound
					}),
					Flag: cqrs.HandlerFunc[command.Flag, domain.Attempt](func(_ context.Context, c command.Flag) (domain.Attempt, error) {
						intervene("flag", c.Request)
						return domain.Attempt{}, domain.ErrNotFound
					}),
					Void: cqrs.HandlerFunc[command.Void, domain.Attempt](func(_ context.Context, c command.Void) (domain.Attempt, error) {
						intervene("void", c.Request)
						return domain.Attempt{}, domain.ErrNotFound
					}),
					Reset: cqrs.HandlerFunc[command.Reset, domain.Attempt](func(_ context.Context, c command.Reset) (domain.Attempt, error) {
						intervene("reset", c.Request)
						return domain.Attempt{}, domain.ErrNotFound
					}),
				},
			}
			students := cqrs.HandlerFunc[identityquery.StudentAccount, identitydomain.Account](func(_ context.Context, q identityquery.StudentAccount) (identitydomain.Account, error) {
				return identitydomain.Account{ID: q.ID}, nil
			})
			h := attemptshttp.NewAttempts(app, nil, students, nil)
			ctx := reachContext(t, principal)
			id := uuid.New()
			note := "ghi chú"
			for op, call := range map[string]func() (any, error){
				"monitor": func() (any, error) {
					return h.GetAssignmentMonitor(ctx, openapi.GetAssignmentMonitorRequestObject{Id: id})
				},
				"review": func() (any, error) {
					return h.GetAttemptForReview(ctx, openapi.GetAttemptForReviewRequestObject{Id: id})
				},
				"events": func() (any, error) { return h.GetAttemptEvents(ctx, openapi.GetAttemptEventsRequestObject{Id: id}) },
				"byQuestion": func() (any, error) {
					return h.ListAnswersForQuestion(ctx, openapi.ListAnswersForQuestionRequestObject{Id: id})
				},
				"note": func() (any, error) {
					return h.SetAttemptNote(ctx, openapi.SetAttemptNoteRequestObject{Id: id, Body: &openapi.SetAttemptNoteJSONRequestBody{Note: &note}})
				},
				"grade": func() (any, error) {
					return h.GradeAttempt(ctx, openapi.GradeAttemptRequestObject{Id: id, Body: &openapi.GradeAttemptJSONRequestBody{}})
				},
				"finish": func() (any, error) { return h.FinishGrading(ctx, openapi.FinishGradingRequestObject{Id: id}) },
				"extend": func() (any, error) {
					return h.ExtendAttempt(ctx, openapi.ExtendAttemptRequestObject{Id: id, Body: &openapi.ExtendAttemptJSONRequestBody{Minutes: 5, Reason: "thêm"}})
				},
				"flag": func() (any, error) {
					return h.FlagAttempt(ctx, openapi.FlagAttemptRequestObject{Id: id, Body: &openapi.FlagAttemptJSONRequestBody{Flagged: true}})
				},
				"void": func() (any, error) {
					return h.VoidAttempt(ctx, openapi.VoidAttemptRequestObject{Id: id, Body: &openapi.VoidAttemptJSONRequestBody{Reason: "huỷ"}})
				},
				"reset": func() (any, error) {
					return h.ResetAttempt(ctx, openapi.ResetAttemptRequestObject{Id: id, Body: &openapi.ResetAttemptJSONRequestBody{Reason: "làm lại"}})
				},
			} {
				if _, err := call(); err != nil {
					t.Fatalf("%s: %v", op, err)
				}
			}
			for op, count := range map[string]int{"monitor": 1, "expire": 1, "review": 2, "timeline": 2, "byQuestion": 1, "note": 1, "grade": 1, "finish": 1, "extend": 1, "flag": 1, "void": 1, "reset": 1} {
				if got := scopes[op]; len(got) != count {
					t.Errorf("%s ran %d times, want %d", op, len(got), count)
				}
				for _, scope := range scopes[op] {
					if scope != want {
						t.Errorf("%s ran in %+v, want %+v", op, scope, want)
					}
				}
			}
		})
	}
}

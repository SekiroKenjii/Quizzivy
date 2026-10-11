package router_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"quizzivy/internal/core/router"
	attemptsapp "quizzivy/internal/modules/attempts/application"
	attemptsquery "quizzivy/internal/modules/attempts/application/query"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	identityquery "quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/shared/cqrs"
	"testing"
	"time"

	attemptsdomain "quizzivy/internal/modules/attempts/domain"
	identitydomain "quizzivy/internal/modules/identity/domain"
)

type fakeReview struct{ rv attemptsdomain.Review }

func (f fakeReview) app() *attemptsapp.Application {
	review := func(context.Context, attemptsquery.Review) (attemptsdomain.Review, error) { return f.rv, nil }
	timeline := func(context.Context, attemptsquery.Timeline) (attemptsdomain.Timeline, error) {
		return attemptsdomain.Timeline{}, nil
	}
	return &attemptsapp.Application{Queries: attemptsapp.Queries{
		Review:   cqrs.HandlerFunc[attemptsquery.Review, attemptsdomain.Review](review),
		Timeline: cqrs.HandlerFunc[attemptsquery.Timeline, attemptsdomain.Timeline](timeline),
	}}
}

type fakeStudents struct{ account identitydomain.Account }

func (f fakeStudents) handler() attemptshttp.Students {
	get := func(context.Context, identityquery.StudentAccount) (identitydomain.Account, error) {
		return f.account, nil
	}
	return cqrs.HandlerFunc[identityquery.StudentAccount, identitydomain.Account](get)
}

type reviewedSitter struct {
	FullName string  `json:"fullName"`
	Role     *string `json:"role"`
}

func reviewWith(t *testing.T, account identitydomain.Account) (reviewedSitter, string) {
	t.Helper()
	at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	issuer := testIssuer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	review := fakeReview{rv: attemptsdomain.Review{
		Attempt: attemptsdomain.Attempt{
			ID: "01935000-0000-7000-8000-00000000dd07", StudentID: account.ID,
			Status: attemptsdomain.Submitted, StartedAt: at, DeadlineAt: at.Add(time.Hour),
		},
		TestTitle: "Unit 5", MaxAttempts: 1,
		Answers: map[string]attemptsdomain.ReviewAnswer{}, AudioPlays: map[string]int{},
	}}
	handler, err := router.New(router.Deps{
		Principals: newFakePrincipals(),
		DB:         fakeDB{},
		Modules:    router.Modules{Attempts: attemptshttp.NewAttempts(review.app(), nil, fakeStudents{account: account}.handler(), nil)},
		Tokens:     issuer,
	}, logger, []string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatalf("core.NewRouter: %v", err)
	}
	token, err := issuer.Issue("01935000-0000-7000-8000-0000000000a1", 0)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/teacher/attempts/01935000-0000-7000-8000-00000000dd07", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Student   reviewedSitter `json:"student"`
		TestTitle string         `json:"testTitle"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Student, body.TestTitle
}

// A disabled account is refused a session, but its papers are still the
// teacher's to read: the review must not go through the session's user read.
func TestAReviewOpensADisabledStudentsPaper(t *testing.T) {
	sitter, title := reviewWith(t, identitydomain.Account{
		ID: "01935000-0000-7000-8000-0000000000a2", Email: "an@example.com", FullName: "Nguyễn Văn An",
		HasPassword: true, CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	})
	if sitter.FullName != "Nguyễn Văn An" || sitter.Role != nil || title != "Unit 5" {
		t.Errorf("sitter = %+v, title = %q", sitter, title)
	}
}

func TestAReviewOpensAStaffSittersPaper(t *testing.T) {
	sitter, title := reviewWith(t, identitydomain.Account{
		ID: "01935000-0000-7000-8000-0000000000a3", Email: "admin@example.com", FullName: "Quản trị",
		HasPassword: true, CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	})
	if sitter.FullName != "Quản trị" || sitter.Role != nil || title != "Unit 5" {
		t.Errorf("sitter = %+v, title = %q", sitter, title)
	}
}

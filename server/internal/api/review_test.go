package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"testing"
	"time"

	attemptsdomain "quizzivy/internal/modules/attempts/domain"
	identitydomain "quizzivy/internal/modules/identity/domain"
)

type fakeReview struct{ rv attemptsdomain.Review }

func (f fakeReview) Get(context.Context, string) (attemptsdomain.Review, error) { return f.rv, nil }
func (f fakeReview) Grade(context.Context, string, string, []attemptsdomain.GradeItem) (attemptsdomain.Score, error) {
	return attemptsdomain.Score{}, nil
}
func (f fakeReview) SetNote(context.Context, string, *string) error { return nil }
func (f fakeReview) AnswersForQuestion(context.Context, string, string) (attemptsdomain.ByQuestion, error) {
	return attemptsdomain.ByQuestion{}, nil
}
func (f fakeReview) Finish(context.Context, string) (attemptsdomain.Attempt, error) {
	return attemptsdomain.Attempt{}, nil
}

type fakeStudents struct{ student identitydomain.Student }

func (f fakeStudents) Get(context.Context, string) (identitydomain.Student, error) {
	return f.student, nil
}

type fakeIntegrity struct{}

func (fakeIntegrity) Timeline(context.Context, string) (attemptsdomain.Timeline, error) {
	return attemptsdomain.Timeline{}, nil
}

// A disabled account is refused a session, but its papers are still the
// teacher's to read: the review must not go through the session's user read.
func TestAReviewOpensADisabledStudentsPaper(t *testing.T) {
	const studentID = "01935000-0000-7000-8000-0000000000a2"
	disabledAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	issuer := testIssuer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	review := fakeReview{rv: attemptsdomain.Review{
		Attempt: attemptsdomain.Attempt{
			ID: "01935000-0000-7000-8000-00000000dd07", StudentID: studentID,
			Status: attemptsdomain.Submitted, StartedAt: disabledAt, DeadlineAt: disabledAt.Add(time.Hour),
		},
		TestTitle: "Unit 5", MaxAttempts: 1,
		Answers: map[string]attemptsdomain.ReviewAnswer{}, AudioPlays: map[string]int{},
	}}
	students := fakeStudents{student: identitydomain.Student{
		ID: studentID, Email: "an@example.com", FullName: "Nguyễn Văn An",
		HasPassword: true, CreatedAt: disabledAt, DisabledAt: &disabledAt,
	}}
	router, err := NewRouter(Deps{
		DB:      fakeDB{},
		Modules: Modules{Attempts: attemptshttp.NewAttempts(nil, review, fakeIntegrity{}, nil, students, nil)},
		Tokens:  issuer,
	}, logger, []string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	token, err := issuer.Issue("01935000-0000-7000-8000-0000000000a1", "admin")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/attempts/01935000-0000-7000-8000-00000000dd07", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Student struct {
			FullName string `json:"fullName"`
			Role     string `json:"role"`
		} `json:"student"`
		TestTitle string `json:"testTitle"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Student.FullName != "Nguyễn Văn An" || body.Student.Role != "student" || body.TestTitle != "Unit 5" {
		t.Errorf("body = %+v", body)
	}
}

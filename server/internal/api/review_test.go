package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quizzivy/internal/attempts"
	"quizzivy/internal/integrity"
	"quizzivy/internal/paging"
	"quizzivy/internal/review"
	"quizzivy/internal/students"
)

type fakeReview struct{ rv review.Review }

func (f fakeReview) Get(context.Context, string) (review.Review, error) { return f.rv, nil }
func (f fakeReview) Grade(context.Context, string, string, []review.Item) (attempts.Score, error) {
	return attempts.Score{}, nil
}
func (f fakeReview) SetNote(context.Context, string, *string) error { return nil }
func (f fakeReview) Finish(context.Context, string) (attempts.Attempt, error) {
	return attempts.Attempt{}, nil
}

type fakeStudents struct{ student students.Student }

func (f fakeStudents) List(context.Context, students.ListInput) ([]students.Student, paging.Page, error) {
	return nil, paging.Page{}, nil
}
func (f fakeStudents) Facets(context.Context, students.ListInput) (students.Facets, error) {
	return students.Facets{}, nil
}
func (f fakeStudents) Get(context.Context, string) (students.Student, error) { return f.student, nil }
func (f fakeStudents) Create(context.Context, students.Request, students.CreateInput) (students.Student, error) {
	return f.student, nil
}
func (f fakeStudents) Update(context.Context, students.Request, students.UpdateInput) (students.Student, error) {
	return f.student, nil
}
func (f fakeStudents) ResetPassword(context.Context, students.Request, string, string, time.Time) error {
	return nil
}

type fakeIntegrity struct{}

func (fakeIntegrity) Timeline(context.Context, string) (integrity.Timeline, error) {
	return integrity.Timeline{}, nil
}

// A disabled account is refused a session, but its papers are still the
// teacher's to read: the review must not go through the session's user read.
func TestAReviewOpensADisabledStudentsPaper(t *testing.T) {
	const studentID = "01935000-0000-7000-8000-0000000000a2"
	disabledAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	issuer := testIssuer(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router, err := NewRouter(Deps{
		DB: fakeDB{},
		Review: fakeReview{rv: review.Review{
			Attempt: attempts.Attempt{
				ID: "01935000-0000-7000-8000-00000000dd07", StudentID: studentID,
				Status: attempts.Submitted, StartedAt: disabledAt, DeadlineAt: disabledAt.Add(time.Hour),
			},
			TestTitle: "Unit 5", MaxAttempts: 1,
			Answers: map[string]review.Answer{}, AudioPlays: map[string]int{},
		}},
		Students: fakeStudents{student: students.Student{
			ID: studentID, Email: "an@example.com", FullName: "Nguyễn Văn An",
			HasPassword: true, CreatedAt: disabledAt, DisabledAt: &disabledAt,
		}},
		Integrity: fakeIntegrity{},
		Tokens:    issuer,
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

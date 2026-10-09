package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/cqrs"

	"github.com/google/uuid"
)

func altQuestions(section, withAlt, without, asset uuid.UUID, alt string) []domain.Question {
	image := &domain.Media{ID: asset.String(), Kind: "image", MimeType: "image/png", Filename: "meo.png", Bytes: 100}
	return []domain.Question{
		{ID: withAlt.String(), SectionID: section.String(), Type: "short_answer", Prompt: "Con gì?", Media: image, MediaAlt: &alt},
		{ID: without.String(), SectionID: section.String(), Type: "short_answer", Prompt: "Con gì nữa?", Media: image},
	}
}

func serveAs(t *testing.T, student uuid.UUID, serve func(ctx context.Context, w http.ResponseWriter) error) []map[string]any {
	t.Helper()
	var failure error
	handler := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
		return httpx.Principal{UserID: student.String()}, nil
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { failure = serve(r.Context(), w) }))
	request := httptest.NewRequest(http.MethodGet, "/app/attempts/x", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if failure != nil {
		t.Fatal(failure)
	}
	for _, key := range []string{"isCorrect", "acceptedAnswers", "sampleAnswer"} {
		if strings.Contains(response.Body.String(), `"`+key+`"`) {
			t.Fatalf("payload leaked %s", key)
		}
	}
	var body struct {
		Questions []map[string]any `json:"questions"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("not JSON: %s", response.Body.String())
	}
	return body.Questions
}

func checkAlt(t *testing.T, questions []map[string]any, alt string) {
	t.Helper()
	if len(questions) != 2 {
		t.Fatalf("questions = %v, want two", questions)
	}
	if questions[0]["mediaAlt"] != alt {
		t.Fatalf("first question = %v, want mediaAlt %q beside its media", questions[0], alt)
	}
	if questions[0]["media"] == nil {
		t.Fatalf("first question lost its media: %v", questions[0])
	}
	if _, present := questions[1]["mediaAlt"]; present {
		t.Fatalf("a question with no alt text carries mediaAlt: %v", questions[1])
	}
}

func TestTheStudentsQuestionCarriesTheFrozenAltTextBesideItsImage(t *testing.T) {
	student, attempt, section, withAlt, without, asset := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	alt := "Một chú mèo ngồi trên ghế"
	session := domain.Session{Attempt: domain.Attempt{ID: attempt.String()}, SessionID: uuid.NewString(), Questions: altQuestions(section, withAlt, without, asset, alt)}
	app := &application.Application{Queries: application.Queries{Get: cqrs.HandlerFunc[query.Get, domain.Session](func(context.Context, query.Get) (domain.Session, error) {
		return session, nil
	})}}
	transport := attemptshttp.NewAttempts(app, &groupMedia{student: student.String(), asset: asset.String()}, nil, nil)

	questions := serveAs(t, student, func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.GetAttempt(ctx, openapi.GetAttemptRequestObject{Id: attempt})
		if err != nil {
			return err
		}
		return response.VisitGetAttemptResponse(w)
	})
	checkAlt(t, questions, alt)
}

func TestTheResultQuestionCarriesTheFrozenAltTextBesideItsImage(t *testing.T) {
	student, attempt, section, withAlt, without, asset := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	alt := "Một chú mèo ngồi trên ghế"
	var results []domain.ResultQuestion
	for _, q := range altQuestions(section, withAlt, without, asset, alt) {
		results = append(results, domain.ResultQuestion{Question: q})
	}
	app := &application.Application{Queries: application.Queries{Result: cqrs.HandlerFunc[query.Result, domain.Result](func(context.Context, query.Result) (domain.Result, error) {
		return domain.Result{Attempt: domain.Attempt{ID: attempt.String()}, Questions: results}, nil
	})}}
	transport := attemptshttp.NewAttempts(app, &groupMedia{student: student.String(), asset: asset.String()}, nil, nil)

	questions := serveAs(t, student, func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.GetAttemptResult(ctx, openapi.GetAttemptResultRequestObject{Id: attempt})
		if err != nil {
			return err
		}
		return response.VisitGetAttemptResultResponse(w)
	})
	checkAlt(t, questions, alt)
}

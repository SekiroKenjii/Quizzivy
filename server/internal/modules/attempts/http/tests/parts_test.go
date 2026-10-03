package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestThePapersPartsReachTheSessionAndTheResultAsAuthored(t *testing.T) {
	student, attempt, first, second, question := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	guide := "Đọc đoạn văn rồi trả lời."
	parts := []domain.Section{{ID: first.String(), Title: "Phần 1"}, {ID: second.String(), Title: "Phần 2", Instructions: &guide}}
	asked := domain.Question{ID: question.String(), SectionID: second.String(), Type: "short_answer", Prompt: "Viết một câu."}
	app := &application.Application{Queries: application.Queries{
		Get: cqrs.HandlerFunc[query.Get, domain.Session](func(context.Context, query.Get) (domain.Session, error) {
			return domain.Session{Attempt: domain.Attempt{ID: attempt.String()}, SessionID: uuid.NewString(), Sections: parts, Questions: []domain.Question{asked}}, nil
		}),
		Result: cqrs.HandlerFunc[query.Result, domain.Result](func(context.Context, query.Result) (domain.Result, error) {
			return domain.Result{Attempt: domain.Attempt{ID: attempt.String()}, Sections: parts, Questions: []domain.ResultQuestion{{Question: asked}}}, nil
		}),
	}}
	transport := attemptshttp.NewAttempts(app, nil, nil, nil)

	for name, visit := range map[string]func(context.Context, http.ResponseWriter) error{
		"session": func(ctx context.Context, w http.ResponseWriter) error {
			response, err := transport.GetAttempt(ctx, openapi.GetAttemptRequestObject{Id: attempt})
			if err != nil {
				return err
			}
			return response.VisitGetAttemptResponse(w)
		},
		"result": func(ctx context.Context, w http.ResponseWriter) error {
			response, err := transport.GetAttemptResult(ctx, openapi.GetAttemptResultRequestObject{Id: attempt})
			if err != nil {
				return err
			}
			return response.VisitGetAttemptResultResponse(w)
		},
	} {
		t.Run(name, func(t *testing.T) {
			var failure error
			handler := httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
				return httpx.Principal{UserID: student.String()}, nil
			})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { failure = visit(r.Context(), w) }))
			request := httptest.NewRequest(http.MethodGet, "/app/attempts/"+attempt.String(), nil)
			request.Header.Set("Authorization", "Bearer fixture")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if failure != nil {
				t.Fatal(failure)
			}

			var body struct {
				Sections []struct {
					ID           string  `json:"id"`
					Title        string  `json:"title"`
					Instructions *string `json:"instructions"`
				} `json:"sections"`
				Questions []struct {
					SectionID string `json:"sectionId"`
				} `json:"questions"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Sections) != 2 || len(body.Questions) != 1 {
				t.Fatalf("%d parts and %d questions, want 2 and 1: %s", len(body.Sections), len(body.Questions), response.Body.String())
			}
			if got := body.Sections[0]; got.ID != first.String() || got.Title != "Phần 1" || got.Instructions != nil {
				t.Errorf("first part is %+v", got)
			}
			if got := body.Sections[1]; got.ID != second.String() || got.Title != "Phần 2" || got.Instructions == nil || *got.Instructions != guide {
				t.Errorf("second part is %+v", got)
			}
			if body.Questions[0].SectionID != second.String() {
				t.Errorf("the question is in %s, want the second part %s", body.Questions[0].SectionID, second)
			}
		})
	}
}

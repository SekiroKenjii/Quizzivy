package http_test

import (
	"context"
	"net/http"
	"testing"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"quizzivy/internal/shared/cqrs"

	"github.com/google/uuid"
)

func TestTheResultQuestionCarriesTheChooseNRuleTheSummaryCountsBy(t *testing.T) {
	student, attempt, section := uuid.New(), uuid.New(), uuid.New()
	two := 2
	results := []domain.ResultQuestion{
		{Question: domain.Question{ID: uuid.NewString(), SectionID: section.String(), Type: "multiple_choice", Prompt: "Chọn hai.", SelectCount: &two}},
		{Question: domain.Question{ID: uuid.NewString(), SectionID: section.String(), Type: "multiple_choice", Prompt: "Chọn tất cả."}},
	}
	app := &application.Application{Queries: application.Queries{Result: cqrs.HandlerFunc[query.Result, domain.Result](func(context.Context, query.Result) (domain.Result, error) {
		return domain.Result{Attempt: domain.Attempt{ID: attempt.String()}, Questions: results}, nil
	})}}
	transport := attemptshttp.NewAttempts(app, &groupMedia{student: student.String()}, nil, nil)

	questions := serveAs(t, student, func(ctx context.Context, w http.ResponseWriter) error {
		response, err := transport.GetAttemptResult(ctx, openapi.GetAttemptResultRequestObject{Id: attempt})
		if err != nil {
			return err
		}
		return response.VisitGetAttemptResultResponse(w)
	})
	if got := questions[0]["selectCount"]; got != float64(2) {
		t.Errorf("choose two: selectCount = %v, want 2", got)
	}
	if _, present := questions[1]["selectCount"]; present {
		t.Errorf("a plain multiple choice carries selectCount: %v", questions[1])
	}
}

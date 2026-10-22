package http_test

import (
	"context"
	"testing"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	"quizzivy/internal/shared/cqrs"

	"github.com/google/uuid"
)

func byQuestionTransport(out domain.ByQuestion) attemptshttp.Attempts {
	app := &application.Application{Queries: application.Queries{AnswersForQuestion: cqrs.HandlerFunc[query.AnswersForQuestion, domain.ByQuestion](
		func(context.Context, query.AnswersForQuestion) (domain.ByQuestion, error) { return out, nil })}}
	return attemptshttp.NewAttempts(app, nil, nil, nil)
}

func answersFor(t *testing.T, q domain.ReviewQuestion, items ...domain.QuestionAnswer) openapi.ListAnswersForQuestion200JSONResponse {
	t.Helper()
	transport := byQuestionTransport(domain.ByQuestion{Question: q, PublishedAt: time.Now(), Number: 1, Count: 1, Items: items})
	response, err := transport.ListAnswersForQuestion(context.Background(), openapi.ListAnswersForQuestionRequestObject{
		Id: uuid.New(), Params: openapi.ListAnswersForQuestionParams{QuestionId: uuid.New()}})
	if err != nil {
		t.Fatal(err)
	}
	return response.(openapi.ListAnswersForQuestion200JSONResponse)
}

func TestTheByQuestionReadCarriesAMatchingQuestionsItemsKeysAndRules(t *testing.T) {
	question, first, second, item := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	reuse := true
	body := answersFor(t, domain.ReviewQuestion{
		ID: question, Type: "matching", Prompt: "Match", Points: 2, MatchAllowReuse: &reuse,
		Options:    []domain.ReviewOption{{ID: first, Key: "A", Ordinal: 0, Text: "One"}, {ID: second, Key: "B", Ordinal: 1, Text: "Two"}},
		MatchItems: []domain.ReviewMatchItem{{ID: item, Key: "i", Text: "Item", CorrectOptionID: second, Ordinal: 0}},
	})
	q := body.Question
	if q.MatchItems == nil || len(*q.MatchItems) != 1 || (*q.MatchItems)[0].CorrectOptionId.String() != second || (*q.MatchItems)[0].Key != "i" {
		t.Errorf("match items %+v, want the item keyed to the second option", q.MatchItems)
	}
	if q.Options == nil || (*q.Options)[1].Key == nil || *(*q.Options)[1].Key != "B" {
		t.Errorf("options %+v, want their keys", q.Options)
	}
	if q.MatchAllowReuse == nil || !*q.MatchAllowReuse || q.Rubric != nil {
		t.Errorf("matchAllowReuse %v rubric %v", q.MatchAllowReuse, q.Rubric)
	}
}

func TestTheByQuestionReadCarriesAnEssaysRubricBandAndCriteria(t *testing.T) {
	criterion, n := uuid.NewString(), 2
	band := 6.5
	body := answersFor(t,
		domain.ReviewQuestion{ID: uuid.NewString(), Type: "essay", Prompt: "Write", Points: 9, MaxWords: &n,
			Rubric: []domain.RubricCriterion{{ID: criterion, Label: "Task", Ordinal: 0}}},
		domain.QuestionAnswer{AttemptID: uuid.NewString(), StudentID: uuid.NewString(), StudentName: "An", AttemptNo: 1,
			Payload: []byte(`{"type":"text","value":"An essay."}`), Band: &band, Criteria: []domain.CriterionBand{{CriterionID: criterion, Band: 6.5}}})
	if body.Question.Rubric == nil || len(*body.Question.Rubric) != 1 || (*body.Question.Rubric)[0].Id.String() != criterion {
		t.Errorf("rubric %+v", body.Question.Rubric)
	}
	if body.Question.MaxWords == nil || *body.Question.MaxWords != 2 || body.Question.MatchItems != nil {
		t.Errorf("maxWords %v matchItems %v", body.Question.MaxWords, body.Question.MatchItems)
	}
	row := body.Items[0]
	if row.Band == nil || *row.Band != 6.5 || row.Criteria == nil || len(*row.Criteria) != 1 || (*row.Criteria)[0].CriterionId.String() != criterion {
		t.Errorf("row band %v criteria %+v", row.Band, row.Criteria)
	}
}

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/model"
	"quizzivy/internal/modules/tests/domain"
	"testing"
	"time"
)

func TestMoveQuestionWithoutGroupAuthoringIsUnavailable(t *testing.T) {
	app := application.New(nil)

	_, err := app.Commands.MoveQuestion.Handle(context.Background(), command.MoveQuestion{Move: model.QuestionMove{
		TestID: "019535d9-3df7-79fb-b466-000000000001", QuestionID: "019535d9-3df7-79fb-b466-000000000002", ExpectedTestUpdatedAt: time.Now(),
	}})

	if !errors.Is(err, domain.ErrGroupUnavailable) {
		t.Fatalf("got %v, want the explicit unavailable answer a 501 is made from", err)
	}
}

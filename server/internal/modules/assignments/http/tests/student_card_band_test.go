package http_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/assignments/application"
	"quizzivy/internal/modules/assignments/application/query"
	"quizzivy/internal/modules/assignments/domain"
	assignmentshttp "quizzivy/internal/modules/assignments/http"
	"quizzivy/internal/shared/cqrs"
)

func TestACompletedCardCarriesThePapersBand(t *testing.T) {
	band := 6.5
	transport := assignmentshttp.NewAssignments(&application.Application{Queries: application.Queries{
		ForStudent: cqrs.HandlerFunc[query.ForStudent, domain.StudentSections](func(context.Context, query.ForStudent) (domain.StudentSections, error) {
			return domain.StudentSections{Completed: []domain.StudentCard{
				{ID: uuid.NewString(), AttemptsUsed: 1, Score: &domain.Score{Earned: 6.5, Total: 9, Band: &band}},
				{ID: uuid.NewString(), AttemptsUsed: 1, Score: &domain.Score{Earned: 1, Total: 2}},
			}}, nil
		}),
	}})
	var response openapi.ListMyAssignmentsResponseObject
	refusedIn(t, "", func(ctx context.Context, _ http.ResponseWriter) error {
		var err error
		response, err = transport.ListMyAssignments(ctx, openapi.ListMyAssignmentsRequestObject{})
		return err
	})
	cards := response.(openapi.ListMyAssignments200JSONResponse).Completed
	if len(cards) != 2 {
		t.Fatalf("cards %+v, want two", cards)
	}
	if cards[0].Score == nil || cards[0].Score.Band == nil || *cards[0].Score.Band != 6.5 {
		t.Errorf("banded card score %+v, want band 6.5", cards[0].Score)
	}
	if cards[1].Score == nil || cards[1].Score.Band != nil {
		t.Errorf("a paper in points carries a band: %+v", cards[1].Score)
	}
}

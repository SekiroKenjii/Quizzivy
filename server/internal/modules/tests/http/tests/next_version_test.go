package http_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"testing"

	"github.com/google/uuid"
)

func TestATestReadCarriesTheNextVersionApartFromTheCurrentOne(t *testing.T) {
	cases := []struct {
		name          string
		current, next int
	}{
		{"never published", 0, 1},
		{"the newest version is the default", 3, 4},
		{"the newest version was deleted", 1, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := uuid.New()
			app := &application.Application{Queries: application.Queries{Get: cqrs.HandlerFunc[query.Get, domain.Test](func(context.Context, query.Get) (domain.Test, error) {
				return domain.Test{ID: id.String(), TotalPoints: "1", CurrentVersion: c.current, NextVersion: c.next}, nil
			})}}
			who := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.ContentTestsWrite)}
			response, err := testshttp.NewTests(app, nil).GetTest(contextAs(t, who), openapi.GetTestRequestObject{Id: id})
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			if err := response.VisitGetTestResponse(rec); err != nil {
				t.Fatal(err)
			}
			var body struct {
				CurrentVersion *int `json:"currentVersion"`
				NextVersion    *int `json:"nextVersion"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.NextVersion == nil || *body.NextVersion != c.next {
				t.Fatalf("nextVersion in %s, want %d", rec.Body.String(), c.next)
			}
			if body.CurrentVersion == nil || *body.CurrentVersion != c.current {
				t.Fatalf("currentVersion in %s, want %d", rec.Body.String(), c.current)
			}
		})
	}
}

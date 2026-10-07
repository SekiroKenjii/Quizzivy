package http_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"testing"
)

func TestTestReadSerializesDraftSkillsAsArrayIncludingEmpty(t *testing.T) {
	for _, skills := range [][]string{nil, {}, {"grammar", "reading"}} {
		id := uuid.New()
		app := &application.Application{Queries: application.Queries{Get: cqrs.HandlerFunc[query.Get, domain.Test](func(context.Context, query.Get) (domain.Test, error) {
			return domain.Test{ID: id.String(), TotalPoints: "1", Skills: skills}, nil
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
		var body map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		var got []string
		if err := json.Unmarshal(body["skills"], &got); err != nil {
			t.Fatal(err)
		}
		if got == nil || len(got) != len(skills) {
			t.Fatalf("skills array lost: %s", rec.Body.String())
		}
		for i, value := range skills {
			if got[i] != value {
				t.Fatalf("skill changed: %v", got)
			}
		}
	}
}

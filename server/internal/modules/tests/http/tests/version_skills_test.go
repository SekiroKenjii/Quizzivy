package http_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	testshttp "quizzivy/internal/modules/tests/http"
	"quizzivy/internal/shared/cqrs"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

func skillsOf(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	if string(raw) == "null" || raw == nil {
		t.Fatalf("skills is %s, want an array", raw)
	}
	var got []string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestEveryVersionAnswerCarriesItsSkillsAsAnArrayIncludingAnEmptyOne(t *testing.T) {
	for _, skills := range [][]string{nil, {}, {"grammar", "reading"}} {
		version := domain.Version{ID: uuid.NewString(), Version: 1, TotalPoints: "1.00", Skills: skills, PublishedAt: time.Now(), PublishedBy: "Giáo viên"}
		app := &application.Application{
			Commands: application.Commands{Publish: cqrs.HandlerFunc[command.Publish, domain.Version](func(context.Context, command.Publish) (domain.Version, error) {
				return version, nil
			})},
			Queries: application.Queries{ListVersions: cqrs.HandlerFunc[query.ListVersions, []domain.Version](func(context.Context, query.ListVersions) ([]domain.Version, error) {
				return []domain.Version{version}, nil
			})},
		}
		transport := testshttp.NewTests(app, nil)
		ctx := contextAs(t, publishPrincipal())

		published, err := transport.PublishTest(ctx, openapi.PublishTestRequestObject{Id: uuid.New()})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		if err := published.VisitPublishTestResponse(rec); err != nil {
			t.Fatal(err)
		}
		var one map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &one); err != nil {
			t.Fatal(err)
		}
		if got := skillsOf(t, one["skills"]); !slices.Equal(got, append([]string{}, skills...)) {
			t.Errorf("publishTest answered skills %v for %v", got, skills)
		}

		listed, err := transport.ListTestVersions(ctx, openapi.ListTestVersionsRequestObject{Id: uuid.New()})
		if err != nil {
			t.Fatal(err)
		}
		rec = httptest.NewRecorder()
		if err := listed.VisitListTestVersionsResponse(rec); err != nil {
			t.Fatal(err)
		}
		var many struct {
			Items []map[string]json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &many); err != nil {
			t.Fatal(err)
		}
		if len(many.Items) != 1 {
			t.Fatalf("%d items, want 1", len(many.Items))
		}
		if got := skillsOf(t, many.Items[0]["skills"]); !slices.Equal(got, append([]string{}, skills...)) {
			t.Errorf("listTestVersions answered skills %v for %v", got, skills)
		}
	}
}

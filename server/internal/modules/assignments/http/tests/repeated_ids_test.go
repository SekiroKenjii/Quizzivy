package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/assignments/application"
	"quizzivy/internal/modules/assignments/application/command"
	"quizzivy/internal/modules/assignments/application/query"
	"quizzivy/internal/modules/assignments/domain"
	assignmentshttp "quizzivy/internal/modules/assignments/http"
	"quizzivy/internal/shared/cqrs"
)

func TestAnIdNamedTwiceIsRefusedBeforeAnythingRunsAndTwoDifferentOnesAreNot(t *testing.T) {
	var reached []string
	app := &application.Application{
		Commands: application.Commands{
			Duplicate: cqrs.HandlerFunc[command.Duplicate, domain.Assignment](func(context.Context, command.Duplicate) (domain.Assignment, error) {
				reached = append(reached, "duplicate")
				return domain.Assignment{}, nil
			}),
			SetOverrides: cqrs.HandlerFunc[command.SetOverrides, []domain.StudentOverride](func(context.Context, command.SetOverrides) ([]domain.StudentOverride, error) {
				reached = append(reached, "overrides")
				return nil, nil
			}),
		},
		Queries: application.Queries{
			List: cqrs.HandlerFunc[query.List, query.ListResult](func(context.Context, query.List) (query.ListResult, error) {
				reached = append(reached, "list")
				return query.ListResult{}, nil
			}),
			Facets: cqrs.HandlerFunc[query.Facets, domain.Facets](func(context.Context, query.Facets) (domain.Facets, error) {
				reached = append(reached, "facets")
				return domain.Facets{}, nil
			}),
		},
	}
	transport := assignmentshttp.NewAssignments(app)
	listing := func(ids ...openapi.Uuid) refusing {
		return func(ctx context.Context, w http.ResponseWriter) error {
			response, err := transport.ListAssignments(ctx, openapi.ListAssignmentsRequestObject{Params: openapi.ListAssignmentsParams{ClassId: &ids}})
			if err != nil {
				return err
			}
			return response.VisitListAssignmentsResponse(w)
		}
	}
	duplicating := func(ids ...openapi.Uuid) refusing {
		return func(ctx context.Context, w http.ResponseWriter) error {
			response, err := transport.DuplicateAssignment(ctx, openapi.DuplicateAssignmentRequestObject{
				Id: uuid.New(), Body: &openapi.DuplicateAssignmentJSONRequestBody{ClassIds: ids},
			})
			if err != nil {
				return err
			}
			return response.VisitDuplicateAssignmentResponse(w)
		}
	}
	overridingStudents := func(ids ...openapi.Uuid) refusing {
		return func(ctx context.Context, w http.ResponseWriter) error {
			response, err := transport.SetStudentOverrides(ctx, openapi.SetStudentOverridesRequestObject{
				Id: uuid.New(), Body: &openapi.SetStudentOverridesJSONRequestBody{StudentIds: ids, Reason: "Ốm"},
			})
			if err != nil {
				return err
			}
			return response.VisitSetStudentOverridesResponse(w)
		}
	}

	one, two := uuid.New(), uuid.New()
	for _, c := range []struct {
		name   string
		serve  func(...openapi.Uuid) refusing
		field  string
		vi, en string
		runs   []string
	}{
		{"listing by a class", listing, "classId", "Mỗi lớp chỉ được nêu một lần.", "Each class can be named only once.", []string{"list", "facets"}},
		{"copying into a class", duplicating, "classIds", "Mỗi lớp chỉ được nêu một lần.", "Each class can be named only once.", []string{"duplicate"}},
		{"overriding a student", overridingStudents, "studentIds", "Mỗi học viên chỉ được nêu một lần.", "Each student can be named only once.", []string{"overrides"}},
	} {
		for _, language := range []struct{ accept, message string }{{"", c.vi}, {"en", c.en}} {
			reached = nil
			response := refusedIn(t, language.accept, c.serve(one, two, one))
			var body openapi.ErrorResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("%s with Accept-Language %q: %v in %s", c.name, language.accept, err, response.Body.String())
			}
			if response.Code != http.StatusBadRequest || body.Error.Code != openapi.VALIDATIONFAILED || body.Error.Message != language.message {
				t.Errorf("%s with Accept-Language %q answered %d %s %q, want 400 VALIDATION_FAILED %q",
					c.name, language.accept, response.Code, body.Error.Code, body.Error.Message, language.message)
			}
			if body.Error.Details == nil || (*body.Error.Details)[c.field] != language.message {
				t.Errorf("%s with Accept-Language %q carries details %v, want %s named", c.name, language.accept, body.Error.Details, c.field)
			}
			if len(reached) != 0 {
				t.Errorf("%s reached %v for a request it refuses", c.name, reached)
			}
		}

		reached = nil
		if response := refusedIn(t, "", c.serve(one, two)); response.Code == http.StatusBadRequest {
			t.Errorf("%s with two different ids answered 400: %s", c.name, response.Body.String())
		}
		if len(reached) != len(c.runs) {
			t.Errorf("%s with two different ids reached %v, want %v", c.name, reached, c.runs)
		}
	}
}

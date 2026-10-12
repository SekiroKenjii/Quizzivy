package http_test

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/dashboard/application"
	"quizzivy/internal/modules/dashboard/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"reflect"
	"testing"
)

func TestDashboardDestinationsUseEitherReviewPermissionAndOwnEvenForAdmins(t *testing.T) {
	for _, test := range []struct {
		name    string
		keys    []access.Key
		allowed bool
	}{
		{"workspace only", []access.Key{access.ContentTestsWrite}, false},
		{"grading", []access.Key{access.TeachingGrading}, true},
		{"intervention", []access.Key{access.TeachingAttemptsIntervene}, true},
		{"both", []access.Key{access.TeachingGrading, access.TeachingAttemptsIntervene}, true},
		{"scope all without review", []access.Key{access.ContentTestsWrite, access.ScopeAll}, false},
		{"admin", access.All(), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(test.keys...)}
			ctx := gated(t, principal, "GET /teacher/dashboard")
			assignment, attempt := uuid.NewString(), uuid.NewString()
			var seen query.Summary
			app := &application.Application{Queries: application.Queries{Summary: cqrs.HandlerFunc[query.Summary, domain.Summary](func(_ context.Context, q query.Summary) (domain.Summary, error) {
				seen = q
				out := domain.Summary{FlaggedAttempts: 3, Home: domain.Home{TakingNow: domain.TakingNow{Students: 2, Assignments: 1, AssignmentID: &assignment}}}
				if q.CanReviewFlagged {
					out.NewestFlaggedAttempt = &domain.FlaggedAttempt{AssignmentID: assignment, AttemptID: attempt}
				}
				return out, nil
			})}}
			response, err := dashboardhttp.NewDashboard(app).GetDashboard(ctx, openapi.GetDashboardRequestObject{})
			if err != nil {
				t.Fatal(err)
			}
			if seen.Scope != (access.Scope{UserID: principal.UserID}) || seen.CanReviewFlagged != test.allowed || seen.Range != "14d" {
				t.Fatalf("query=%+v", seen)
			}
			out := response.(openapi.GetDashboard200JSONResponse)
			if out.FlaggedAttempts != 3 || out.TakingNow.Students != 2 || out.TakingNow.Assignments != 1 || out.TakingNow.AssignmentId == nil || out.TakingNow.AssignmentId.String() != assignment {
				t.Fatalf("output=%+v", out)
			}
			body, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			var value map[string]json.RawMessage
			if err = json.Unmarshal(body, &value); err != nil {
				t.Fatal(err)
			}
			raw, exists := value["newestFlaggedAttempt"]
			if !exists {
				t.Fatal("required destination absent")
			}
			if test.allowed {
				var pair map[string]string
				if err = json.Unmarshal(raw, &pair); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(pair, map[string]string{"assignmentId": assignment, "attemptId": attempt}) {
					t.Fatalf("pair=%v", pair)
				}
			} else if string(raw) != "null" {
				t.Fatalf("denied=%s", raw)
			}
		})
	}
}

func TestDashboardEmptyDestinationsSerializeRequiredNulls(t *testing.T) {
	principal := access.Principal{UserID: uuid.NewString(), Permissions: access.NewSet(access.TeachingGrading)}
	app := &application.Application{Queries: application.Queries{Summary: cqrs.HandlerFunc[query.Summary, domain.Summary](func(context.Context, query.Summary) (domain.Summary, error) { return domain.Summary{}, nil })}}
	response, err := dashboardhttp.NewDashboard(app).GetDashboard(gated(t, principal, "GET /teacher/dashboard"), openapi.GetDashboardRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]json.RawMessage
	if err = json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if string(out["newestFlaggedAttempt"]) != "null" {
		t.Fatalf("flagged=%s", out["newestFlaggedAttempt"])
	}
	var taking map[string]json.RawMessage
	if err = json.Unmarshal(out["takingNow"], &taking); err != nil {
		t.Fatal(err)
	}
	if len(taking) != 3 || string(taking["assignmentId"]) != "null" || string(taking["students"]) != "0" || string(taking["assignments"]) != "0" {
		t.Fatalf("taking=%s", out["takingNow"])
	}
}

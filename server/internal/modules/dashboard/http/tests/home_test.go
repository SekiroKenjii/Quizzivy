package http_test

import (
	"context"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/dashboard/application"
	"quizzivy/internal/modules/dashboard/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"testing"
	"time"
)

func TestDashboardRangeAndNewFieldsReachTheResponse(t *testing.T) {
	for _, value := range []string{"", "7d", "14d", "30d"} {
		t.Run(value, func(t *testing.T) {
			var got query.Summary
			now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
			average := 63
			app := &application.Application{Queries: application.Queries{Summary: cqrs.HandlerFunc[query.Summary, domain.Summary](func(_ context.Context, q query.Summary) (domain.Summary, error) {
				got = q
				return domain.Summary{Home: domain.Home{TakingNow: domain.TakingNow{Students: 2, Assignments: 3}, Submissions: domain.Submissions{Days: []domain.Day{{Date: "2026-10-05", Count: 4}}, Total: 4, AveragePercent: &average}, Today: []domain.Today{{Kind: "opens", At: now, AssignmentID: principals()["an Admin"].UserID, Title: "paper", NotSubmitted: 2}}, RecentActivity: []domain.Activity{{Kind: "started", At: now, StudentName: "student", Subject: "paper", Flagged: true}}}}, nil
			})}}
			req := openapi.GetDashboardRequestObject{}
			if value != "" {
				v := openapi.GetDashboardParamsRange(value)
				req.Params.Range = &v
			}
			resp, err := dashboardhttp.NewDashboard(app).GetDashboard(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			want := value
			if want == "" {
				want = "14d"
			}
			if got.Range != want {
				t.Fatalf("range=%q want%q", got.Range, want)
			}
			out := resp.(openapi.GetDashboard200JSONResponse)
			if out.TakingNow.Students != 2 || out.TakingNow.Assignments != 3 || len(out.Today) != 1 || out.Today[0].NotSubmitted != 2 || len(out.RecentActivity) != 1 || !out.RecentActivity[0].Flagged || out.Submissions.Total != 4 || out.Submissions.AveragePercent == nil || *out.Submissions.AveragePercent != 63 || out.Submissions.Days[0].Date.Format("2006-01-02") != "2026-10-05" {
				t.Fatalf("response=%+v", out)
			}
		})
	}
}

func TestNavAlwaysUsesOwnScopeAndResolvedGradingPermission(t *testing.T) {
	people := principals()
	people["without grading"] = access.Principal{UserID: people["a Teacher"].UserID, Permissions: access.NewSet(access.ContentTestsWrite)}
	for name, principal := range people {
		t.Run(name, func(t *testing.T) {
			var got query.Nav
			app := &application.Application{Queries: application.Queries{Nav: cqrs.HandlerFunc[query.Nav, domain.Nav](func(_ context.Context, q query.Nav) (domain.Nav, error) { got = q; return domain.Nav{}, nil })}}
			if _, err := dashboardhttp.NewDashboard(app).GetTeacherSummary(gated(t, principal, "GET /teacher/summary"), openapi.GetTeacherSummaryRequestObject{}); err != nil {
				t.Fatal(err)
			}
			if got.Scope != (access.Scope{UserID: principal.UserID}) || got.CanGrade != principal.Permissions.Has(access.TeachingGrading) {
				t.Fatalf("nav query=%+v", got)
			}
		})
	}
}

func TestMissingNavPortMapsToNotImplemented(t *testing.T) {
	var seen access.Scope
	app := application.New(recordingHome{seen: &seen})
	if _, err := dashboardhttp.NewDashboard(app).GetTeacherSummary(context.Background(), openapi.GetTeacherSummaryRequestObject{}); !errors.Is(err, httpx.ErrNotImplemented) {
		t.Fatalf("missing port=%v", err)
	}
}

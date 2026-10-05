package http

import (
	"context"
	"errors"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/dashboard/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"time"
)

func (h Dashboard) GetTeacherSummary(ctx context.Context, _ openapi.GetTeacherSummaryRequestObject) (openapi.GetTeacherSummaryResponseObject, error) {
	if h.app == nil || h.app.Queries.Nav == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, _ := httpx.PrincipalFromContext(ctx)
	out, err := h.app.Queries.Nav.Handle(ctx, query.Nav{Scope: httpapi.ScopeFromContext(ctx).Own(), CanGrade: principal.Access.Permissions.Has(access.TeachingGrading)})
	if errors.Is(err, domain.ErrNotificationsUnavailable) {
		return nil, httpx.ErrNotImplemented
	}
	if err != nil {
		return nil, err
	}
	return openapi.GetTeacherSummary200JSONResponse{LiveAssignments: out.LiveAssignments, AnswersToGrade: out.AnswersToGrade, UnreadNotifications: out.UnreadNotifications}, nil
}

func toAPISubmissions(in domain.Submissions) (openapi.DashboardSubmissions, error) {
	out := openapi.DashboardSubmissions{Days: make([]openapi.DashboardDay, len(in.Days)), Total: in.Total, AveragePercent: in.AveragePercent}
	for i, row := range in.Days {
		date, err := time.Parse("2006-01-02", row.Date)
		if err != nil {
			return out, err
		}
		out.Days[i] = openapi.DashboardDay{Date: openapi_types.Date{Time: date}, Count: row.Count}
	}
	return out, nil
}

func toAPIToday(in []domain.Today) []openapi.DashboardToday {
	out := make([]openapi.DashboardToday, len(in))
	for i, row := range in {
		out[i] = openapi.DashboardToday{Kind: openapi.DashboardTodayKind(row.Kind), At: row.At, AssignmentId: httpapi.ParseUUID(row.AssignmentID), Title: row.Title, NotSubmitted: row.NotSubmitted}
	}
	return out
}

func toAPIActivity(in []domain.Activity) []openapi.DashboardActivity {
	out := make([]openapi.DashboardActivity, len(in))
	for i, row := range in {
		out[i] = openapi.DashboardActivity{Kind: openapi.DashboardActivityKind(row.Kind), At: row.At, StudentName: row.StudentName, Subject: row.Subject, Flagged: row.Flagged}
	}
	return out
}

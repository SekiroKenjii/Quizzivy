package http

import (
	"context"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/dashboard/application"
	"quizzivy/internal/modules/dashboard/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

type Dashboard struct {
	app *application.Application
}

func NewDashboard(app *application.Application) Dashboard {
	return Dashboard{app: app}
}

func (h Dashboard) GetDashboard(ctx context.Context, request openapi.GetDashboardRequestObject) (openapi.GetDashboardResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	rangeValue := "14d"
	if request.Params.Range != nil {
		rangeValue = string(*request.Params.Range)
	}
	principal, _ := httpx.PrincipalFromContext(ctx)
	canReview := principal.Access.Permissions.Has(access.TeachingGrading) || principal.Access.Permissions.Has(access.TeachingAttemptsIntervene)
	summary, err := h.app.Queries.Summary.Handle(ctx, query.Summary{Scope: httpapi.ScopeFromContext(ctx).Own(), Range: rangeValue, CanReviewFlagged: canReview})
	if err != nil {
		return nil, err
	}
	submissions, err := toAPISubmissions(summary.Submissions)
	if err != nil {
		return nil, err
	}
	recent := make([]openapi.AttemptListRow, len(summary.Recent))
	for i, r := range summary.Recent {
		recent[i] = toAPIAttemptListRow(r)
	}
	var next *openapi.ClosingAssignment
	if summary.NextClosing != nil {
		row := summary.NextClosing
		next = &openapi.ClosingAssignment{Id: httpapi.ParseUUID(row.ID), Title: row.Title, ClosesAt: row.ClosesAt, SubmittedCount: row.SubmittedCount, TargetCount: row.TargetCount}
	}
	var flagged *openapi.DashboardFlaggedAttempt
	if summary.NewestFlaggedAttempt != nil {
		row := summary.NewestFlaggedAttempt
		flagged = &openapi.DashboardFlaggedAttempt{AssignmentId: httpapi.ParseUUID(row.AssignmentID), AttemptId: httpapi.ParseUUID(row.AttemptID)}
	}
	var takingAssignment *openapi_types.UUID
	if summary.TakingNow.AssignmentID != nil {
		id := httpapi.ParseUUID(*summary.TakingNow.AssignmentID)
		takingAssignment = &id
	}
	return openapi.GetDashboard200JSONResponse{
		NewestFlaggedAttempt: flagged,
		ClosingSoon:          &summary.ClosingSoon, WaitingStudents: &summary.WaitingStudents,
		OldestWaitingAt: summary.OldestWaitingAt, TotalStudents: &summary.TotalStudents, NextClosing: next,
		TakingNow:   openapi.DashboardTakingNow{Students: summary.TakingNow.Students, Assignments: summary.TakingNow.Assignments, AssignmentId: takingAssignment},
		Submissions: submissions, Today: toAPIToday(summary.Today), RecentActivity: toAPIActivity(summary.RecentActivity),
		OpenAssignments: summary.OpenAssignments,
		AwaitingGrading: summary.AwaitingGrading,
		ActiveStudents:  summary.ActiveStudents,
		FlaggedAttempts: summary.FlaggedAttempts,
		RecentAttempts:  recent,
	}, nil
}

func (h Dashboard) ListAttempts(ctx context.Context, request openapi.ListAttemptsRequestObject) (openapi.ListAttemptsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	q := domain.ListQuery{Flagged: request.Params.Flagged, PendingGrading: request.Params.PendingGrading, Scope: httpapi.ScopeFromContext(ctx)}
	if request.Params.Status != nil {
		status := string(*request.Params.Status)
		q.Status = &status
	}
	if request.Params.Page != nil {
		q.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		q.Limit = *request.Params.Limit
	}
	listResult, err := h.app.Queries.List.Handle(ctx, query.List{Query: q})
	found, page := listResult.Items, listResult.Page
	if err != nil {
		return nil, err
	}
	out := openapi.ListAttempts200JSONResponse{
		Items: make([]openapi.AttemptListRow, len(found)),
		Page:  page.Number, PageSize: page.Size, Total: page.Total,
	}
	for i, r := range found {
		out.Items[i] = toAPIAttemptListRow(r)
	}
	return out, nil
}

func toAPIAttemptListRow(r domain.Recent) openapi.AttemptListRow {
	pending := r.PendingManual
	return openapi.AttemptListRow{
		Id:            httpapi.ParseUUID(r.ID),
		StudentId:     httpapi.ParseUUID(r.StudentID),
		StudentName:   r.StudentName,
		AssignmentId:  httpapi.ParseUUID(r.AssignmentID),
		TestTitle:     r.TestTitle,
		Status:        openapi.AttemptStatus(r.Status),
		SubmittedAt:   r.SubmittedAt,
		Flagged:       r.Flagged,
		PendingManual: &pending,
	}
}

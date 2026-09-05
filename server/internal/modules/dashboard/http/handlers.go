package http

import (
	"context"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/paging"
)

// Service is the slice of the application this transport needs.
type Service interface {
	Summary(ctx context.Context) (domain.Summary, error)
	List(ctx context.Context, q domain.ListQuery) ([]domain.Recent, paging.Page, error)
}

type Dashboard struct {
	svc Service
}

func NewDashboard(svc Service) Dashboard {
	return Dashboard{svc: svc}
}

func (h Dashboard) GetDashboard(ctx context.Context, _ openapi.GetDashboardRequestObject) (openapi.GetDashboardResponseObject, error) {
	if h.svc == nil {
		return nil, httpx.ErrNotImplemented
	}
	summary, err := h.svc.Summary(ctx)
	if err != nil {
		return nil, err
	}
	recent := make([]openapi.AttemptListRow, len(summary.Recent))
	for i, r := range summary.Recent {
		recent[i] = toAPIAttemptListRow(r)
	}
	return openapi.GetDashboard200JSONResponse{
		OpenAssignments: summary.OpenAssignments,
		AwaitingGrading: summary.AwaitingGrading,
		ActiveStudents:  summary.ActiveStudents,
		FlaggedAttempts: summary.FlaggedAttempts,
		RecentAttempts:  recent,
	}, nil
}

func (h Dashboard) ListAttempts(ctx context.Context, request openapi.ListAttemptsRequestObject) (openapi.ListAttemptsResponseObject, error) {
	if h.svc == nil {
		return nil, httpx.ErrNotImplemented
	}
	q := domain.ListQuery{Flagged: request.Params.Flagged, PendingGrading: request.Params.PendingGrading}
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
	found, page, err := h.svc.List(ctx, q)
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

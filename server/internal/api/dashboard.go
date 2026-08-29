package api

import (
	"context"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
)

// GetDashboard answers §8's /admin: five figures and the last ten attempts, in
// one round trip rather than five.
func (s *Server) GetDashboard(ctx context.Context, _ openapi.GetDashboardRequestObject) (openapi.GetDashboardResponseObject, error) {
	if s.Deps.Dashboard == nil {
		return nil, httpx.ErrNotImplemented
	}

	summary, err := s.Deps.Dashboard.Get(ctx)
	if err != nil {
		return nil, err
	}

	recent := make([]openapi.AttemptListRow, len(summary.Recent))
	for i, r := range summary.Recent {
		row := openapi.AttemptListRow{
			Id:            parseUUID(r.ID),
			StudentId:     parseUUID(r.StudentID),
			StudentName:   r.StudentName,
			AssignmentId:  parseUUID(r.AssignmentID),
			TestTitle:     r.TestTitle,
			Status:        openapi.AttemptStatus(r.Status),
			Flagged:       r.Flagged,
			PendingManual: &r.PendingManual,
		}
		if r.SubmittedAt != nil {
			row.SubmittedAt = r.SubmittedAt
		}
		recent[i] = row
	}

	return openapi.GetDashboard200JSONResponse{
		OpenAssignments: summary.OpenAssignments,
		AwaitingGrading: summary.AwaitingGrading,
		ActiveStudents:  summary.ActiveStudents,
		FlaggedAttempts: summary.FlaggedAttempts,
		RecentAttempts:  recent,
	}, nil
}

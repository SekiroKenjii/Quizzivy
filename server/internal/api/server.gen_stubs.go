package api

import (
	"context"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
)

// Server implements the generated StrictServerInterface.
//
// Every operation is a stub until its phase builds it. They return
// httpx.ErrNotImplemented, which the strict handler's error hook renders as a
// 501 in the standard error envelope -- so an unbuilt endpoint is honestly
// unbuilt rather than a 404 that looks like a routing bug.
//
// This file is generated once from the interface and then owned by hand: as
// each operation is implemented, its stub is replaced in place.
type Server struct {
	Deps Deps
}

// Deps is what handlers need. It grows as phases add capability.
type Deps struct {
	DB           DB
	Auth         AuthService
	Join         JoinService
	Classes      ClassesService
	Media        MediaService
	Questions    QuestionsService
	Tests        TestsService
	Publisher    PublishService
	Dashboard    DashboardService
	Assignments  AssignmentsService
	Attempts     AttemptsService
	Students     StudentsService
	Tokens       TokenVerifier
	RefreshTTL   time.Duration
	CookieSecure bool
}

var _ openapi.StrictServerInterface = (*Server)(nil)

func (s *Server) GetAssignmentMonitor(_ context.Context, _ openapi.GetAssignmentMonitorRequestObject) (openapi.GetAssignmentMonitorResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ListAttempts(_ context.Context, _ openapi.ListAttemptsRequestObject) (openapi.ListAttemptsResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetAttemptForReview(_ context.Context, _ openapi.GetAttemptForReviewRequestObject) (openapi.GetAttemptForReviewResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetAttemptEvents(_ context.Context, _ openapi.GetAttemptEventsRequestObject) (openapi.GetAttemptEventsResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ExtendAttempt(_ context.Context, _ openapi.ExtendAttemptRequestObject) (openapi.ExtendAttemptResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) FinishGrading(_ context.Context, _ openapi.FinishGradingRequestObject) (openapi.FinishGradingResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GradeAttempt(_ context.Context, _ openapi.GradeAttemptRequestObject) (openapi.GradeAttemptResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ResetAttempt(_ context.Context, _ openapi.ResetAttemptRequestObject) (openapi.ResetAttemptResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) VoidAttempt(_ context.Context, _ openapi.VoidAttemptRequestObject) (openapi.VoidAttemptResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) CreateClass(_ context.Context, _ openapi.CreateClassRequestObject) (openapi.CreateClassResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ListMyAssignments(_ context.Context, _ openapi.ListMyAssignmentsRequestObject) (openapi.ListMyAssignmentsResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetMyAssignment(_ context.Context, _ openapi.GetMyAssignmentRequestObject) (openapi.GetMyAssignmentResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) FlushEvents(_ context.Context, _ openapi.FlushEventsRequestObject) (openapi.FlushEventsResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) GetAttemptResult(_ context.Context, _ openapi.GetAttemptResultRequestObject) (openapi.GetAttemptResultResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) SubmitAttempt(_ context.Context, _ openapi.SubmitAttemptRequestObject) (openapi.SubmitAttemptResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

func (s *Server) ListMyClasses(_ context.Context, _ openapi.ListMyClassesRequestObject) (openapi.ListMyClassesResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

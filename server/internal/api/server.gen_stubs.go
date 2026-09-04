package api

import (
	"context"
	"log/slog"
	"time"

	"quizzivy/gen/openapi"
	"quizzivy/internal/httpx"
)

// Server implements the generated StrictServerInterface.
type Server struct {
	Deps Deps
	// Logger is nil in tests; read it through logOf.
	Logger *slog.Logger
}

// logOf is the server's logger, or one that discards. Not a method: stubs_test
// reads Server's method set to find unimplemented operations.
func logOf(s *Server) *slog.Logger {
	if s.Logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return s.Logger
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

func (s *Server) GetAttemptResult(_ context.Context, _ openapi.GetAttemptResultRequestObject) (openapi.GetAttemptResultResponseObject, error) {
	return nil, httpx.ErrNotImplemented
}

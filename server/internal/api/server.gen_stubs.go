package api

import (
	"log/slog"
	"time"

	"quizzivy/gen/openapi"
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
	Review       ReviewService
	Integrity    IntegrityService
	Students     StudentsService
	Tokens       TokenVerifier
	RefreshTTL   time.Duration
	CookieSecure bool
}

var _ openapi.StrictServerInterface = (*Server)(nil)

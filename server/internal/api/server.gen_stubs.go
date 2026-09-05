package api

import (
	"log/slog"
	"time"

	"quizzivy/gen/openapi"
	classeshttp "quizzivy/internal/modules/classes/http"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
)

// Server implements the generated StrictServerInterface by embedding each module's handlers.
type Server struct {
	dashboardhttp.Dashboard
	classeshttp.Classes
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
	Modules      Modules
	DB           DB
	Auth         AuthService
	Media        MediaService
	Questions    QuestionsService
	Tests        TestsService
	Publisher    PublishService
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

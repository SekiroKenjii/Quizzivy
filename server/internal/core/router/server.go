package router

import (
	"log/slog"

	"quizzivy/gen/openapi"
	assignmentshttp "quizzivy/internal/modules/assignments/http"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	classeshttp "quizzivy/internal/modules/classes/http"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	identityhttp "quizzivy/internal/modules/identity/http"
	importshttp "quizzivy/internal/modules/imports/http"
	mediahttp "quizzivy/internal/modules/media/http"
	questionshttp "quizzivy/internal/modules/questions/http"
	testshttp "quizzivy/internal/modules/tests/http"
)

// Server implements the generated StrictServerInterface by embedding each module's handlers.
type Server struct {
	dashboardhttp.Dashboard
	classeshttp.Classes
	importshttp.Imports
	identityhttp.Identity
	questionshttp.Questions
	mediahttp.Media
	testshttp.Tests
	assignmentshttp.Assignments
	attemptshttp.Attempts
	Deps Deps
	// Logger is nil in tests; read it through logOf.
	Logger *slog.Logger
}

// Deps is what handlers need. It grows as phases add capability.
type Deps struct {
	Modules    Modules
	DB         DB
	Tokens     TokenVerifier
	Docs       *identitytoken.Issuer
	DocsPublic bool
}

var _ openapi.StrictServerInterface = (*Server)(nil)

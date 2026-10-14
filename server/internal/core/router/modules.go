package router

import (
	aihttp "quizzivy/internal/modules/ai/http"
	assignmentshttp "quizzivy/internal/modules/assignments/http"
	attemptshttp "quizzivy/internal/modules/attempts/http"
	audithttp "quizzivy/internal/modules/audit/http"
	availabilityhttp "quizzivy/internal/modules/availability/http"
	classeshttp "quizzivy/internal/modules/classes/http"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
	identityhttp "quizzivy/internal/modules/identity/http"
	importshttp "quizzivy/internal/modules/imports/http"
	mediahttp "quizzivy/internal/modules/media/http"
	notificationshttp "quizzivy/internal/modules/notifications/http"
	organizationhttp "quizzivy/internal/modules/organization/http"
	questionshttp "quizzivy/internal/modules/questions/http"
	testshttp "quizzivy/internal/modules/tests/http"
)

// Modules is every module's transport, as core assembled it.
type Modules struct {
	AI            aihttp.AI
	Audit         audithttp.Audit
	Imports       importshttp.Imports
	Dashboard     dashboardhttp.Dashboard
	Classes       classeshttp.Classes
	Identity      identityhttp.Identity
	Questions     questionshttp.Questions
	Media         mediahttp.Media
	Tests         testshttp.Tests
	Assignments   assignmentshttp.Assignments
	Attempts      attemptshttp.Attempts
	Availability  availabilityhttp.Availability
	Notifications notificationshttp.Notifications
	Organization  organizationhttp.Organization
}

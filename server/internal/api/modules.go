package api

import (
	classeshttp "quizzivy/internal/modules/classes/http"
	dashboardhttp "quizzivy/internal/modules/dashboard/http"
)

// Modules is every module's transport, as core assembled it.
type Modules struct {
	Dashboard dashboardhttp.Dashboard
	Classes   classeshttp.Classes
}

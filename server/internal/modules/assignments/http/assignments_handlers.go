package http

import (
	"quizzivy/internal/modules/assignments/application"
)

type Assignments struct {
	app *application.Application
}

func NewAssignments(app *application.Application) Assignments {
	return Assignments{app: app}
}

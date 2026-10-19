package http

import (
	"quizzivy/internal/modules/classes/application"
)

const noStore = "no-store"

type Classes struct {
	app *application.Application
}

func NewClasses(app *application.Application) Classes {
	return Classes{app: app}
}

package http

import (
	"time"

	"quizzivy/internal/modules/identity/application"
)

type Identity struct {
	app          *application.Application
	refreshTTL   time.Duration
	cookieSecure bool
}

func NewIdentity(app *application.Application, refreshTTL time.Duration, cookieSecure bool) Identity {
	return Identity{app: app, refreshTTL: refreshTTL, cookieSecure: cookieSecure}
}

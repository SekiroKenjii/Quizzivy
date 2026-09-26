package http

import (
	"time"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/token"
)

type Identity struct {
	app          *application.Application
	refreshTTL   time.Duration
	cookieSecure bool
	docs         *token.Issuer
}

// NewIdentity builds the identity transport. docs issues API reference
// sessions; nil leaves openDocsSession answering 501.
func NewIdentity(app *application.Application, refreshTTL time.Duration, cookieSecure bool, docs *token.Issuer) Identity {
	return Identity{app: app, refreshTTL: refreshTTL, cookieSecure: cookieSecure, docs: docs}
}

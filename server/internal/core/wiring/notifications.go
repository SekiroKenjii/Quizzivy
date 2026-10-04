package wiring

import (
	notificationsapp "quizzivy/internal/modules/notifications/application"
	notificationshttp "quizzivy/internal/modules/notifications/http"
	notificationsrepo "quizzivy/internal/modules/notifications/repositories"
	"quizzivy/internal/platform/db"
)

func notifications(dbx db.Context) *notificationsapp.Application {
	return notificationsapp.New(notificationsrepo.NewPostgres(dbx))
}

func notificationsTransport(app *notificationsapp.Application) notificationshttp.Notifications {
	return notificationshttp.NewNotifications(app)
}

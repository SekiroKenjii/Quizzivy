package ports

import (
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	"quizzivy/internal/shared/cqrs"
)

// Notifier tells a user that something happened, in the app: the notifications module's Notify command. A nil Notifier tells no one.
type Notifier = cqrs.CommandHandler[notificationscommand.Notify, cqrs.Nothing]

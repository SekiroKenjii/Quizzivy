package command

import (
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

func reach(a actor.Actor) access.Scope {
	return access.Scope{UserID: a.ID, All: a.Scope.All}
}

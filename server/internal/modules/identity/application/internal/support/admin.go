package support

import (
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/actor"
)

// Admin carries what the Admin console's Users handlers share: the directory, the
// access module's say on roles, and the service and students helpers they call.
type Admin struct {
	Repo     domain.AdminUsers
	Roles    ports.AssignableRole
	Service  *Service
	Students *Students
}

// Caller is who a write acts as, in the terms the access module's queries take.
func Caller(req domain.WriteRequest) accessdomain.Caller {
	return accessdomain.Caller{
		Actor:       actor.Actor{ID: req.ActorID, IP: req.IP, UserAgent: req.UserAgent, Scope: req.Scope()},
		Permissions: req.Grants,
	}
}

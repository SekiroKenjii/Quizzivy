package actor

import (
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/opt"
)

// Actor is who is acting and from where, as every audited command records it,
// and whose rows the command may reach.
type Actor struct {
	ID        string
	IP        string
	UserAgent string
	Scope     access.Scope
}

func (a Actor) IPValue() *string        { return opt.String(a.IP) }
func (a Actor) UserAgentValue() *string { return opt.String(a.UserAgent) }

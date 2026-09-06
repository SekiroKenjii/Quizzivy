package actor

import "quizzivy/internal/shared/opt"

// Actor is who is acting and from where, as every audited command records it.
type Actor struct {
	ID        string
	IP        string
	UserAgent string
}

func (a Actor) IPValue() *string        { return opt.String(a.IP) }
func (a Actor) UserAgentValue() *string { return opt.String(a.UserAgent) }

package model

import (
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"time"
)

// GroupMutation names the observed aggregate/test revisions, the authenticated
// actor and the permissions the caller holds, which the write narrows to the
// key its target needs.
type GroupMutation struct {
	ID                    string
	ExpectedRevision      int64
	ExpectedTestUpdatedAt time.Time
	Actor                 actor.Actor
	Grants                access.Set
}

// At binds a mutation to the application clock for auditing.
func (m GroupMutation) At(now time.Time) domain.GroupMutation {
	return domain.GroupMutation{ID: m.ID, ExpectedRevision: m.ExpectedRevision, ExpectedTestUpdatedAt: m.ExpectedTestUpdatedAt, ActorID: m.Actor.ID, IP: m.Actor.IP, UserAgent: m.Actor.UserAgent, Now: now, Scope: m.Actor.Scope, Grants: m.Grants}
}

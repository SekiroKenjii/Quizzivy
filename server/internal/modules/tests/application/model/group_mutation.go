package model

import (
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/actor"
	"time"
)

// GroupMutation names the observed aggregate/test revisions and the authenticated actor.
type GroupMutation struct {
	ID                    string
	ExpectedRevision      int64
	ExpectedTestUpdatedAt time.Time
	Actor                 actor.Actor
}

// At binds a mutation to the application clock for auditing.
func (m GroupMutation) At(now time.Time) domain.GroupMutation {
	return domain.GroupMutation{ID: m.ID, ExpectedRevision: m.ExpectedRevision, ExpectedTestUpdatedAt: m.ExpectedTestUpdatedAt, ActorID: m.Actor.ID, IP: m.Actor.IP, UserAgent: m.Actor.UserAgent, Now: now}
}

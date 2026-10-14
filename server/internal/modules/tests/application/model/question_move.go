package model

import (
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"time"
)

// QuestionMove names the draft test revision and the revision of every group a move touches, as the caller observed them, with the authenticated actor and the permissions the caller holds.
type QuestionMove struct {
	TestID                string
	QuestionID            string
	ExpectedTestUpdatedAt time.Time
	To                    domain.MoveTarget
	ExpectedGroups        []domain.GroupRevision
	Actor                 actor.Actor
	Grants                access.Set
}

// At binds a move to the application clock for auditing.
func (m QuestionMove) At(now time.Time) domain.MoveQuestionInput {
	return domain.MoveQuestionInput{TestID: m.TestID, QuestionID: m.QuestionID, ExpectedTestUpdatedAt: m.ExpectedTestUpdatedAt, To: m.To, ExpectedGroups: m.ExpectedGroups,
		ActorID: m.Actor.ID, Now: now, IP: m.Actor.IP, UserAgent: m.Actor.UserAgent, Scope: m.Actor.Scope, Grants: m.Grants}
}

package command

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/content"
	"quizzivy/internal/shared/opt"
	"time"

	"github.com/google/uuid"
)

// CreateForTeacher makes a class TeacherID teaches on the actor's behalf, with
// self-join on and, while the centre lets students join with a code, a new
// code as Rotate issues it with its defaults, in one transaction. A schedule
// label or room that is blank once trimmed is stored as none.
type CreateForTeacher struct {
	Name          string
	TeacherID     string
	ScheduleLabel *string
	Room          *string
	Actor         actor.Actor
}

type CreateForTeacherHandler struct {
	*support.Service
	Codes *support.Enrolment
}

func (s CreateForTeacherHandler) Handle(ctx context.Context, cmd CreateForTeacher) (domain.Class, error) {
	var c content.Composer
	name := c.Text("name", cmd.Name, domain.MaxClassName)
	scheduleLabel := c.Optional("scheduleLabel", cmd.ScheduleLabel, domain.MaxScheduleLabel)
	room := c.Optional("room", cmd.Room, domain.MaxRoom)
	if err := c.Err(); err != nil {
		return domain.Class{}, err
	}
	classID, err := uuid.NewV7()
	if err != nil {
		return domain.Class{}, fmt.Errorf("class id: %w", err)
	}
	now := s.Codes.Now()
	in := domain.CreateForTeacherInput{
		ClassID: classID.String(), Name: name, TeacherID: cmd.TeacherID,
		ScheduleLabel: domain.LabelOf(scheduleLabel), Room: domain.LabelOf(room),
		ActorUserID: cmd.Actor.ID, Now: now, IP: opt.String(cmd.Actor.IP), UserAgent: opt.String(cmd.Actor.UserAgent),
	}
	on, err := s.Codes.JoinCodesOn(ctx)
	if err != nil {
		return domain.Class{}, err
	}
	if on {
		if in.Code, err = s.newCode(ctx, in.ClassID, cmd.Actor.ID, now); err != nil {
			return domain.Class{}, err
		}
	}
	class, err := s.Repo.CreateForTeacher(ctx, in)
	if err != nil {
		return domain.Class{}, err
	}
	return class, s.AttachAverage(ctx, &class)
}

func (s CreateForTeacherHandler) newCode(ctx context.Context, classID, actorID string, now time.Time) (*domain.NewCode, error) {
	code, err := domain.JoinCodes.Generate()
	if err != nil {
		return nil, err
	}
	codeID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("join code id: %w", err)
	}
	sealed, err := s.Codes.Keys.Seal(classID, codeID.String(), code)
	if err != nil {
		return nil, err
	}
	maxUses := domain.DefaultMaxUses
	return &domain.NewCode{
		CodeID:     codeID.String(),
		CodeHash:   s.Codes.Keys.Hash(code),
		Ciphertext: sealed,
		KeyID:      s.Codes.Keys.CurrentID(),
		Hint:       domain.JoinCodes.Hint(code),
		ExpiresAt:  domain.CodeExpiry(now, domain.DefaultExpiryDays, s.Codes.ZoneOf(ctx, actorID)),
		MaxUses:    &maxUses,
	}, nil
}

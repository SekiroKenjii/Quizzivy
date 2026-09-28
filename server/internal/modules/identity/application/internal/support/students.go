package support

import (
	"context"
	"errors"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/stats"
	"time"
)

// Students carries what the students handlers share: their ports and the helpers they call.
type Students struct {
	Repo       domain.Students
	Stats      stats.Source
	Principals ports.Principals
	Now        func() time.Time
}

func NewStudents(repo domain.Students, stats stats.Source) *Students {
	return &Students{Repo: repo, Stats: stats, Principals: noPrincipals{}, Now: time.Now}
}

type noPrincipals struct{}

func (noPrincipals) Forget(string) {}

func (noPrincipals) Resolve(_ context.Context, userID string) (access.Principal, error) {
	return access.Principal{UserID: userID}, nil
}

func (s *Students) WithStats(ctx context.Context, scope access.Scope, student domain.Student) (domain.Student, error) {
	list := []domain.Student{student}
	if err := s.AttachStats(ctx, scope, list); err != nil {
		return domain.Student{}, err
	}
	return list[0], nil
}

func (s *Students) AttachStats(ctx context.Context, scope access.Scope, students []domain.Student) error {
	ids := make([]string, len(students))
	for i, st := range students {
		ids[i] = st.ID
	}
	byStudent, err := s.Stats.StudentStats(ctx, scope, ids)
	if err != nil {
		return err
	}
	for i := range students {
		students[i].Stats = byStudent[students[i].ID]
	}
	return nil
}

// MayActOn checks the guards a write to a student's account passes before it
// runs, in the order that keeps another teacher's student indistinguishable
// from a missing one: the student must be one the request reaches
// (ErrStudentNotFound); when needsManage, the actor must manage accounts; and
// the student's permissions, except learning.take_tests, must be a subset of
// the actor's (ErrForbidden for either).
func (s *Students) MayActOn(ctx context.Context, req domain.WriteRequest, id string, needsManage bool) error {
	if _, err := s.Repo.Get(ctx, req.Scope(), id); err != nil {
		return err
	}
	if needsManage && !req.ManagesUsers() {
		return domain.ErrForbidden
	}
	target, err := s.Principals.Resolve(ctx, id)
	if errors.Is(err, accessdomain.ErrUnknownUser) {
		return domain.ErrStudentNotFound
	}
	if err != nil {
		return err
	}
	if !access.CanActOn(req.Grants, target.Permissions) {
		return domain.ErrForbidden
	}
	return nil
}

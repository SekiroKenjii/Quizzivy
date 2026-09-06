package query

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
	"time"
)

type ForStudent struct {
	StudentID string
	Now       time.Time
}

type ForStudentHandler struct {
	*support.Service
}

func (s ForStudentHandler) Handle(ctx context.Context, q ForStudent) (domain.StudentSections, error) {
	return s.Repo.ForStudent(ctx, q.StudentID, q.Now)
}

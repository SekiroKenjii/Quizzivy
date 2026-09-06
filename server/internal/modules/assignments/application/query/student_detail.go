package query

import (
	"context"
	"quizzivy/internal/modules/assignments/application/internal/support"
	"quizzivy/internal/modules/assignments/domain"
)

type StudentDetail struct {
	ID        string
	StudentID string
}

type StudentDetailHandler struct {
	*support.Service
}

func (s StudentDetailHandler) Handle(ctx context.Context, q StudentDetail) (domain.StudentDetail, error) {
	return s.Repo.StudentDetail(ctx, q.ID, q.StudentID)
}

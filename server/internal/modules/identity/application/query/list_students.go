package query

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/paging"
)

type ListStudents struct {
	Query domain.StudentQuery
}

type ListStudentsResult struct {
	Items []domain.Student
	Page  paging.Page
}

type ListStudentsHandler struct {
	*support.Students
}

func (s ListStudentsHandler) Handle(ctx context.Context, q ListStudents) (ListStudentsResult, error) {
	found, page, err := s.Repo.List(ctx, q.Query)
	if err != nil {
		return ListStudentsResult{Items: nil, Page: paging.Page{}}, err
	}
	if err := s.AttachStats(ctx, found); err != nil {
		return ListStudentsResult{Items: nil, Page: paging.Page{}}, err
	}
	return ListStudentsResult{Items: found, Page: page}, nil
}

package query

import (
	"context"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/paging"
)

type Members struct {
	ClassID string
	Input   domain.MembersInput
}

type MembersResult struct {
	Items []domain.Member
	Page  paging.Page
}

type MembersHandler struct {
	*support.Service
}

func (s MembersHandler) Handle(ctx context.Context, q Members) (MembersResult, error) {
	members, page, err := s.Repo.Members(ctx, q.ClassID, q.Input)
	if err != nil {
		return MembersResult{Items: nil, Page: paging.Page{}}, err
	}
	if err := s.AttachStats(ctx, members); err != nil {
		return MembersResult{Items: nil, Page: paging.Page{}}, err
	}
	return MembersResult{Items: members, Page: page}, nil
}

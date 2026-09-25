package query

import (
	"context"
	"quizzivy/internal/modules/tests/application/internal/support"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/paging"
)

// Groups selects independent bank graphs without exposing their grading content.
type Groups struct{ Input domain.GroupListInput }
type GroupsResult struct {
	Items []domain.GroupSummary
	Page  paging.Page
}
type GroupsHandler struct{ *support.Groups }

func (s GroupsHandler) Handle(ctx context.Context, q Groups) (GroupsResult, error) {
	if s.Repo == nil {
		return GroupsResult{}, domain.ErrGroupUnavailable
	}
	items, page, err := s.Repo.List(ctx, q.Input)
	return GroupsResult{Items: items, Page: page}, err
}

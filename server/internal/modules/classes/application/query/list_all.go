package query

import (
	"context"
	"log/slog"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/paging"
)

// ListAll reads a page of every teacher's classes for the Admin console, each
// with its teacher's photo signed and its active code opened, with the tab
// counts beside them. A photo that cannot be signed and a code that cannot be
// opened are left out, not an error.
type ListAll struct {
	Input domain.AdminListInput
}

type ListAllResult struct {
	Items  []domain.AdminClass
	Page   paging.Page
	Facets domain.AdminFacets
}

type ListAllHandler struct {
	*support.Service
	Codes *support.Enrolment
}

func (s ListAllHandler) Handle(ctx context.Context, q ListAll) (ListAllResult, error) {
	items, page, err := s.Repo.ListAll(ctx, q.Input)
	if err != nil {
		return ListAllResult{}, err
	}
	facets, err := s.Repo.AdminFacets(ctx, q.Input)
	if err != nil {
		return ListAllResult{}, err
	}
	if err := s.openCodes(ctx, items); err != nil {
		return ListAllResult{}, err
	}
	s.signAvatars(ctx, items)
	return ListAllResult{Items: items, Page: page, Facets: facets}, nil
}

func (s ListAllHandler) openCodes(ctx context.Context, items []domain.AdminClass) error {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item.JoinCode != nil {
			ids = append(ids, item.ID)
		}
	}
	stored, err := s.Repo.ActiveCodes(ctx, access.Scope{All: true}, ids)
	if err != nil {
		return err
	}
	for i := range items {
		code, ok := stored[items[i].ID]
		if !ok {
			continue
		}
		active, err := s.Codes.Read(code)
		if err != nil {
			if s.Codes.Logger != nil {
				s.Codes.Logger.Warn("a join code could not be opened", slog.String("class_id", items[i].ID), slog.Any("err", err))
			}
			continue
		}
		items[i].Code = active.Code
	}
	return nil
}

func (s ListAllHandler) signAvatars(ctx context.Context, items []domain.AdminClass) {
	if s.Avatars == nil {
		return
	}
	signed := map[string]*string{}
	for i := range items {
		teacher := &items[i].Teacher
		if teacher.AvatarKey == nil {
			continue
		}
		url, seen := signed[*teacher.AvatarKey]
		if !seen {
			if signedURL, err := s.Avatars.AvatarURL(ctx, *teacher.AvatarKey); err == nil && signedURL != "" {
				url = &signedURL
			}
			signed[*teacher.AvatarKey] = url
		}
		teacher.AvatarURL = url
	}
}

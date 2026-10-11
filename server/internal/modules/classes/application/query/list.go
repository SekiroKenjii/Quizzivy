package query

import (
	"context"
	"log/slog"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/paging"
)

// List reads a page of the classes Input's scope reaches, each with its
// average score. OpenCodes asks for the readable join code of each: the caller
// sets it only for a request that asked for codes and a caller who holds
// teaching.classes.write, the permission getJoinCode needs, and a code then
// opens exactly when ActiveCode would open it for the same scope and class.
type List struct {
	Input     domain.ListInput
	OpenCodes bool
}

type ListResult struct {
	Items []domain.ListedClass
	Page  paging.Page
}

type ListHandler struct {
	*support.Service
	Codes *support.Enrolment
}

func (s ListHandler) Handle(ctx context.Context, q List) (ListResult, error) {
	found, page, err := s.Repo.List(ctx, q.Input)
	if err != nil {
		return ListResult{}, err
	}
	items := make([]domain.ListedClass, len(found))
	classes := make([]*domain.Class, len(found))
	for i := range found {
		items[i].Class = found[i]
		classes[i] = &items[i].Class
	}
	if err := s.AttachAverage(ctx, classes...); err != nil {
		return ListResult{}, err
	}
	if q.OpenCodes {
		if err := s.openCodes(ctx, q.Input, items); err != nil {
			return ListResult{}, err
		}
	}
	return ListResult{Items: items, Page: page}, nil
}

func (s ListHandler) openCodes(ctx context.Context, in domain.ListInput, items []domain.ListedClass) error {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if item.JoinCode != nil {
			ids = append(ids, item.ID)
		}
	}
	stored, err := s.Repo.ActiveCodes(ctx, in.Scope, ids)
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

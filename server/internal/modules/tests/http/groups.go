package http

import (
	"context"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/tests/application/command"
	"quizzivy/internal/modules/tests/application/model"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/actor"
	"time"
)

// GetQuestionGroup reads one teacher-only independent graph.
func (h Tests) GetQuestionGroup(ctx context.Context, request openapi.GetQuestionGroupRequestObject) (openapi.GetQuestionGroupResponseObject, error) {
	if h.app == nil || h.app.Queries.Group == nil {
		return nil, httpx.ErrNotImplemented
	}
	group, err := h.app.Queries.Group.Handle(ctx, query.Group{ID: request.Id.String()})
	return h.groupReply(ctx, group, err, 200)
}

// CreateQuestionGroup creates a bank or section graph without publishing it.
func (h Tests) CreateQuestionGroup(ctx context.Context, request openapi.CreateQuestionGroupRequestObject) (openapi.CreateQuestionGroupResponseObject, error) {
	if h.app == nil || h.app.Commands.CreateGroup == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	mutation, ok := groupMutation(ctx, "", 0, request.Body.ExpectedTestUpdatedAt)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	bundle, err := readGroupBundle(request.Body.Bundle)
	if err != nil {
		return nil, err
	}
	var owner *string
	if request.Body.OwnerSectionId != nil {
		id := request.Body.OwnerSectionId.String()
		owner = &id
	}
	group, err := h.app.Commands.CreateGroup.Handle(ctx, command.CreateGroup{Bundle: bundle, OwnerSectionID: owner, ExpectedTestUpdatedAt: mutation.ExpectedTestUpdatedAt, Actor: mutation.Actor})
	return h.groupReply(ctx, group, err, 201)
}

// UpdateQuestionGroup saves a complete graph under its observed revisions.
func (h Tests) UpdateQuestionGroup(ctx context.Context, request openapi.UpdateQuestionGroupRequestObject) (openapi.UpdateQuestionGroupResponseObject, error) {
	if h.app == nil || h.app.Commands.UpdateGroup == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	mutation, ok := groupMutation(ctx, request.Id.String(), request.Body.ExpectedRevision, request.Body.ExpectedTestUpdatedAt)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	bundle, err := readGroupBundle(request.Body.Bundle)
	if err != nil {
		return nil, err
	}
	group, err := h.app.Commands.UpdateGroup.Handle(ctx, command.UpdateGroup{Mutation: mutation, Bundle: bundle})
	return h.groupReply(ctx, group, err, 200)
}

// CopyQuestionGroup copies all context and member identities into an independent destination.
func (h Tests) CopyQuestionGroup(ctx context.Context, request openapi.CopyQuestionGroupRequestObject) (openapi.CopyQuestionGroupResponseObject, error) {
	if h.app == nil || h.app.Commands.CopyGroup == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	mutation, ok := groupMutation(ctx, request.Id.String(), request.Body.ExpectedRevision, request.Body.ExpectedTestUpdatedAt)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	var owner *string
	if request.Body.OwnerSectionId != nil {
		id := request.Body.OwnerSectionId.String()
		owner = &id
	}
	group, err := h.app.Commands.CopyGroup.Handle(ctx, command.CopyGroup{Mutation: mutation, OwnerSectionID: owner})
	return h.groupReply(ctx, group, err, 201)
}

// ArchiveQuestionGroup changes only an independent bank group's archive state.
func (h Tests) ArchiveQuestionGroup(ctx context.Context, request openapi.ArchiveQuestionGroupRequestObject) (openapi.ArchiveQuestionGroupResponseObject, error) {
	if h.app == nil || h.app.Commands.ArchiveGroup == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	mutation, ok := groupMutation(ctx, request.Id.String(), request.Body.ExpectedRevision, nil)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	group, err := h.app.Commands.ArchiveGroup.Handle(ctx, command.ArchiveGroup{Mutation: mutation, Archived: request.Body.Archived})
	return h.groupReply(ctx, group, err, 200)
}

// DeleteQuestionGroup removes a complete bank or draft graph while retaining published copies.
func (h Tests) DeleteQuestionGroup(ctx context.Context, request openapi.DeleteQuestionGroupRequestObject) (openapi.DeleteQuestionGroupResponseObject, error) {
	if h.app == nil || h.app.Commands.DeleteGroup == nil {
		return nil, httpx.ErrNotImplemented
	}
	mutation, ok := groupMutation(ctx, request.Id.String(), request.Params.ExpectedRevision, request.Params.ExpectedTestUpdatedAt)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	_, err := h.app.Commands.DeleteGroup.Handle(ctx, command.DeleteGroup{Mutation: mutation})
	if err != nil {
		return groupFailure(ctx, err)
	}
	return &groupResponse{status: 204}, nil
}

// ListQuestionGroups lists bounded bank summaries without reading grading content.
func (h Tests) ListQuestionGroups(ctx context.Context, request openapi.ListQuestionGroupsRequestObject) (openapi.ListQuestionGroupsResponseObject, error) {
	if h.app == nil || h.app.Queries.Groups == nil {
		return nil, httpx.ErrNotImplemented
	}
	in := domain.GroupListInput{}
	if request.Params.Page != nil {
		in.Page = *request.Params.Page
	}
	if request.Params.Limit != nil {
		in.Limit = *request.Params.Limit
	}
	if request.Params.Q != nil {
		in.Query = *request.Params.Q
	}
	if request.Params.Tag != nil {
		in.Tag = *request.Params.Tag
	}
	if request.Params.Status != nil {
		in.Status = string(*request.Params.Status)
	}
	groups, err := h.app.Queries.Groups.Handle(ctx, query.Groups{Input: in})
	if err != nil {
		return groupFailure(ctx, err)
	}
	return groupListReply(groups)
}

func groupMutation(ctx context.Context, id string, revision int64, updated *time.Time) (model.GroupMutation, bool) {
	req, ok := testRequest(ctx, id)
	out := model.GroupMutation{ID: id, ExpectedRevision: revision, Actor: actor.Actor{ID: req.ActorID, IP: req.IP, UserAgent: req.UserAgent}}
	if updated != nil {
		out.ExpectedTestUpdatedAt = *updated
	}
	return out, ok
}

package http

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/stats"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

func msgClassNotFound(ctx context.Context) string {
	return httpx.Text(ctx, "Không tìm thấy lớp học.", "The class was not found.")
}

func msgStudentNotFound(ctx context.Context) string {
	return httpx.Text(ctx, "Không tìm thấy học viên.", "The student was not found.")
}

// GetClass implements GET /teacher/classes/{id} (§6.4).
func (h Classes) GetClass(ctx context.Context, request openapi.GetClassRequestObject) (openapi.GetClassResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	class, err := h.app.Queries.Get.Handle(ctx, query.Get{ClassID: request.Id.String(), Scope: httpapi.ScopeFromContext(ctx)})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return openapi.GetClass404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgClassNotFound(ctx))),
			}, nil
		}
		return nil, err
	}
	return openapi.GetClass200JSONResponse(toAPIAdminClass(class)), nil
}

// UpdateClass implements PATCH /teacher/classes/{id}.
func (h Classes) UpdateClass(ctx context.Context, request openapi.UpdateClassRequestObject) (openapi.UpdateClassResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	who, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	in := domain.UpdateInput{}
	if request.Body.Name != nil {
		in.Name = request.Body.Name
	}
	if request.Body.Description != nil {
		in.Description = request.Body.Description
	}
	var err error
	if in.ScheduleLabel, err = textPatch(request.Body.ScheduleLabel); err != nil {
		return nil, err
	}
	if in.Room, err = textPatch(request.Body.Room); err != nil {
		return nil, err
	}
	if request.Body.SelfJoinEnabled != nil {
		in.SelfJoinEnabled = request.Body.SelfJoinEnabled
	}

	class, err := h.app.Commands.Update.Handle(ctx, command.Update{ClassID: request.Id.String(), Input: in, Scope: who.Scope})
	if err == nil && request.Body.Archived != nil {
		class, err = h.app.Commands.Archive.Handle(ctx, command.Archive{ClassID: request.Id.String(), Archived: *request.Body.Archived, Actor: who})
	}
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return openapi.UpdateClass404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgClassNotFound(ctx))),
			}, nil
		}
		return nil, err
	}
	return openapi.UpdateClass200JSONResponse(toAPIAdminClass(class)), nil
}

func (h Classes) CreateClass(ctx context.Context, request openapi.CreateClassRequestObject) (openapi.CreateClassResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	selfJoin := true
	if request.Body.SelfJoinEnabled != nil {
		selfJoin = *request.Body.SelfJoinEnabled
	}
	who, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	class, err := h.app.Commands.Create.Handle(ctx, command.Create{
		Name: request.Body.Name, Description: request.Body.Description,
		ScheduleLabel: request.Body.ScheduleLabel, Room: request.Body.Room, SelfJoin: selfJoin, Actor: who,
	})
	if err != nil {
		return nil, err
	}
	return openapi.CreateClass201JSONResponse(toAPIAdminClass(class)), nil
}

// ListClasses implements GET /teacher/classes.
func (h Classes) ListClasses(ctx context.Context, request openapi.ListClassesRequestObject) (openapi.ListClassesResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	in := domain.ListInput{Scope: httpapi.ScopeFromContext(ctx)}
	if request.Params.Q != nil {
		in.Query = string(*request.Params.Q)
	}
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}
	if request.Params.Status != nil {
		in.Status = string(*request.Params.Status)
	}

	principal, _ := httpx.PrincipalFromContext(ctx)
	asked := request.Params.WithCodes != nil && *request.Params.WithCodes
	openCodes := asked && principal.Access.Permissions.Has(access.TeachingClassesWrite)

	listResult, err := h.app.Queries.List.Handle(ctx, query.List{Input: in, OpenCodes: openCodes})
	found, page := listResult.Items, listResult.Page
	if err != nil {
		return nil, err
	}
	facets, err := h.app.Queries.Facets.Handle(ctx, query.Facets{Query: in.Query, Scope: in.Scope})
	if err != nil {
		return nil, err
	}
	items := make([]openapi.ClassListItem, 0, len(found))
	for _, c := range found {
		items = append(items, toAPIListItem(c))
	}
	noStore := "no-store"
	var out openapi.ListClasses200JSONResponse
	out.Headers.CacheControl = &noStore
	out.Body.Items, out.Body.Page, out.Body.PageSize, out.Body.Total = items, page.Number, page.Size, page.Total
	out.Body.Facets = openapi.ClassFacets{
		All: facets.All, Joinable: facets.Joinable, Archived: facets.Archived, Students: facets.Students,
	}
	return out, nil
}

// ListClassMembers implements GET /teacher/classes/{id}/members (§6.4).
func (h Classes) ListClassMembers(ctx context.Context, request openapi.ListClassMembersRequestObject) (openapi.ListClassMembersResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	in := domain.MembersInput{}
	if request.Params.Q != nil {
		in.Query = string(*request.Params.Q)
	}
	if request.Params.Page != nil {
		in.Page = int(*request.Params.Page)
	}
	if request.Params.Limit != nil {
		in.Limit = int(*request.Params.Limit)
	}

	membersResult, err := h.app.Queries.Members.Handle(ctx, query.Members{ClassID: request.Id.String(), Input: in, Scope: httpapi.ScopeFromContext(ctx)})
	found, page := membersResult.Items, membersResult.Page
	if err != nil {
		return nil, err
	}
	items := make([]openapi.ClassMember, 0, len(found))
	for _, m := range found {
		items = append(items, toAPIMember(m))
	}
	return openapi.ListClassMembers200JSONResponse{
		Items: items, Page: page.Number, PageSize: page.Size, Total: page.Total,
	}, nil
}

func (h Classes) AddClassMember(ctx context.Context, request openapi.AddClassMemberRequestObject) (openapi.AddClassMemberResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	who, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	m, err := h.app.Commands.AddMember.Handle(ctx, command.AddMember{ClassID: request.Id.String(), UserID: request.Body.UserId.String(), Actor: who})
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrNotFound):
		return openapi.AddClassMember404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgClassNotFound(ctx)))}, nil
	case errors.Is(err, domain.ErrNotAStudent):
		return openapi.AddClassMember404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, msgStudentNotFound(ctx)))}, nil
	default:
		return nil, err
	}

	return openapi.AddClassMember201JSONResponse(toAPIMember(m)), nil
}

func toAPIMember(m domain.Member) openapi.ClassMember {
	return openapi.ClassMember{
		UserId:   httpapi.ParseUUID(m.UserID),
		FullName: m.FullName,
		Email:    openapi_types.Email(m.Email),

		JoinedVia:    openapi.ClassMemberJoinedVia(m.JoinedVia),
		JoinedAt:     m.JoinedAt,
		JoinCodeHint: m.JoinCodeHint,
		Stats:        httpapi.StudentStats(m.Stats),
	}
}

// RemoveClassMember implements DELETE /teacher/classes/{id}/members/{userId}.
func (h Classes) RemoveClassMember(ctx context.Context, request openapi.RemoveClassMemberRequestObject) (openapi.RemoveClassMemberResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	who, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	_, err := h.app.Commands.RemoveMember.Handle(ctx, command.RemoveMember{ClassID: request.Id.String(), UserID: request.UserId.String(), Actor: who})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return openapi.RemoveClassMember404JSONResponse{
				NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, msgClassNotFound(ctx))),
			}, nil
		}
		return nil, err
	}
	return openapi.RemoveClassMember204Response{}, nil
}

func textPatch(raw json.RawMessage) (domain.TextPatch, error) {
	if len(raw) == 0 {
		return domain.TextPatch{}, nil
	}
	var value *string
	if err := json.Unmarshal(raw, &value); err != nil {
		return domain.TextPatch{}, err
	}
	return domain.TextPatch{Set: true, Value: value}, nil
}

func toAPIScore(score *stats.ClassScore) *openapi.AttemptScore {
	if score == nil {
		return nil
	}
	return &openapi.AttemptScore{Earned: score.Earned, Total: score.Total, PendingManual: score.PendingManual}
}

func toAPIAdminClass(c domain.Class) openapi.Class {
	out := openapi.Class{
		Id:                  httpapi.ParseUUID(c.ID),
		Name:                c.Name,
		Description:         c.Description,
		ScheduleLabel:       c.ScheduleLabel,
		Room:                c.Room,
		StudentCount:        c.StudentCount,
		OpenAssignmentCount: c.OpenAssignmentCount,
		SelfJoinEnabled:     c.SelfJoinEnabled,
		ArchivedAt:          c.ArchivedAt,
		CreatedAt:           c.CreatedAt,
		AverageScore:        toAPIScore(c.AverageScore),
	}
	if jc := c.JoinCode; jc != nil {
		out.JoinCode = &openapi.JoinCodeInfo{
			Hint:      jc.Hint,
			ExpiresAt: jc.ExpiresAt,
			MaxUses:   jc.MaxUses,
			UsesCount: jc.UsesCount,
		}
	}
	return out
}

func toAPIListItem(c domain.ListedClass) openapi.ClassListItem {
	out := openapi.ClassListItem{
		Id:                  httpapi.ParseUUID(c.ID),
		Name:                c.Name,
		Description:         c.Description,
		ScheduleLabel:       c.ScheduleLabel,
		Room:                c.Room,
		StudentCount:        c.StudentCount,
		OpenAssignmentCount: c.OpenAssignmentCount,
		SelfJoinEnabled:     c.SelfJoinEnabled,
		ArchivedAt:          c.ArchivedAt,
		CreatedAt:           c.CreatedAt,
		AverageScore:        toAPIScore(c.AverageScore),
	}
	if jc := c.JoinCode; jc != nil {
		out.JoinCode = &openapi.JoinCode{
			Hint:      jc.Hint,
			Legacy:    jc.Legacy,
			ExpiresAt: jc.ExpiresAt,
			MaxUses:   jc.MaxUses,
			UsesCount: jc.UsesCount,
		}
		if c.Code != "" {
			grouped := domain.JoinCodes.Format(c.Code)
			out.JoinCode.Code = &grouped
		}
	}
	return out
}

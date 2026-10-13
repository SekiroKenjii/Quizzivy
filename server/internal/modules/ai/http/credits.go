package http

import (
	"context"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/ai/application/command"
	"quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
)

func dateOf(t time.Time) openapi_types.Date { return openapi_types.Date{Time: t} }

func requestOf(r domain.Request) openapi.AiCreditRequest {
	out := openapi.AiCreditRequest{
		Id: httpapi.ParseUUID(r.ID), UserId: httpapi.ParseUUID(r.UserID), Month: dateOf(r.Month), Amount: int(r.Amount),
		Reason: r.Reason, Status: openapi.AiRequestStatus(r.Status), CreatedAt: r.CreatedAt.UTC(),
	}
	if r.DecidedAt != nil {
		out.DecidedAt = new(r.DecidedAt.UTC())
	}
	return out
}

func rulesOf(r domain.Rules) openapi.AiRules {
	return openapi.AiRules{PoolCredits: int(r.PoolCredits), DefaultAllowance: int(r.DefaultAllowance), WarnPercent: r.WarnPercent, OnTeacherLimit: openapi.AiRulesOnTeacherLimit(r.OnTeacherLimit)}
}

func poolOf(p domain.Pool) openapi.AiPool {
	return openapi.AiPool{Plan: int(p.Plan), Added: int(p.Added), Used: int(p.Used), Reserved: int(p.Reserved), Total: int(p.Total()), Left: int(p.Left())}
}

func teacherOf(t domain.TeacherMonth) openapi.AiTeacherMonth {
	return openapi.AiTeacherMonth{
		UserId: httpapi.ParseUUID(t.UserID), Name: t.Name, Email: openapi_types.Email(t.Email), Allowance: int(t.Allowance), Granted: int(t.Granted),
		Used: int(t.Used), Reserved: int(t.Reserved), Custom: t.Custom, Paused: t.Paused, Status: openapi.AiTeacherStatus(t.Status),
	}
}

// GetAiUsage implements GET /teacher/ai/usage.
func (h AI) GetAiUsage(ctx context.Context, _ openapi.GetAiUsageRequestObject) (openapi.GetAiUsageResponseObject, error) {
	who, _, ok := h.caller(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	usage, err := h.app.Queries.Usage.Handle(ctx, query.Usage{UserID: who.ID})
	if err != nil {
		return failed[openapi.GetAiUsageResponseObject](ctx, err)
	}
	out := openapi.GetAiUsage200JSONResponse{
		Month: dateOf(usage.Month), Used: int(usage.Used), Allowance: int(usage.Allowance), Left: int(usage.Left),
		Paused: usage.Paused, Requests: usage.Requests, Categories: make([]openapi.AiCategorySum, len(usage.Categories)),
	}
	for i, c := range usage.Categories {
		out.Categories[i] = openapi.AiCategorySum{Category: openapi.AiCreditCategory(c.Category), Credits: int(c.Credits)}
	}
	if usage.Pending != nil {
		out.Pending = new(requestOf(*usage.Pending))
	}
	return out, nil
}

// RequestAiCredits implements POST /teacher/ai/credit-requests.
func (h AI) RequestAiCredits(ctx context.Context, request openapi.RequestAiCreditsRequestObject) (openapi.RequestAiCreditsResponseObject, error) {
	who, _, ok := h.caller(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	filed, err := h.app.Commands.RequestCredits.Handle(ctx, command.RequestCredits{Actor: who, Amount: int64(request.Body.Amount), Reason: request.Body.Reason})
	if err != nil {
		return failed[openapi.RequestAiCreditsResponseObject](ctx, err)
	}
	return openapi.RequestAiCredits201JSONResponse(requestOf(filed)), nil
}

func (h AI) author(ctx context.Context) (domain.Author, bool) {
	who, grants, ok := h.caller(ctx)
	if !ok {
		return domain.Author{}, false
	}
	return domain.AuthorOf(who, grants), true
}

// GetAiCredits implements GET /admin/ai/credits.
func (h AI) GetAiCredits(ctx context.Context, _ openapi.GetAiCreditsRequestObject) (openapi.GetAiCreditsResponseObject, error) {
	_, ok := h.author(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	view, err := h.app.Queries.AdminCredits.Handle(ctx, query.AdminCredits{})
	if err != nil {
		return failed[openapi.GetAiCreditsResponseObject](ctx, err)
	}
	out := openapi.GetAiCredits200JSONResponse{
		Month: dateOf(view.Month), Pool: poolOf(view.Pool), DaysToGo: view.DaysToGo, OnPace: int(view.OnPace), WithinPool: view.WithinPool,
		Allowances: int(view.Allowances), Active: view.Active, Idle: view.Idle, Rules: rulesOf(view.Rules),
		Teachers: make([]openapi.AiTeacherMonth, len(view.Teachers)), Requests: make([]openapi.AiPendingRequest, len(view.Requests)),
	}
	for i, t := range view.Teachers {
		out.Teachers[i] = teacherOf(t)
	}
	for i, r := range view.Requests {
		out.Requests[i] = openapi.AiPendingRequest{
			Id: httpapi.ParseUUID(r.ID), UserId: httpapi.ParseUUID(r.UserID), TeacherName: r.TeacherName, Month: dateOf(r.Month),
			Amount: int(r.Amount), Reason: r.Reason, CreatedAt: r.CreatedAt.UTC(), Used: int(r.Used), Allowance: int(r.Allowance),
		}
	}
	return out, nil
}

// UpdateAiRules implements PATCH /admin/ai/rules.
func (h AI) UpdateAiRules(ctx context.Context, request openapi.UpdateAiRulesRequestObject) (openapi.UpdateAiRulesResponseObject, error) {
	by, ok := h.author(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	cmd := command.SetRules{By: by}
	if v := request.Body.DefaultAllowance; v != nil {
		cmd.DefaultAllowance = new(int64(*v))
	}
	cmd.WarnPercent = request.Body.WarnPercent
	if v := request.Body.OnTeacherLimit; v != nil {
		cmd.OnTeacherLimit = new(domain.Limit(*v))
	}
	change, err := h.app.Commands.SetRules.Handle(ctx, cmd)
	if err != nil {
		return failed[openapi.UpdateAiRulesResponseObject](ctx, err)
	}
	return openapi.UpdateAiRules200JSONResponse{Rules: rulesOf(change.Rules), Rewritten: int(change.Rewritten)}, nil
}

// AddAiCredits implements POST /admin/ai/credits/additions.
func (h AI) AddAiCredits(ctx context.Context, request openapi.AddAiCreditsRequestObject) (openapi.AddAiCreditsResponseObject, error) {
	by, ok := h.author(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	pool, err := h.app.Commands.AddCredits.Handle(ctx, command.AddCredits{By: by, Credits: int64(request.Body.Credits)})
	if err != nil {
		return failed[openapi.AddAiCreditsResponseObject](ctx, err)
	}
	return openapi.AddAiCredits201JSONResponse(poolOf(pool)), nil
}

// SetAiAllowance implements PUT /admin/ai/teachers/{userId}/allowance.
func (h AI) SetAiAllowance(ctx context.Context, request openapi.SetAiAllowanceRequestObject) (openapi.SetAiAllowanceResponseObject, error) {
	by, ok := h.author(ctx)
	if !h.enabled {
		return h.off(ctx), nil
	}
	if !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	cmd := command.SetAllowance{By: by, UserID: request.UserId.String(), Custom: request.Body.Custom, Paused: request.Body.Paused}
	if v := request.Body.Credits; v != nil {
		cmd.Credits = int64(*v)
	}
	month, err := h.app.Commands.SetAllowance.Handle(ctx, cmd)
	if err != nil {
		return failed[openapi.SetAiAllowanceResponseObject](ctx, err)
	}
	return openapi.SetAiAllowance200JSONResponse(teacherOf(month)), nil
}

func (h AI) decide(ctx context.Context, id string, approve bool) (openapi.AiCreditDecision, *failure, error) {
	by, ok := h.author(ctx)
	if !ok {
		return openapi.AiCreditDecision{}, nil, httpx.ErrNotImplemented
	}
	decided, err := h.app.Commands.DecideRequest.Handle(ctx, command.DecideRequest{By: by, ID: id, Approve: approve})
	if err != nil {
		f, err := failureOf(ctx, err)
		return openapi.AiCreditDecision{}, f, err
	}
	return openapi.AiCreditDecision{Request: requestOf(decided.Request), Allowance: int(decided.Allowance)}, nil, nil
}

// ApproveAiCreditRequest implements POST /admin/ai/credit-requests/{id}/approve.
func (h AI) ApproveAiCreditRequest(ctx context.Context, request openapi.ApproveAiCreditRequestRequestObject) (openapi.ApproveAiCreditRequestResponseObject, error) {
	if !h.enabled {
		return h.off(ctx), nil
	}
	decision, refused, err := h.decide(ctx, request.Id.String(), true)
	if refused != nil {
		return refused, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.ApproveAiCreditRequest200JSONResponse(decision), nil
}

// DeclineAiCreditRequest implements POST /admin/ai/credit-requests/{id}/decline.
func (h AI) DeclineAiCreditRequest(ctx context.Context, request openapi.DeclineAiCreditRequestRequestObject) (openapi.DeclineAiCreditRequestResponseObject, error) {
	if !h.enabled {
		return h.off(ctx), nil
	}
	decision, refused, err := h.decide(ctx, request.Id.String(), false)
	if refused != nil {
		return refused, nil
	}
	if err != nil {
		return nil, err
	}
	return openapi.DeclineAiCreditRequest200JSONResponse(decision), nil
}

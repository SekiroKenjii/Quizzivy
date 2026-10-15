// Package http serves the Admin console's audit log: its list, with the counts
// of its tabs, and its download.
package http

import (
	"context"
	"errors"
	"log/slog"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/audit/application"
	"quizzivy/internal/modules/audit/application/query"
	"quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/uaparse"
)

const dateLayout = "2006-01-02"

// Audit is the transport of the audit log: GET /admin/audit-log and its export.
type Audit struct {
	app    *application.Application
	logger *slog.Logger
}

// NewAudit serves the log through app. A nil app answers 501.
func NewAudit(app *application.Application) Audit {
	return Audit{app: app, logger: slog.New(slog.DiscardHandler)}
}

// WithLogger returns the transport logging to logger an export that failed
// after its first byte, which can no longer be answered with an error.
func (h Audit) WithLogger(logger *slog.Logger) Audit {
	h.logger = logger
	return h
}

// ListAuditLog implements GET /admin/audit-log. Every day of the range is the
// viewer's own, in their stored time zone.
func (h Audit) ListAuditLog(ctx context.Context, request openapi.ListAuditLogRequestObject) (openapi.ListAuditLogResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return openapi.ListAuditLog401JSONResponse{UnauthorizedJSONResponse: openapi.UnauthorizedJSONResponse(sessionInvalid(ctx))}, nil
	}
	result, err := h.app.Queries.List.Handle(ctx, listQuery(principal.UserID, request.Params))
	if refused, ok := refusal(ctx, err); ok {
		return openapi.ListAuditLog400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(refused)}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]openapi.AuditEntry, len(result.Entries))
	for i, entry := range result.Entries {
		items[i] = ToAPIEntry(entry)
	}
	return openapi.ListAuditLog200JSONResponse{
		Items: items, Facets: toAPIFacets(result.Facets),
		Page: result.Page.Number, PageSize: result.Page.Size, Total: result.Page.Total,
	}, nil
}

func listQuery(viewerID string, p openapi.ListAuditLogParams) query.List {
	q := query.List{ViewerID: viewerID}
	if p.Category != nil {
		q.Category = domain.Category(*p.Category)
	}
	if p.Q != nil {
		q.Query = *p.Q
	}
	if p.Range != nil {
		q.Range = domain.Range(*p.Range)
	}
	if p.From != nil {
		q.From = p.From.Format(dateLayout)
	}
	if p.To != nil {
		q.To = p.To.Format(dateLayout)
	}
	if p.Page != nil {
		q.Page = *p.Page
	}
	if p.Limit != nil {
		q.Limit = *p.Limit
	}
	return q
}

func refusal(ctx context.Context, err error) (openapi.ErrorResponse, bool) {
	var badRange *domain.RangeError
	switch {
	case errors.As(err, &badRange):
		return httpapi.FieldError(ctx, badRange.Field, rangeMessage(ctx, badRange.Problem)), true
	case errors.Is(err, domain.ErrUnknownCategory):
		return httpapi.FieldError(ctx, "category", httpx.Text(ctx, "Nhóm này không tồn tại.", "That category does not exist.")), true
	case errors.Is(err, domain.ErrUnknownRange):
		return httpapi.FieldError(ctx, "range", httpx.Text(ctx, "Khoảng thời gian này không tồn tại.", "That range does not exist.")), true
	}
	return openapi.ErrorResponse{}, false
}

func rangeMessage(ctx context.Context, problem domain.RangeProblem) string {
	switch problem {
	case domain.RangeNotCustom:
		return httpx.Text(ctx, "Chỉ dùng ngày này khi chọn khoảng thời gian tuỳ chọn.", "Dates are used only with the custom range.")
	case domain.RangeIncomplete:
		return httpx.Text(ctx, "Khoảng tuỳ chọn cần cả ngày bắt đầu và ngày kết thúc.", "A custom range needs both a start date and an end date.")
	case domain.RangeReversed:
		return httpx.Text(ctx, "Ngày kết thúc không được trước ngày bắt đầu.", "The end date cannot be before the start date.")
	}
	return httpx.Text(ctx, "Ngày không hợp lệ.", "That is not a valid date.")
}

func sessionInvalid(ctx context.Context) openapi.ErrorResponse {
	return httpapi.Error(ctx, openapi.UNAUTHORIZED,
		httpx.Text(ctx, "Phiên đăng nhập không hợp lệ. Vui lòng đăng nhập lại.", "Your session is not valid. Please sign in again."))
}

func toAPIFacets(f domain.Facets) openapi.AuditCategoryFacets {
	return openapi.AuditCategoryFacets{
		All: f.All, Accounts: f.Accounts, Classes: f.Classes, Attempts: f.Attempts,
		Tests: f.Tests, Media: f.Media, Imports: f.Imports,
	}
}

// ToAPIEntry is the contract's AuditEntry for e: its actor, category, entity, device and changes as the audit log's list shows them.
func ToAPIEntry(e domain.Entry) openapi.AuditEntry {
	out := openapi.AuditEntry{
		Id: e.ID, OccurredAt: e.OccurredAt, Action: e.Action, Ip: e.IP,
		Entity:  openapi.AuditEntity{Type: e.EntityType, Label: e.Label},
		Changes: make([]openapi.AuditChange, len(e.Changes)),
	}
	if e.EntityID != nil {
		out.Entity.Id = new(httpapi.ParseUUID(*e.EntityID))
	}
	if e.Actor != nil {
		out.Actor = &openapi.AuditActor{Id: httpapi.ParseUUID(e.Actor.ID), Name: e.Actor.Name}
		if e.AvatarURL != "" {
			out.Actor.AvatarUrl = &e.AvatarURL
		}
	}
	if e.Category != nil {
		out.Category = new(openapi.AuditCategory(*e.Category))
	}
	if label := deviceOf(e.UserAgent); label != "" {
		out.Device = &label
	}
	for i, change := range e.Changes {
		out.Changes[i] = openapi.AuditChange{Field: change.Field, From: change.From, To: change.To}
	}
	if e.Details != nil {
		out.Details = &e.Details
	}
	return out
}

func deviceOf(userAgent *string) string {
	if userAgent == nil {
		return ""
	}
	return uaparse.Parse(*userAgent).Label()
}

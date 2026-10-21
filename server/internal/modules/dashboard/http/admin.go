package http

import (
	"context"
	"errors"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"quizzivy/gen/openapi"
	audithttp "quizzivy/internal/modules/audit/http"
	"quizzivy/internal/modules/dashboard/application/query"
	"quizzivy/internal/modules/dashboard/domain"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

func adminViewer(ctx context.Context) domain.AdminViewer {
	principal, _ := httpx.PrincipalFromContext(ctx)
	keys := principal.Access.Permissions
	return domain.AdminViewer{
		UserID:   principal.UserID,
		Users:    keys.Has(access.PeopleUsersManage),
		Audit:    keys.Has(access.SystemAuditRead),
		Settings: keys.Has(access.SystemSettingsWrite),
		Credits:  keys.Has(access.AICreditsManage),
		All:      keys.Has(access.ScopeAll),
	}
}

// GetAdminOverview implements GET /admin/overview.
func (h Dashboard) GetAdminOverview(ctx context.Context, _ openapi.GetAdminOverviewRequestObject) (openapi.GetAdminOverviewResponseObject, error) {
	if h.app == nil || h.app.Queries.AdminOverview == nil {
		return nil, httpx.ErrNotImplemented
	}
	overview, err := h.app.Queries.AdminOverview.Handle(ctx, query.AdminOverview{Viewer: adminViewer(ctx)})
	if errors.Is(err, domain.ErrAdminUnavailable) {
		return nil, httpx.ErrNotImplemented
	}
	if err != nil {
		return nil, err
	}
	return openapi.GetAdminOverview200JSONResponse(toAPIOverview(overview)), nil
}

// GetAdminSummary implements GET /admin/summary.
func (h Dashboard) GetAdminSummary(ctx context.Context, _ openapi.GetAdminSummaryRequestObject) (openapi.GetAdminSummaryResponseObject, error) {
	if h.app == nil || h.app.Queries.AdminSummary == nil {
		return nil, httpx.ErrNotImplemented
	}
	summary, err := h.app.Queries.AdminSummary.Handle(ctx, query.AdminSummary{Viewer: adminViewer(ctx)})
	if err != nil {
		return nil, err
	}
	return openapi.GetAdminSummary200JSONResponse{MustChangePassword: summary.MustChangePassword}, nil
}

// GetUsage implements GET /admin/usage.
func (h Dashboard) GetUsage(ctx context.Context, _ openapi.GetUsageRequestObject) (openapi.GetUsageResponseObject, error) {
	if h.app == nil || h.app.Queries.Usage == nil {
		return nil, httpx.ErrNotImplemented
	}
	usage, err := h.app.Queries.Usage.Handle(ctx, query.Usage{Viewer: adminViewer(ctx)})
	if err != nil {
		return nil, err
	}
	return openapi.GetUsage200JSONResponse(toAPIUsage(usage)), nil
}

func toAPIOverview(o domain.AdminOverview) openapi.AdminOverview {
	out := openapi.AdminOverview{
		OrganizationName: o.OrganizationName,
		Attention:        make([]openapi.AdminAttention, len(o.Attention)),
	}
	if p := o.People; p != nil {
		out.People = &openapi.AdminOverviewPeople{
			Total: p.Total, ActiveThisWeek: p.ActiveThisWeek, Teachers: p.Teachers, TeachersDisabled: p.TeachersDisabled,
			Students: p.Students, StudentsNewThisMonth: p.StudentsNewThisMonth,
		}
	}
	if c := o.Classes; c != nil {
		out.Classes = &openapi.AdminOverviewClasses{Active: c.Active, Archived: c.Archived}
	}
	if d := o.DailyActive; d != nil {
		days := make([]openapi.AdminDailyActiveDay, len(d.Days))
		for i, day := range d.Days {
			days[i] = openapi.AdminDailyActiveDay{Date: openapi_types.Date{Time: day.Date}, Students: day.Students, Staff: day.Staff}
		}
		out.DailyActive = &openapi.AdminDailyActive{Days: days, Average: d.Average}
	}
	for i, a := range o.Attention {
		out.Attention[i] = openapi.AdminAttention{Kind: openapi.AdminAttentionKind(a.Kind), Count: a.Count, Detail: toAPIDetail(a.Detail)}
	}
	if o.RecentChanges != nil {
		entries := make([]openapi.AuditEntry, len(o.RecentChanges))
		for i, e := range o.RecentChanges {
			entries[i] = audithttp.ToAPIEntry(e)
		}
		out.RecentChanges = &entries
	}
	if o.Storage != nil {
		usage := toAPIUsage(*o.Storage)
		out.Storage = &usage
	}
	return out
}

func toAPIDetail(d *domain.AttentionDetail) *openapi.AdminAttentionDetail {
	if d == nil {
		return nil
	}
	return &openapi.AdminAttentionDetail{
		OldestCreatedAt: d.OldestCreatedAt, ClassName: d.ClassName, TeacherName: d.TeacherName,
		MediaPercent: d.MediaPercent, ImportsPercent: d.ImportsPercent, StoragePercent: d.StoragePercent,
		DatabasePercent: d.DatabasePercent, HeldUntil: d.HeldUntil, Used: d.Used, Allowance: d.Allowance,
	}
}

func toAPIUsage(u domain.Usage) openapi.Usage {
	out := openapi.Usage{
		Seats:    openapi.UsageCount{Used: u.Seats.Used, Limit: u.Seats.Limit},
		Teachers: openapi.UsageCount{Used: u.Teachers.Used, Limit: u.Teachers.Limit},
		Storage:  openapi.UsageBytes{Bytes: u.Storage.Bytes, Limit: u.Storage.Limit},
		Media:    openapi.UsageMedia{Bytes: u.Media.Bytes, LargestLibrary: u.Media.LargestLibrary, PerTeacherLimit: u.Media.PerTeacherLimit},
		Imports: openapi.UsageImports{Bytes: u.Imports.Bytes, Count: u.Imports.Count, OpenBytes: u.Imports.OpenBytes,
			Limit: u.Imports.Limit, PerTeacherLimit: u.Imports.PerTeacherLimit},
		Email: openapi.UsageEmail{Today: u.Email.Today, Month: u.Email.Month, Held: u.Email.Held,
			DailyLimit: u.Email.DailyLimit, MonthlyLimit: u.Email.MonthlyLimit},
		Database:      openapi.UsageBytes{Bytes: u.Database.Bytes, Limit: u.Database.Limit},
		AttemptsKept:  openapi.UsageAttempts{Count: u.AttemptsKept},
		WarnAtPercent: u.WarnAtPercent,
	}
	if c := u.AICredits; c != nil {
		out.AiCredits = &openapi.UsageCredits{Used: c.Used, Pool: c.Pool}
	}
	return out
}

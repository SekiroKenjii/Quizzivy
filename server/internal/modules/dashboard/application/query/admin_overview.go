package query

import (
	"context"
	"time"

	auditquery "quizzivy/internal/modules/audit/application/query"
	auditdomain "quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/modules/dashboard/application/internal/support"
	"quizzivy/internal/modules/dashboard/domain"
	organizationquery "quizzivy/internal/modules/organization/application/query"
)

// AdminOverview reads the Admin console's Overview for Viewer. A section the viewer's keys do not open is left out, and so is an attention row nothing needs. Days are the local dates of the centre's zone.
type AdminOverview struct {
	Viewer domain.AdminViewer
}

type AdminOverviewHandler struct{ *support.Service }

func (s AdminOverviewHandler) Handle(ctx context.Context, q AdminOverview) (domain.AdminOverview, error) {
	if s.Branding == nil {
		return domain.AdminOverview{}, domain.ErrAdminUnavailable
	}
	brand, err := s.Branding.Handle(ctx, organizationquery.Branding{})
	if err != nil {
		return domain.AdminOverview{}, err
	}
	read := &overviewRead{
		service: s.Service,
		viewer:  q.Viewer,
		today:   domain.LocalDay(s.Now(), s.Location),
		out:     domain.AdminOverview{OrganizationName: brand.Name, Attention: []domain.Attention{}},
	}
	for _, step := range []func(context.Context) error{read.credits, read.accounts, read.people, read.classes, read.recentChanges, read.storage} {
		if err := step(ctx); err != nil {
			return domain.AdminOverview{}, err
		}
	}
	return read.out, nil
}

type overviewRead struct {
	service     *support.Service
	viewer      domain.AdminViewer
	today       time.Time
	out         domain.AdminOverview
	viewCredits *domain.CreditsView
	counted     domain.Accounts
}

func (r *overviewRead) credits(ctx context.Context) error {
	view, err := r.service.CreditsView(ctx, r.viewer)
	if err != nil || view == nil {
		return err
	}
	r.viewCredits = view
	if row := view.Attention(); row != nil {
		r.out.Attention = append(r.out.Attention, *row)
	}
	return nil
}

func (r *overviewRead) accounts(ctx context.Context) error {
	if !r.viewer.Users && !r.viewer.Settings {
		return nil
	}
	monthStart := time.Date(r.today.Year(), r.today.Month(), 1, 0, 0, 0, 0, r.service.Location)
	var err error
	r.counted, err = r.service.Repo.Accounts(ctx, monthStart)
	return err
}

func (r *overviewRead) people(ctx context.Context) error {
	if !r.viewer.Users {
		return nil
	}
	active, err := r.service.Repo.ActiveBetween(ctx, r.today.AddDate(0, 0, 1-domain.ActiveWeekDays), r.today)
	if err != nil {
		return err
	}
	a := r.counted
	r.out.People = &domain.People{
		Total: a.Total, ActiveThisWeek: active, Teachers: a.Teachers, TeachersDisabled: a.TeachersDisabled,
		Students: a.Students, StudentsNewThisMonth: a.StudentsNew,
	}
	days, err := r.service.Repo.ActiveByDay(ctx, r.today.AddDate(0, 0, 1-domain.ActiveDays), r.today)
	if err != nil {
		return err
	}
	chart := domain.NewDailyActive(r.today, days)
	r.out.DailyActive = &chart
	if a.MustChangePassword > 0 {
		r.out.Attention = append(r.out.Attention, domain.Attention{Kind: domain.AttentionMustChange, Count: a.MustChangePassword})
	}
	if a.NeverSignedIn > 0 {
		r.out.Attention = append(r.out.Attention, domain.Attention{
			Kind: domain.AttentionNeverSignedIn, Count: a.NeverSignedIn,
			Detail: &domain.AttentionDetail{OldestCreatedAt: a.OldestNeverSignedIn},
		})
	}
	return nil
}

func (r *overviewRead) classes(ctx context.Context) error {
	if !r.viewer.All {
		return nil
	}
	counts, err := r.service.Repo.ClassCounts(ctx)
	if err != nil {
		return err
	}
	r.out.Classes = &counts
	orphaned, err := r.service.Repo.DisabledTeacherClasses(ctx)
	if err != nil || orphaned.Count == 0 {
		return err
	}
	r.out.Attention = append(r.out.Attention, domain.Attention{
		Kind: domain.AttentionDisabledTeachers, Count: orphaned.Count,
		Detail: &domain.AttentionDetail{ClassName: &orphaned.ClassName, TeacherName: &orphaned.TeacherName},
	})
	return nil
}

func (r *overviewRead) recentChanges(ctx context.Context) error {
	if !r.viewer.Audit {
		return nil
	}
	entries := []auditdomain.Entry{}
	if r.service.Recent != nil {
		found, err := r.service.Recent.Handle(ctx, auditquery.Recent{Limit: domain.RecentChanges})
		if err != nil {
			return err
		}
		if found != nil {
			entries = found
		}
	}
	r.out.RecentChanges = entries
	return nil
}

func (r *overviewRead) storage(ctx context.Context) error {
	if !r.viewer.Settings {
		return nil
	}
	usage, err := r.service.Usage(ctx, r.viewCredits)
	if err != nil {
		return err
	}
	r.out.Storage = &usage
	if row := domain.StorageAttention(usage); row != nil {
		r.out.Attention = append(r.out.Attention, *row)
	}
	if row := domain.EmailAttention(usage); row != nil {
		r.out.Attention = append(r.out.Attention, *row)
	}
	return nil
}

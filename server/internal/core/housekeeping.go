package core

import (
	"context"
	"time"

	"quizzivy/internal/core/jobs"
	exportscommand "quizzivy/internal/modules/exports/application/command"
	identitycommand "quizzivy/internal/modules/identity/application/command"
	organizationcommand "quizzivy/internal/modules/organization/application/command"
	"quizzivy/internal/platform/db"
)

func (a *App) housekeepingSteps() []jobs.HousekeepingStep {
	var steps []jobs.HousekeepingStep
	if prune := a.assembly.Identity.Commands.PruneActivityDays; prune != nil {
		steps = append(steps, jobs.HousekeepingStep{Name: "activity_days", Batch: func(ctx context.Context, limit int) (int, error) {
			return prune.Handle(ctx, identitycommand.PruneActivityDays{Limit: limit})
		}})
	}
	if outbox := a.assembly.Mail.Outbox; outbox != nil {
		dbx := db.NewContext(a.pool)
		steps = append(steps, jobs.HousekeepingStep{Name: "email_outbox", Batch: func(ctx context.Context, limit int) (int, error) {
			return outbox.Prune(ctx, dbx, time.Now(), limit)
		}})
	}
	if prune := a.assembly.Identity.Commands.PruneSetPasswordLinks; prune != nil {
		steps = append(steps, jobs.HousekeepingStep{Name: "set_password_links", Batch: func(ctx context.Context, limit int) (int, error) {
			return prune.Handle(ctx, identitycommand.PruneSetPasswordLinks{Limit: limit})
		}})
	}
	if erase := a.assembly.Identity.Commands.EraseExpiredImportSheets; erase != nil {
		steps = append(steps, jobs.HousekeepingStep{Name: "import_sheets", Batch: func(ctx context.Context, limit int) (int, error) {
			return erase.Handle(ctx, identitycommand.EraseExpiredImportSheets{Limit: limit})
		}})
	}
	if prune := a.assembly.Identity.Commands.PruneUserImports; prune != nil {
		steps = append(steps, jobs.HousekeepingStep{Name: "import_jobs", Batch: func(ctx context.Context, limit int) (int, error) {
			return prune.Handle(ctx, identitycommand.PruneUserImports{Limit: limit})
		}})
	}
	if exports := a.assembly.Exports; exports != nil {
		steps = append(steps, jobs.HousekeepingStep{Name: "data_exports", Batch: func(ctx context.Context, limit int) (int, error) {
			expired, err := exports.Commands.Expire.Handle(ctx, exportscommand.Expire{Limit: limit})
			return expired.Marked, err
		}})
	}
	if sweep := a.assembly.Organization.Commands.SweepLogos; sweep != nil {
		steps = append(steps, jobs.HousekeepingStep{Name: "branding_objects", Batch: func(ctx context.Context, limit int) (int, error) {
			swept, err := sweep.Handle(ctx, organizationcommand.SweepLogos{Limit: limit})
			return swept.Removed, err
		}})
	}
	return steps
}

func (a *App) housekeep(ctx context.Context) {
	jobs.Housekeep(ctx, a.logger, a.housekeepingSteps())
}

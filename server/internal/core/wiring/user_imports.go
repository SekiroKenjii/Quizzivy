package wiring

import (
	"context"
	"log/slog"

	"quizzivy/internal/core/adapters"
	identityapp "quizzivy/internal/modules/identity/application"
	identityports "quizzivy/internal/modules/identity/application/ports"
	identityrepo "quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/email"
)

// attachUserImports opens previewUserImport, createUserImport and
// getUserImport, which need the import worker to create the accounts: without
// it they answer 501. Both deliveries also need SECRETS_KEY, to seal the link
// an email carries and the passwords a hand-out sheet holds.
func attachUserImports(ctx context.Context, cfg config.Config, logger *slog.Logger, dbx db.Context, app *identityapp.Application, roles identityports.RoleChoices) {
	if !cfg.ImportProcessing {
		logger.Info("user imports disabled (no import worker configured)")
		return
	}
	wake := adapters.NewWorkerWake(context.WithoutCancel(ctx), cfg.ImportWorkerWakeURL, logger)
	budget := adapters.UserImportBudget{Budget: email.Budget{DB: dbx, Limit: cfg.Email.DailyLimit, Location: cfg.AppLocation}}
	app.SetUserImports(identityrepo.NewUserImports(dbx), roles, adapters.UserImportTables{}, budget, wake, cfg.SecretsKey != nil)
}

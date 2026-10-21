package wiring

import (
	"context"
	"fmt"
	"log/slog"

	"quizzivy/internal/core/adapters"
	identityapp "quizzivy/internal/modules/identity/application"
	identityports "quizzivy/internal/modules/identity/application/ports"
	identityworker "quizzivy/internal/modules/identity/application/worker"
	identityrepo "quizzivy/internal/modules/identity/repositories"
	organizationapp "quizzivy/internal/modules/organization/application"
	organizationrepo "quizzivy/internal/modules/organization/repositories"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/email"
	"quizzivy/internal/platform/secrets"
)

func userImportSealer(cfg config.Config) (identityports.Sealer, error) {
	if cfg.SecretsKey == nil {
		return nil, nil
	}
	sealer, err := secrets.New(cfg.SecretsKey, secrets.PurposeUserImportSheet)
	if err != nil {
		return nil, fmt.Errorf("SECRETS_KEY: %w", err)
	}
	return sealer, nil
}

func attachUserImports(ctx context.Context, cfg config.Config, logger *slog.Logger, dbx db.Context, app *identityapp.Application, roles identityports.RoleChoices) error {
	if !cfg.ImportProcessing {
		logger.Info("user imports disabled (no import worker configured)")
		return nil
	}
	sealer, err := userImportSealer(cfg)
	if err != nil {
		return err
	}
	wake := adapters.NewWorkerWake(context.WithoutCancel(ctx), cfg.ImportWorkerWakeURL, logger)
	budget := adapters.UserImportBudget{Budget: email.Budget{DB: dbx, Limits: cfg.Limits, Location: cfg.AppLocation}}
	app.SetUserImports(identityrepo.NewUserImports(dbx), roles, adapters.UserImportTables{}, budget, wake, sealer)
	return nil
}

// UserImportsWorker assembles the runner of the user imports over the worker's
// own database context. Its repository issues the invite of an account created
// for an email link into outbox, the worker's mail outbox, and its runner
// reads the importer's rights and the centre's sign-in policy from the same
// modules the API does.
func UserImportsWorker(ctx context.Context, cfg config.ImportWorker, dbx db.Context, outbox *email.Outbox, logger *slog.Logger) (identityworker.UserImports, error) {
	principals, err := accessModule(ctx, dbx)
	if err != nil {
		return identityworker.UserImports{}, err
	}
	sealer, err := userImportSealer(cfg.Base)
	if err != nil {
		return identityworker.UserImports{}, err
	}
	boundPasswordHashing(cfg.Base, logger)
	organizationApp := organizationapp.New(organizationrepo.NewPostgres(dbx), logger, cfg.Base.GoogleEnabled())
	links := identityrepo.NewSetPasswordLinks(dbx, outbox)
	return identityworker.UserImports{
		Queue:      identityrepo.NewUserImports(dbx).WithLinks(links),
		Roles:      principals.Queries.AssignableRole,
		Principals: principals,
		Policy:     organizationApp.Queries.SignInPolicy,
		Links:      links,
		Sealer:     sealer,
		Logger:     logger,
	}, nil
}

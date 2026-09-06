// Package wiring is the composition root's assembly: every module built against the live database, its ports satisfied by platform adapters or by another module's handlers.
package wiring

import (
	"context"
	"log/slog"

	"quizzivy/internal/core/router"
	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	identityapp "quizzivy/internal/modules/identity/application"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
)

// Assembly is what Build produces: the transports the router serves, the token verifier the auth middleware needs, and the identity application the background jobs drive.
type Assembly struct {
	Modules  router.Modules
	Tokens   *identitytoken.Issuer
	Identity *identityapp.Application
}

// Build assembles every module against the pool, in dependency order: attempts' student statistics feed classes and identity, classes' enrolment feeds identity's Google sign-in, media feeds questions, tests and attempts.
func Build(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *db.Pool) (Assembly, error) {
	dbx := db.NewContext(pool.Pool)
	stats := attemptsrepo.NewStudentStats(dbx)

	classesApp := classes(dbx, stats)
	identityApp, tokens, err := identity(cfg, logger, dbx, stats, classesApp.Commands.EnrolNewMember)
	if err != nil {
		return Assembly{}, err
	}
	mediaApp, mediaRepo, err := media(ctx, cfg, logger, dbx)
	if err != nil {
		return Assembly{}, err
	}
	questionsApp, questionsRepo := questions(dbx, mediaApp)
	testsApp := tests(dbx, questionsRepo, mediaRepo)
	attemptsApp := attempts(dbx)

	return Assembly{
		Modules: router.Modules{
			Dashboard:   dashboard(dbx),
			Classes:     classesTransport(classesApp),
			Identity:    identityTransport(cfg, identityApp),
			Questions:   questionsTransport(questionsApp, mediaApp),
			Media:       mediaTransport(mediaApp),
			Tests:       testsTransport(testsApp, mediaApp),
			Assignments: assignments(dbx),
			Attempts:    attemptsTransport(attemptsApp, mediaApp, identityApp, logger),
		},
		Tokens:   tokens,
		Identity: identityApp,
	}, nil
}

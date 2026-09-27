// Package wiring is the composition root's assembly: every module built against the live database, its ports satisfied by platform adapters or by another module's handlers.
package wiring

import (
	"context"
	"log/slog"

	"quizzivy/internal/core/adapters"
	"quizzivy/internal/core/router"
	accessapp "quizzivy/internal/modules/access/application"
	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	identityapp "quizzivy/internal/modules/identity/application"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	importsworker "quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/httpx"
)

// Assembly is what Build produces: the transports the router serves, the token verifiers the auth middleware and the docs gate need, the access application that resolves who a request acts as, and the identity application the background jobs drive.
type Assembly struct {
	Modules       router.Modules
	Principals    *accessapp.Application
	Tokens        *identitytoken.Issuer
	Docs          *identitytoken.Issuer
	Identity      *identityapp.Application
	ImportSweeper *importsworker.Sweeper
	Maintenance   httpx.MaintenanceSource
}

// Build refuses when app.permissions lacks a key this binary was compiled with, then assembles every module against the pool in dependency order: attempts' student statistics feed classes and identity, classes' enrolment feeds identity's Google sign-in, media feeds questions, tests and attempts.
func Build(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *db.Pool) (Assembly, error) {
	dbx := db.NewContext(pool.Pool)
	principals, err := accessModule(ctx, dbx)
	if err != nil {
		return Assembly{}, err
	}
	stats := attemptsrepo.NewStudentStats(dbx)

	classesApp := classes(dbx, stats)
	identityApp, tokens, err := identity(cfg, logger, dbx, stats, classesApp.Commands.EnrolNewMember)
	if err != nil {
		return Assembly{}, err
	}
	docs, err := identitytoken.NewDocsIssuer(cfg.JWTSigningKey)
	if err != nil {
		return Assembly{}, err
	}
	mediaApp, mediaRepo, err := media(ctx, cfg, logger, dbx)
	if err != nil {
		return Assembly{}, err
	}
	importsTransport, sweeper, err := imports(ctx, cfg, logger, dbx, mediaApp)
	if err != nil {
		return Assembly{}, err
	}
	questionsApp, questionsRepo := questions(dbx, mediaApp)
	testsApp := tests(dbx, questionsRepo, mediaRepo, mediaApp)
	attemptsApp := attempts(dbx).WithGroupContexts(testsApp.Queries.GroupContexts)
	availabilityApp, availabilityTransport := availability(dbx, logger)

	return Assembly{
		Modules: router.Modules{
			Imports:      importsTransport,
			Dashboard:    dashboard(dbx),
			Classes:      classesTransport(classesApp),
			Identity:     identityTransport(cfg, identityApp, docs),
			Questions:    questionsTransport(questionsApp, mediaApp),
			Media:        mediaTransport(mediaApp),
			Tests:        testsTransport(testsApp, mediaApp),
			Assignments:  assignments(dbx),
			Attempts:     attemptsTransport(attemptsApp, mediaApp, identityApp, logger),
			Availability: availabilityTransport,
		},
		Principals:    principals,
		Maintenance:   adapters.MaintenanceGate{Current: availabilityApp.Queries.CurrentWindow},
		ImportSweeper: sweeper,
		Tokens:        tokens,
		Docs:          docs,
		Identity:      identityApp,
	}, nil
}

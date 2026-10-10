// Package wiring is the composition root's assembly: every module built against the live database, its ports satisfied by platform adapters or by another module's handlers.
package wiring

import (
	"context"
	"fmt"
	"log/slog"

	"quizzivy/internal/core/adapters"
	"quizzivy/internal/core/router"
	accessapp "quizzivy/internal/modules/access/application"
	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	classesapp "quizzivy/internal/modules/classes/application"
	classesdomain "quizzivy/internal/modules/classes/domain"
	identityapp "quizzivy/internal/modules/identity/application"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	importsworker "quizzivy/internal/modules/imports/application/worker"
	notificationsapp "quizzivy/internal/modules/notifications/application"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/httpx"
)

// Assembly is what Build produces: the transports the router serves, the token verifiers the auth middleware and the docs gate need, the access application that resolves who a request acts as, and the identity and notifications applications the background jobs drive.
type Assembly struct {
	Modules       router.Modules
	Principals    *accessapp.Application
	Tokens        *identitytoken.Issuer
	Docs          *identitytoken.Issuer
	Identity      *identityapp.Application
	Classes       *classesapp.Application
	Notifications *notificationsapp.Application
	ImportSweeper *importsworker.Sweeper
	Maintenance   httpx.MaintenanceSource
}

// Build refuses when app.permissions lacks a key this binary was compiled with, or when the join-code keys derive a zero or shared key id, then assembles every module against the pool in dependency order: notifications depends on no module and is built first, so that its Notify command can be handed to any module as a port; attempts' student statistics feed classes and identity, classes' enrolment feeds identity's Google sign-in, media feeds questions, tests and attempts.
func Build(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *db.Pool) (Assembly, error) {
	dbx := db.NewContext(pool.Pool)
	principals, err := accessModule(ctx, dbx)
	if err != nil {
		return Assembly{}, err
	}
	notificationsApp := notifications(dbx)
	stats := attemptsrepo.NewStudentStats(dbx)

	keys, err := classesdomain.NewJoinCodeKeys(cfg.JoinCodeKey, cfg.JoinCodeKeyPrevious)
	if err != nil {
		return Assembly{}, fmt.Errorf("JOIN_CODE_KEY: %w", err)
	}
	previousKeyID, rotating := keys.PreviousID()
	logger.Info("join code keys", "current_key_id", keys.CurrentID(), "previous_key_id", previousKeyID, "rotating", rotating)
	classesApp := classes(dbx, stats, keys, notificationsApp.Commands.Notify, logger)
	identityApp, tokens, err := identity(cfg, logger, dbx, stats, classesApp.Commands.EnrolNewMember)
	if err != nil {
		return Assembly{}, err
	}
	identityApp.SetPrincipals(principals)
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
	attemptsApp := attempts(dbx, notificationsApp.Commands.Notify, logger).WithGroupContexts(testsApp.Queries.GroupContexts)
	availabilityApp, availabilityTransport := availability(dbx, logger)

	return Assembly{
		Modules: router.Modules{
			Imports:       importsTransport,
			Dashboard:     dashboard(dbx, notificationsApp, identityApp),
			Classes:       classesTransport(classesApp),
			Identity:      identityTransport(cfg, identityApp, docs),
			Questions:     questionsTransport(questionsApp, mediaApp),
			Media:         mediaTransport(mediaApp),
			Tests:         testsTransport(testsApp, mediaApp),
			Assignments:   assignments(dbx, notificationsApp.Commands.Notify, logger),
			Attempts:      attemptsTransport(attemptsApp, mediaApp, identityApp, logger),
			Availability:  availabilityTransport,
			Notifications: notificationsTransport(notificationsApp, logger),
		},
		Principals:    principals,
		Maintenance:   adapters.MaintenanceGate{Current: availabilityApp.Queries.CurrentWindow},
		ImportSweeper: sweeper,
		Tokens:        tokens,
		Docs:          docs,
		Identity:      identityApp,
		Classes:       classesApp,
		Notifications: notificationsApp,
	}, nil
}

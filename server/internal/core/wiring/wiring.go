// Package wiring is the composition root's assembly: every module built against the live database, its ports satisfied by platform adapters or by another module's handlers.
package wiring

import (
	"context"
	"fmt"
	"log/slog"

	"quizzivy/internal/core/adapters"
	"quizzivy/internal/core/router"
	accessapp "quizzivy/internal/modules/access/application"
	aiapp "quizzivy/internal/modules/ai/application"
	aiports "quizzivy/internal/modules/ai/application/ports"
	attemptsrepo "quizzivy/internal/modules/attempts/repositories"
	classesapp "quizzivy/internal/modules/classes/application"
	classesdomain "quizzivy/internal/modules/classes/domain"
	identityapp "quizzivy/internal/modules/identity/application"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	importsworker "quizzivy/internal/modules/imports/application/worker"
	mediaapp "quizzivy/internal/modules/media/application"
	notificationsapp "quizzivy/internal/modules/notifications/application"
	organizationapp "quizzivy/internal/modules/organization/application"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/platform/imagesafe"
)

// Assembly is what Build produces: the transports the router serves, the token verifiers the auth middleware and the docs gate need, the access application that resolves who a request acts as, and the identity, classes, notifications and media applications, the mail outbox and dispatcher and the import sweeper the background jobs drive.
type Assembly struct {
	Modules       router.Modules
	Principals    *accessapp.Application
	Tokens        *identitytoken.Issuer
	Docs          *identitytoken.Issuer
	Identity      *identityapp.Application
	Classes       *classesapp.Application
	Notifications *notificationsapp.Application
	Organization  *organizationapp.Application
	ImportSweeper *importsworker.Sweeper
	Media         *mediaapp.Application
	Maintenance   httpx.MaintenanceSource
	// Mail is the outbox the modules enqueue into and the dispatcher that sends it.
	Mail Mail
	// AI is the provider behind the AI assistant's port, nil when no ANTHROPIC_API_KEY is set.
	AI aiports.Completer
	// AICredits is the credits model the AI operations reserve and settle through and the console sets allowances through. It is built with or without the provider.
	AICredits *aiapp.Application
}

// Option changes how Build assembles the application.
type Option func(*options)

type options struct {
	completer    aiports.Completer
	hasCompleter bool
}

// WithCompleter serves the AI assistant from completer in place of the configured provider, a nil one turning the assistant off. It is for tests that drive the whole application against a provider they control.
func WithCompleter(completer aiports.Completer) Option {
	return func(o *options) { o.completer, o.hasCompleter = completer, true }
}

// Build refuses when app.permissions lacks a key this binary was compiled with, or when the join-code keys derive a zero or shared key id, then assembles every module against the pool in dependency order: notifications depends on no module and is built first, so that its Notify command can be handed to any module as a port; attempts' student statistics feed classes and identity, classes' enrolment feeds identity's Google sign-in, media feeds questions, tests and attempts.
func Build(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *db.Pool, opts ...Option) (Assembly, error) {
	var chosen options
	for _, opt := range opts {
		opt(&chosen)
	}
	dbx := db.NewContext(pool.Pool)
	principals, err := accessModule(ctx, dbx)
	if err != nil {
		return Assembly{}, err
	}
	notificationsApp := notifications(dbx)
	images := imagesafe.New(imagesafe.NewGate(photoDecodeSlots))
	organizationApp, err := organization(ctx, cfg, logger, dbx, images)
	if err != nil {
		return Assembly{}, err
	}
	stats := attemptsrepo.NewStudentStats(dbx)

	keys, err := classesdomain.NewJoinCodeKeys(cfg.JoinCodeKey, cfg.JoinCodeKeyPrevious)
	if err != nil {
		return Assembly{}, fmt.Errorf("JOIN_CODE_KEY: %w", err)
	}
	previousKeyID, rotating := keys.PreviousID()
	logger.Info("join code keys", "current_key_id", keys.CurrentID(), "previous_key_id", previousKeyID, "rotating", rotating)
	classesApp := classes(dbx, stats, keys, notificationsApp.Commands.Notify, logger).WithSignInPolicy(organizationApp.Queries.SignInPolicy)
	identityApp, tokens, err := identity(cfg, logger, dbx, stats, classesApp.Commands.EnrolNewMember)
	if err != nil {
		return Assembly{}, err
	}
	identityApp.SetPrincipals(principals)
	identityApp.SetSignInPolicy(organizationApp.Queries.SignInPolicy)
	attachClassPorts(classesApp, identityApp, stats)
	if err := attachAvatars(ctx, cfg, logger, identityApp, images); err != nil {
		return Assembly{}, err
	}
	docs, err := identitytoken.NewDocsIssuer(cfg.JWTSigningKey)
	if err != nil {
		return Assembly{}, err
	}
	completer, err := ai(cfg, logger)
	if err != nil {
		return Assembly{}, err
	}
	if chosen.hasCompleter {
		completer = chosen.completer
	}
	mediaApp, mediaRepo, err := media(ctx, cfg, logger, dbx)
	if err != nil {
		return Assembly{}, err
	}
	importsTransport, sweeper, err := imports(ctx, cfg, logger, dbx, mediaApp)
	if err != nil {
		return Assembly{}, err
	}
	importsTransport = importsTransport.WithAI(completer != nil)
	questionsApp, questionsRepo := questions(dbx, mediaApp)
	testsApp := tests(dbx, questionsRepo, mediaRepo, mediaApp)
	attemptsApp := attempts(dbx, notificationsApp.Commands.Notify, logger).WithGroupContexts(testsApp.Queries.GroupContexts).WithZones(adapters.ProfileZone{Query: identityApp.Queries.EffectiveZone})
	availabilityApp, availabilityTransport := availability(dbx, logger, cfg.AIEnabled())
	aiApp, aiTransport, err := aiAssistant(cfg, dbx, notificationsApp.Commands.Notify, logger, completer, aiModules{
		access: principals, identity: identityApp, questions: questionsApp, tests: testsApp, attempts: attemptsApp,
	})
	if err != nil {
		return Assembly{}, err
	}

	mail, err := BuildMail(cfg, dbx, logger)
	if err != nil {
		return Assembly{}, err
	}

	return Assembly{
		Mail: mail,
		Modules: router.Modules{
			AI:            aiTransport,
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
			Organization:  organizationTransport(organizationApp),
		},
		Principals:    principals,
		Maintenance:   adapters.MaintenanceGate{Current: availabilityApp.Queries.CurrentWindow},
		AI:            completer,
		AICredits:     aiApp,
		ImportSweeper: sweeper,
		Media:         mediaApp,
		Tokens:        tokens,
		Docs:          docs,
		Identity:      identityApp,
		Classes:       classesApp,
		Notifications: notificationsApp,
		Organization:  organizationApp,
	}, nil
}

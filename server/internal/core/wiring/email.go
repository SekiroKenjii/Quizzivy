package wiring

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"quizzivy/internal/core/jobs"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	organizationquery "quizzivy/internal/modules/organization/application/query"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/email"
	"quizzivy/internal/platform/secrets"
	"quizzivy/internal/shared/cqrs"
)

const (
	fallbackOrganizationName = "Quizzivy"
	devAppURL                = "http://localhost:5173"
	resendTimeout            = 15 * time.Second
	emailRetry               = time.Minute
	emailBusy                = 10 * time.Second
	emailFloor               = time.Second
)

// Mail is the outgoing mail of one process: the outbox its modules enqueue
// into and the dispatcher that sends the outbox's rows. Dispatcher is nil
// when the process has no sender; the API and the worker both build one.
type Mail struct {
	Outbox     *email.Outbox
	Dispatcher *email.Dispatcher
}

// Run dispatches until ctx ends, on this process's enqueues and on the
// earliest pending row's due time.
func (m Mail) Run(ctx context.Context, logger *slog.Logger) {
	jobs.RunEmail(ctx, logger, m.Dispatcher, jobs.EmailSchedule{
		Wake:  m.Outbox.Wake(),
		Retry: emailRetry, Busy: emailBusy, Floor: emailFloor,
	})
}

// Branding is the organization's branding read as it heads every email.
type Branding = cqrs.QueryHandler[organizationquery.Branding, organizationmodel.Branding]

// BuildMail wires the email platform over cfg: Resend when its group is
// configured and the sink directory otherwise, which the configuration refuses
// in production. Every header names the centre by branding's short name, else
// its name, else Quizzivy. A missing SECRETS_KEY leaves the outbox unable to
// seal a secret and every sealed row failing closed.
func BuildMail(cfg config.Config, dbx db.Context, branding Branding, logger *slog.Logger) (Mail, error) {
	var sealer *secrets.Sealer
	if cfg.SecretsKey != nil {
		var err error
		if sealer, err = secrets.New(cfg.SecretsKey, secrets.PurposeEmailOutbox); err != nil {
			return Mail{}, fmt.Errorf("SECRETS_KEY: %w", err)
		}
	}
	renderer, err := email.NewRenderer()
	if err != nil {
		return Mail{}, err
	}
	var sender email.Sender = email.Sink{Dir: cfg.Email.SinkDir, Logger: logger}
	appURL := cfg.Email.AppURL
	if cfg.Email.Enabled() {
		sender = email.Resend{APIKey: cfg.Email.APIKey.Reveal(), From: cfg.Email.From, Client: &http.Client{Timeout: resendTimeout}}
	} else {
		logger.Warn("email goes to the sink directory, not to a provider", "dir", cfg.Email.SinkDir)
		appURL = devAppURL
	}
	return Mail{
		Outbox: email.NewOutbox(sealer),
		Dispatcher: &email.Dispatcher{
			DB: dbx, Sealer: sealer, Sender: sender, Renderer: renderer,
			Org: organizationName(branding), AppURL: appURL, ReplyTo: cfg.Email.ReplyTo,
			DailyLimit: cfg.Email.DailyLimit, Interval: cfg.Email.SendInterval(),
			Location: cfg.AppLocation, Logger: logger,
		},
	}, nil
}

func organizationName(branding Branding) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		b, err := branding.Handle(ctx, organizationquery.Branding{})
		if err != nil {
			return "", err
		}
		for _, name := range []string{b.ShortName, b.Name} {
			if name != "" {
				return name, nil
			}
		}
		return fallbackOrganizationName, nil
	}
}

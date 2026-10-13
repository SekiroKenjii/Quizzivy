package application

import (
	"log/slog"
	"time"

	"quizzivy/internal/modules/ai/application/command"
	"quizzivy/internal/modules/ai/application/internal/support"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/application/query"
	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/shared/cqrs"
)

// Application is every use case of the credits model: commands change it, queries read it. Reserve, Settle, Release and Finish are the only code that moves a balance on the call path.
type Application struct {
	Commands Commands
	Queries  Queries
	service  *support.Service
}

// SetClock replaces the clock that decides which month it is and which reservations are stale.
func (a *Application) SetClock(now func() time.Time) {
	a.service.Now = now
}

type Commands struct {
	Reserve        cqrs.CommandHandler[command.Reserve, domain.Reservation]
	Settle         cqrs.CommandHandler[command.Settle, domain.Settled]
	Release        cqrs.CommandHandler[command.Release, cqrs.Nothing]
	Finish         cqrs.CommandHandler[command.Finish, domain.Settled]
	ReleaseStale   cqrs.CommandHandler[command.ReleaseStale, domain.Released]
	RequestCredits cqrs.CommandHandler[command.RequestCredits, domain.Request]
	DecideRequest  cqrs.CommandHandler[command.DecideRequest, domain.Decided]
	SetAllowance   cqrs.CommandHandler[command.SetAllowance, domain.TeacherMonth]
	AddCredits     cqrs.CommandHandler[command.AddCredits, domain.Pool]
	SetRules       cqrs.CommandHandler[command.SetRules, domain.RulesChange]
}

type Queries struct {
	Usage        cqrs.QueryHandler[query.Usage, domain.MonthUsage]
	AdminCredits cqrs.QueryHandler[query.AdminCredits, domain.AdminCredits]
}

// Options is what the application takes from outside its module: the price of each tier, the calendar zone months are counted in, the way to tell a user (nil tells no one) and the logger.
type Options struct {
	Prices   domain.Prices
	Zone     *time.Location
	Notifier ports.Notifier
	Logger   *slog.Logger
}

// New builds the application over repo. A nil Zone counts months in Asia/Ho_Chi_Minh's fixed offset.
func New(repo domain.Repository, opts Options) *Application {
	zone := opts.Zone
	if zone == nil {
		zone = time.FixedZone(domain.DefaultZone, 7*60*60)
	}
	service := support.NewService(repo, opts.Prices, zone)
	service.Notifier = opts.Notifier
	if opts.Logger != nil {
		service.Logger = opts.Logger
	}
	return &Application{
		Commands: Commands{
			Reserve:        command.ReserveHandler{Service: service},
			Settle:         command.SettleHandler{Service: service},
			Release:        command.ReleaseHandler{Service: service},
			Finish:         command.FinishHandler{Service: service},
			ReleaseStale:   command.ReleaseStaleHandler{Service: service},
			RequestCredits: command.RequestCreditsHandler{Service: service},
			DecideRequest:  command.DecideRequestHandler{Service: service},
			SetAllowance:   command.SetAllowanceHandler{Service: service},
			AddCredits:     command.AddCreditsHandler{Service: service},
			SetRules:       command.SetRulesHandler{Service: service},
		},
		Queries: Queries{
			Usage:        query.UsageHandler{Service: service},
			AdminCredits: query.AdminCreditsHandler{Service: service},
		},
		service: service,
	}
}

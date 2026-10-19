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

// Application is every use case of the AI assistant. The commands change the credits model and the queries read it. The eight suggestion queries also spend: each reserves its feature's ceiling, calls the provider and settles what it billed, through Reserve and Finish, which with Settle and Release are the only code that moves a balance on the call path. The bank clean-up makes several calls under its one reservation and ends it with one Finish of their summed usage. No query writes a grade, a question or a comment.
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
	Generate     cqrs.QueryHandler[query.Generate, query.Generated]
	Check        cqrs.QueryHandler[query.Check, query.Checked]
	Variant      cqrs.QueryHandler[query.Variant, query.Varied]
	Explanation  cqrs.QueryHandler[query.Explanation, query.Explained]
	Distractors  cqrs.QueryHandler[query.Distractors, query.Suggested]
	Score        cqrs.QueryHandler[query.Score, query.Scored]
	Insights     cqrs.QueryHandler[query.Insights, query.Interpreted]
	Cleanup      cqrs.QueryHandler[query.Cleanup, query.Cleaned]
}

// Options is what the application takes from outside its module: the price of each tier, the calendar zone months are counted in, the way to tell a user (nil tells no one), the logger and the time one bank clean-up or one variant may spend with the provider (zero is support.DefaultCleanupBudget). Completer is the provider and nil when none is configured, so every suggestion answers domain.ErrOff; Tests, Questions, Grading, Analysis and People are what the suggestions read.
type Options struct {
	Prices        domain.Prices
	Zone          *time.Location
	Notifier      ports.Notifier
	Logger        *slog.Logger
	CleanupBudget time.Duration
	Completer     ports.Completer
	Tests         ports.Tests
	Questions     ports.Questions
	Grading       ports.Grading
	Analysis      ports.Analysis
	People        ports.People
}

// New builds the application over repo. A nil Zone counts months in Asia/Ho_Chi_Minh's fixed offset.
func New(repo domain.Repository, opts Options) *Application {
	zone := opts.Zone
	if zone == nil {
		zone = time.FixedZone(domain.DefaultZone, 7*60*60)
	}
	service := support.NewService(repo, opts.Prices, zone)
	service.Notifier = opts.Notifier
	service.Completer, service.Tests, service.Questions = opts.Completer, opts.Tests, opts.Questions
	service.Grading, service.Analysis, service.People = opts.Grading, opts.Analysis, opts.People
	if opts.Logger != nil {
		service.Logger = opts.Logger
	}
	if opts.CleanupBudget > 0 {
		service.CleanupBudget = opts.CleanupBudget
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
			Generate:     query.GenerateHandler{Service: service},
			Check:        query.CheckHandler{Service: service},
			Variant:      query.VariantHandler{Service: service},
			Explanation:  query.ExplanationHandler{Service: service},
			Distractors:  query.DistractorsHandler{Service: service},
			Score:        query.ScoreHandler{Service: service},
			Insights:     query.InsightsHandler{Service: service},
			Cleanup:      query.CleanupHandler{Service: service},
		},
		service: service,
	}
}

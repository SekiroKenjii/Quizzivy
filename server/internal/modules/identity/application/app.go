package application

import (
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/stats"
	"time"
)

// Application is every use case of the module: commands change it, queries read it.
type Application struct {
	Commands Commands
	Queries  Queries
	service  *support.Service
	students *support.Students
}

func (a *Application) SetClock(now func() time.Time) {
	a.service.SetClock(now)
}

func (a *Application) SetGoogle(p ports.GoogleProvider, enroller ports.SelfEnroller) {
	a.service.SetGoogle(p, enroller)
}

type Commands struct {
	ChangePassword       cqrs.CommandHandler[command.ChangePassword, cqrs.Nothing]
	CreateStudent        cqrs.CommandHandler[command.CreateStudent, command.CreateStudentResult]
	GoogleSignIn         cqrs.CommandHandler[command.GoogleSignIn, model.GoogleSignInResult]
	LinkGoogle           cqrs.CommandHandler[command.LinkGoogle, domain.User]
	Login                cqrs.CommandHandler[command.Login, model.Session]
	Logout               cqrs.CommandHandler[command.Logout, cqrs.Nothing]
	PruneExpiredTokens   cqrs.CommandHandler[command.PruneExpiredTokens, int64]
	Refresh              cqrs.CommandHandler[command.Refresh, model.RefreshResult]
	Rename               cqrs.CommandHandler[command.Rename, domain.User]
	ResetStudentPassword cqrs.CommandHandler[command.ResetStudentPassword, string]
	UnlinkGoogle         cqrs.CommandHandler[command.UnlinkGoogle, cqrs.Nothing]
	UpdateStudent        cqrs.CommandHandler[command.UpdateStudent, domain.Student]
}

type Queries struct {
	CurrentUser   cqrs.QueryHandler[query.CurrentUser, domain.User]
	GetStudent    cqrs.QueryHandler[query.GetStudent, domain.Student]
	ListStudents  cqrs.QueryHandler[query.ListStudents, query.ListStudentsResult]
	StudentFacets cqrs.QueryHandler[query.StudentFacets, domain.StudentFacets]
}

func New(users domain.Users, tokens *token.Issuer, refreshTTL time.Duration, repo domain.Students, stats stats.Source) *Application {
	service := support.NewService(users, tokens, refreshTTL)
	students := support.NewStudents(repo, stats)
	return &Application{
		Commands: Commands{
			ChangePassword:       command.ChangePasswordHandler{Service: service},
			CreateStudent:        command.CreateStudentHandler{Students: students},
			GoogleSignIn:         command.GoogleSignInHandler{Service: service},
			LinkGoogle:           command.LinkGoogleHandler{Service: service},
			Login:                command.LoginHandler{Service: service},
			Logout:               command.LogoutHandler{Service: service},
			PruneExpiredTokens:   command.PruneExpiredTokensHandler{Service: service},
			Refresh:              command.RefreshHandler{Service: service},
			Rename:               command.RenameHandler{Service: service},
			ResetStudentPassword: command.ResetStudentPasswordHandler{Students: students},
			UnlinkGoogle:         command.UnlinkGoogleHandler{Service: service},
			UpdateStudent:        command.UpdateStudentHandler{Students: students},
		},
		Queries: Queries{
			CurrentUser:   query.CurrentUserHandler{Service: service},
			GetStudent:    query.GetStudentHandler{Students: students},
			ListStudents:  query.ListStudentsHandler{Students: students},
			StudentFacets: query.StudentFacetsHandler{Students: students},
		},
		service:  service,
		students: students,
	}
}

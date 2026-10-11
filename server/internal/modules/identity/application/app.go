package application

import (
	"context"
	"log/slog"
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

// SetAvatars attaches profile photos: the object store that keeps them and the
// processor that makes them from an upload. Without both, setAvatar answers
// 501 and no avatarUrl is signed; deleteAvatar still clears the key.
func (a *Application) SetAvatars(store ports.ObjectStore, photos ports.PhotoProcessor, logger *slog.Logger) {
	a.service.SetAvatars(store, photos, logger)
}

// SetPrincipals attaches the access module's principal cache, so a disable or
// a password reset takes effect on this machine's next request, and a new
// session shows the user the permissions their role holds. Without it,
// sessions carry none.
func (a *Application) SetPrincipals(p ports.Principals) {
	a.students.Principals = p
	a.service.Principals = p
}

// SetTemporaryPasswords replaces how a student's temporary password and its hash are made. Tests only.
func (a *Application) SetTemporaryPasswords(source func(ctx context.Context) (password, hash string, err error)) {
	a.students.Password = source
}

// SetLogger attaches the logger the students commands report faults to. Without it they log nowhere.
func (a *Application) SetLogger(logger *slog.Logger) {
	a.students.Logger = logger
}

type Commands struct {
	DeleteStudent          cqrs.CommandHandler[command.DeleteStudent, cqrs.Nothing]
	RemoveAvatar           cqrs.CommandHandler[command.RemoveAvatar, domain.User]
	SetAvatar              cqrs.CommandHandler[command.SetAvatar, domain.User]
	ChangePassword         cqrs.CommandHandler[command.ChangePassword, cqrs.Nothing]
	CreateStudent          cqrs.CommandHandler[command.CreateStudent, command.CreateStudentResult]
	GoogleSignIn           cqrs.CommandHandler[command.GoogleSignIn, model.GoogleSignInResult]
	LinkGoogle             cqrs.CommandHandler[command.LinkGoogle, domain.User]
	Login                  cqrs.CommandHandler[command.Login, model.Session]
	Logout                 cqrs.CommandHandler[command.Logout, cqrs.Nothing]
	PruneExpiredTokens     cqrs.CommandHandler[command.PruneExpiredTokens, int64]
	Refresh                cqrs.CommandHandler[command.Refresh, model.RefreshResult]
	UpdateProfile          cqrs.CommandHandler[command.UpdateProfile, domain.User]
	UpdatePreferences      cqrs.CommandHandler[command.UpdatePreferences, domain.Preferences]
	ResetStudentPassword   cqrs.CommandHandler[command.ResetStudentPassword, string]
	ResetStudentsPasswords cqrs.CommandHandler[command.ResetStudentsPasswords, domain.BulkReset]
	RevokeOtherSessions    cqrs.CommandHandler[command.RevokeOtherSessions, int]
	RevokeSession          cqrs.CommandHandler[command.RevokeSession, cqrs.Nothing]
	UnlinkGoogle           cqrs.CommandHandler[command.UnlinkGoogle, cqrs.Nothing]
	UpdateStudent          cqrs.CommandHandler[command.UpdateStudent, domain.Student]
}

type Queries struct {
	AvatarURL      cqrs.QueryHandler[query.AvatarURL, string]
	CurrentUser    cqrs.QueryHandler[query.CurrentUser, domain.User]
	EffectiveZone  cqrs.QueryHandler[query.EffectiveZone, string]
	GetStudent     cqrs.QueryHandler[query.GetStudent, domain.Student]
	ListSessions   cqrs.QueryHandler[query.ListSessions, []domain.Session]
	ListStudents   cqrs.QueryHandler[query.ListStudents, query.ListStudentsResult]
	StudentAccount cqrs.QueryHandler[query.StudentAccount, domain.Account]
	StudentFacets  cqrs.QueryHandler[query.StudentFacets, domain.StudentFacets]
}

func New(users domain.Users, tokens *token.Issuer, refreshTTL time.Duration, repo domain.Students, stats stats.Source) *Application {
	service := support.NewService(users, tokens, refreshTTL)
	students := support.NewStudents(repo, stats)
	return &Application{
		Commands: Commands{
			DeleteStudent:          command.DeleteStudentHandler{Students: students},
			RemoveAvatar:           command.RemoveAvatarHandler{Service: service},
			SetAvatar:              command.SetAvatarHandler{Service: service},
			ChangePassword:         command.ChangePasswordHandler{Service: service},
			CreateStudent:          command.CreateStudentHandler{Students: students},
			GoogleSignIn:           command.GoogleSignInHandler{Service: service},
			LinkGoogle:             command.LinkGoogleHandler{Service: service},
			Login:                  command.LoginHandler{Service: service},
			Logout:                 command.LogoutHandler{Service: service},
			PruneExpiredTokens:     command.PruneExpiredTokensHandler{Service: service},
			Refresh:                command.RefreshHandler{Service: service},
			UpdateProfile:          command.UpdateProfileHandler{Service: service},
			UpdatePreferences:      command.UpdatePreferencesHandler{Service: service},
			ResetStudentPassword:   command.ResetStudentPasswordHandler{Students: students},
			ResetStudentsPasswords: command.ResetStudentsPasswordsHandler{Students: students},
			RevokeOtherSessions:    command.RevokeOtherSessionsHandler{Service: service},
			RevokeSession:          command.RevokeSessionHandler{Service: service},
			UnlinkGoogle:           command.UnlinkGoogleHandler{Service: service},
			UpdateStudent:          command.UpdateStudentHandler{Students: students},
		},
		Queries: Queries{
			AvatarURL:      query.AvatarURLHandler{Service: service},
			CurrentUser:    query.CurrentUserHandler{Service: service},
			EffectiveZone:  query.EffectiveZoneHandler{Service: service},
			GetStudent:     query.GetStudentHandler{Students: students},
			ListSessions:   query.ListSessionsHandler{Service: service},
			ListStudents:   query.ListStudentsHandler{Students: students},
			StudentAccount: query.StudentAccountHandler{Students: students},
			StudentFacets:  query.StudentFacetsHandler{Students: students},
		},
		service:  service,
		students: students,
	}
}

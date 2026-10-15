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
	activity *support.Activity
}

func (a *Application) SetClock(now func() time.Time) {
	a.service.SetClock(now)
	if a.activity != nil {
		a.activity.Now = now
	}
}

// SetActivity starts recording who was active, counting days in zone, and makes Commands.StampActivity and Commands.PruneActivityDays available. Without it no request stamps anything and nothing is pruned. RunActivity writes what it collects.
func (a *Application) SetActivity(store domain.Activity, zone *time.Location, logger *slog.Logger) {
	a.activity = support.NewActivity(store, zone)
	a.activity.Log = logger
	a.activity.Now = a.service.Now
	a.Commands.StampActivity = command.StampActivityHandler{Activity: a.activity}
	a.Commands.PruneActivityDays = command.PruneActivityDaysHandler{Activity: a.activity}
}

// RunActivity writes the activity the application collects, every few seconds while any waits, until ctx ends. It returns at once without SetActivity.
func (a *Application) RunActivity(ctx context.Context) {
	if a.activity != nil {
		a.activity.Run(ctx)
	}
}

// FlushActivity writes what the application has collected now.
func (a *Application) FlushActivity(ctx context.Context) {
	if a.activity != nil {
		a.activity.Flush(ctx)
	}
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

// SetSignInPolicy attaches how the centre lets people sign in: password and
// Google sign-in on or off, the Google domains allowed, and whether a
// temporary password must be changed at the first sign-in. Without it every
// way in is open and the change is forced.
func (a *Application) SetSignInPolicy(p ports.SignInPolicy) {
	a.service.Policy = p
	a.students.Policy = p
}

// SetPasswordLinks attaches the store of set-password links, which opens
// issuing them, checking one and using one, and makes Commands.PruneSetPasswordLinks
// available. Without it no link can be issued, every link a caller presents is
// invalid and nothing is pruned.
func (a *Application) SetPasswordLinks(links domain.SetPasswordLinks) {
	a.service.Links = links
	a.Commands.PruneSetPasswordLinks = command.PruneSetPasswordLinksHandler{Service: a.service}
}

// SetAdminUsers attaches the Admin console's account directory and the access
// module's say on which roles a caller may give, which opens createUser,
// listUsers and getUser.
// Without it Commands.CreateUser, Queries.ListUsers and Queries.GetUser are
// nil and their operations answer 501.
func (a *Application) SetAdminUsers(repo domain.AdminUsers, roles ports.AssignableRole) {
	admin := &support.Admin{Repo: repo, Roles: roles, Service: a.service, Students: a.students}
	a.Commands.CreateUser = command.CreateUserHandler{Admin: admin}
	a.Queries.ListUsers = query.ListUsersHandler{Admin: admin}
	a.Queries.GetUser = query.GetUserHandler{Admin: admin}
}

// SetUserImports attaches the store of user imports, the access module's say on
// which roles a caller may give, the reader of CSV and XLSX files, and the
// worker's wake signal, which opens previewUserImport, createUserImport and
// getUserImport, and downloadUserImportSheet, and makes
// Commands.EraseExpiredImportSheets available. budget and wake may be nil;
// sealer is nil on a server without SECRETS_KEY, which can deliver neither a
// link by email nor a hand-out sheet. Without it Commands.PreviewUserImport,
// Commands.CreateUserImport, Commands.DownloadUserImportSheet,
// Commands.EraseExpiredImportSheets and Queries.GetUserImport are nil and their
// operations answer 501.
func (a *Application) SetUserImports(repo domain.UserImports, roles ports.RoleChoices, tables ports.TableReader, budget ports.EmailBudget, wake ports.Wake, sealer ports.Sealer) {
	imports := support.NewImports(repo, roles, tables, budget, wake, a.service, sealer)
	a.Commands.PreviewUserImport = command.PreviewUserImportHandler{Imports: imports}
	a.Commands.CreateUserImport = command.CreateUserImportHandler{Imports: imports}
	a.Commands.DownloadUserImportSheet = command.DownloadUserImportSheetHandler{Imports: imports}
	a.Commands.EraseExpiredImportSheets = command.EraseExpiredImportSheetsHandler{Imports: imports}
	a.Queries.GetUserImport = query.GetUserImportHandler{Imports: imports}
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
	DeleteStudent            cqrs.CommandHandler[command.DeleteStudent, cqrs.Nothing]
	RemoveAvatar             cqrs.CommandHandler[command.RemoveAvatar, domain.User]
	SetAvatar                cqrs.CommandHandler[command.SetAvatar, domain.User]
	ChangePassword           cqrs.CommandHandler[command.ChangePassword, cqrs.Nothing]
	CreateStudent            cqrs.CommandHandler[command.CreateStudent, command.CreateStudentResult]
	CreateUser               cqrs.CommandHandler[command.CreateUser, command.CreateUserResult]
	CreateUserImport         cqrs.CommandHandler[command.CreateUserImport, command.CreateUserImportResult]
	DownloadUserImportSheet  cqrs.CommandHandler[command.DownloadUserImportSheet, []domain.SheetRow]
	EraseExpiredImportSheets cqrs.CommandHandler[command.EraseExpiredImportSheets, int]
	GoogleSignIn             cqrs.CommandHandler[command.GoogleSignIn, model.GoogleSignInResult]
	IssueSetPasswordLink     cqrs.CommandHandler[command.IssueSetPasswordLink, domain.IssuedLink]
	LinkGoogle               cqrs.CommandHandler[command.LinkGoogle, domain.User]
	PreviewUserImport        cqrs.CommandHandler[command.PreviewUserImport, command.UserImportPreview]
	Login                    cqrs.CommandHandler[command.Login, model.Session]
	Logout                   cqrs.CommandHandler[command.Logout, cqrs.Nothing]
	PruneActivityDays        cqrs.CommandHandler[command.PruneActivityDays, int]
	PruneExpiredTokens       cqrs.CommandHandler[command.PruneExpiredTokens, int64]
	PruneSetPasswordLinks    cqrs.CommandHandler[command.PruneSetPasswordLinks, int]
	Refresh                  cqrs.CommandHandler[command.Refresh, model.RefreshResult]
	RequestSetPasswordLink   cqrs.CommandHandler[command.RequestSetPasswordLink, cqrs.Nothing]
	SetPasswordWithLink      cqrs.CommandHandler[command.SetPasswordWithLink, model.Session]
	StampActivity            cqrs.CommandHandler[command.StampActivity, cqrs.Nothing]
	UpdateProfile            cqrs.CommandHandler[command.UpdateProfile, domain.User]
	UpdatePreferences        cqrs.CommandHandler[command.UpdatePreferences, domain.Preferences]
	ResetStudentPassword     cqrs.CommandHandler[command.ResetStudentPassword, string]
	ResetStudentsPasswords   cqrs.CommandHandler[command.ResetStudentsPasswords, domain.BulkReset]
	RevokeOtherSessions      cqrs.CommandHandler[command.RevokeOtherSessions, int]
	RevokeSession            cqrs.CommandHandler[command.RevokeSession, cqrs.Nothing]
	UnlinkGoogle             cqrs.CommandHandler[command.UnlinkGoogle, cqrs.Nothing]
	UpdateStudent            cqrs.CommandHandler[command.UpdateStudent, domain.Student]
}

type Queries struct {
	AvatarURL            cqrs.QueryHandler[query.AvatarURL, string]
	CheckSetPasswordLink cqrs.QueryHandler[query.CheckSetPasswordLink, domain.SetPasswordLink]
	CurrentUser          cqrs.QueryHandler[query.CurrentUser, domain.User]
	EffectiveZone        cqrs.QueryHandler[query.EffectiveZone, string]
	GetStudent           cqrs.QueryHandler[query.GetStudent, domain.Student]
	ListSessions         cqrs.QueryHandler[query.ListSessions, []domain.Session]
	ListStudents         cqrs.QueryHandler[query.ListStudents, query.ListStudentsResult]
	ListUsers            cqrs.QueryHandler[query.ListUsers, query.ListUsersResult]
	GetUser              cqrs.QueryHandler[query.GetUser, query.GetUserResult]
	GetUserImport        cqrs.QueryHandler[query.GetUserImport, query.GetUserImportResult]
	StudentAccount       cqrs.QueryHandler[query.StudentAccount, domain.Account]
	StudentFacets        cqrs.QueryHandler[query.StudentFacets, domain.StudentFacets]
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
			IssueSetPasswordLink:   command.IssueSetPasswordLinkHandler{Service: service},
			LinkGoogle:             command.LinkGoogleHandler{Service: service},
			Login:                  command.LoginHandler{Service: service},
			Logout:                 command.LogoutHandler{Service: service},
			PruneExpiredTokens:     command.PruneExpiredTokensHandler{Service: service},
			Refresh:                command.RefreshHandler{Service: service},
			RequestSetPasswordLink: command.RequestSetPasswordLinkHandler{Service: service},
			SetPasswordWithLink:    command.SetPasswordWithLinkHandler{Service: service},
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
			AvatarURL:            query.AvatarURLHandler{Service: service},
			CheckSetPasswordLink: query.CheckSetPasswordLinkHandler{Service: service},
			CurrentUser:          query.CurrentUserHandler{Service: service},
			EffectiveZone:        query.EffectiveZoneHandler{Service: service},
			GetStudent:           query.GetStudentHandler{Students: students},
			ListSessions:         query.ListSessionsHandler{Service: service},
			ListStudents:         query.ListStudentsHandler{Students: students},
			StudentAccount:       query.StudentAccountHandler{Students: students},
			StudentFacets:        query.StudentFacetsHandler{Students: students},
		},
		service:  service,
		students: students,
	}
}

package application

import (
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/stats"
	"time"
)

// Application is every use case of the module: commands change it, queries read it.
type Application struct {
	Commands  Commands
	Queries   Queries
	enrolment *support.Enrolment
	service   *support.Service
}

func (a *Application) SetClock(now func() time.Time) {
	a.enrolment.SetClock(now)
}

type Commands struct {
	AddMember      cqrs.CommandHandler[command.AddMember, domain.Member]
	Archive        cqrs.CommandHandler[command.Archive, domain.Class]
	Create         cqrs.CommandHandler[command.Create, domain.Class]
	EnrolExisting  cqrs.CommandHandler[command.EnrolExisting, domain.EnrolResult]
	EnrolNewMember cqrs.CommandHandler[command.EnrolNewMember, domain.EnrolResult]
	RemoveMember   cqrs.CommandHandler[command.RemoveMember, cqrs.Nothing]
	Revoke         cqrs.CommandHandler[command.Revoke, cqrs.Nothing]
	Rotate         cqrs.CommandHandler[command.Rotate, domain.Rotated]
	Update         cqrs.CommandHandler[command.Update, domain.Class]
}

type Queries struct {
	ActiveCode cqrs.QueryHandler[query.ActiveCode, *domain.IssuedCode]
	Facets     cqrs.QueryHandler[query.Facets, domain.Facets]
	Get        cqrs.QueryHandler[query.Get, domain.Class]
	List       cqrs.QueryHandler[query.List, query.ListResult]
	ListMine   cqrs.QueryHandler[query.ListMine, []domain.MyClass]
	Members    cqrs.QueryHandler[query.Members, query.MembersResult]
	Preview    cqrs.QueryHandler[query.Preview, domain.PreviewResult]
}

func New(repo domain.Repository, stats stats.Source) *Application {
	enrolment := support.NewEnrolment(repo)
	service := support.NewService(repo, stats)
	return &Application{
		Commands: Commands{
			AddMember:      command.AddMemberHandler{Service: service},
			Archive:        command.ArchiveHandler{Service: service},
			Create:         command.CreateHandler{Service: service},
			EnrolExisting:  command.EnrolExistingHandler{Enrolment: enrolment},
			EnrolNewMember: command.EnrolNewMemberHandler{Enrolment: enrolment},
			RemoveMember:   command.RemoveMemberHandler{Service: service},
			Revoke:         command.RevokeHandler{Enrolment: enrolment},
			Rotate:         command.RotateHandler{Enrolment: enrolment},
			Update:         command.UpdateHandler{Service: service},
		},
		Queries: Queries{
			ActiveCode: query.ActiveCodeHandler{Enrolment: enrolment},
			Facets:     query.FacetsHandler{Service: service},
			Get:        query.GetHandler{Service: service},
			List:       query.ListHandler{Service: service},
			ListMine:   query.ListMineHandler{Service: service},
			Members:    query.MembersHandler{Service: service},
			Preview:    query.PreviewHandler{Enrolment: enrolment},
		},
		enrolment: enrolment,
		service:   service,
	}
}

package application

import (
	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/internal/support"
	"quizzivy/internal/modules/attempts/application/ports"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/cqrs"
)

// Application is every use case of the module: commands change it, queries read it.
type Application struct {
	Commands  Commands
	Queries   Queries
	integrity *support.Integrity
	review    *support.Review
	service   *support.Service
}

// WithGroupContexts supplies the frozen shared-context reader before serving attempts.
func (a *Application) WithGroupContexts(groups ports.GroupContexts) *Application {
	a.service.Groups = groups
	return a
}

type Commands struct {
	ExpireDue       cqrs.CommandHandler[command.ExpireDue, cqrs.Nothing]
	Extend          cqrs.CommandHandler[command.Extend, domain.Attempt]
	Finish          cqrs.CommandHandler[command.Finish, domain.Attempt]
	Flag            cqrs.CommandHandler[command.Flag, domain.Attempt]
	Flush           cqrs.CommandHandler[command.Flush, cqrs.Nothing]
	Grade           cqrs.CommandHandler[command.Grade, domain.Score]
	RecordPlay      cqrs.CommandHandler[command.RecordPlay, domain.Plays]
	RecordGroupPlay cqrs.CommandHandler[command.RecordGroupPlay, domain.GroupPlays]
	Reset           cqrs.CommandHandler[command.Reset, domain.Attempt]
	Save            cqrs.CommandHandler[command.Save, domain.SaveResult]
	SetNote         cqrs.CommandHandler[command.SetNote, cqrs.Nothing]
	StartOrResume   cqrs.CommandHandler[command.StartOrResume, domain.Session]
	Submit          cqrs.CommandHandler[command.Submit, domain.Attempt]
	Void            cqrs.CommandHandler[command.Void, domain.Attempt]
}

type Queries struct {
	AnswersForQuestion cqrs.QueryHandler[query.AnswersForQuestion, domain.ByQuestion]
	Get                cqrs.QueryHandler[query.Get, domain.Session]
	Monitor            cqrs.QueryHandler[query.Monitor, domain.Monitor]
	Result             cqrs.QueryHandler[query.Result, domain.Result]
	Review             cqrs.QueryHandler[query.Review, domain.Review]
	Timeline           cqrs.QueryHandler[query.Timeline, domain.Timeline]
}

func New(repo domain.TimelineRepository, reviewRepo domain.ReviewRepository, store domain.Repository) *Application {
	integrity := support.NewIntegrity(repo)
	review := support.NewReview(reviewRepo)
	service := support.NewService(store)
	return &Application{
		Commands: Commands{
			ExpireDue:       command.ExpireDueHandler{Service: service},
			Extend:          command.ExtendHandler{Service: service},
			Finish:          command.FinishHandler{Review: review},
			Flag:            command.FlagHandler{Service: service},
			Flush:           command.FlushHandler{Service: service},
			Grade:           command.GradeHandler{Review: review},
			RecordPlay:      command.RecordPlayHandler{Service: service},
			RecordGroupPlay: command.RecordGroupPlayHandler{Service: service},
			Reset:           command.ResetHandler{Service: service},
			Save:            command.SaveHandler{Service: service},
			SetNote:         command.SetNoteHandler{Review: review},
			StartOrResume:   command.StartOrResumeHandler{Service: service},
			Submit:          command.SubmitHandler{Service: service},
			Void:            command.VoidHandler{Service: service},
		},
		Queries: Queries{
			AnswersForQuestion: query.AnswersForQuestionHandler{Review: review},
			Get:                query.GetHandler{Service: service},
			Monitor:            query.MonitorHandler{Service: service},
			Result:             query.ResultHandler{Service: service},
			Review:             query.ReviewHandler{Review: review},
			Timeline:           query.TimelineHandler{Integrity: integrity},
		},
		integrity: integrity,
		review:    review,
		service:   service,
	}
}

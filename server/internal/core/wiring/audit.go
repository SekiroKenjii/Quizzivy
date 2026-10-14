package wiring

import (
	"log/slog"

	"quizzivy/internal/core/adapters"
	auditapp "quizzivy/internal/modules/audit/application"
	audithttp "quizzivy/internal/modules/audit/http"
	auditrepo "quizzivy/internal/modules/audit/repositories"
	identityapp "quizzivy/internal/modules/identity/application"
	"quizzivy/internal/platform/db"
)

func audit(dbx db.Context, identity *identityapp.Application, logger *slog.Logger) (*auditapp.Application, audithttp.Audit) {
	app := auditapp.New(auditrepo.NewPostgres(dbx)).
		WithZones(adapters.ProfileZone{Query: identity.Queries.EffectiveZone}).
		WithAvatars(adapters.TeacherPhotos{Query: identity.Queries.AvatarURL})
	return app, audithttp.NewAudit(app).WithLogger(logger)
}

package wiring

import (
	"context"

	accessapp "quizzivy/internal/modules/access/application"
	accessrepo "quizzivy/internal/modules/access/repositories"
	"quizzivy/internal/platform/db"
)

func accessModule(ctx context.Context, dbx db.Context) (*accessapp.Application, error) {
	repo := accessrepo.NewPostgres(dbx)
	app := accessapp.New(repo).WithRoleStore(repo)
	if err := app.CheckCatalogue(ctx); err != nil {
		return nil, err
	}
	return app, nil
}

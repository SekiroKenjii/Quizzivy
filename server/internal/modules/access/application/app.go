// Package application is the access use cases: who a request acts as, what a
// role's users may do, and the Roles & permissions matrix.
package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"quizzivy/internal/modules/access/application/internal/support"
	"quizzivy/internal/modules/access/application/model"
	"quizzivy/internal/modules/access/application/query"
	"quizzivy/internal/modules/access/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

// ErrCatalogueBehind means the database lacks a permission this binary was
// compiled with: its migrations have not run.
var ErrCatalogueBehind = errors.New("access: app.permissions lacks a compiled key")

// Application is every use case of the module.
type Application struct {
	Queries Queries
	service *support.Service
	repo    domain.Repository
}

// Queries are the module's reads.
type Queries struct {
	ResolvePrincipal cqrs.QueryHandler[query.ResolvePrincipal, access.Principal]
	RolePermissions  cqrs.QueryHandler[query.RolePermissions, access.Set]
	Matrix           cqrs.QueryHandler[query.Matrix, []model.RoleGrants]
}

// New builds the application over the repository, with one principal cache
// shared by every caller. The Admin's wildcard expands to the compiled
// catalogue.
func New(repo domain.Repository) *Application {
	service := support.NewService(repo, access.All())
	return &Application{
		Queries: Queries{
			ResolvePrincipal: query.ResolvePrincipalHandler{Service: service},
			RolePermissions:  query.RolePermissionsHandler{Service: service},
			Matrix:           query.MatrixHandler{Repo: repo, Service: service},
		},
		service: service,
		repo:    repo,
	}
}

// CheckCatalogue refuses when app.permissions lacks a key this binary was
// compiled with. Extra rows, written by a newer migration during a rollout,
// are ignored, so an older machine keeps serving.
func (a *Application) CheckCatalogue(ctx context.Context) error {
	stored, err := a.repo.Catalogue(ctx)
	if err != nil {
		return fmt.Errorf("access: read the catalogue: %w", err)
	}
	var missing []access.Key
	for _, k := range access.All() {
		if !slices.Contains(stored, k) {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %v", ErrCatalogueBehind, missing)
	}
	return nil
}

// Forget drops a user's cached principal on this machine, so a write that
// changed the user's access applies on the next request.
func (a *Application) Forget(userID string) { a.service.Forget(userID) }

// ForgetAll drops every cached principal and role grant on this machine, for a
// write that changed a role's grants.
func (a *Application) ForgetAll() { a.service.ForgetAll() }

// SetClock replaces the principal cache's time source. Tests only.
func (a *Application) SetClock(now func() time.Time) { a.service.SetClock(now) }

// SetLimit replaces the principal cache's bound. Tests only.
func (a *Application) SetLimit(limit int) { a.service.SetLimit(limit) }

// Cached returns how many principals are cached. Tests only.
func (a *Application) Cached() int { return a.service.Len() }

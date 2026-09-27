// Package http is the availability module's transport: GET /public/status.
package http

import (
	"context"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/availability/application"
	"quizzivy/internal/modules/availability/application/query"
	"quizzivy/internal/platform/httpx"
)

const cacheControl = "public, max-age=30"

// Availability serves the public maintenance status.
type Availability struct {
	app *application.Application
}

// NewAvailability builds the transport. A nil application answers 501.
func NewAvailability(app *application.Application) Availability {
	return Availability{app: app}
}

// GetPublicStatus implements GET /public/status: the next window that has not
// ended, or null, cacheable for as long as the snapshot behind it.
func (h Availability) GetPublicStatus(ctx context.Context, _ openapi.GetPublicStatusRequestObject) (openapi.GetPublicStatusResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	status, err := h.app.Queries.CurrentWindow.Handle(ctx, query.CurrentWindow{})
	if err != nil {
		return nil, err
	}
	body := openapi.PublicStatus{}
	if status.Window != nil {
		body.Maintenance = &openapi.MaintenanceWindow{
			StartsAt: status.Window.StartsAt.UTC(),
			EndsAt:   status.Window.EndsAt.UTC(),
			Active:   status.Active,
		}
	}
	return openapi.GetPublicStatus200JSONResponse{
		Body:    body,
		Headers: openapi.GetPublicStatus200ResponseHeaders{CacheControl: cacheControl},
	}, nil
}

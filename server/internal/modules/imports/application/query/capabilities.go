package query

import (
	"context"
	"quizzivy/internal/modules/imports/domain"
)

type Capabilities struct{}

// CapabilitiesResult states what this deployment does with an import once intake is configured.
type CapabilitiesResult struct {
	Processing bool
	Retention  domain.Retention
}

type CapabilitiesHandler struct {
	Processing bool
	Retention  domain.Retention
}

func (h CapabilitiesHandler) Handle(context.Context, Capabilities) (CapabilitiesResult, error) {
	return CapabilitiesResult(h), nil
}

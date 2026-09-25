package query

import "context"

type Capabilities struct{}

// CapabilitiesResult states what this deployment does with an import once intake is configured.
type CapabilitiesResult struct {
	Processing bool
}

type CapabilitiesHandler struct{ Processing bool }

func (h CapabilitiesHandler) Handle(context.Context, Capabilities) (CapabilitiesResult, error) {
	return CapabilitiesResult(h), nil
}

package httpx

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
)

// ValidateRequests checks every incoming request against api/openapi.yaml.
//
// Without this, the contract's constraints are decorative on the server side.
// oapi-codegen generates types and binds JSON; it does not enforce minLength,
// maxLength, format, enum, required, or additionalProperties. A `password` with
// `minLength: 8` accepted an empty string, and `email` with `format: email`
// accepted anything at all -- every endpoint had to re-state its own rules in
// Go, or silently not have any.
//
// Authentication is deliberately NOT delegated here. The validator will happily
// enforce security requirements, but it knows nothing about our tokens, so it
// would either reject everything or need a second copy of RequireAuth. The
// AuthenticationFunc therefore always succeeds: by the time a request reaches
// the validator, RequireAuth has already run.
func ValidateRequests(spec *openapi3.T) (func(http.Handler) http.Handler, error) {
	// The spec's `servers` block describes production URLs. Left in place, the
	// router refuses every request whose host is not one of them -- which is
	// every request in development and in tests.
	stripped := *spec
	stripped.Servers = nil

	// Built here only to turn a bad spec into a returned error. The middleware
	// constructor builds its own and PANICS on failure, which would surface as
	// a crash at the first request rather than a refusal to start.
	if _, err := gorillamux.NewRouter(&stripped); err != nil {
		return nil, err
	}

	return nethttpmiddleware.OapiRequestValidatorWithOptions(&stripped,
		&nethttpmiddleware.Options{
			Options: openapi3filter.Options{
				AuthenticationFunc: func(context.Context, *openapi3filter.AuthenticationInput) error {
					return nil
				},
			},
			ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, r *http.Request, opts nethttpmiddleware.ErrorHandlerOpts) {
				if opts.StatusCode == http.StatusNotFound {
					// The mux already matched a route, so a 404 here means the
					// two routers disagree. Say so rather than pretending the
					// endpoint does not exist.
					WriteError(w, r, http.StatusNotFound, CodeNotFound, "Không tìm thấy đường dẫn.")
					return
				}
				WriteError(w, r, http.StatusBadRequest, CodeValidationFailed, validationMessage(err))
			},
		}), nil
}

// validationMessage turns kin-openapi's error into something a person can act
// on. Its default rendering embeds the whole failing schema, which is both
// unreadable and hands an anonymous caller the internals of the contract.
//
// Every field here is optional in practice: a body error has no Parameter, a
// parameter error has no schema pointer, and an unparseable body has neither.
// Dereferencing without checking turns a malformed request into a panic, which
// is a denial of service that any anonymous caller can trigger at will.
func validationMessage(err error) string {
	const generic = "Dữ liệu gửi lên không hợp lệ."

	var reqErr *openapi3filter.RequestError
	if !errors.As(err, &reqErr) {
		return generic
	}

	if field := failingField(reqErr); field != "" {
		return "Trường \"" + field + "\" không hợp lệ."
	}
	if reqErr.Parameter != nil {
		return "Tham số \"" + reqErr.Parameter.Name + "\" không hợp lệ."
	}
	return generic
}

// failingField names the offending field, preferring the JSON pointer from the
// schema error because it locates a field inside the body; a parameter name
// only applies when the failure was in the path or query string.
func failingField(reqErr *openapi3filter.RequestError) string {
	var schemaErr *openapi3.SchemaError
	if errors.As(reqErr.Err, &schemaErr) {
		if pointer := schemaErr.JSONPointer(); len(pointer) > 0 {
			return strings.Join(pointer, ".")
		}
	}
	// A MultiError arrives when several fields fail at once. Naming the first
	// is more useful than naming none, and the client re-submits anyway.
	var multi openapi3.MultiError
	if errors.As(reqErr.Err, &multi) {
		for _, e := range multi {
			var se *openapi3.SchemaError
			if errors.As(e, &se) {
				if pointer := se.JSONPointer(); len(pointer) > 0 {
					return strings.Join(pointer, ".")
				}
			}
		}
	}
	return ""
}

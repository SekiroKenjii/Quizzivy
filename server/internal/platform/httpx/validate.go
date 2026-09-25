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

// ValidateRequests checks every request against api/openapi.yaml after authentication middleware; streaming bodies retain parameter checks without duplicate security/body buffering.
func ValidateRequests(spec *openapi3.T) (func(http.Handler) http.Handler, error) {
	stripped := *spec
	stripped.Servers = nil
	stripped.Paths = openapi3.NewPaths()
	for path, item := range spec.Paths.Map() {
		copyItem := *item
		for method, op := range item.Operations() {
			if streamingOperation(op) {
				copyOperation := *op
				copyOperation.RequestBody = nil
				copyOperation.Security = &openapi3.SecurityRequirements{}
				copyItem.SetOperation(method, &copyOperation)
			}
		}
		stripped.Paths.Set(path, &copyItem)
	}
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
					WriteError(w, r, http.StatusNotFound, CodeNotFound, "Không tìm thấy đường dẫn.")
					return
				}
				WriteError(w, r, http.StatusBadRequest, CodeValidationFailed, validationMessage(err))
			},
		}), nil
}

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

func failingField(reqErr *openapi3filter.RequestError) string {
	var schemaErr *openapi3.SchemaError
	if errors.As(reqErr.Err, &schemaErr) {
		if pointer := schemaErr.JSONPointer(); len(pointer) > 0 {
			return strings.Join(pointer, ".")
		}
	}
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

// StreamingBodyRoutes lists the file-upload operations, keyed by the
// `METHOD /path` pattern the mux matches on. The validator checks their parameters
// while leaving their bodies to bounded streaming handlers and content inspection.
func StreamingBodyRoutes(spec *openapi3.T) map[string]struct{} {
	streaming := map[string]struct{}{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if streamingOperation(op) {
				streaming[method+" "+path] = struct{}{}
			}
		}
	}
	return streaming
}

func streamingOperation(op *openapi3.Operation) bool {
	if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
		return false
	}
	for mediaType := range op.RequestBody.Value.Content {
		if strings.HasPrefix(mediaType, "multipart/") {
			return true
		}
	}
	return false
}

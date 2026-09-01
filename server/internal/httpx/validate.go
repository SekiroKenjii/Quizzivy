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
// Without it the contract's constraints are decorative server-side: oapi-codegen
// binds types but enforces no minLength, format, enum or additionalProperties.
// Authentication is not delegated here -- RequireAuth has already run, so the
// AuthenticationFunc always succeeds. File-upload routes are skipped; see
// StreamingBodyRoutes.
func ValidateRequests(spec *openapi3.T) (func(http.Handler) http.Handler, error) {
	stripped := *spec
	stripped.Servers = nil
	if _, err := gorillamux.NewRouter(&stripped); err != nil {
		return nil, err
	}

	streaming := StreamingBodyRoutes(&stripped)

	return nethttpmiddleware.OapiRequestValidatorWithOptions(&stripped,
		&nethttpmiddleware.Options{
			Skipper: func(r *http.Request) bool {
				_, isStreaming := streaming[r.Pattern]
				return isStreaming
			},
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
// `METHOD /path` pattern the mux matches on. The validator skips them: it
// buffers and decodes the whole body, which defeats the handler's streaming,
// and it would gate on a Content-Type the endpoint must not trust.
//
// Nothing is lost. Auth runs earlier, these operations declare no parameters
// (TestStreamingRoutesHaveNoParameters), and the handler's own checks are
// stronger than anything a `format: binary` schema can express.
func StreamingBodyRoutes(spec *openapi3.T) map[string]struct{} {
	streaming := map[string]struct{}{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
				continue
			}
			for mediaType := range op.RequestBody.Value.Content {
				if strings.HasPrefix(mediaType, "multipart/") {
					streaming[method+" "+path] = struct{}{}
					break
				}
			}
		}
	}
	return streaming
}

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
// Routes whose body is a binary upload are skipped -- see StreamingBodyRoutes.
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

// StreamingBodyRoutes lists the operations whose request body is a file upload,
// keyed by the `METHOD /path` pattern the mux matches on.
//
// These are skipped by the validator for two independent reasons, either of
// which alone would be disqualifying.
//
// The first is memory. openapi3filter validates a body by io.ReadAll-ing it and
// then decoding each part, so a 10 MB upload -- the §11.1 cap -- allocated
// 87.2 MB before the handler ran at all. The upload handler streams to a temp
// file precisely so a large body never sits in RAM on a 512 MB instance (R-13);
// validating it first made that pointless and put concurrent uploads on the
// same OOM path as unbounded Argon2id.
//
// Worth knowing if this is ever revisited: with the content-type gate below
// still in place the figure was 44.8 MB, because rejecting early meant the file
// part was never decoded. Removing that gate alone would have made the memory
// problem twice as bad. The two fixes belong together.
//
// The second is correctness. `encoding.<part>.contentType` makes the validator
// reject a part whose Content-Type header is not on the list -- and §11.1 says
// in as many words that this endpoint never trusts that header or the file
// extension. It is also a check that stops nobody, since a caller can label
// anything `audio/mpeg`. What it did stop was honest clients: Go's own
// multipart.CreateFormFile and curl both default to `application/octet-stream`,
// and macOS reports `audio/x-m4a` for m4a, so real uploads failed with a 400
// that named no field, instead of reaching the sniffer that decides the answer.
//
// Nothing is lost by skipping. Auth ran in earlier middleware, these operations
// declare no parameters (TestStreamingRoutesHaveNoParameters holds that true),
// and the handler's size -> magic-bytes -> duration checks are strictly stronger
// than anything the schema can say about a `format: binary` string.
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

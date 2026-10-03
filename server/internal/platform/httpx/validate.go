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
	"golang.org/x/text/language"
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
				WriteError(w, r, http.StatusBadRequest, CodeValidationFailed, validationMessage(r, err))
			},
		}), nil
}

func validationMessage(r *http.Request, err error) string {
	const generic = "Dữ liệu gửi lên không hợp lệ."

	var reqErr *openapi3filter.RequestError
	if !errors.As(err, &reqErr) {
		return generic
	}

	if failsAtNewPassword(reqErr) {
		return newPasswordRule(r)
	}
	if field := failingField(reqErr); field != "" {
		return "Trường \"" + field + "\" không hợp lệ."
	}
	if reqErr.Parameter != nil {
		return "Tham số \"" + reqErr.Parameter.Name + "\" không hợp lệ."
	}
	return generic
}

func failsAtNewPassword(reqErr *openapi3filter.RequestError) bool {
	var failures openapi3.MultiError
	if !errors.As(reqErr.Err, &failures) {
		return false
	}
	for _, failure := range failures {
		var schemaErr *openapi3.SchemaError
		if errors.As(failure, &schemaErr) && strings.HasPrefix(schemaErr.Reason, `error at "/newPassword": `) {
			return true
		}
	}
	return false
}

func newPasswordRule(r *http.Request) string {
	if _, index := language.MatchStrings(maintenanceLanguages, r.Header.Get("Accept-Language")); index == 1 {
		return "The new password needs 8 to 512 characters and a number or symbol."
	}
	return "Mật khẩu mới cần từ 8 đến 512 ký tự và có số hoặc ký hiệu."
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

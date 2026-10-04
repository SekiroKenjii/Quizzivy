package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/google/uuid"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
	"golang.org/x/text/language"
)

// ValidateRequests checks every request against api/openapi.yaml after authentication middleware; streaming bodies retain parameter checks without duplicate security/body buffering. It also refuses a body map key that is not a uuid where the contract's `propertyNames` says `Uuid`, which the schema validator does not check. It also refuses a JSON body in which an object repeats a member name, which the schema validator reads as its last occurrence and the handler's decoder merges; a body the validator re-encoded to fill a default is passed on as the validator read it, without the repeat. A refusal names the field or parameter whose value broke its rule or, when no value did, the required top-level property the body lacks; an unknown property and a body that is not an object get the generic sentence. The sentences are Vietnamese or, when Accept-Language prefers it, English.
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

	validator := nethttpmiddleware.OapiRequestValidatorWithOptions(&stripped,
		&nethttpmiddleware.Options{
			Options: openapi3filter.Options{
				AuthenticationFunc: func(context.Context, *openapi3filter.AuthenticationInput) error {
					return nil
				},
				SchemaValidationOptions: []openapi3.SchemaValidationOption{
					openapi3.WithStringFormatValidator("email", openapi3.NewCallbackValidator(func(value string) error {
						_, err := mail.ParseAddress(value)
						return err
					})),
				},
			},
			ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, r *http.Request, opts nethttpmiddleware.ErrorHandlerOpts) {
				if opts.StatusCode == http.StatusNotFound {
					WriteError(w, r, http.StatusNotFound, CodeNotFound, TextFor(r, "Không tìm thấy đường dẫn.", "The path was not found."))
					return
				}
				WriteError(w, r, http.StatusBadRequest, CodeValidationFailed, validationMessage(r, err))
			},
		})
	keys := uuidMapKeys(uuidKeyedMaps(&stripped))
	members := uniqueMembers(jsonBodies(&stripped))
	return func(next http.Handler) http.Handler { return validator(members(keys(next))) }, nil
}

// WriteMalformedBody answers 400 VALIDATION_FAILED with the generic
// validation sentence, in Vietnamese or, when Accept-Language prefers it,
// English, for a body the generated handler could not decode.
func WriteMalformedBody(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, http.StatusBadRequest, CodeValidationFailed, genericSentence(r))
}

// WriteMalformedParameter answers 400 VALIDATION_FAILED naming the path,
// query, header or cookie parameter that did not bind; with an empty name it
// answers as WriteMalformedBody does.
func WriteMalformedParameter(w http.ResponseWriter, r *http.Request, name string) {
	if name == "" {
		WriteMalformedBody(w, r)
		return
	}
	WriteError(w, r, http.StatusBadRequest, CodeValidationFailed, parameterSentence(r, name))
}

func uuidKeyedMaps(spec *openapi3.T) map[string][][]string {
	keyed := map[string][][]string{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
				continue
			}
			media := op.RequestBody.Value.Content.Get("application/json")
			if media == nil || media.Schema == nil {
				continue
			}
			if paths := keyedPaths(media.Schema.Value, nil, map[*openapi3.Schema]bool{}); len(paths) > 0 {
				keyed[method+" "+path] = paths
			}
		}
	}
	return keyed
}

func keyedPaths(schema *openapi3.Schema, at []string, seen map[*openapi3.Schema]bool) [][]string {
	if schema == nil || seen[schema] {
		return nil
	}
	seen[schema] = true
	defer delete(seen, schema)
	var paths [][]string
	if names := schema.PropertyNames; names != nil && names.Value != nil && names.Value.Format == "uuid" {
		paths = append(paths, slices.Clone(at))
	}
	for name, property := range schema.Properties {
		if property != nil {
			paths = append(paths, keyedPaths(property.Value, append(slices.Clone(at), name), seen)...)
		}
	}
	return paths
}

func uuidMapKeys(keyed map[string][][]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if path, found := malformedKeyPath(r, keyed[r.Pattern]); found {
				WriteError(w, r, http.StatusBadRequest, CodeValidationFailed, fieldSentence(r, strings.Join(path, ".")))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func malformedKeyPath(r *http.Request, paths [][]string) ([]string, bool) {
	if len(paths) == 0 || r.Body == nil {
		return nil, false
	}
	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return nil, false
	}
	for _, path := range paths {
		if !uuidKeys(body, path) {
			return path, true
		}
	}
	return nil, false
}

func uuidKeys(object []byte, path []string) bool {
	decoder := json.NewDecoder(bytes.NewReader(object))
	if opening, err := decoder.Token(); err != nil || opening != json.Delim('{') {
		return true
	}
	for decoder.More() {
		name, err := decoder.Token()
		if err != nil {
			return true
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return true
		}
		if key, _ := name.(string); !uuidKeysOfMember(key, value, path) {
			return false
		}
	}
	return true
}

func uuidKeysOfMember(name string, value []byte, path []string) bool {
	if len(path) == 0 {
		return len(name) == 36 && uuid.Validate(name) == nil
	}
	return name != path[0] || uuidKeys(value, path[1:])
}

func jsonBodies(spec *openapi3.T) map[string]struct{} {
	bodies := map[string]struct{}{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op == nil || op.RequestBody == nil || op.RequestBody.Value == nil {
				continue
			}
			if op.RequestBody.Value.Content.Get("application/json") != nil {
				bodies[method+" "+path] = struct{}{}
			}
		}
	}
	return bodies
}

func uniqueMembers(bodies map[string]struct{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			media, _, _ := strings.Cut(r.Header.Get("Content-Type"), ";")
			if _, declared := bodies[r.Pattern]; !declared || r.Body == nil || media != "application/json" {
				next.ServeHTTP(w, r)
				return
			}
			body, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			top, repeated := repeatedMember(body)
			if !repeated {
				next.ServeHTTP(w, r)
				return
			}
			message := genericSentence(r)
			if top != "" {
				message = fieldSentence(r, top)
			}
			WriteError(w, r, http.StatusBadRequest, CodeValidationFailed, message)
		})
	}
}

func repeatedMember(body []byte) (top string, repeated bool) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var open []map[string]struct{}
	name := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return "", false
		}
		if err != nil {
			return "", true
		}
		switch value := token.(type) {
		case json.Delim:
			switch value {
			case '{':
				open = append(open, map[string]struct{}{})
				name = true
				continue
			case '[':
				open = append(open, nil)
				name = false
				continue
			}
			open = open[:len(open)-1]
		case string:
			if name {
				if len(open) == 1 {
					top = value
				}
				members := open[len(open)-1]
				if _, seen := members[value]; seen {
					return top, true
				}
				members[value] = struct{}{}
				name = false
				continue
			}
		}
		if len(open) == 0 {
			return "", false
		}
		name = open[len(open)-1] != nil
	}
}

func validationMessage(r *http.Request, err error) string {
	var reqErr *openapi3filter.RequestError
	if !errors.As(err, &reqErr) {
		return genericSentence(r)
	}

	if failsAtNewPassword(reqErr) {
		return newPasswordRule(r)
	}
	if field := failingField(reqErr); field != "" {
		return fieldSentence(r, field)
	}
	if field := missingProperty(r, reqErr); field != "" {
		return fieldSentence(r, field)
	}
	if reqErr.Parameter != nil {
		return parameterSentence(r, reqErr.Parameter.Name)
	}
	return genericSentence(r)
}

func genericSentence(r *http.Request) string {
	return TextFor(r, "Dữ liệu gửi lên không hợp lệ.", "The submitted data is not valid.")
}

func fieldSentence(r *http.Request, field string) string {
	return fmt.Sprintf(TextFor(r, "Trường \"%s\" không hợp lệ.", "The field \"%s\" is not valid."), field)
}

func parameterSentence(r *http.Request, name string) string {
	return fmt.Sprintf(TextFor(r, "Tham số \"%s\" không hợp lệ.", "The parameter \"%s\" is not valid."), name)
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
	var failures openapi3.MultiError
	if !errors.As(reqErr.Err, &failures) {
		return ""
	}
	var fields []string
	for _, failure := range failures {
		var located *openapi3.SchemaError
		if !errors.As(failure, &located) {
			continue
		}
		if pointer := located.JSONPointer(); len(pointer) > 0 {
			fields = append(fields, strings.Join(pointer, "."))
			continue
		}
		if field := locatedField(located.Reason); field != "" {
			fields = append(fields, field)
		}
	}
	if len(fields) == 0 {
		return ""
	}
	return slices.Min(fields)
}

func locatedField(reason string) string {
	rest, ok := strings.CutPrefix(reason, `error at "`)
	if !ok {
		return ""
	}
	pointer, _, ok := strings.Cut(rest, `": `)
	if !ok {
		return ""
	}
	return strings.ReplaceAll(strings.TrimPrefix(pointer, "/"), "/", ".")
}

func missingProperty(r *http.Request, reqErr *openapi3filter.RequestError) string {
	var schemaErr *openapi3.SchemaError
	if reqErr.RequestBody == nil || r.Body == nil || !errors.As(reqErr.Err, &schemaErr) {
		return ""
	}
	media := reqErr.RequestBody.Content.Get("application/json")
	if media == nil || media.Schema == nil || media.Schema.Value == nil {
		return ""
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(body, &object) != nil || object == nil {
		return ""
	}
	for _, name := range media.Schema.Value.Required {
		if _, present := object[name]; !present {
			return name
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

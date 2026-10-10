package router_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/internal/core/router"
)

var theCredentialMinters = map[string]bool{
	"Login":                true,
	"GoogleAuth":           true,
	"RefreshSession":       true,
	"ResetStudentPassword": true,
	"CreateStudent":        true,
	"RotateJoinCode":       true,
	"GetJoinCode":          true,
	"ListClasses":          true,
	"OpenDocsSession":      true,
}

const joinCodeShape = "{code, expiresAt}"

func propertyNames(ref *openapi3.SchemaRef, seen map[*openapi3.Schema]bool, out map[string]bool) {
	if ref == nil || ref.Value == nil || seen[ref.Value] {
		return
	}
	seen[ref.Value] = true
	s := ref.Value
	if s.Properties["code"] != nil && s.Properties["expiresAt"] != nil {
		out[joinCodeShape] = true
	}
	for name, prop := range s.Properties {
		out[name] = true
		propertyNames(prop, seen, out)
	}
	propertyNames(s.Items, seen, out)
	propertyNames(s.AdditionalProperties.Schema, seen, out)
	for _, group := range []openapi3.SchemaRefs{s.AllOf, s.OneOf, s.AnyOf} {
		for _, sub := range group {
			propertyNames(sub, seen, out)
		}
	}
}

func mintsACredential(op *openapi3.Operation) bool {
	if op.Responses == nil {
		return false
	}
	for status, response := range op.Responses.Map() {
		if !strings.HasPrefix(status, "2") || response.Value == nil {
			continue
		}
		media := response.Value.Content.Get("application/json")
		if media == nil {
			continue
		}
		names := map[string]bool{}
		propertyNames(media.Schema, map[*openapi3.Schema]bool{}, names)
		if names["temporaryPassword"] || names["accessToken"] || names[joinCodeShape] {
			return true
		}
	}
	return false
}

func TestEveryCredentialMinterHasARateLimit(t *testing.T) {
	spec := freshSpec(t)
	byAddress, byPrincipal := router.RateLimits(), router.PrincipalRateLimits()
	found := map[string]string{}
	eachOperation(spec, func(pattern string, op *openapi3.Operation) {
		if theCredentialMinters[op.OperationID] {
			found[op.OperationID] = pattern
		}
	})
	for id := range theCredentialMinters {
		pattern, ok := found[id]
		if !ok {
			t.Errorf("%s is not in the contract", id)
			continue
		}
		_, perAddress := byAddress.Lookup(pattern)
		_, perActor := byPrincipal.Lookup(pattern)
		if !perAddress && !perActor {
			t.Errorf("%s (%s) mints a credential and has no rate limit", id, pattern)
		}
	}
}

func TestNoOperationMintsACredentialOffTheList(t *testing.T) {
	spec := freshSpec(t)
	var unlisted, detected []string
	eachOperation(spec, func(pattern string, op *openapi3.Operation) {
		if !mintsACredential(op) {
			return
		}
		detected = append(detected, op.OperationID)
		if !theCredentialMinters[op.OperationID] {
			unlisted = append(unlisted, op.OperationID+" ("+pattern+")")
		}
	})
	sort.Strings(unlisted)
	if len(unlisted) > 0 {
		t.Errorf("these operations return a temporary password, an access token or a join code but are not credential minters with a rate limit: %v", unlisted)
	}
	sort.Strings(detected)
	want := []string{"CreateStudent", "GetJoinCode", "GoogleAuth", "ListClasses", "Login", "RefreshSession", "ResetStudentPassword", "RotateJoinCode"}
	if strings.Join(detected, ",") != strings.Join(want, ",") {
		t.Errorf("detected minters %v, want %v: the detector is looking at the wrong thing", detected, want)
	}
}

func TestTheDetectorSeesATokenInsideAnArrayOfObjects(t *testing.T) {
	bulk := &openapi3.Operation{Responses: openapi3.NewResponses()}
	item := openapi3.NewObjectSchema().WithProperty("temporaryPassword", openapi3.NewStringSchema())
	body := openapi3.NewObjectSchema().WithProperty("items", openapi3.NewArraySchema().WithItems(item))
	bulk.Responses.Set("200", &openapi3.ResponseRef{Value: openapi3.NewResponse().WithJSONSchema(body)})
	if !mintsACredential(bulk) {
		t.Error("items[].temporaryPassword was not detected")
	}
}

func TestTheDetectorSeesATokenInsideAMapOfObjects(t *testing.T) {
	bulk := &openapi3.Operation{Responses: openapi3.NewResponses()}
	item := openapi3.NewObjectSchema().WithProperty("temporaryPassword", openapi3.NewStringSchema())
	body := openapi3.NewObjectSchema().WithProperty("passwords", openapi3.NewObjectSchema().WithAdditionalProperties(item))
	bulk.Responses.Set("200", &openapi3.ResponseRef{Value: openapi3.NewResponse().WithJSONSchema(body)})
	if !mintsACredential(bulk) {
		t.Error("passwords{*}.temporaryPassword was not detected")
	}
}

func TestTheDetectorSeesAJoinCodeAtAnyDepth(t *testing.T) {
	code := openapi3.NewObjectSchema().
		WithProperty("code", openapi3.NewStringSchema()).
		WithProperty("expiresAt", openapi3.NewDateTimeSchema())
	for name, body := range map[string]*openapi3.Schema{
		"nested":   openapi3.NewObjectSchema().WithProperty("joinCode", code),
		"in items": openapi3.NewObjectSchema().WithProperty("items", openapi3.NewArraySchema().WithItems(code)),
		"in oneOf": openapi3.NewOneOfSchema(code, openapi3.NewObjectSchema()),
	} {
		op := &openapi3.Operation{Responses: openapi3.NewResponses()}
		op.Responses.Set("201", &openapi3.ResponseRef{Value: openapi3.NewResponse().WithJSONSchema(body)})
		if !mintsACredential(op) {
			t.Errorf("a join code %s was not detected", name)
		}
	}
	notice := openapi3.NewObjectSchema().WithProperty("notices", openapi3.NewArraySchema().WithItems(
		openapi3.NewObjectSchema().WithProperty("code", openapi3.NewStringSchema())))
	op := &openapi3.Operation{Responses: openapi3.NewResponses()}
	op.Responses.Set("200", &openapi3.ResponseRef{Value: openapi3.NewResponse().WithJSONSchema(notice)})
	if mintsACredential(op) {
		t.Error("a notice's code with no expiry was taken for a join code")
	}
}

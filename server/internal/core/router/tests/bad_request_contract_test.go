package router_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/gen/openapi"
)

func jsonBodyOperations(t *testing.T) []*openapi3.Operation {
	t.Helper()
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	var operations []*openapi3.Operation
	for _, item := range spec.Paths.Map() {
		for _, op := range item.Operations() {
			if op.RequestBody == nil || op.RequestBody.Value == nil {
				continue
			}
			if op.RequestBody.Value.Content.Get("application/json") != nil {
				operations = append(operations, op)
			}
		}
	}
	return operations
}

func TestEveryOperationWithAJSONBodyDeclaresA400(t *testing.T) {
	var missing []string
	for _, op := range jsonBodyOperations(t) {
		if op.Responses == nil || op.Responses.Value("400") == nil {
			missing = append(missing, op.OperationID)
		}
	}
	if len(missing) != 0 {
		slices.Sort(missing)
		t.Fatalf("%s: declare '400': { $ref: '#/components/responses/BadRequest' } on it: the request validator answers 400 on every operation that takes a JSON body", strings.Join(missing, ", "))
	}
}

func TestEveryOperationWhoseParametersCanBeRefusedDeclaresA400(t *testing.T) {
	var missing []string
	for _, op := range refusableParameterOperations(t) {
		if op.Responses == nil || op.Responses.Value("400") == nil {
			missing = append(missing, op.OperationID)
		}
	}
	if len(missing) != 0 {
		slices.Sort(missing)
		t.Fatalf("%s: declare '400': { $ref: '#/components/responses/BadRequest' } on it: the parameter binding and the request validator answer 400 on every operation with a parameter they can refuse", strings.Join(missing, ", "))
	}
}

func refusableParameterOperations(t *testing.T) []*openapi3.Operation {
	t.Helper()
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	var operations []*openapi3.Operation
	for _, item := range spec.Paths.Map() {
		for _, op := range item.Operations() {
			if slices.ContainsFunc(slices.Concat(item.Parameters, op.Parameters), func(ref *openapi3.ParameterRef) bool { return refusable(ref.Value) }) {
				operations = append(operations, op)
			}
		}
	}
	return operations
}

func refusable(p *openapi3.Parameter) bool {
	if p.In == openapi3.ParameterInQuery && p.Required {
		return true
	}
	if p.Schema == nil || p.Schema.Value == nil {
		return false
	}
	schema := p.Schema.Value
	if schema.Type.Is(openapi3.TypeArray) && schema.Items != nil && schema.Items.Value != nil {
		schema = schema.Items.Value
	}
	return !schema.Type.Is(openapi3.TypeString) || schema.Format != "" || schema.Pattern != "" ||
		len(schema.Enum) > 0 || schema.MinLength > 0 || schema.MaxLength != nil
}

func TestTheParameterWalkSeesTheOperationsItIsAbout(t *testing.T) {
	operations := refusableParameterOperations(t)
	if len(operations) < 120 {
		t.Errorf("walk found %d operations with a parameter that can be refused, want at least 120", len(operations))
	}
	for _, id := range []string{"getClass", "listAttempts", "getDashboard", "revokeSession"} {
		if !slices.ContainsFunc(operations, func(op *openapi3.Operation) bool { return strings.EqualFold(op.OperationID, id) }) {
			t.Errorf("walk missed %s among the operations with a parameter that can be refused", id)
		}
	}
}

func TestTheWalkSeesTheOperationsItIsAbout(t *testing.T) {
	operations := jsonBodyOperations(t)
	if len(operations) < 45 {
		t.Errorf("walk found %d operations with a JSON body, want at least 45", len(operations))
	}
	for _, id := range []string{"saveAnswers", "login", "submitAttempt"} {
		if !slices.ContainsFunc(operations, func(op *openapi3.Operation) bool { return strings.EqualFold(op.OperationID, id) }) {
			t.Errorf("walk missed %s among the operations with a JSON body", id)
		}
	}
}

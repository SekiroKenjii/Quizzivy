package api

import (
	"testing"

	"quizzivy/gen/openapi"
	"quizzivy/internal/assignments"
	"quizzivy/internal/media"
	"quizzivy/internal/questions"
	"quizzivy/internal/students"
	"quizzivy/internal/tests"
)

// The contract used to declare one shared `limit` default of 25 that no server
// used -- four stores answered 20 and one answered 24, and nothing anywhere
// compared the two. A client sizing a list from the published contract got a
// short page and no error.
//
// Each operation carries its own default now, and this is the thing that keeps
// them honest: the constant and the contract are edited in different files, so
// without an assertion they drift again the first time someone tunes one.
func TestEveryPageSizeMatchesItsContract(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}

	// PascalCase: oapi-codegen normalises operationIds in the embedded spec, so
	// these read as the Go method names rather than the YAML's camelCase.
	want := map[string]int{
		"ListTests":       tests.DefaultLimit,
		"ListQuestions":   questions.DefaultLimit,
		"ListMedia":       media.DefaultLimit,
		"ListAssignments": assignments.DefaultLimit,
		"ListStudents":    students.DefaultLimit,
	}

	seen := map[string]bool{}
	for _, item := range spec.Paths.Map() {
		for _, op := range item.Operations() {
			constant, tracked := want[op.OperationID]
			if !tracked {
				continue
			}
			seen[op.OperationID] = true

			for _, p := range op.Parameters {
				if p.Value == nil || p.Value.Name != "limit" {
					continue
				}
				declared, ok := p.Value.Schema.Value.Default.(float64)
				if !ok {
					t.Errorf("%s: limit has no default in the contract, so a client "+
						"cannot know the page size it will get", op.OperationID)
					continue
				}
				if int(declared) != constant {
					t.Errorf("%s: contract says %d, the store uses %d",
						op.OperationID, int(declared), constant)
				}
			}
		}
	}

	for name := range want {
		if !seen[name] {
			t.Errorf("%s is tracked here but no longer in the contract", name)
		}
	}
}

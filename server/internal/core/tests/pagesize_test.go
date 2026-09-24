package core_test

import (
	"testing"

	"quizzivy/gen/openapi"
	assignmentsrepo "quizzivy/internal/modules/assignments/repositories"
	identityrepo "quizzivy/internal/modules/identity/repositories"
	mediarepo "quizzivy/internal/modules/media/repositories"
	questionsrepo "quizzivy/internal/modules/questions/repositories"
	testsrepo "quizzivy/internal/modules/tests/repositories"
)

// The contract used to declare one shared `limit` default of 25 that no server
// used -- four stores answered 20 and one answered 24, and nothing anywhere
// compared the two. A client sizing a list from the published contract got a
// short page and no error.
func TestEveryPageSizeMatchesItsContract(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}

	want := map[string]int{
		"ListTests":          testsrepo.DefaultLimit,
		"ListQuestionGroups": testsrepo.DefaultLimit,
		"ListQuestions":      questionsrepo.DefaultLimit,
		"ListMedia":          mediarepo.DefaultLimit,
		"ListAssignments":    assignmentsrepo.DefaultLimit,
		"ListStudents":       identityrepo.DefaultLimit,
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

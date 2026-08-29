package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"

	"quizzivy/gen/openapi"
)

// unimplemented is every operation that still returns 501, listed on purpose.
//
// T-2.15 built the test-detail screen against `/preview` and `/versions` while
// both were stubs. Nothing failed: the contract had them, the generated client
// had them, the screen rendered its error state, and it took E2E 1a against a
// live API to notice. A list that has to be edited when a stub is replaced
// makes "is this endpoint real?" answerable without running anything.
//
// Delete a name here when you implement it. Adding one is a phase boundary, not
// a routine change.
var unimplemented = []string{
	// Phase 3 — assignments and taking a test
	"FlushEvents",
	"GetAttempt",
	"GetMyAssignment",
	"ListMyAssignments",
	"RecordAudioPlay",
	"SaveAnswers",
	"StartOrResumeAttempt",
	"SubmitAttempt",

	// Phase 4 — monitoring, grading and results
	"ExtendAttempt",
	"FinishGrading",
	"GetAssignmentMonitor",
	"GetAttemptEvents",
	"GetAttemptForReview",
	"GetAttemptResult",
	"GradeAttempt",
	"ListAttempts",
	"ResetAttempt",
	"VoidAttempt",

	// Classes and students beyond what §6.4 needed in Phase 1
	"CreateClass",
	"CreateStudent",
	"GetStudent",
	"ListMyClasses",
	"ResetStudentPassword",
	"UpdateStudent",
}

// stubbedOperations reads the names off server.gen_stubs.go, which is the file
// that by convention holds nothing but stubs: implementing an operation moves
// its method out of this file.
func stubbedOperations(t *testing.T) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "server.gen_stubs.go", nil, 0)
	if err != nil {
		t.Fatalf("parse server.gen_stubs.go: %v", err)
	}

	var names []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		ident, ok := star.X.(*ast.Ident)
		if !ok || ident.Name != "Server" {
			continue
		}
		names = append(names, fn.Name.Name)
	}
	slices.Sort(names)
	return names
}

func TestTheListOfUnimplementedOperationsIsAccurate(t *testing.T) {
	found := stubbedOperations(t)

	want := slices.Clone(unimplemented)
	slices.Sort(want)

	if !slices.Equal(found, want) {
		missing := difference(want, found)
		extra := difference(found, want)
		if len(missing) > 0 {
			t.Errorf("listed as unimplemented but no longer stubbed (delete from the list): %v", missing)
		}
		if len(extra) > 0 {
			t.Errorf("still a stub but not listed (add it, and check no screen depends on it): %v", extra)
		}
	}
}

// A stub that no operation in the contract names is dead code; an operation the
// contract names that is neither implemented nor listed is the T-2.15 failure.
func TestEveryStubNamesARealOperation(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}

	operations := map[string]bool{}
	for _, item := range spec.Paths.Map() {
		for _, op := range item.Operations() {
			operations[op.OperationID] = true
		}
	}

	for _, name := range stubbedOperations(t) {
		if !operations[name] {
			t.Errorf("stub %s has no operationId in api/openapi.yaml", name)
		}
	}
}

// The two the E2E caught. Named explicitly so a revert is loud.
func TestThePhase2ScreensDoNotRestOnStubs(t *testing.T) {
	stubs := stubbedOperations(t)

	for _, name := range []string{
		"ListTestVersions", // test detail's version history
		"PreviewTest",      // test detail's student-eye preview
		"ListQuestions",    // the question bank list
		"PublishTest",      // the builder's publish gate
		"UpdateTest",       // the builder's autosave
		"CreateQuestion",   // the builder's "Thêm câu hỏi"
		"UploadMedia",      // the audio upload the probe runs behind
		"GetDashboard",     // §8's /admin
	} {
		if slices.Contains(stubs, name) {
			t.Errorf("%s is a stub, but a Phase 2 screen calls it", name)
		}
	}
}

func difference(a, b []string) []string {
	var out []string
	for _, s := range a {
		if !slices.Contains(b, s) {
			out = append(out, s)
		}
	}
	return out
}

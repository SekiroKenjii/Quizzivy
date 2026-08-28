package httpx

import (
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"quizzivy/internal/ratelimit"
)

// AssertPublicRoutesLimited cross-references the contract against the limiter
// registry and reports any unauthenticated operation with no policy.
//
// §14 requires every public endpoint to be rate-limited, and §6.5 explains why:
// these are the only endpoints reachable without a session, and one of them
// takes a bearer secret. A checklist in a PR template is a promise; this is a
// process that refuses to start.
//
// The source of truth is api/openapi.yaml, embedded in the generated code, so
// adding a public operation to the contract is what triggers the requirement --
// nobody has to remember.
func AssertPublicRoutesLimited(spec *openapi3.T, reg *ratelimit.Registry) error {
	var missing []string

	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op == nil || !isPublicOperation(op) {
				continue
			}
			pattern := fmt.Sprintf("%s %s", method, path)
			if _, ok := reg.Lookup(pattern); !ok {
				missing = append(missing, pattern)
			}
		}
	}

	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf(
		"these operations are public in api/openapi.yaml but have no rate limit (§6.5, §14):\n  %s",
		strings.Join(missing, "\n  "),
	)
}

// isPublicOperation reports whether the operation opts out of the global
// security requirement -- `security: []`, or an entry with no schemes.
func isPublicOperation(op *openapi3.Operation) bool {
	if op.Security == nil {
		return false
	}
	reqs := *op.Security
	if len(reqs) == 0 {
		return true
	}
	for _, req := range reqs {
		if len(req) == 0 {
			return true
		}
	}
	return false
}

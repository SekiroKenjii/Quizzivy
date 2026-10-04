package httpx

import (
	"fmt"
	"sort"
	"strings"

	"quizzivy/internal/platform/ratelimit"

	"github.com/getkin/kin-openapi/openapi3"
)

// AssertPublicRoutesLimited cross-references the contract against the limiter
// registry and reports any unauthenticated operation with no policy.
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
		"these operations are public in api/openapi.yaml but have no rate limit (spec 6.5 and 14):\n  %s",
		strings.Join(missing, "\n  "),
	)
}

// AssertPrincipalRoutesGated cross-references a registry of per-user limits
// against the contract and reports every entry PrincipalRateLimit could never
// apply: one on an operation that does not require scheme, where
// RequirePermission resolves no principal, and one that names no operation of
// the contract.
func AssertPrincipalRoutesGated(spec *openapi3.T, scheme string, reg *ratelimit.Registry) error {
	gated := map[string]bool{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op != nil {
				gated[method+" "+path] = requiresScheme(op.Security, spec.Security, scheme)
			}
		}
	}

	var problems []string
	for _, pattern := range reg.Patterns() {
		requires, known := gated[pattern]
		switch {
		case !known:
			problems = append(problems, pattern+": is not an operation in api/openapi.yaml")
		case !requires:
			problems = append(problems, pattern+": is an open operation, where no principal is resolved")
		}
	}

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf(
		"these per-user rate limits can never apply:\n  %s",
		strings.Join(problems, "\n  "),
	)
}

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

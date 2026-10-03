package httpx

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"quizzivy/internal/shared/access"
)

// PermissionExtension is the operation field api/openapi.yaml declares an
// operation's permission in.
const PermissionExtension = "x-permission"

const adminTree = "/admin/"

// PermissionRequirements reads every operation's x-permission into a map keyed
// by route pattern ("POST /teacher/tests"), the form r.Pattern takes. An
// operation that requires scheme must declare one, and one that does not must
// declare none. It fails, naming every offender, when that rule breaks, when a
// value is neither a catalogue key nor a pseudo-key, when a list is empty or
// repeats a key, when a hidden key appears outside /admin/, when a path lies
// outside every tree, or when a key does not belong to its path's tree:
// /teacher/ takes content, teaching and people.students keys and
// workspace.teacher; /admin/ takes people.users.manage, people.roles.manage,
// system keys, scope.all and workspace.admin; /app/ takes learning.take_tests;
// /auth/ and /me/ take self; /public/ and /join/ hold open operations only.
func PermissionRequirements(spec *openapi3.T, scheme string) (map[string]access.Requirement, error) {
	out := map[string]access.Requirement{}
	var problems []string
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op == nil {
				continue
			}
			pattern := method + " " + path
			keys, err := declaredPermission(op, path, !requiresScheme(op.Security, spec.Security, scheme))
			switch {
			case err != nil:
				problems = append(problems, pattern+": "+err.Error())
			case keys != nil:
				out[pattern] = access.AnyOf(keys...)
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return nil, fmt.Errorf("%s in api/openapi.yaml is wrong for these operations:\n  %s",
			PermissionExtension, strings.Join(problems, "\n  "))
	}
	return out, nil
}

func declaredPermission(op *openapi3.Operation, path string, open bool) ([]access.Key, error) {
	tree, known := treeOf(path)
	if !known {
		return nil, errors.New("lies outside every path tree")
	}
	raw, declared := op.Extensions[PermissionExtension]
	switch {
	case open && declared:
		return nil, errors.New("an open operation declares a permission")
	case open:
		return nil, nil
	case !declared:
		return nil, errors.New("declares no permission")
	}
	keys, err := permissionKeys(raw)
	if err != nil {
		return nil, err
	}
	seen := map[access.Key]bool{}
	for _, k := range keys {
		switch {
		case seen[k]:
			return nil, fmt.Errorf("repeats %q", k)
		case !k.Known() && !k.Pseudo():
			return nil, fmt.Errorf("%q is neither a catalogue key nor a pseudo-key", k)
		case k.Hidden() && !strings.HasPrefix(path, adminTree):
			return nil, fmt.Errorf("the hidden key %q is declared outside %s", k, adminTree)
		case !tree.allows(k):
			return nil, fmt.Errorf("%q does not belong under %s", k, tree.prefix)
		}
		seen[k] = true
	}
	return keys, nil
}

func permissionKeys(raw any) ([]access.Key, error) {
	switch v := raw.(type) {
	case string:
		return []access.Key{access.Key(v)}, nil
	case []any:
		if len(v) == 0 {
			return nil, errors.New("declares an empty list")
		}
		keys := make([]access.Key, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("lists %v, which is not a key", item)
			}
			keys = append(keys, access.Key(s))
		}
		return keys, nil
	}
	return nil, fmt.Errorf("declares %v, which is neither a key nor a list of keys", raw)
}

type pathTree struct {
	prefix string
	allows func(access.Key) bool
}

var pathTrees = []pathTree{
	{"/teacher/", func(k access.Key) bool {
		return k == access.WorkspaceTeacher || hasAnyPrefix(string(k), "content.", "teaching.", "people.students.")
	}},
	{adminTree, func(k access.Key) bool {
		return k == access.WorkspaceAdmin || k == access.PeopleUsersManage || k == access.PeopleRolesManage ||
			k == access.ScopeAll || strings.HasPrefix(string(k), "system.")
	}},
	{"/app/", func(k access.Key) bool { return k == access.LearningTakeTests }},
	{"/auth/", func(k access.Key) bool { return k == access.Self }},
	{"/me/", func(k access.Key) bool { return k == access.Self }},
	{"/public/", func(access.Key) bool { return false }},
	{"/join/", func(access.Key) bool { return false }},
}

func treeOf(path string) (pathTree, bool) {
	for _, t := range pathTrees {
		if strings.HasPrefix(path, t.prefix) {
			return t, true
		}
	}
	return pathTree{}, false
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

package router_test

import (
	"context"
	"sync"

	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/access"
)

var builtinGrants = map[access.Builtin]access.Set{
	access.BuiltinAdmin: access.NewSet(access.All()...).Without(access.LearningTakeTests),
	access.BuiltinTeacher: access.NewSet(
		access.ContentTestsWrite, access.ContentTestsPublish, access.ContentQuestionsWrite,
		access.ContentMediaWrite, access.ContentShare,
		access.TeachingClassesWrite, access.TeachingAssignmentsWrite, access.TeachingGrading,
		access.TeachingAttemptsIntervene, access.TeachingAttendance,
		access.PeopleStudentsRead, access.PeopleStudentsCreate, access.PeopleStudentsResetPassword,
	),
	access.BuiltinAssistant: access.NewSet(
		access.ContentTestsWrite, access.ContentQuestionsWrite, access.TeachingAssignmentsWrite,
		access.TeachingGrading, access.TeachingAttendance, access.PeopleStudentsRead,
	),
	access.BuiltinStudent: access.NewSet(access.LearningTakeTests),
}

func builtinPrincipal(userID string, builtin access.Builtin) access.Principal {
	return access.Principal{UserID: userID, RoleID: "role-" + string(builtin), BuiltinKey: builtin, Permissions: builtinGrants[builtin]}
}

type fakePrincipals struct {
	mu    sync.Mutex
	users map[string]access.Principal
	calls int
}

func newFakePrincipals() *fakePrincipals {
	return &fakePrincipals{users: map[string]access.Principal{}}
}

func (f *fakePrincipals) set(p access.Principal) *fakePrincipals {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[p.UserID] = p
	return f
}

func (f *fakePrincipals) forget(userID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.users, userID)
}

func (f *fakePrincipals) Resolve(_ context.Context, userID string) (access.Principal, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if p, ok := f.users[userID]; ok {
		if p.UserID == "" {
			return access.Principal{}, httpx.ErrUnknownPrincipal
		}
		return p, nil
	}
	return builtinPrincipal(userID, access.BuiltinAdmin), nil
}

//go:build integration

package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/shared/access"
)

type sessionPrincipals struct {
	principals map[string]access.Principal
	fail       bool
}

func (p sessionPrincipals) Forget(string) {}

func (p sessionPrincipals) Resolve(_ context.Context, userID string) (access.Principal, error) {
	if p.fail {
		return access.Principal{}, errors.New("database unreachable")
	}
	return p.principals[userID], nil
}

func TestASessionShowsTheUserTheirPermissions(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	svc.SetPrincipals(sessionPrincipals{principals: map[string]access.Principal{
		id: {UserID: id, Permissions: access.NewSet(access.LearningTakeTests)},
	}})
	session, err := svc.Commands.Login.Handle(context.Background(), command.Login{Email: email, Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(session.User.Permissions.Keys(), []access.Key{access.LearningTakeTests}) {
		t.Errorf("session permissions = %v, want [learning.take_tests]", session.User.Permissions.Keys())
	}
}

func TestASessionIsNotIssuedWhenPermissionsCannotBeRead(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)
	svc.SetPrincipals(sessionPrincipals{fail: true})
	if _, err := svc.Commands.Login.Handle(context.Background(), command.Login{Email: email, Password: testPassword}); err == nil {
		t.Error("a session was issued although the user's permissions could not be read")
	}
}

package application_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/modules/identity/domain"
)

type contextSpy struct {
	domain.Users
	revokeErr      error
	revokeDeadline time.Time
	revokeHasLimit bool
	revokeValue    any
	rotateErr      error
}

type spyKey struct{}

func (s *contextSpy) RevokeFamilyByToken(ctx context.Context, _ []byte, _ time.Time) (string, error) {
	s.revokeErr = ctx.Err()
	s.revokeDeadline, s.revokeHasLimit = ctx.Deadline()
	s.revokeValue = ctx.Value(spyKey{})
	return "family", nil
}

func (s *contextSpy) Rotate(ctx context.Context, _ []byte, _ domain.RefreshTokenRecord, _ time.Time) (domain.RotateResult, error) {
	s.rotateErr = ctx.Err()
	return domain.RotateResult{Outcome: domain.RotateUnknown}, nil
}

func spiedApp(t *testing.T, spy *contextSpy) *application.Application {
	t.Helper()
	issuer, err := token.NewIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return application.New(spy, issuer, 30*24*time.Hour, nil, nil)
}

func cancelledRequest() context.Context {
	request, cancel := context.WithCancel(context.WithValue(context.Background(), spyKey{}, "kept"))
	cancel()
	return request
}

func TestALogoutRevokesOnAContextTheEndOfTheRequestCannotCancel(t *testing.T) {
	spy := &contextSpy{}
	started := time.Now()

	if _, err := spiedApp(t, spy).Commands.Logout.Handle(cancelledRequest(), command.Logout{Token: "presented"}); err != nil {
		t.Fatalf("logout: %v", err)
	}
	finished := time.Now()

	if spy.revokeErr != nil {
		t.Errorf("the revoke ran on a context that read %v: a client that hangs up would leave the family live", spy.revokeErr)
	}
	if !spy.revokeHasLimit || spy.revokeDeadline.After(finished.Add(command.LogoutBudget)) || !spy.revokeDeadline.After(started.Add(command.LogoutBudget-time.Second)) {
		t.Errorf("the revoke's deadline = %v (set: %v), want one %s from the call", spy.revokeDeadline, spy.revokeHasLimit, command.LogoutBudget)
	}
	if spy.revokeValue != "kept" {
		t.Errorf("the revoke lost the request's values: %v", spy.revokeValue)
	}
}

func TestARefreshRotatesOnTheRequestsOwnContext(t *testing.T) {
	spy := &contextSpy{}

	if _, err := spiedApp(t, spy).Commands.Refresh.Handle(cancelledRequest(), command.Refresh{Token: "presented"}); err == nil {
		t.Fatal("a rotation the spy reports unknown answered nil")
	}

	if spy.rotateErr != context.Canceled {
		t.Errorf("the rotation ran on a context that read %v, want context.Canceled: finishing a rotation for a client that has gone commits a successor nobody receives", spy.rotateErr)
	}
}

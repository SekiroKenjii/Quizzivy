package application_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/domain"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
)

func loginStore() *policyStore {
	store := newPolicyStore()
	store.add(policyUser("good", "good@example.com"), "")
	store.add(suspended(policyUser("suspended", "suspended@example.com")), "")
	store.add(withoutPassword(policyUser("google-only", "google-only@example.com")), "")
	return store
}

var loginAttempts = []struct {
	name  string
	login command.Login
}{
	{"the right password", command.Login{Email: "good@example.com", Password: policyPassword}},
	{"a wrong password", command.Login{Email: "good@example.com", Password: "sai-mật-khẩu"}},
	{"an unknown email", command.Login{Email: "nobody@example.com", Password: policyPassword}},
	{"a disabled account with the right password", command.Login{Email: "suspended@example.com", Password: policyPassword}},
	{"an account with no password", command.Login{Email: "google-only@example.com", Password: policyPassword}},
}

func TestPasswordSignInOffRefusesEveryLoginAlikeWithoutLookingAtTheEmail(t *testing.T) {
	store := loginStore()
	app := policyApp(t, store, policyOf(policyWith(passwordOff)))

	var first error
	for _, attempt := range loginAttempts {
		session, err := app.Commands.Login.Handle(context.Background(), attempt.login)
		if !errors.Is(err, domain.ErrPasswordSignInDisabled) {
			t.Fatalf("%s: error = %v, want ErrPasswordSignInDisabled", attempt.name, err)
		}
		if session.AccessToken != "" || session.RefreshToken != "" {
			t.Errorf("%s: a session came with the refusal", attempt.name)
		}
		if first == nil {
			first = err
		}
		if err.Error() != first.Error() {
			t.Errorf("%s: error %q differs from %q", attempt.name, err, first)
		}
		if len(store.calls) != 0 {
			t.Fatalf("%s: the user store was consulted: %v", attempt.name, store.calls)
		}
	}
}

func TestPasswordSignInOnLeavesLoginAsItWas(t *testing.T) {
	for name, policy := range map[string]ports.SignInPolicy{
		"with no policy port":      nil,
		"with every way in on":     policyOf(openPolicy()),
		"with Google turned off":   policyOf(policyWith(googleOff)),
		"with the change not made": policyOf(policyWith(keepTemporary)),
	} {
		t.Run(name, func(t *testing.T) {
			store := loginStore()
			app := policyApp(t, store, policy)
			ctx := context.Background()

			session, err := app.Commands.Login.Handle(ctx, loginAttempts[0].login)
			if err != nil {
				t.Fatalf("%s: %v", loginAttempts[0].name, err)
			}
			if session.AccessToken == "" || session.RefreshToken == "" || session.User.ID != "good" {
				t.Errorf("%s: session = %+v", loginAttempts[0].name, session)
			}
			if !store.called("CreateRefreshToken") {
				t.Error("no refresh token was stored for the session")
			}
			for _, attempt := range loginAttempts[1:] {
				if _, err := app.Commands.Login.Handle(ctx, attempt.login); !errors.Is(err, domain.ErrInvalidCredentials) {
					t.Errorf("%s: error = %v, want ErrInvalidCredentials", attempt.name, err)
				}
			}
		})
	}
}

func TestPasswordSignInSwitchedBackOnAcceptsTheRightPasswordAgain(t *testing.T) {
	store := loginStore()
	current := openPolicy()
	app := policyApp(t, store, switchablePolicy(func() organizationmodel.SignInPolicy { return current }))
	ctx := context.Background()

	if _, err := app.Commands.Login.Handle(ctx, loginAttempts[0].login); err != nil {
		t.Fatalf("with password sign-in on: %v", err)
	}
	current = policyWith(passwordOff)
	if _, err := app.Commands.Login.Handle(ctx, loginAttempts[0].login); !errors.Is(err, domain.ErrPasswordSignInDisabled) {
		t.Fatalf("after it was turned off: error = %v, want ErrPasswordSignInDisabled", err)
	}
	current = openPolicy()
	if _, err := app.Commands.Login.Handle(ctx, loginAttempts[0].login); err != nil {
		t.Fatalf("after it was turned back on: %v", err)
	}
}

func TestAPolicyThatCannotBeReadFailsTheLoginAsItself(t *testing.T) {
	store := loginStore()
	unreadable := errors.New("the settings snapshot could not be read")
	app := policyApp(t, store, unreadablePolicy(unreadable))

	_, err := app.Commands.Login.Handle(context.Background(), loginAttempts[0].login)
	if !errors.Is(err, unreadable) {
		t.Fatalf("error = %v, want the failure to read the policy", err)
	}
	if errors.Is(err, domain.ErrPasswordSignInDisabled) || errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("error = %v reads as a refusal, not as a fault", err)
	}
	if len(store.calls) != 0 {
		t.Errorf("the user store was consulted: %v", store.calls)
	}
}

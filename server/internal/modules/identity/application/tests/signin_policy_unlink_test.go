package application_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/actor"
)

func unlink(t *testing.T, store *policyStore, policy ports.SignInPolicy) error {
	t.Helper()
	_, err := policyApp(t, store, policy).Commands.UnlinkGoogle.Handle(context.Background(),
		command.UnlinkGoogle{Actor: actor.Actor{ID: "member", IP: "203.0.113.31", UserAgent: "go-test"}})
	return err
}

func TestUnlinkingGoogleIsRefusedWhilePasswordSignInIsOffWhateverTheAccountHolds(t *testing.T) {
	store := newPolicyStore().add(policyUser("member", "an@school.edu"), "google-sub-1")

	err := unlink(t, store, policyOf(policyWith(passwordOff)))

	if !errors.Is(err, domain.ErrLastLoginMethod) {
		t.Fatalf("error = %v, want ErrLastLoginMethod", err)
	}
	if store.unlinked != 0 || store.wrote() {
		t.Errorf("the refusal wrote to the store: %v", store.calls)
	}
}

func TestUnlinkingGoogleIsRefusedFromAnAccountWithNoPasswordUnderAnyPolicy(t *testing.T) {
	for name, policy := range map[string]ports.SignInPolicy{
		"with no policy port":  nil,
		"with every way in on": policyOf(openPolicy()),
		"with password off":    policyOf(policyWith(passwordOff)),
	} {
		t.Run(name, func(t *testing.T) {
			store := newPolicyStore().add(withoutPassword(policyUser("member", "an@school.edu")), "google-sub-1")

			err := unlink(t, store, policy)

			if !errors.Is(err, domain.ErrLastLoginMethod) {
				t.Fatalf("error = %v, want ErrLastLoginMethod", err)
			}
			if store.unlinked != 0 {
				t.Error("the identity was unlinked")
			}
		})
	}
}

func TestUnlinkingGoogleIsAsItWasWhilePasswordSignInIsOn(t *testing.T) {
	for name, policy := range map[string]ports.SignInPolicy{
		"with no policy port":      nil,
		"with every way in on":     policyOf(openPolicy()),
		"with Google turned off":   policyOf(policyWith(googleOff)),
		"with a domain list":       policyOf(policyWith(onlyDomains("school.edu"))),
		"with the change not made": policyOf(policyWith(keepTemporary)),
	} {
		t.Run(name, func(t *testing.T) {
			store := newPolicyStore().add(policyUser("member", "an@gmail.com"), "google-sub-1")

			if err := unlink(t, store, policy); err != nil {
				t.Fatalf("UnlinkGoogle: %v", err)
			}
			if store.unlinked != 1 || !store.called("WriteAudit") {
				t.Errorf("unlinked %d identities; calls = %v", store.unlinked, store.calls)
			}
		})
	}
}

func TestAPolicyThatCannotBeReadFailsUnlinkingAsItself(t *testing.T) {
	unreadable := errors.New("the settings snapshot could not be read")
	store := newPolicyStore().add(policyUser("member", "an@school.edu"), "google-sub-1")

	err := unlink(t, store, unreadablePolicy(unreadable))

	if !errors.Is(err, unreadable) || errors.Is(err, domain.ErrLastLoginMethod) {
		t.Fatalf("error = %v, want the failure to read the policy", err)
	}
	if store.unlinked != 0 {
		t.Error("the identity was unlinked")
	}
}

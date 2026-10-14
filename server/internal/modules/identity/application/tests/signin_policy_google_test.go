package application_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/domain"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
)

type googleRig struct {
	app      *application.Application
	store    *policyStore
	google   *countingGoogle
	enroller *refusingEnroller
}

func newGoogleRig(t *testing.T, store *policyStore, policy ports.SignInPolicy, email string) *googleRig {
	t.Helper()
	rig := &googleRig{
		app:      policyApp(t, store, policy),
		store:    store,
		google:   &countingGoogle{identity: model.GoogleIdentity{Subject: "google-sub-1", Email: email, EmailVerified: true, Name: "Nguyễn Văn An"}},
		enroller: &refusingEnroller{},
	}
	rig.app.SetGoogle(rig.google, rig.enroller.handler())
	return rig
}

func (r *googleRig) signIn(joinCode string) (model.GoogleSignInResult, error) {
	return r.app.Commands.GoogleSignIn.Handle(context.Background(), command.GoogleSignIn{
		Code: "c", CodeVerifier: "v", RedirectURI: "https://app.quizzivy.com/cb", JoinCode: joinCode, IP: "203.0.113.11", UserAgent: "go-test",
	})
}

func (r *googleRig) link(userID string) (domain.User, error) {
	return r.app.Commands.LinkGoogle.Handle(context.Background(), command.LinkGoogle{
		UserID: userID, Code: "c", CodeVerifier: "v", RedirectURI: "https://app.quizzivy.com/cb", IP: "203.0.113.30", UserAgent: "go-test",
	})
}

func linkedStore(email string) *policyStore {
	return newPolicyStore().add(policyUser("member", email), "google-sub-1")
}

func unlinkedStore(email string) *policyStore {
	return newPolicyStore().add(policyUser("member", email), "")
}

func TestGoogleSignInOffRefusesBeforeGoogleIsCalled(t *testing.T) {
	const email = "an@school.edu"
	off := policyOf(policyWith(googleOff))
	for name, run := range map[string]func(*googleRig) error{
		"a sign-in of a linked account": func(r *googleRig) error { _, err := r.signIn(""); return err },
		"a sign-in with a join code":    func(r *googleRig) error { _, err := r.signIn("ABCD2345"); return err },
		"linking":                       func(r *googleRig) error { _, err := r.link("member"); return err },
	} {
		t.Run(name, func(t *testing.T) {
			rig := newGoogleRig(t, linkedStore(email), off, email)

			err := run(rig)

			if !errors.Is(err, domain.ErrGoogleSignInDisabled) {
				t.Fatalf("error = %v, want ErrGoogleSignInDisabled", err)
			}
			if calls := rig.google.calls(); calls != 0 {
				t.Errorf("Google was called %d times", calls)
			}
			if got := rig.enroller.calls.Load(); got != 0 {
				t.Errorf("the enroller was called %d times", got)
			}
			if rig.store.wrote() {
				t.Errorf("the refusal wrote to the store: %v", rig.store.calls)
			}
		})
	}
}

func TestGoogleSignInOffLeavesPasswordSignInAlone(t *testing.T) {
	store := newPolicyStore().add(policyUser("member", "an@school.edu"), "")
	app := policyApp(t, store, policyOf(policyWith(googleOff)))

	session, err := app.Commands.Login.Handle(context.Background(), command.Login{Email: "an@school.edu", Password: policyPassword})
	if err != nil {
		t.Fatalf("password sign-in with Google off: %v", err)
	}
	if session.User.ID != "member" {
		t.Errorf("signed in as %s", session.User.ID)
	}
}

func TestAnAllowedDomainListRefusesAGoogleAddressOutsideIt(t *testing.T) {
	const outside = "an@gmail.com"
	list := policyOf(policyWith(onlyDomains("school.edu", "college.edu")))
	for name, c := range map[string]struct {
		store *policyStore
		run   func(*googleRig) error
	}{
		"a sign-in of an account already linked": {
			linkedStore(outside),
			func(r *googleRig) error { _, err := r.signIn(""); return err },
		},
		"a sign-in that would link by email": {
			unlinkedStore(outside),
			func(r *googleRig) error { _, err := r.signIn(""); return err },
		},
		"a sign-in that would join a class": {
			newPolicyStore(),
			func(r *googleRig) error { _, err := r.signIn("ABCD2345"); return err },
		},
		"linking": {
			unlinkedStore("member@school.edu"),
			func(r *googleRig) error { _, err := r.link("member"); return err },
		},
	} {
		t.Run(name, func(t *testing.T) {
			rig := newGoogleRig(t, c.store, list, outside)

			err := c.run(rig)

			if !errors.Is(err, domain.ErrGoogleDomainNotAllowed) {
				t.Fatalf("error = %v, want ErrGoogleDomainNotAllowed", err)
			}
			if rig.store.wrote() {
				t.Errorf("the refusal wrote to the store: %v", rig.store.calls)
			}
			if got := rig.enroller.calls.Load(); got != 0 {
				t.Errorf("the enroller was called %d times", got)
			}
		})
	}
}

func TestAnAllowedDomainListAdmitsAGoogleAddressInsideItInAnyCase(t *testing.T) {
	list := policyOf(policyWith(onlyDomains("school.edu")))
	for _, email := range []string{"an@school.edu", "An@School.Edu", "AN@SCHOOL.EDU"} {
		t.Run(email, func(t *testing.T) {
			signedIn, err := newGoogleRig(t, linkedStore(email), list, email).signIn("")
			if err != nil {
				t.Fatalf("a sign-in of a linked account: %v", err)
			}
			if signedIn.Session.User.ID != "member" || signedIn.Session.AccessToken == "" {
				t.Errorf("session = %+v", signedIn.Session)
			}

			linking := newGoogleRig(t, unlinkedStore("an.old@example.com"), list, email)
			if _, err := linking.link("member"); err != nil {
				t.Fatalf("linking: %v", err)
			}
			if linking.store.linked != 1 {
				t.Errorf("linked %d identities, want 1", linking.store.linked)
			}

			byEmail, err := newGoogleRig(t, unlinkedStore(email), list, email).signIn("")
			if err != nil {
				t.Fatalf("a sign-in that links by email: %v", err)
			}
			if byEmail.Session.User.ID != "member" {
				t.Errorf("signed in as %s", byEmail.Session.User.ID)
			}

			joining := newGoogleRig(t, newPolicyStore(), list, email)
			var rejected domain.JoinCodeRejected
			if _, err := joining.signIn("ABCD2345"); !errors.As(err, &rejected) {
				t.Fatalf("a sign-in with a join code: error = %v, want to reach the enroller", err)
			}
			if got := joining.enroller.calls.Load(); got != 1 {
				t.Errorf("the enroller was called %d times, want 1", got)
			}
		})
	}
}

func TestAnAllowedDomainListDoesNotAdmitAnAddressThatOnlyContainsTheDomain(t *testing.T) {
	list := policyOf(policyWith(onlyDomains("school.edu")))
	for _, email := range []string{"an@notschool.edu", "an@school.edu.evil.example", "school.edu@gmail.com"} {
		t.Run(email, func(t *testing.T) {
			rig := newGoogleRig(t, linkedStore(email), list, email)

			if _, err := rig.signIn(""); !errors.Is(err, domain.ErrGoogleDomainNotAllowed) {
				t.Fatalf("error = %v, want ErrGoogleDomainNotAllowed", err)
			}
		})
	}
}

func TestNoAllowedDomainsAdmitsEveryGoogleAddress(t *testing.T) {
	for name, policy := range map[string]ports.SignInPolicy{
		"with no policy port":       nil,
		"with no list":              policyOf(policyWith(onlyDomains())),
		"with an empty list":        policyOf(policyWith(func(p *organizationmodel.SignInPolicy) { p.AllowedGoogleDomains = []string{} })),
		"with password sign-in off": policyOf(policyWith(passwordOff)),
	} {
		t.Run(name, func(t *testing.T) {
			const email = "an@gmail.com"

			signedIn, err := newGoogleRig(t, linkedStore(email), policy, email).signIn("")
			if err != nil {
				t.Fatalf("a sign-in of a linked account: %v", err)
			}
			if signedIn.Session.User.ID != "member" {
				t.Errorf("signed in as %s", signedIn.Session.User.ID)
			}

			linking := newGoogleRig(t, unlinkedStore("an.old@example.com"), policy, email)
			if _, err := linking.link("member"); err != nil {
				t.Fatalf("linking: %v", err)
			}
		})
	}
}

func TestAGoogleAddressWithoutAnAtSignIsOutsideAnyDomainList(t *testing.T) {
	rig := newGoogleRig(t, linkedStore("an"), policyOf(policyWith(onlyDomains("school.edu"))), "an")

	if _, err := rig.signIn(""); !errors.Is(err, domain.ErrGoogleDomainNotAllowed) {
		t.Fatalf("error = %v, want ErrGoogleDomainNotAllowed", err)
	}
}

func TestAPolicyThatCannotBeReadFailsAGoogleSignInBeforeGoogleIsCalled(t *testing.T) {
	unreadable := errors.New("the settings snapshot could not be read")
	rig := newGoogleRig(t, linkedStore("an@school.edu"), unreadablePolicy(unreadable), "an@school.edu")

	if _, err := rig.signIn(""); !errors.Is(err, unreadable) {
		t.Fatalf("sign-in: error = %v, want the failure to read the policy", err)
	}
	if _, err := rig.link("member"); !errors.Is(err, unreadable) {
		t.Fatalf("linking: error = %v, want the failure to read the policy", err)
	}
	if calls := rig.google.calls(); calls != 0 {
		t.Errorf("Google was called %d times", calls)
	}
}

//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/shared/actor"
	"testing"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

// §15's two rules: a Google account belongs to one Quizzivy account, and an
// account never ends up with no way in.

func linkService(t *testing.T, pool *pgxpool.Pool, identity model.GoogleIdentity) *application.Application {
	t.Helper()
	svc := newService(t, pool)
	svc.SetGoogle(&stubGoogle{identity: identity}, nil)
	return svc
}

func link(svc *application.Application, userID string) (domain.User, error) {
	return svc.Commands.LinkGoogle.Handle(context.Background(), command.LinkGoogle{UserID: userID, Code: "c", CodeVerifier: "v",
		RedirectURI: "https://app.quizzivy.com/cb", IP: "203.0.113.30", UserAgent: "go-test",
	})
}

func TestLinkingAttachesGoogleToTheSignedInAccount(t *testing.T) {
	pool := newPool(t)
	id, email := makeUser(t, pool)
	svc := linkService(t, pool, verifiedIdentity(email))

	user, err := link(svc, id)
	if err != nil {
		t.Fatalf("LinkGoogle: %v", err)
	}
	if len(user.LinkedProviders) != 1 || user.LinkedProviders[0] != "google" {
		t.Errorf("linkedProviders = %v, want [google]", user.LinkedProviders)
	}
	if !user.HasPassword() {
		t.Error("linking Google removed the password")
	}

	var action string
	if err := pool.QueryRow(context.Background(),
		`SELECT action FROM app.audit_log WHERE actor_user_id = $1 ORDER BY id DESC LIMIT 1`,
		id).Scan(&action); err != nil {
		t.Fatalf("the link was not audited: %v", err)
	}
	if action != "user.google_linked" {
		t.Errorf("audit action = %q", action)
	}
}

func TestLinkingTheSameGoogleAccountAgainIsANoOp(t *testing.T) {
	pool := newPool(t)
	id, email := makeUser(t, pool)
	svc := linkService(t, pool, verifiedIdentity(email))

	if _, err := link(svc, id); err != nil {
		t.Fatal(err)
	}
	user, err := link(svc, id)
	if err != nil {
		t.Fatalf("re-linking the same account: %v", err)
	}
	if len(user.LinkedProviders) != 1 {
		t.Errorf("linkedProviders = %v, want exactly one", user.LinkedProviders)
	}
}

func TestAGoogleAccountAlreadyBoundToAnotherUserIsRejected(t *testing.T) {
	pool := newPool(t)
	firstID, _ := makeUser(t, pool)
	secondID, _ := makeUser(t, pool)
	identity := verifiedIdentity("a-personal-gmail@example.com")
	svc := linkService(t, pool, identity)
	if _, err := link(svc, firstID); err != nil {
		t.Fatal(err)
	}

	// The same Google account, now offered to a different Quizzivy user.
	if _, err := link(svc, secondID); !errors.Is(err, domain.ErrIdentityAlreadyLinked) {
		t.Fatalf("error = %v, want ErrIdentityAlreadyLinked", err)
	}
}

func TestASecondGoogleAccountCannotBeAddedToOneUser(t *testing.T) {
	pool := newPool(t)
	id, email := makeUser(t, pool)

	if _, err := link(linkService(t, pool, verifiedIdentity(email)), id); err != nil {
		t.Fatal(err)
	}

	other := model.GoogleIdentity{Subject: "a-different-google-account", Email: email, EmailVerified: true}
	if _, err := link(linkService(t, pool, other), id); !errors.Is(err, domain.ErrIdentityAlreadyLinked) {
		t.Fatalf("error = %v, want ErrIdentityAlreadyLinked", err)
	}
}

func TestLinkingAGoogleAddressThatIsAnotherAccountsEmailIsRejected(t *testing.T) {
	pool := newPool(t)
	mineID, _ := makeUser(t, pool)
	_, victimEmail := makeUser(t, pool)

	svc := linkService(t, pool, verifiedIdentity(victimEmail))
	if _, err := link(svc, mineID); !errors.Is(err, domain.ErrEmailBelongsToAnotherUser) {
		t.Fatalf("error = %v, want ErrEmailBelongsToAnotherUser", err)
	}
}

func TestAnUnverifiedAddressCannotBeLinked(t *testing.T) {
	pool := newPool(t)
	id, email := makeUser(t, pool)
	identity := verifiedIdentity(email)
	identity.EmailVerified = false

	if _, err := link(linkService(t, pool, identity), id); !errors.Is(err, domain.ErrGoogleEmailUnverified) {
		t.Fatalf("error = %v, want ErrEmailUnverified", err)
	}

	var links int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.user_identities WHERE user_id = $1`, id).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if links != 0 {
		t.Error("an unverified address was linked anyway")
	}
}

func TestUnlinkingFromAPasswordlessAccountIsRefused(t *testing.T) {
	pool := newPool(t)
	id, email := makeUser(t, pool, googleOnly)
	linkGoogleSubject(t, pool, id, "google-sub-"+email, email)

	svc := newService(t, pool)
	if _, err := svc.Commands.UnlinkGoogle.Handle(context.Background(), command.UnlinkGoogle{Actor: actor.Actor{ID: id, IP: "203.0.113.31", UserAgent: "go-test"}}); !errors.Is(err, domain.ErrLastLoginMethod) {
		t.Fatalf("error = %v, want ErrLastLoginMethod", err)
	}

	// And the identity is still there.
	var links int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.user_identities WHERE user_id = $1`, id).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if links != 1 {
		t.Errorf("identities = %d, want 1 -- the refusal did not hold", links)
	}
}

func TestUnlinkingWorksWhenAPasswordRemains(t *testing.T) {
	pool := newPool(t)
	id, email := makeUser(t, pool)
	linkGoogleSubject(t, pool, id, "google-sub-"+email, email)
	svc := newService(t, pool)
	ctx := context.Background()

	if _, err := svc.Commands.UnlinkGoogle.Handle(ctx, command.UnlinkGoogle{Actor: actor.Actor{ID: id, IP: "203.0.113.32", UserAgent: "go-test"}}); err != nil {
		t.Fatalf("UnlinkGoogle: %v", err)
	}
	user, err := svc.Queries.CurrentUser.Handle(ctx, query.CurrentUser{UserID: id})
	if err != nil {
		t.Fatal(err)
	}
	if len(user.LinkedProviders) != 0 {
		t.Errorf("linkedProviders = %v, want empty", user.LinkedProviders)
	}
	// The account is still reachable, which is the whole point of the rule.
	if _, err := svc.Commands.Login.Handle(ctx, command.Login{Email: email, Password: testPassword}); err != nil {
		t.Errorf("the account cannot be signed into after unlinking: %v", err)
	}

	var action string
	if err := pool.QueryRow(ctx,
		`SELECT action FROM app.audit_log WHERE actor_user_id = $1 ORDER BY id DESC LIMIT 1`,
		id).Scan(&action); err != nil {
		t.Fatalf("the unlink was not audited: %v", err)
	}
	if action != "user.google_unlinked" {
		t.Errorf("audit action = %q", action)
	}
}

func TestUnlinkingWhenNothingIsLinkedSucceeds(t *testing.T) {
	pool := newPool(t)
	id, _ := makeUser(t, pool)
	svc := newService(t, pool)

	for i := range 2 {
		if _, err := svc.Commands.UnlinkGoogle.Handle(context.Background(), command.UnlinkGoogle{Actor: actor.Actor{ID: id, IP: "", UserAgent: ""}}); err != nil {
			t.Fatalf("unlink %d: %v", i+1, err)
		}
	}
}

func TestASuspendedAccountCanNeitherLinkNorUnlink(t *testing.T) {
	pool := newPool(t)
	id, email := makeUser(t, pool, disabled)
	svc := linkService(t, pool, verifiedIdentity(email))
	ctx := context.Background()

	if _, err := link(svc, id); !errors.Is(err, domain.ErrAccountDisabled) {
		t.Errorf("link: error = %v, want ErrAccountDisabled", err)
	}
	if _, err := svc.Commands.UnlinkGoogle.Handle(ctx, command.UnlinkGoogle{Actor: actor.Actor{ID: id, IP: "", UserAgent: ""}}); !errors.Is(err, domain.ErrAccountDisabled) {
		t.Errorf("unlink: error = %v, want ErrAccountDisabled", err)
	}
}

func TestLinkingIsUnavailableWhenGoogleIsUnconfigured(t *testing.T) {
	pool := newPool(t)
	id, _ := makeUser(t, pool)

	if _, err := link(newService(t, pool), id); !errors.Is(err, domain.ErrGoogleUnavailable) {
		t.Fatalf("error = %v, want ErrGoogleUnavailable", err)
	}
}

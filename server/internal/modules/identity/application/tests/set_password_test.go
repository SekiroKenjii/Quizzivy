package application_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/audit"
)

var linkNow = time.Date(2026, 10, 14, 12, 0, 0, 123456789, time.UTC)

type linkStore struct {
	order        []string
	link         domain.SetPasswordLink
	findErr      error
	claimErr     error
	claimed      domain.ClaimedLink
	recipient    domain.LinkRecipient
	recipientErr error
	enqueueErr   error

	finds     int
	claims    []domain.ClaimLink
	inserted  []domain.NewLink
	mails     []domain.LinkMail
	audits    []audit.Entry
	hooks     []func()
	committed bool
}

func (s *linkStore) log(step string) { s.order = append(s.order, step) }

func (s *linkStore) InTx(ctx context.Context, fn func(ctx context.Context, tx domain.LinkTx) error) error {
	s.hooks = nil
	if err := fn(ctx, s); err != nil {
		s.log("rollback")
		return err
	}
	s.log("commit")
	s.committed = true
	for _, hook := range s.hooks {
		hook()
	}
	return nil
}

func (s *linkStore) Find(context.Context, []byte) (domain.SetPasswordLink, error) {
	s.finds++
	s.log("find")
	return s.link, s.findErr
}

func (s *linkStore) Claim(_ context.Context, in domain.ClaimLink) (domain.ClaimedLink, error) {
	s.log("claim")
	s.claims = append(s.claims, in)
	return s.claimed, s.claimErr
}

func (s *linkStore) Recipient(context.Context, string) (domain.LinkRecipient, error) {
	s.log("recipient")
	return s.recipient, s.recipientErr
}

func (s *linkStore) RevokeUnused(context.Context, string, time.Time) error {
	s.log("revoke-unused")
	return nil
}

func (s *linkStore) Insert(_ context.Context, link domain.NewLink) error {
	s.log("insert")
	s.inserted = append(s.inserted, link)
	return nil
}

func (s *linkStore) EndSessions(context.Context, string, time.Time) error {
	s.log("end-sessions")
	return nil
}

func (s *linkStore) Enqueue(_ context.Context, mail domain.LinkMail) error {
	s.log("enqueue")
	s.mails = append(s.mails, mail)
	return s.enqueueErr
}

func (s *linkStore) Audit(_ context.Context, entry audit.Entry) error {
	s.log("audit")
	s.audits = append(s.audits, entry)
	return nil
}

func (s *linkStore) AfterCommit(fn func()) { s.hooks = append(s.hooks, fn) }

type linkPrincipals struct {
	store   *linkStore
	forgets []string
}

func (p *linkPrincipals) Forget(id string) {
	p.forgets = append(p.forgets, id)
	p.store.log("forget")
}

func (p *linkPrincipals) Resolve(_ context.Context, id string) (access.Principal, error) {
	return access.Principal{UserID: id}, nil
}

type linkWorld struct {
	app        *application.Application
	links      *linkStore
	principals *linkPrincipals
	users      *policyStore
}

func newLinkWorld(t *testing.T) *linkWorld {
	t.Helper()
	users := newPolicyStore().add(policyUser("account", "an@example.test"), "")
	links := &linkStore{
		recipient: domain.LinkRecipient{ID: "account", Email: "an@example.test", FullName: "Nguyễn Văn An"},
		link:      domain.SetPasswordLink{UserID: "account", Email: "an@example.test", FullName: "Nguyễn Văn An", Purpose: domain.LinkInvite, ExpiresAt: linkNow.Add(time.Hour)},
		claimed:   domain.ClaimedLink{UserID: "account", Purpose: domain.LinkInvite},
	}
	principals := &linkPrincipals{store: links}
	app := policyApp(t, users, policyOf(openPolicy()))
	app.SetPasswordLinks(links)
	app.SetPrincipals(principals)
	app.SetClock(func() time.Time { return linkNow })
	return &linkWorld{app: app, links: links, principals: principals, users: users}
}

func (w *linkWorld) issue(cmd command.IssueSetPasswordLink) (domain.IssuedLink, error) {
	var issued domain.IssuedLink
	err := w.links.InTx(context.Background(), func(ctx context.Context, tx domain.LinkTx) error {
		cmd.Tx = tx
		var err error
		issued, err = w.app.Commands.IssueSetPasswordLink.Handle(ctx, cmd)
		return err
	})
	return issued, err
}

func (w *linkWorld) setPassword(token, password string) error {
	_, err := w.app.Commands.SetPasswordWithLink.Handle(context.Background(), command.SetPasswordWithLink{Token: token, NewPassword: password, UserAgent: "link-test", IP: "203.0.113.7"})
	return err
}

func TestAnInviteIsStoredHashedAndMailedWithTheTokenAndEndsNoSession(t *testing.T) {
	w := newLinkWorld(t)

	issued, err := w.issue(command.IssueSetPasswordLink{UserID: "account", Purpose: domain.LinkInvite, By: "teacher"})
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"recipient", "revoke-unused", "insert", "enqueue", "commit"}; !slices.Equal(w.links.order, want) {
		t.Fatalf("steps = %v, want %v", w.links.order, want)
	}
	mail, link := w.links.mails[0], w.links.inserted[0]
	if mail.Kind != domain.MailAccountInvite || mail.To != "an@example.test" || mail.UserID != "account" || mail.FullName != "Nguyễn Văn An" {
		t.Errorf("mail = %+v", mail)
	}
	if len(mail.Token) != 43 || strings.ContainsAny(mail.Token, "+/=") {
		t.Errorf("token %q is not 32 bytes of unpadded base64url", mail.Token)
	}
	if !slices.Equal(link.TokenHash, domain.HashSetPasswordToken(mail.Token)) {
		t.Error("the stored hash is not the SHA-256 of the mailed token")
	}
	if link.Purpose != domain.LinkInvite || link.CreatedBy != "teacher" || link.UserID != "account" {
		t.Errorf("link = %+v", link)
	}
	if link.Now.Nanosecond()%1000 != 0 {
		t.Errorf("the link's clock %v is not whole microseconds, which the 24 hour check needs", link.Now)
	}
	if want := link.Now.Add(24 * time.Hour); !issued.ExpiresAt.Equal(want) || issued.SignInInvite {
		t.Errorf("issued = %+v, want it to expire %v", issued, want)
	}
	if len(w.principals.forgets) != 0 {
		t.Errorf("an invite forgot %v", w.principals.forgets)
	}
}

func TestAResetEndsTheSessionsAndForgetsThePrincipalOnlyAfterTheCommit(t *testing.T) {
	w := newLinkWorld(t)

	if _, err := w.issue(command.IssueSetPasswordLink{UserID: "account", Purpose: domain.LinkReset, By: "teacher"}); err != nil {
		t.Fatal(err)
	}

	want := []string{"recipient", "revoke-unused", "insert", "end-sessions", "enqueue", "commit", "forget"}
	if !slices.Equal(w.links.order, want) {
		t.Fatalf("steps = %v, want %v", w.links.order, want)
	}
	if got := w.links.mails[0].Kind; got != domain.MailPasswordReset {
		t.Errorf("mail kind = %v, want the reset's", got)
	}
}

func TestAResetTheOwnerAsksForKeepsTheirSessions(t *testing.T) {
	w := newLinkWorld(t)

	if _, err := w.issue(command.IssueSetPasswordLink{UserID: "account", Purpose: domain.LinkReset, By: "account", KeepSessions: true}); err != nil {
		t.Fatal(err)
	}

	if slices.Contains(w.links.order, "end-sessions") || len(w.principals.forgets) != 0 {
		t.Errorf("a self-requested reset ended sessions: %v, forgot %v", w.links.order, w.principals.forgets)
	}
}

func TestAnIssueThatRollsBackForgetsNobody(t *testing.T) {
	w := newLinkWorld(t)
	w.links.enqueueErr = domain.ErrSetPasswordUnavailable

	_, err := w.issue(command.IssueSetPasswordLink{UserID: "account", Purpose: domain.LinkReset})

	if !errors.Is(err, domain.ErrSetPasswordUnavailable) {
		t.Fatalf("err = %v, want ErrSetPasswordUnavailable", err)
	}
	if len(w.principals.forgets) != 0 || w.links.committed || w.links.order[len(w.links.order)-1] != "rollback" {
		t.Errorf("a rolled-back issue ran its after-commit work: order %v, forgot %v", w.links.order, w.principals.forgets)
	}
}

func TestWithPasswordSignInOffAResetIsRefusedBeforeAnythingIsRead(t *testing.T) {
	w := newLinkWorld(t)
	w.app.SetSignInPolicy(policyOf(policyWith(passwordOff)))

	_, err := w.issue(command.IssueSetPasswordLink{UserID: "account", Purpose: domain.LinkReset})

	if !errors.Is(err, domain.ErrPasswordSignInDisabled) {
		t.Fatalf("err = %v, want ErrPasswordSignInDisabled", err)
	}
	if want := []string{"rollback"}; !slices.Equal(w.links.order, want) {
		t.Errorf("steps = %v: a refused reset read or wrote something", w.links.order)
	}
}

func TestWithPasswordSignInOffAnInviteSendsASignInInviteAndStoresNoLink(t *testing.T) {
	w := newLinkWorld(t)
	w.app.SetSignInPolicy(policyOf(policyWith(passwordOff)))

	issued, err := w.issue(command.IssueSetPasswordLink{UserID: "account", Purpose: domain.LinkInvite})
	if err != nil {
		t.Fatal(err)
	}

	if !issued.SignInInvite || !issued.ExpiresAt.IsZero() {
		t.Errorf("issued = %+v, want a sign-in invite with no expiry", issued)
	}
	if want := []string{"recipient", "enqueue", "commit"}; !slices.Equal(w.links.order, want) {
		t.Errorf("steps = %v, want %v", w.links.order, want)
	}
	if mail := w.links.mails[0]; mail.Kind != domain.MailSignInInvite || mail.Token != "" {
		t.Errorf("mail = %+v, want a sign-in invite that carries no token", mail)
	}
}

func TestADisabledAccountIsIssuedNothing(t *testing.T) {
	w := newLinkWorld(t)
	w.links.recipient.Disabled = true

	_, err := w.issue(command.IssueSetPasswordLink{UserID: "account", Purpose: domain.LinkInvite})

	if !errors.Is(err, domain.ErrAccountDisabled) {
		t.Fatalf("err = %v, want ErrAccountDisabled", err)
	}
	if len(w.links.inserted) != 0 || len(w.links.mails) != 0 {
		t.Errorf("a disabled account was issued a link or a mail: %+v %+v", w.links.inserted, w.links.mails)
	}
}

func TestAnUnknownAccountIsIssuedNothing(t *testing.T) {
	w := newLinkWorld(t)
	w.links.recipientErr = domain.ErrUserNotFound

	if _, err := w.issue(command.IssueSetPasswordLink{UserID: "nobody", Purpose: domain.LinkInvite}); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("err = %v, want ErrUserNotFound", err)
	}
}

func TestWithoutAStoreNoLinkCanBeIssuedOrUsed(t *testing.T) {
	app := policyApp(t, newPolicyStore(), policyOf(openPolicy()))
	app.SetPrincipals(&linkPrincipals{store: &linkStore{}})

	if _, err := app.Commands.RequestSetPasswordLink.Handle(context.Background(), command.RequestSetPasswordLink{UserID: "account"}); !errors.Is(err, domain.ErrSetPasswordUnavailable) {
		t.Errorf("request without a store: %v, want ErrSetPasswordUnavailable", err)
	}
	if err := (&linkWorld{app: app}).setPassword("anything", "một-mật-khẩu-9"); !errors.Is(err, domain.ErrSetPasswordLinkInvalid) {
		t.Errorf("use without a store: %v, want ErrSetPasswordLinkInvalid", err)
	}
	if _, err := app.Queries.CheckSetPasswordLink.Handle(context.Background(), query.CheckSetPasswordLink{Token: "anything"}); !errors.Is(err, domain.ErrSetPasswordLinkInvalid) {
		t.Errorf("check without a store: %v, want ErrSetPasswordLinkInvalid", err)
	}
}

func TestEveryLinkThatCannotBeUsedIsRefusedWithTheSameErrorAndNoPasswordIsHashed(t *testing.T) {
	used, revoked := linkNow.Add(-time.Minute), linkNow.Add(-time.Minute)
	for name, mutate := range map[string]func(*linkStore){
		"unknown":            func(s *linkStore) { s.findErr = domain.ErrSetPasswordLinkInvalid },
		"used":               func(s *linkStore) { s.link.UsedAt = &used },
		"revoked":            func(s *linkStore) { s.link.RevokedAt = &revoked },
		"expired":            func(s *linkStore) { s.link.ExpiresAt = linkNow.Add(-time.Second) },
		"expiring right now": func(s *linkStore) { s.link.ExpiresAt = linkNow },
		"account disabled":   func(s *linkStore) { s.link.AccountDisabled = true },
	} {
		t.Run(name, func(t *testing.T) {
			w := newLinkWorld(t)
			mutate(w.links)

			err := w.setPassword("a-token", "một-mật-khẩu-9")
			if !errors.Is(err, domain.ErrSetPasswordLinkInvalid) {
				t.Fatalf("set: %v, want ErrSetPasswordLinkInvalid", err)
			}
			if len(w.links.claims) != 0 || len(w.principals.forgets) != 0 || w.users.called("CreateRefreshToken") {
				t.Errorf("an unusable link changed something: claims %d, forgets %v, session stored %v", len(w.links.claims), w.principals.forgets, w.users.called("CreateRefreshToken"))
			}
			if _, err := w.app.Queries.CheckSetPasswordLink.Handle(context.Background(), query.CheckSetPasswordLink{Token: "a-token"}); !errors.Is(err, domain.ErrSetPasswordLinkInvalid) {
				t.Errorf("check: %v, want the same ErrSetPasswordLinkInvalid", err)
			}
		})
	}
}

func TestACheckNamesTheAccountAndWhatTheLinkIsFor(t *testing.T) {
	w := newLinkWorld(t)
	w.links.link.Purpose = domain.LinkReset

	got, err := w.app.Queries.CheckSetPasswordLink.Handle(context.Background(), query.CheckSetPasswordLink{Token: "a-token"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "an@example.test" || got.FullName != "Nguyễn Văn An" || got.Purpose != domain.LinkReset || !got.ExpiresAt.Equal(linkNow.Add(time.Hour)) {
		t.Errorf("check = %+v", got)
	}
	if w.links.finds != 1 || len(w.links.claims) != 0 {
		t.Errorf("a check looked %d times and claimed %d: it must read once and change nothing", w.links.finds, len(w.links.claims))
	}
}

func TestAWeakPasswordIsRefusedBeforeTheTokenIsLookedUp(t *testing.T) {
	for name, c := range map[string]struct {
		password string
		want     error
	}{
		"too short": {"ngắn1", domain.ErrPasswordTooShort},
		"too long":  {strings.Repeat("a1", domain.MaxPasswordLength), domain.ErrPasswordTooLong},
	} {
		t.Run(name, func(t *testing.T) {
			w := newLinkWorld(t)
			if err := w.setPassword("a-token", c.password); !errors.Is(err, c.want) {
				t.Fatalf("set: %v, want %v", err, c.want)
			}
			if w.links.finds != 0 {
				t.Error("a refused password read the token")
			}
		})
	}
}

func TestWithPasswordSignInOffASetIsRefusedBeforeTheTokenIsLookedUp(t *testing.T) {
	w := newLinkWorld(t)
	w.app.SetSignInPolicy(policyOf(policyWith(passwordOff)))

	if err := w.setPassword("a-token", "một-mật-khẩu-9"); !errors.Is(err, domain.ErrPasswordSignInDisabled) {
		t.Fatalf("set: %v, want ErrPasswordSignInDisabled", err)
	}
	if w.links.finds != 0 {
		t.Error("the policy answer depended on the token: it read it")
	}
}

func TestUsingALinkSetsTheHashedPasswordForgetsThePrincipalAndSignsIn(t *testing.T) {
	w := newLinkWorld(t)

	session, err := w.app.Commands.SetPasswordWithLink.Handle(context.Background(), command.SetPasswordWithLink{
		Token: "a-token", NewPassword: "một-mật-khẩu-9", UserAgent: "link-test", IP: "203.0.113.7",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(w.links.claims) != 1 {
		t.Fatalf("claims = %d, want 1", len(w.links.claims))
	}
	claim := w.links.claims[0]
	if !slices.Equal(claim.TokenHash, domain.HashSetPasswordToken("a-token")) {
		t.Error("the claim names a hash other than the token's")
	}
	if ok, err := domain.Passwords.Verify(context.Background(), "một-mật-khẩu-9", claim.NewHash); err != nil || !ok {
		t.Errorf("the claim's hash does not verify the password (%v)", err)
	}
	if strings.Contains(claim.NewHash, "một-mật-khẩu-9") {
		t.Error("the claim carries the password in the clear")
	}
	if claim.IP == nil || *claim.IP != "203.0.113.7" || claim.UserAgent == nil || *claim.UserAgent != "link-test" {
		t.Errorf("the claim's origin = %v, %v", claim.IP, claim.UserAgent)
	}
	if !slices.Equal(w.principals.forgets, []string{"account"}) {
		t.Errorf("forgot %v, want the account once", w.principals.forgets)
	}
	if want := []string{"find", "claim", "forget"}; !slices.Equal(w.links.order, want) {
		t.Errorf("steps = %v, want %v: the principal must be forgotten after the claim commits and before the session is issued", w.links.order, want)
	}
	if session.AccessToken == "" || session.RefreshToken == "" || session.User.ID != "account" {
		t.Errorf("session = %+v", session)
	}
	if !w.users.called("CreateRefreshToken") || !w.users.called("StampSignIn") {
		t.Errorf("the session was not stored and stamped: %v", w.users.calls)
	}
}

func TestAClaimThatLosesTheRaceIssuesNoSession(t *testing.T) {
	w := newLinkWorld(t)
	w.links.claimErr = domain.ErrSetPasswordLinkInvalid

	if err := w.setPassword("a-token", "một-mật-khẩu-9"); !errors.Is(err, domain.ErrSetPasswordLinkInvalid) {
		t.Fatalf("set: %v, want ErrSetPasswordLinkInvalid", err)
	}
	if len(w.principals.forgets) != 0 || w.users.called("CreateRefreshToken") {
		t.Errorf("a lost claim forgot %v or stored a session", w.principals.forgets)
	}
}

func TestAnAccountDisabledAfterTheClaimGetsNoSession(t *testing.T) {
	w := newLinkWorld(t)
	w.users.add(suspended(policyUser("account", "an@example.test")), "")

	if err := w.setPassword("a-token", "một-mật-khẩu-9"); !errors.Is(err, domain.ErrSetPasswordLinkInvalid) {
		t.Fatalf("set: %v, want ErrSetPasswordLinkInvalid", err)
	}
	if w.users.called("CreateRefreshToken") {
		t.Error("a disabled account was signed in")
	}
}

func TestAskingForALinkFromSettingsIssuesAResetAndAuditsIt(t *testing.T) {
	w := newLinkWorld(t)

	_, err := w.app.Commands.RequestSetPasswordLink.Handle(context.Background(), command.RequestSetPasswordLink{UserID: "account", IP: "203.0.113.7", UserAgent: "link-test"})
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"recipient", "revoke-unused", "insert", "enqueue", "audit", "commit"}; !slices.Equal(w.links.order, want) {
		t.Fatalf("steps = %v, want %v: a self request keeps its sessions and audits in the same transaction", w.links.order, want)
	}
	if link := w.links.inserted[0]; link.Purpose != domain.LinkReset || link.CreatedBy != "account" {
		t.Errorf("link = %+v, want a reset the account asked for itself", link)
	}
	entry := w.links.audits[0]
	if entry.Action != "user.password_link_requested" || entry.Entity != "user" || *entry.EntityID != "account" || *entry.ActorUserID != "account" || !entry.OccurredAt.Equal(linkNow) {
		t.Errorf("audit = %+v", entry)
	}
	if entry.Diff != nil {
		t.Errorf("the audit row carries a diff: %s", entry.Diff)
	}
}

func TestAskingForALinkIsRefusedForADisabledAccountAndWithPasswordSignInOffAndAuditsNothing(t *testing.T) {
	for name, c := range map[string]struct {
		arrange func(*linkWorld)
		want    error
	}{
		"disabled account": {func(w *linkWorld) { w.links.recipient.Disabled = true }, domain.ErrAccountDisabled},
		"password off":     {func(w *linkWorld) { w.app.SetSignInPolicy(policyOf(policyWith(passwordOff))) }, domain.ErrPasswordSignInDisabled},
		"unknown account":  {func(w *linkWorld) { w.links.recipientErr = domain.ErrUserNotFound }, domain.ErrUserNotFound},
		"cannot send":      {func(w *linkWorld) { w.links.enqueueErr = domain.ErrSetPasswordUnavailable }, domain.ErrSetPasswordUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			w := newLinkWorld(t)
			c.arrange(w)

			_, err := w.app.Commands.RequestSetPasswordLink.Handle(context.Background(), command.RequestSetPasswordLink{UserID: "account"})

			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			if len(w.links.audits) != 0 || w.links.committed {
				t.Errorf("a refused request audited %d times and committed %v", len(w.links.audits), w.links.committed)
			}
		})
	}
}

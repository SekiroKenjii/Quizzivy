//go:build integration

package application_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/email"
	"quizzivy/internal/platform/secrets"
)

const (
	linkOldPassword = "mật-khẩu-cũ-1"
	linkNewPassword = "mật-khẩu-mới-9"
)

func linkAppPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		t.Skip("APP_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var role string
	if err := pool.QueryRow(context.Background(), `SELECT current_user`).Scan(&role); err != nil || role != "quizzivy_app" {
		t.Fatalf("the app pool connects as %q (%v), want quizzivy_app", role, err)
	}
	return pool
}

type dbLinks struct {
	admin   *pgxpool.Pool
	sealer  *secrets.Sealer
	outbox  *email.Outbox
	links   *repositories.SetPasswordLinks
	app     *application.Application
	forgets *committedForgets
	marker  string
}

func newDBLinks(t *testing.T) *dbLinks {
	t.Helper()
	admin, appdb := newPool(t), linkAppPool(t)
	key, err := secrets.ParseKey(base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	sealer, err := secrets.New(key, secrets.PurposeEmailOutbox)
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := token.NewIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	w := &dbLinks{admin: admin, sealer: sealer, outbox: email.NewOutbox(sealer), marker: "link" + nonce(t) + ".example.test"}
	dbx := db.NewContext(appdb)
	w.links = repositories.NewSetPasswordLinks(dbx, w.outbox)
	w.app = application.New(repositories.NewUsers(dbx), issuer, 30*24*time.Hour, nil, nil)
	w.app.SetPasswordLinks(w.links)
	w.forgets = &committedForgets{pool: admin, calls: map[string][]accountState{}}
	w.app.SetPrincipals(w.forgets)
	t.Cleanup(func() {
		ctx := context.Background()
		owned := `SELECT id FROM app.users WHERE email LIKE '%@' || $1`
		for _, stmt := range []string{
			`DELETE FROM app.email_outbox WHERE to_address LIKE '%@' || $1`,
			`DELETE FROM app.refresh_tokens WHERE user_id IN (` + owned + `)`,
			`DELETE FROM app.audit_log WHERE entity_id IN (` + owned + `) OR actor_user_id IN (` + owned + `)`,
			`DELETE FROM app.user_identities WHERE user_id IN (` + owned + `)`,
			`DELETE FROM app.users WHERE email LIKE '%@' || $1`,
		} {
			if _, err := admin.Exec(ctx, stmt, w.marker); err != nil {
				t.Errorf("cleanup %q: %v", stmt, err)
			}
		}
	})
	return w
}

func (w *dbLinks) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := w.admin.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (w *dbLinks) account(t *testing.T, name string, password bool) string {
	t.Helper()
	var hash *string
	if password {
		h := "old-hash-" + name
		hash = &h
	}
	var id string
	if err := w.admin.QueryRow(context.Background(), `
		INSERT INTO app.users (email, full_name, role_id, password_hash, must_change_password)
		VALUES ($1, $2, (SELECT id FROM app.roles WHERE builtin_key = 'student'), $3::text, $3::text IS NOT NULL)
		RETURNING id::text`, name+"-"+uuid.NewString()[:8]+"@"+w.marker, "Nguyễn "+name, hash).Scan(&id); err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		w.exec(t, `INSERT INTO app.refresh_tokens (user_id, family_id, token_hash, expires_at) VALUES ($1::uuid, gen_random_uuid(), sha256(($2 || $1)::bytea), now() + interval '30 days')`, id, string(rune('a'+i)))
	}
	return id
}

func (w *dbLinks) issue(t *testing.T, userID string, purpose domain.SetPasswordPurpose, by string, keep bool) (domain.IssuedLink, error) {
	t.Helper()
	var issued domain.IssuedLink
	err := w.links.InTx(context.Background(), func(ctx context.Context, tx domain.LinkTx) error {
		var err error
		issued, err = w.app.Commands.IssueSetPasswordLink.Handle(ctx, command.IssueSetPasswordLink{Tx: tx, UserID: userID, Purpose: purpose, By: by, KeepSessions: keep})
		return err
	})
	return issued, err
}

type mailedLink struct {
	template string
	token    string
	rowID    string
	secret   []byte
}

func (w *dbLinks) lastMail(t *testing.T, userID string) mailedLink {
	t.Helper()
	var m mailedLink
	if err := w.admin.QueryRow(context.Background(), `
		SELECT id::text, template, secret_ciphertext FROM app.email_outbox WHERE to_user_id = $1::uuid ORDER BY id DESC LIMIT 1`, userID).Scan(&m.rowID, &m.template, &m.secret); err != nil {
		t.Fatalf("no mail for %s: %v", userID, err)
	}
	if m.secret != nil {
		plain, err := w.sealer.Open(m.secret, []byte(m.rowID))
		if err != nil {
			t.Fatalf("open the sealed token: %v", err)
		}
		m.token = string(plain)
	}
	return m
}

func (w *dbLinks) mails(t *testing.T, userID string) int {
	t.Helper()
	var n int
	if err := w.admin.QueryRow(context.Background(), `SELECT count(*) FROM app.email_outbox WHERE to_user_id = $1::uuid`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

type storedLink struct {
	Purpose   string
	CreatedBy *string
	Created   time.Time
	Expires   time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
}

func (w *dbLinks) rows(t *testing.T, userID string) []storedLink {
	t.Helper()
	found, err := w.admin.Query(context.Background(), `
		SELECT purpose, created_by::text, created_at, expires_at, used_at, revoked_at
		  FROM app.set_password_tokens WHERE user_id = $1::uuid ORDER BY created_at, id`, userID)
	if err != nil {
		t.Fatal(err)
	}
	defer found.Close()
	var out []storedLink
	for found.Next() {
		var l storedLink
		if err := found.Scan(&l.Purpose, &l.CreatedBy, &l.Created, &l.Expires, &l.UsedAt, &l.RevokedAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, l)
	}
	if err := found.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (w *dbLinks) state(t *testing.T, id string) accountState {
	t.Helper()
	s, err := readAccount(context.Background(), w.admin, id)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (w *dbLinks) use(token, password string) error {
	_, err := w.app.Commands.SetPasswordWithLink.Handle(context.Background(), command.SetPasswordWithLink{Token: token, NewPassword: password, UserAgent: "link-db-test", IP: "203.0.113.7"})
	return err
}

func (w *dbLinks) usable(token string) bool {
	_, err := w.app.Queries.CheckSetPasswordLink.Handle(context.Background(), query.CheckSetPasswordLink{Token: token})
	return err == nil
}

func TestAnInviteIsStoredAsAHashAndItsMailHoldsTheSealedToken(t *testing.T) {
	w := newDBLinks(t)
	teacher, student := w.account(t, "gv", true), w.account(t, "hs", false)

	issued, err := w.issue(t, student, domain.LinkInvite, teacher, false)
	if err != nil {
		t.Fatal(err)
	}

	rows := w.rows(t, student)
	if len(rows) != 1 {
		t.Fatalf("%d links stored, want 1", len(rows))
	}
	link := rows[0]
	if link.Purpose != "invite" || link.CreatedBy == nil || *link.CreatedBy != teacher || link.UsedAt != nil || link.RevokedAt != nil {
		t.Errorf("link = %+v", link)
	}
	if got := link.Expires.Sub(link.Created); got != 24*time.Hour || !link.Expires.Equal(issued.ExpiresAt) {
		t.Errorf("the link lives %v and expires %v, want 24h and %v", got, link.Expires, issued.ExpiresAt)
	}

	mail := w.lastMail(t, student)
	if mail.template != "account_invite" || len(mail.token) != 43 {
		t.Fatalf("mail = %+v, want an account invite that seals a 43 character token", mail)
	}
	var stored []byte
	if err := w.admin.QueryRow(context.Background(), `SELECT token_hash FROM app.set_password_tokens WHERE user_id = $1::uuid`, student).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(domain.HashSetPasswordToken(mail.token)) {
		t.Error("the stored hash is not the SHA-256 of the sealed token")
	}
	var dump string
	if err := w.admin.QueryRow(context.Background(), `
		SELECT (SELECT coalesce(string_agg(to_jsonb(t)::text, ' '), '') FROM app.set_password_tokens t WHERE t.user_id = $1::uuid) || ' ' ||
		       (SELECT coalesce(string_agg(to_jsonb(o)::text, ' '), '') FROM app.email_outbox o WHERE o.to_user_id = $1::uuid) || ' ' ||
		       (SELECT coalesce(string_agg(to_jsonb(a)::text, ' '), '') FROM app.audit_log a WHERE a.entity_id = $1::uuid)`, student).Scan(&dump); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dump, mail.token) {
		t.Errorf("the token is stored in the clear: %s", dump)
	}
	select {
	case <-w.outbox.Wake():
	default:
		t.Error("the dispatcher was not woken after the commit")
	}
	if !w.usable(mail.token) {
		t.Error("a fresh link is not usable")
	}
}

func TestIssuingAnotherLinkRevokesTheEarlierOneAndOnlyTheNewestWorks(t *testing.T) {
	w := newDBLinks(t)
	student := w.account(t, "hs", true)

	if _, err := w.issue(t, student, domain.LinkInvite, "", false); err != nil {
		t.Fatal(err)
	}
	first := w.lastMail(t, student).token
	if _, err := w.issue(t, student, domain.LinkReset, "", false); err != nil {
		t.Fatal(err)
	}
	second := w.lastMail(t, student)

	rows := w.rows(t, student)
	if len(rows) != 2 || rows[0].RevokedAt == nil || rows[1].RevokedAt != nil {
		t.Fatalf("links = %+v, want the first revoked and the second live", rows)
	}
	if w.usable(first) || !w.usable(second.token) || second.template != "password_reset_link" {
		t.Errorf("the first link usable = %v, the second usable = %v (%s)", w.usable(first), w.usable(second.token), second.template)
	}
	if err := w.use(first, linkNewPassword); !errors.Is(err, domain.ErrSetPasswordLinkInvalid) {
		t.Errorf("using the revoked link: %v, want ErrSetPasswordLinkInvalid", err)
	}
}

func TestAResetEndsTheAccountsSessionsAndForgetsItAfterTheCommitUnlessTheOwnerAsked(t *testing.T) {
	w := newDBLinks(t)
	byTeacher, byOwner := w.account(t, "gv-hoi", true), w.account(t, "chu", true)

	if _, err := w.issue(t, byTeacher, domain.LinkReset, "", false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.issue(t, byOwner, domain.LinkReset, byOwner, true); err != nil {
		t.Fatal(err)
	}

	ended, kept := w.state(t, byTeacher), w.state(t, byOwner)
	if ended.Epoch != 1 || ended.LiveFamilies != 0 {
		t.Errorf("a reset someone else asked for left %+v, want the epoch moved and no live family", ended)
	}
	if calls := w.forgets.of(byTeacher); len(calls) != 1 || calls[0].Epoch != 1 || calls[0].LiveFamilies != 0 {
		t.Errorf("the account was forgotten as %+v, want once, after the commit", calls)
	}
	if kept.Epoch != 0 || kept.LiveFamilies != 2 || len(w.forgets.of(byOwner)) != 0 {
		t.Errorf("a reset the owner asked for left %+v, forgot %d times, want the sessions untouched", kept, len(w.forgets.of(byOwner)))
	}
}

func TestUsingALinkSetsThePasswordSignsInAndEndsEveryOtherSession(t *testing.T) {
	w := newDBLinks(t)
	student := w.account(t, "hs", true)
	if _, err := w.issue(t, student, domain.LinkReset, "", true); err != nil {
		t.Fatal(err)
	}
	spare := w.lastMail(t, student).token
	if _, err := w.issue(t, student, domain.LinkReset, "", true); err != nil {
		t.Fatal(err)
	}
	token := w.lastMail(t, student).token
	before := w.state(t, student)

	session, err := w.app.Commands.SetPasswordWithLink.Handle(context.Background(), command.SetPasswordWithLink{
		Token: token, NewPassword: linkNewPassword, UserAgent: "link-db-test", IP: "203.0.113.7",
	})
	if err != nil {
		t.Fatal(err)
	}

	after := w.state(t, student)
	if ok, err := domain.Passwords.Verify(context.Background(), linkNewPassword, after.Hash); err != nil || !ok {
		t.Errorf("the stored hash does not verify the new password (%v)", err)
	}
	if after.MustChange || after.Epoch != before.Epoch+1 || after.LiveFamilies != 1 {
		t.Errorf("account after = %+v, before = %+v: want the forced change cleared, the epoch moved once and only the new session live", after, before)
	}
	if session.AccessToken == "" || session.RefreshToken == "" || session.User.ID != student {
		t.Errorf("session = %+v", session)
	}
	if calls := w.forgets.of(student); len(calls) != 1 || calls[0].Epoch != after.Epoch {
		t.Errorf("forgotten as %+v, want once at the new epoch", calls)
	}

	var method string
	var changedAt *time.Time
	if err := w.admin.QueryRow(context.Background(), `SELECT last_sign_in_method, password_changed_at FROM app.users WHERE id = $1::uuid`, student).Scan(&method, &changedAt); err != nil {
		t.Fatal(err)
	}
	if method != "set_password_link" || changedAt == nil {
		t.Errorf("sign-in method %q, password changed at %v, want set_password_link and a time", method, changedAt)
	}

	rows := w.rows(t, student)
	if len(rows) != 2 || rows[0].RevokedAt == nil || rows[1].UsedAt == nil || rows[1].RevokedAt != nil {
		t.Errorf("links = %+v, want the earlier one revoked and the used one used", rows)
	}
	if err := w.use(spare, "một-mật-khẩu-khác-7"); !errors.Is(err, domain.ErrSetPasswordLinkInvalid) {
		t.Errorf("using the earlier link afterwards: %v", err)
	}

	var actor, entity, diff, ip, agent string
	if err := w.admin.QueryRow(context.Background(), `
		SELECT actor_user_id::text, entity_id::text, diff::text, host(ip), user_agent
		  FROM app.audit_log WHERE entity_id = $1::uuid AND action = 'user.password_set'`, student).Scan(&actor, &entity, &diff, &ip, &agent); err != nil {
		t.Fatalf("no audit row: %v", err)
	}
	if actor != student || ip != "203.0.113.7" || agent != "link-db-test" || !strings.Contains(diff, `"purpose"`) || strings.Contains(diff, token) || strings.Contains(diff, after.Hash) {
		t.Errorf("audit = actor %s, ip %s, agent %s, diff %s", actor, ip, agent, diff)
	}
}

func TestALinkSetsThePasswordOnceAndAnyLaterUseIsInvalid(t *testing.T) {
	w := newDBLinks(t)
	student := w.account(t, "hs", false)
	if _, err := w.issue(t, student, domain.LinkInvite, "", false); err != nil {
		t.Fatal(err)
	}
	token := w.lastMail(t, student).token

	if err := w.use(token, linkNewPassword); err != nil {
		t.Fatal(err)
	}
	first := w.state(t, student)
	if err := w.use(token, "một-mật-khẩu-khác-7"); !errors.Is(err, domain.ErrSetPasswordLinkInvalid) {
		t.Fatalf("a second use: %v, want ErrSetPasswordLinkInvalid", err)
	}

	if after := w.state(t, student); after != first {
		t.Errorf("a second use changed the account: %+v became %+v", first, after)
	}
	if w.usable(token) {
		t.Error("a used link still reads usable")
	}
}

func TestEightSubmitsOfOneLinkAtOnceSetThePasswordOnce(t *testing.T) {
	w := newDBLinks(t)
	student := w.account(t, "hs", false)
	if _, err := w.issue(t, student, domain.LinkInvite, "", false); err != nil {
		t.Fatal(err)
	}
	token := w.lastMail(t, student).token

	const submits = 8
	passwords := make([]string, submits)
	errs := make([]error, submits)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range submits {
		passwords[i] = fmt.Sprintf("mật-khẩu-thứ-%d-ok", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = w.use(token, passwords[i])
		}()
	}
	close(start)
	wg.Wait()

	winner := -1
	for i, err := range errs {
		switch {
		case err == nil && winner == -1:
			winner = i
		case err == nil:
			t.Errorf("submits %d and %d both succeeded", winner, i)
		case !errors.Is(err, domain.ErrSetPasswordLinkInvalid):
			t.Errorf("submit %d failed with %v, want ErrSetPasswordLinkInvalid", i, err)
		}
	}
	if winner == -1 {
		t.Fatal("no submit succeeded")
	}
	after := w.state(t, student)
	for i, password := range passwords {
		ok, err := domain.Passwords.Verify(context.Background(), password, after.Hash)
		if err != nil {
			t.Fatal(err)
		}
		if ok != (i == winner) {
			t.Errorf("password %d verifies = %v, want %v", i, ok, i == winner)
		}
	}
	var audits int
	if err := w.admin.QueryRow(context.Background(), `SELECT count(*) FROM app.audit_log WHERE entity_id = $1::uuid AND action = 'user.password_set'`, student).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 1 || after.LiveFamilies != 1 || after.Epoch != 1 {
		t.Errorf("audit rows %d, state %+v: want one audit row, the epoch moved once and only the winner's session live", audits, after)
	}
}

func TestADisabledAccountsLinkIsInvalidAndStaysUnused(t *testing.T) {
	w := newDBLinks(t)
	student := w.account(t, "hs", true)
	if _, err := w.issue(t, student, domain.LinkReset, "", false); err != nil {
		t.Fatal(err)
	}
	token := w.lastMail(t, student).token
	w.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, student)
	before := w.state(t, student)

	if w.usable(token) {
		t.Error("the check read a disabled account's link as usable")
	}
	if err := w.use(token, linkNewPassword); !errors.Is(err, domain.ErrSetPasswordLinkInvalid) {
		t.Errorf("using it: %v, want ErrSetPasswordLinkInvalid", err)
	}
	_, claimErr := w.links.Claim(context.Background(), domain.ClaimLink{TokenHash: domain.HashSetPasswordToken(token), NewHash: "hash-that-must-not-land", Now: time.Now()})
	if !errors.Is(claimErr, domain.ErrSetPasswordLinkInvalid) {
		t.Errorf("claiming it directly: %v, want ErrSetPasswordLinkInvalid", claimErr)
	}

	if rows := w.rows(t, student); rows[0].UsedAt != nil {
		t.Error("a refused claim left the link used")
	}
	if after := w.state(t, student); after != before {
		t.Errorf("a refused claim changed the account: %+v became %+v", before, after)
	}
}

func TestIssuingForADisabledAccountWritesNothing(t *testing.T) {
	w := newDBLinks(t)
	student := w.account(t, "hs", true)
	w.exec(t, `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, student)

	_, err := w.issue(t, student, domain.LinkInvite, "", false)

	if !errors.Is(err, domain.ErrAccountDisabled) {
		t.Fatalf("err = %v, want ErrAccountDisabled", err)
	}
	if len(w.rows(t, student)) != 0 || w.mails(t, student) != 0 {
		t.Error("a link or a mail was written for a disabled account")
	}
}

func TestALinkExpiresAtExactlyTwentyFourHours(t *testing.T) {
	w := newDBLinks(t)
	student := w.account(t, "hs", true)
	issuedAt := time.Now().Truncate(time.Microsecond)
	clock := issuedAt
	w.app.SetClock(func() time.Time { return clock })
	if _, err := w.issue(t, student, domain.LinkInvite, "", false); err != nil {
		t.Fatal(err)
	}
	token := w.lastMail(t, student).token

	for name, c := range map[string]struct {
		at   time.Time
		want bool
	}{
		"just issued":                        {issuedAt, true},
		"a microsecond before 24 hours":      {issuedAt.Add(24*time.Hour - time.Microsecond), true},
		"exactly 24 hours":                   {issuedAt.Add(24 * time.Hour), false},
		"a microsecond after 24 hours":       {issuedAt.Add(24*time.Hour + time.Microsecond), false},
		"a day and a half":                   {issuedAt.Add(36 * time.Hour), false},
		"the clock set back before issuing":  {issuedAt.Add(-time.Hour), true},
		"back to a microsecond before 24 hr": {issuedAt.Add(24*time.Hour - time.Microsecond), true},
	} {
		clock = c.at
		if got := w.usable(token); got != c.want {
			t.Errorf("%s: check usable = %v, want %v", name, got, c.want)
		}
	}

	clock = issuedAt.Add(24 * time.Hour)
	if err := w.use(token, linkNewPassword); !errors.Is(err, domain.ErrSetPasswordLinkInvalid) {
		t.Errorf("using it at exactly 24 hours: %v, want ErrSetPasswordLinkInvalid", err)
	}
	if rows := w.rows(t, student); rows[0].UsedAt != nil {
		t.Error("a claim at the expiry left the link used")
	}
	clock = issuedAt.Add(24*time.Hour - time.Microsecond)
	if err := w.use(token, linkNewPassword); err != nil {
		t.Errorf("using it a microsecond before expiry: %v", err)
	}
}

func TestAGoogleOnlyAccountAskingForALinkKeepsItsSessionAndItsGoogleLink(t *testing.T) {
	w := newDBLinks(t)
	student := w.account(t, "gg", false)
	w.exec(t, `INSERT INTO app.user_identities (user_id, provider, provider_user_id, email_at_link) VALUES ($1::uuid, 'google', $2, 'gg@example.test')`, student, "sub-"+uuid.NewString())

	if _, err := w.app.Commands.RequestSetPasswordLink.Handle(context.Background(), command.RequestSetPasswordLink{UserID: student, IP: "203.0.113.7", UserAgent: "link-db-test"}); err != nil {
		t.Fatal(err)
	}

	asked := w.state(t, student)
	if asked.Epoch != 0 || asked.LiveFamilies != 2 || len(w.forgets.of(student)) != 0 {
		t.Errorf("asking for a link left %+v, forgot %d times: want the sessions untouched", asked, len(w.forgets.of(student)))
	}
	rows := w.rows(t, student)
	if len(rows) != 1 || rows[0].Purpose != "reset" || rows[0].CreatedBy == nil || *rows[0].CreatedBy != student {
		t.Fatalf("links = %+v, want one reset the account asked for itself", rows)
	}
	var audits int
	if err := w.admin.QueryRow(context.Background(), `SELECT count(*) FROM app.audit_log WHERE entity_id = $1::uuid AND actor_user_id = $1::uuid AND action = 'user.password_link_requested'`, student).Scan(&audits); err != nil || audits != 1 {
		t.Errorf("%d audit rows (%v), want 1", audits, err)
	}

	token := w.lastMail(t, student).token
	if err := w.use(token, linkNewPassword); err != nil {
		t.Fatal(err)
	}
	var identities int
	if err := w.admin.QueryRow(context.Background(), `SELECT count(*) FROM app.user_identities WHERE user_id = $1::uuid`, student).Scan(&identities); err != nil || identities != 1 {
		t.Errorf("%d Google links after setting a password (%v), want the account to keep its one", identities, err)
	}
}

func TestAnIssueThatRollsBackLeavesNoLinkNoMailAndNoWakeUp(t *testing.T) {
	w := newDBLinks(t)
	student := w.account(t, "hs", true)
	boom := errors.New("the caller failed after issuing")

	err := w.links.InTx(context.Background(), func(ctx context.Context, tx domain.LinkTx) error {
		if _, err := w.app.Commands.IssueSetPasswordLink.Handle(ctx, command.IssueSetPasswordLink{Tx: tx, UserID: student, Purpose: domain.LinkReset}); err != nil {
			return err
		}
		return boom
	})

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if len(w.rows(t, student)) != 0 || w.mails(t, student) != 0 {
		t.Error("a rolled-back issue left a link or a mail")
	}
	if after := w.state(t, student); after.Epoch != 0 || after.LiveFamilies != 2 || len(w.forgets.of(student)) != 0 {
		t.Errorf("a rolled-back reset changed the account or forgot it: %+v", after)
	}
	select {
	case <-w.outbox.Wake():
		t.Error("the dispatcher was woken for a message that was rolled back")
	default:
	}
}

func TestIssuingAndUsingLinksAtOnceNeverDeadlock(t *testing.T) {
	w := newDBLinks(t)
	student := w.account(t, "hs", true)

	for round := range 12 {
		if _, err := w.issue(t, student, domain.LinkReset, "", false); err != nil {
			t.Fatalf("round %d: %v", round, err)
		}
		token := w.lastMail(t, student).token
		errs := make([]error, 2)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			errs[0] = w.use(token, fmt.Sprintf("mật-khẩu-vòng-%d-ok", round))
		}()
		go func() {
			defer wg.Done()
			_, errs[1] = w.issue(t, student, domain.LinkReset, "", false)
		}()
		wg.Wait()

		if errs[0] != nil && !errors.Is(errs[0], domain.ErrSetPasswordLinkInvalid) {
			t.Fatalf("round %d: using the link failed with %v", round, errs[0])
		}
		if errs[1] != nil {
			t.Fatalf("round %d: issuing failed with %v", round, errs[1])
		}
		var live int
		if err := w.admin.QueryRow(context.Background(), `SELECT count(*) FROM app.set_password_tokens WHERE user_id = $1::uuid AND used_at IS NULL AND revoked_at IS NULL`, student).Scan(&live); err != nil {
			t.Fatal(err)
		}
		if live > 1 {
			t.Fatalf("round %d: %d unused links are live at once, want at most the newest", round, live)
		}
	}
}

//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	accessapp "quizzivy/internal/modules/access/application"
	accessdomain "quizzivy/internal/modules/access/domain"
	accessrepo "quizzivy/internal/modules/access/repositories"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/shared/access"
)

type forgottenAt struct {
	Disabled bool
	Epoch    int
	RoleID   string
	Hash     string
}

type recordingPrincipals struct {
	inner *accessapp.Application
	w     *usersWorld
	mu    sync.Mutex
	seen  map[string][]forgottenAt
}

func (r *recordingPrincipals) Resolve(ctx context.Context, id string) (access.Principal, error) {
	return r.inner.Resolve(ctx, id)
}

func (r *recordingPrincipals) Forget(id string) {
	var at forgottenAt
	_ = r.w.admin.QueryRow(context.Background(), `
		SELECT u.disabled_at IS NOT NULL, u.session_epoch, u.role_id::text, coalesce(u.password_hash, '')
		  FROM app.users u WHERE u.id = $1::uuid`, id).Scan(&at.Disabled, &at.Epoch, &at.RoleID, &at.Hash)
	r.inner.Forget(id)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen[id] = append(r.seen[id], at)
}

func (r *recordingPrincipals) of(id string) []forgottenAt {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.seen[id])
}

type writesWorld struct {
	*usersWorld
	principals *recordingPrincipals
}

func newWritesWorld(t *testing.T) *writesWorld {
	t.Helper()
	return withPrincipals(newUsersWorld(t, true))
}

func withPrincipals(w *usersWorld) *writesWorld {
	p := &recordingPrincipals{inner: accessapp.New(accessrepo.NewPostgres(w.dbx)), w: w, seen: map[string][]forgottenAt{}}
	w.app.SetPrincipals(p)
	return &writesWorld{usersWorld: w, principals: p}
}

type accountRow struct {
	RoleID       string
	Disabled     bool
	Epoch        int
	Hash         string
	MustChange   bool
	LiveSessions int
	LiveLinks    int
	Pending      int
	Cancelled    int
	Sealed       int
	Sent         int
}

func (w *writesWorld) row(t *testing.T, id string) accountRow {
	t.Helper()
	var r accountRow
	err := w.admin.QueryRow(context.Background(), `
		SELECT u.role_id::text, u.disabled_at IS NOT NULL, u.session_epoch, coalesce(u.password_hash, ''), u.must_change_password,
		       (SELECT count(*) FROM app.refresh_tokens t WHERE t.user_id = u.id AND t.revoked_at IS NULL),
		       (SELECT count(*) FROM app.set_password_tokens l WHERE l.user_id = u.id AND l.used_at IS NULL AND l.revoked_at IS NULL),
		       (SELECT count(*) FROM app.email_outbox o WHERE o.to_user_id = u.id AND o.sent_at IS NULL AND o.failed_at IS NULL),
		       (SELECT count(*) FROM app.email_outbox o WHERE o.to_user_id = u.id AND o.last_error_code = 'cancelled'),
		       (SELECT count(*) FROM app.email_outbox o WHERE o.to_user_id = u.id AND o.secret_ciphertext IS NOT NULL),
		       (SELECT count(*) FROM app.email_outbox o WHERE o.to_user_id = u.id AND o.sent_at IS NOT NULL)
		  FROM app.users u WHERE u.id = $1::uuid`, id).Scan(&r.RoleID, &r.Disabled, &r.Epoch, &r.Hash, &r.MustChange,
		&r.LiveSessions, &r.LiveLinks, &r.Pending, &r.Cancelled, &r.Sealed, &r.Sent)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (w *writesWorld) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := w.admin.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (w *writesWorld) account(t *testing.T, name, builtin string) string {
	t.Helper()
	id := w.insertUser(t, name, builtin)
	hash, err := domain.Passwords.Hash(context.Background(), "mat-khau-cu-"+name)
	if err != nil {
		t.Fatal(err)
	}
	w.exec(t, `UPDATE app.users SET password_hash = $2 WHERE id = $1::uuid`, id, hash)
	return id
}

func (w *writesWorld) holdSessions(t *testing.T, id string, n int) {
	t.Helper()
	for range n {
		w.exec(t, `
			INSERT INTO app.refresh_tokens (user_id, family_id, token_hash, expires_at)
			VALUES ($1::uuid, gen_random_uuid(), sha256(gen_random_uuid()::text::bytea), now() + interval '30 days')`, id)
	}
}

func (w *writesWorld) holdLink(t *testing.T, id string) {
	t.Helper()
	w.exec(t, `
		INSERT INTO app.set_password_tokens (user_id, token_hash, purpose, expires_at)
		VALUES ($1::uuid, sha256(gen_random_uuid()::text::bytea), 'invite', now() + interval '1 day')`, id)
}

func (w *writesWorld) holdMail(t *testing.T, id string, sent bool) {
	t.Helper()
	w.exec(t, `
		INSERT INTO app.email_outbox (to_user_id, to_address, template, locale, secret_ciphertext, sent_at)
		SELECT u.id, u.email, 'account_invite', 'vi', 'sealed'::bytea, CASE WHEN $2 THEN now() END
		  FROM app.users u WHERE u.id = $1::uuid`, id, sent)
}

type auditRow struct {
	Actor *string
	IP    *string
	Agent *string
	Diff  map[string]any
}

func (w *writesWorld) audits(t *testing.T, id, action string) []auditRow {
	t.Helper()
	rows, err := w.admin.Query(context.Background(), `
		SELECT actor_user_id::text, host(ip), user_agent, coalesce(diff, '{}'::jsonb)
		  FROM app.audit_log WHERE entity_id = $1::uuid AND action = $2 ORDER BY occurred_at, id`, id, action)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var r auditRow
		var diff []byte
		if err := rows.Scan(&r.Actor, &r.IP, &r.Agent, &diff); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(diff, &r.Diff); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func (w *writesWorld) disable(req domain.WriteRequest, id string) error {
	_, err := w.app.Commands.DisableUser.Handle(context.Background(), command.DisableUser{Request: req, ID: id})
	return err
}

func (w *writesWorld) changeRole(req domain.WriteRequest, id, role string) error {
	_, err := w.app.Commands.ChangeUserRole.Handle(context.Background(), command.ChangeUserRole{Request: req, ID: id, RoleID: role})
	return err
}

func TestADisableEndsEverythingTheAccountHeldAndIsAuditedWithItsOrigin(t *testing.T) {
	w := newWritesWorld(t)
	target := w.account(t, "gv", "teacher")
	w.holdSessions(t, target, 2)
	w.holdLink(t, target)
	w.holdMail(t, target, false)
	w.holdMail(t, target, true)
	before := w.row(t, target)
	if before.LiveSessions != 2 || before.LiveLinks != 1 || before.Pending != 1 || before.Sent != 1 {
		t.Fatalf("setup = %+v", before)
	}

	if err := w.disable(w.everything(), target); err != nil {
		t.Fatal(err)
	}

	after := w.row(t, target)
	if !after.Disabled || after.Epoch != before.Epoch+1 || after.LiveSessions != 0 || after.LiveLinks != 0 {
		t.Errorf("after = %+v, want it disabled, the epoch moved, no live session and no live link", after)
	}
	if after.Pending != 0 || after.Cancelled != 1 || after.Sent != 1 || after.Sealed != 1 {
		t.Errorf("mail = %+v, want the waiting message cancelled with its secret erased and the sent one untouched", after)
	}
	audits := w.audits(t, target, "user.disabled")
	if len(audits) != 1 || audits[0].Actor == nil || *audits[0].Actor != w.actor || audits[0].IP == nil || *audits[0].IP != "203.0.113.5" || *audits[0].Agent != "users-test" {
		t.Errorf("audit = %+v, want one row naming the actor and the request's origin", audits)
	}
	if seen := w.principals.of(target); len(seen) != 1 || !seen[0].Disabled || seen[0].Epoch != before.Epoch+1 {
		t.Errorf("forgot at %+v, want once and only after the disable committed", seen)
	}
}

func TestDisablingADisabledAccountChangesAndAuditsNothing(t *testing.T) {
	w := newWritesWorld(t)
	target := w.account(t, "gv", "teacher")
	if err := w.disable(w.everything(), target); err != nil {
		t.Fatal(err)
	}
	before := w.row(t, target)

	if err := w.disable(w.everything(), target); err != nil {
		t.Fatal(err)
	}

	if after := w.row(t, target); after != before {
		t.Errorf("a second disable changed %+v into %+v", before, after)
	}
	if n := len(w.audits(t, target, "user.disabled")); n != 1 {
		t.Errorf("%d audit rows, want the first disable's only", n)
	}
}

func TestEnableLetsTheAccountInAgainAndWhatTheDisableEndedStaysEnded(t *testing.T) {
	w := newWritesWorld(t)
	target := w.account(t, "hv", "student")
	w.holdSessions(t, target, 1)
	w.holdLink(t, target)
	if err := w.disable(w.everything(), target); err != nil {
		t.Fatal(err)
	}
	disabled := w.row(t, target)

	if _, err := w.app.Commands.EnableUser.Handle(context.Background(), command.EnableUser{Request: w.everything(), ID: target}); err != nil {
		t.Fatal(err)
	}

	after := w.row(t, target)
	if after.Disabled || after.Epoch != disabled.Epoch || after.LiveSessions != 0 || after.LiveLinks != 0 || after.Hash != disabled.Hash {
		t.Errorf("after = %+v from %+v, want only the disabled flag cleared", after, disabled)
	}
	if n := len(w.audits(t, target, "user.enabled")); n != 1 {
		t.Errorf("%d user.enabled rows, want 1", n)
	}
	if seen := w.principals.of(target); len(seen) != 2 || seen[1].Disabled {
		t.Errorf("forgot at %+v, want once per write and the second after the account was enabled", seen)
	}

	if _, err := w.app.Commands.EnableUser.Handle(context.Background(), command.EnableUser{Request: w.everything(), ID: target}); err != nil {
		t.Fatal(err)
	}
	if n := len(w.audits(t, target, "user.enabled")); n != 1 {
		t.Errorf("enabling an enabled account wrote another audit row: %d", n)
	}
}

func TestSigningOutEndsSessionsAndKeepsLinksAndTheCallerMaySignThemselvesOut(t *testing.T) {
	w := newWritesWorld(t)
	target := w.account(t, "gv", "teacher")
	for _, id := range []string{target, w.actor} {
		w.holdSessions(t, id, 2)
		w.holdLink(t, id)
		before := w.row(t, id)

		if _, err := w.app.Commands.SignOutUser.Handle(context.Background(), command.SignOutUser{Request: w.everything(), ID: id}); err != nil {
			t.Fatalf("signing out %s: %v", id, err)
		}

		after := w.row(t, id)
		if after.Epoch != before.Epoch+1 || after.LiveSessions != 0 || after.LiveLinks != 1 || after.Disabled || after.Hash != before.Hash {
			t.Errorf("after = %+v from %+v, want the sessions ended and nothing else", after, before)
		}
		if n := len(w.audits(t, id, "user.signed_out_everywhere")); n != 1 {
			t.Errorf("%d audit rows for %s, want 1", n, id)
		}
		if seen := w.principals.of(id); len(seen) != 1 || seen[0].Epoch != before.Epoch+1 {
			t.Errorf("forgot %s at %+v, want once after the epoch moved", id, seen)
		}
	}
}

func TestAChangedRoleAppliesAtOnceEndsSessionsAndRecordsBothNames(t *testing.T) {
	w := newWritesWorld(t)
	target := w.account(t, "tg", "teacher")
	w.holdSessions(t, target, 1)
	before := w.row(t, target)
	grader := w.customRole(t, access.TeachingGrading)

	if err := w.changeRole(w.everything(), target, grader); err != nil {
		t.Fatal(err)
	}

	after := w.row(t, target)
	if after.RoleID != grader || after.Epoch != before.Epoch+1 || after.LiveSessions != 0 {
		t.Errorf("after = %+v from %+v, want the new role, a moved epoch and no session", after, before)
	}
	resolved, err := w.principals.Resolve(context.Background(), target)
	if err != nil || resolved.RoleID != grader || resolved.Epoch != after.Epoch {
		t.Errorf("the next request resolves to %+v (%v), want the new role at the new epoch: the cache must have been forgotten", resolved, err)
	}
	audits := w.audits(t, target, "user.role_changed")
	if len(audits) != 1 {
		t.Fatalf("%d audit rows, want 1", len(audits))
	}
	var oldName, newName string
	if err := w.admin.QueryRow(context.Background(), `
		SELECT (SELECT name FROM app.roles WHERE builtin_key = 'teacher'), (SELECT name FROM app.roles WHERE id = $1::uuid)`, grader).Scan(&oldName, &newName); err != nil {
		t.Fatal(err)
	}
	if diff, _ := audits[0].Diff["role"].(map[string]any); diff["old"] != oldName || diff["new"] != newName {
		t.Errorf("audit role = %v, want old %q and new %q", audits[0].Diff, oldName, newName)
	}
	if seen := w.principals.of(target); len(seen) != 1 || seen[0].RoleID != grader {
		t.Errorf("forgot at %+v, want once after the role changed", seen)
	}

	if err := w.changeRole(w.everything(), target, grader); err != nil {
		t.Fatal(err)
	}
	if again := w.row(t, target); again != after {
		t.Errorf("giving the role the account holds changed %+v into %+v", after, again)
	}
	if n := len(w.audits(t, target, "user.role_changed")); n != 1 {
		t.Errorf("%d audit rows after a no-op change, want 1", n)
	}
}

func TestAnOwnerKeepsTheirRoleUntilTheirContentMovesButMayChangeToAnotherTeacherRole(t *testing.T) {
	w := newWritesWorld(t)
	owner := w.account(t, "chu", "teacher")
	grader := w.customRole(t, access.TeachingGrading)
	t.Cleanup(func() { w.exec(t, `DELETE FROM app.classes WHERE teacher_id = $1::uuid`, owner) })
	w.exec(t, `INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2::uuid), ($3, $2::uuid)`, "Lớp A "+nonce(t), owner, "Lớp B "+nonce(t))
	before := w.row(t, owner)

	err := w.changeRole(w.everything(), owner, w.role(t, "student"))

	var owns *domain.OwnsContentError
	if !errors.As(err, &owns) || owns.Owned.Classes != 2 {
		t.Fatalf("answered %v, want the counts of what the account owns", err)
	}
	if after := w.row(t, owner); after != before {
		t.Errorf("a refused change altered %+v into %+v", before, after)
	}
	if n := len(w.audits(t, owner, "user.role_changed")); n != 0 {
		t.Errorf("a refused change wrote %d audit rows", n)
	}
	if seen := w.principals.of(owner); len(seen) != 0 {
		t.Errorf("a refused change forgot the account: %+v", seen)
	}

	if err := w.changeRole(w.everything(), owner, grader); err != nil {
		t.Fatalf("a role that opens the teacher workspace: %v", err)
	}
}

func TestARoleThatIsNotAssignableOrNotThereIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	w := newWritesWorld(t)
	target := w.account(t, "gv", "teacher")
	before := w.row(t, target)

	if err := w.changeRole(w.everything(), target, uuid.NewString()); !errors.Is(err, accessdomain.ErrRoleNotFound) {
		t.Errorf("an unknown role: %v, want ErrRoleNotFound", err)
	}
	if err := w.changeRole(w.peopleOnly(), target, w.role(t, "admin")); !errors.Is(err, accessdomain.ErrTargetRoleNotAllowed) && !errors.Is(err, accessdomain.ErrNotAssignable) {
		t.Errorf("a role beyond the caller: %v, want a refusal", err)
	}
	if after := w.row(t, target); after != before {
		t.Errorf("refusals changed %+v into %+v", before, after)
	}
}

func TestAWriteMadeAgainstAStaleRoleIsRefusedAndWritesNothing(t *testing.T) {
	w := newWritesWorld(t)
	target := w.account(t, "gv", "teacher")
	before := w.row(t, target)
	repo := repositories.NewAdminUsers(w.dbx, w.links)
	req := w.everything()
	stale := domain.AccountWrite{UserID: target, RoleID: w.role(t, "student"), Now: time.Now()}

	for name, run := range map[string]func() error{
		"disable":  func() error { return repo.Disable(context.Background(), req, stale) },
		"enable":   func() error { return repo.Enable(context.Background(), req, stale) },
		"sign out": func() error { return repo.SignOut(context.Background(), req, stale) },
		"reset": func() error {
			return repo.ResetPassword(context.Background(), req, domain.AccountPasswordReset{AccountWrite: stale, Delivery: domain.DeliveryTemporaryPassword, Hash: "$argon2id$x"})
		},
		"role change": func() error {
			return repo.ChangeRole(context.Background(), req, domain.RoleChange{AccountWrite: stale, ToRoleID: w.role(t, "assistant"), ToRoleName: "x", ToOpensTeacher: true})
		},
	} {
		if err := run(); !errors.Is(err, domain.ErrRoleChanged) {
			t.Errorf("%s against a stale role: %v, want ErrRoleChanged", name, err)
		}
	}
	if after := w.row(t, target); after != before {
		t.Errorf("stale writes changed %+v into %+v", before, after)
	}
}

func TestATemporaryPasswordResetReplacesTheHashAndEndsWhatTheOldOneOpened(t *testing.T) {
	for name, c := range map[string]struct {
		mutate     func(*usersWorld)
		mustChange bool
	}{
		"the centre forces a change": {func(*usersWorld) {}, true},
		"the centre lets it stay":    {func(w *usersWorld) { w.policy = policyWith(keepTemporary) }, false},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWritesWorld(t)
			c.mutate(w.usersWorld)
			target := w.account(t, "gv", "teacher")
			w.holdSessions(t, target, 2)
			w.holdLink(t, target)
			before := w.row(t, target)

			result, err := w.app.Commands.ResetUserPassword.Handle(context.Background(), command.ResetUserPassword{
				Request: w.everything(), ID: target, Delivery: domain.DeliveryTemporaryPassword,
			})
			if err != nil {
				t.Fatal(err)
			}

			after := w.row(t, target)
			if result.TemporaryPassword == "" || after.Hash == before.Hash || after.Hash == result.TemporaryPassword {
				t.Fatalf("password %q, hash %q (was %q), want a new hash and never the password", result.TemporaryPassword, after.Hash, before.Hash)
			}
			if ok, err := domain.Passwords.Verify(context.Background(), result.TemporaryPassword, after.Hash); err != nil || !ok {
				t.Errorf("the stored hash does not verify the returned password: %v %v", ok, err)
			}
			if ok, _ := domain.Passwords.Verify(context.Background(), "mat-khau-cu-gv", after.Hash); ok {
				t.Error("the old password still verifies")
			}
			if after.MustChange != c.mustChange || after.Epoch != before.Epoch+1 || after.LiveSessions != 0 || after.LiveLinks != 0 || after.Pending != 0 {
				t.Errorf("after = %+v, want must-change %v, a moved epoch, no session, no link and no mail", after, c.mustChange)
			}
			audits := w.audits(t, target, "user.password_reset")
			if len(audits) != 1 || audits[0].Diff["delivery"] != "temporary_password" {
				t.Errorf("audit = %+v, want one row naming the delivery", audits)
			}
			if seen := w.principals.of(target); len(seen) != 1 || seen[0].Hash != after.Hash || seen[0].Epoch != after.Epoch {
				t.Errorf("forgot at %+v, want once after the reset committed", seen)
			}
		})
	}
}

func TestAnEmailLinkResetClearsThePasswordAtOnceAndLeavesExactlyOneLinkAndOneMail(t *testing.T) {
	w := newWritesWorld(t)
	target := w.account(t, "gv", "teacher")
	w.holdSessions(t, target, 1)
	w.holdLink(t, target)
	before := w.row(t, target)

	result, err := w.app.Commands.ResetUserPassword.Handle(context.Background(), command.ResetUserPassword{
		Request: w.everything(), ID: target, Delivery: domain.DeliveryEmailLink,
	})
	if err != nil {
		t.Fatal(err)
	}

	after := w.row(t, target)
	if result.TemporaryPassword != "" || after.Hash != "" || after.MustChange || after.Epoch != before.Epoch+1 || after.LiveSessions != 0 {
		t.Errorf("result %q, after %+v, want no password at all, a moved epoch and no session", result.TemporaryPassword, after)
	}
	var purpose, createdBy string
	if err := w.admin.QueryRow(context.Background(), `
		SELECT purpose, created_by::text FROM app.set_password_tokens
		 WHERE user_id = $1::uuid AND used_at IS NULL AND revoked_at IS NULL`, target).Scan(&purpose, &createdBy); err != nil || purpose != "reset" || createdBy != w.actor {
		t.Errorf("live link = %q by %q (%v), want one reset link from the actor", purpose, createdBy, err)
	}
	if after.LiveLinks != 1 || after.Pending != 1 || after.Sealed != 1 {
		t.Errorf("after = %+v, want one live link and one sealed message waiting", after)
	}
	var template string
	if err := w.admin.QueryRow(context.Background(), `SELECT template FROM app.email_outbox WHERE to_user_id = $1::uuid AND sent_at IS NULL`, target).Scan(&template); err != nil || template != "password_reset_link" {
		t.Errorf("template = %q (%v), want the reset link", template, err)
	}
	if audits := w.audits(t, target, "user.password_reset"); len(audits) != 1 || audits[0].Diff["delivery"] != "email_link" {
		t.Errorf("audit = %+v", audits)
	}
}

func TestAResetOfADisabledAccountIsRefusedAndWritesNothing(t *testing.T) {
	w := newWritesWorld(t)
	target := w.account(t, "gv", "teacher")
	if err := w.disable(w.everything(), target); err != nil {
		t.Fatal(err)
	}
	before := w.row(t, target)

	for _, delivery := range []domain.Delivery{domain.DeliveryTemporaryPassword, domain.DeliveryEmailLink} {
		_, err := w.app.Commands.ResetUserPassword.Handle(context.Background(), command.ResetUserPassword{Request: w.everything(), ID: target, Delivery: delivery})
		if !errors.Is(err, domain.ErrAccountDisabled) {
			t.Errorf("a %s reset: %v, want ErrAccountDisabled", delivery, err)
		}
	}
	if after := w.row(t, target); after != before {
		t.Errorf("refused resets changed %+v into %+v", before, after)
	}
	if n := len(w.audits(t, target, "user.password_reset")); n != 0 {
		t.Errorf("%d audit rows for refused resets", n)
	}
}

func TestAResetWhileSendingIsOffLeavesTheAccountAsItWas(t *testing.T) {
	w := newUsersWorld(t, false)
	ww := withPrincipals(w)
	target := ww.account(t, "gv", "teacher")
	ww.holdSessions(t, target, 1)
	before := ww.row(t, target)

	_, err := ww.app.Commands.ResetUserPassword.Handle(context.Background(), command.ResetUserPassword{Request: ww.everything(), ID: target, Delivery: domain.DeliveryEmailLink})

	if !errors.Is(err, domain.ErrSetPasswordUnavailable) {
		t.Fatalf("answered %v, want ErrSetPasswordUnavailable", err)
	}
	if after := ww.row(t, target); after != before {
		t.Errorf("the failed reset changed %+v into %+v: the old password must still work", before, after)
	}
}

func TestABulkResetOverTheDatabaseResetsWhoItCanAndSharesOneBatchIdAcrossTheAudit(t *testing.T) {
	w := newWritesWorld(t)
	one, two := w.account(t, "mot", "teacher"), w.account(t, "hai", "student")
	gone := w.account(t, "khoa", "teacher")
	if err := w.disable(w.everything(), gone); err != nil {
		t.Fatal(err)
	}
	missing := uuid.NewString()
	w.holdSessions(t, one, 1)
	disabledBefore := w.row(t, gone)

	result, err := w.app.Commands.ResetUsersPasswords.Handle(context.Background(), command.ResetUsersPasswords{
		Request: w.everything(), IDs: []string{one, gone, missing, w.actor, two}, Delivery: domain.DeliveryTemporaryPassword,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Reset) != 2 || result.Reset[0].UserID != one || result.Reset[1].UserID != two {
		t.Fatalf("reset %+v, want the two accounts that could be, in order", result.Reset)
	}
	want := map[string]error{gone: domain.ErrAccountDisabled, missing: domain.ErrUserNotFound, w.actor: domain.ErrCannotActOnSelf}
	if len(result.Failed) != len(want) {
		t.Fatalf("failed %+v", result.Failed)
	}
	for _, failure := range result.Failed {
		if !errors.Is(failure.Reason, want[failure.UserID]) {
			t.Errorf("%s failed with %v, want %v", failure.UserID, failure.Reason, want[failure.UserID])
		}
	}
	for _, reset := range result.Reset {
		row := w.row(t, reset.UserID)
		if ok, err := domain.Passwords.Verify(context.Background(), reset.TemporaryPassword, row.Hash); err != nil || !ok || !row.MustChange {
			t.Errorf("%s: the returned password does not verify against the stored hash (%v %v), must change %v", reset.UserID, ok, err, row.MustChange)
		}
	}
	if after := w.row(t, gone); after != disabledBefore {
		t.Errorf("a refused account changed: %+v became %+v", disabledBefore, after)
	}
	batches := map[string]bool{}
	for _, id := range []string{one, two} {
		var batch string
		if err := w.admin.QueryRow(context.Background(), `SELECT diff->>'batchId' FROM app.audit_log WHERE entity_id = $1::uuid AND action = 'user.password_reset'`, id).Scan(&batch); err != nil {
			t.Fatal(err)
		}
		batches[batch] = true
	}
	if len(batches) != 1 || batches[""] {
		t.Errorf("batch ids = %v, want the one id of the call on both rows", batches)
	}
	if n := w.count(t, `SELECT count(*) FROM app.audit_log WHERE action = 'user.password_reset' AND entity_id = ANY($1::uuid[])`, []string{gone, w.actor}); n != 0 {
		t.Errorf("%d audit rows for accounts that were not reset", n)
	}
}

func TestABulkDisableOverTheDatabaseDisablesWhoItCanAndReportsTheRest(t *testing.T) {
	w := newWritesWorld(t)
	one, two := w.account(t, "mot", "teacher"), w.account(t, "hai", "student")
	w.holdMail(t, one, false)
	missing := uuid.NewString()

	result, err := w.app.Commands.DisableUsers.Handle(context.Background(), command.DisableUsers{
		Request: w.everything(), IDs: []string{one, missing, w.actor, two},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(result.Disabled, []string{one, two}) {
		t.Errorf("disabled %v, want the two that could be", result.Disabled)
	}
	if len(result.Failed) != 2 || result.Failed[0].UserID != missing || !errors.Is(result.Failed[0].Reason, domain.ErrUserNotFound) ||
		result.Failed[1].UserID != w.actor || !errors.Is(result.Failed[1].Reason, domain.ErrCannotActOnSelf) {
		t.Errorf("failed %+v, want the unknown account and the caller, in order", result.Failed)
	}
	if r := w.row(t, one); !r.Disabled || r.Pending != 0 || r.Cancelled != 1 {
		t.Errorf("first = %+v", r)
	}
	if w.row(t, w.actor).Disabled {
		t.Error("the caller was disabled")
	}
	var a, b string
	for id, into := range map[string]*string{one: &a, two: &b} {
		if err := w.admin.QueryRow(context.Background(), `SELECT diff->>'batchId' FROM app.audit_log WHERE entity_id = $1::uuid AND action = 'user.disabled'`, id).Scan(into); err != nil {
			t.Fatal(err)
		}
	}
	if a == "" || a != b {
		t.Errorf("batch ids %q and %q, want one shared id", a, b)
	}
}

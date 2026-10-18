package application_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	accessmodel "quizzivy/internal/modules/access/application/model"
	accessquery "quizzivy/internal/modules/access/application/query"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	organizationmodel "quizzivy/internal/modules/organization/application/model"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
	"quizzivy/internal/shared/validation"
)

type accountShelf struct {
	domain.AdminUsers
	mu      sync.Mutex
	users   map[string]domain.AdminUser
	writes  []string
	reqs    []domain.WriteRequest
	resets  []domain.AccountPasswordReset
	refuse  map[string]error
	changed map[string]int
	held    map[string]int
	panics  map[string]bool
}

func newShelf(ids ...string) *accountShelf {
	s := &accountShelf{users: map[string]domain.AdminUser{}, refuse: map[string]error{}, changed: map[string]int{}, held: map[string]int{}, panics: map[string]bool{}}
	for _, id := range ids {
		s.users[id] = domain.AdminUser{ID: id, FullName: "Tên " + id, Email: id + "@example.test", Status: domain.AccountActive}
	}
	return s
}

func (s *accountShelf) Get(_ context.Context, id string) (domain.AdminUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[id]
	if !ok {
		return domain.AdminUser{}, domain.ErrUserNotFound
	}
	return user, nil
}

func (s *accountShelf) write(what string, req domain.WriteRequest, in domain.AccountWrite) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.panics[in.UserID] {
		panic("the repository broke on " + in.UserID)
	}
	if s.changed[in.UserID] > 0 {
		s.changed[in.UserID]--
		return domain.ErrRoleChanged
	}
	if s.held[in.UserID] > 0 {
		s.held[in.UserID]--
		return fmt.Errorf("%w: deadlock detected", domain.ErrWriteContended)
	}
	if err := s.refuse[in.UserID]; err != nil {
		return err
	}
	s.writes = append(s.writes, what+":"+in.UserID)
	s.reqs = append(s.reqs, req)
	return nil
}

func (s *accountShelf) ChangeRole(_ context.Context, req domain.WriteRequest, in domain.RoleChange) error {
	return s.write("role>"+in.ToRoleID, req, in.AccountWrite)
}

func (s *accountShelf) Disable(_ context.Context, req domain.WriteRequest, in domain.AccountWrite) error {
	return s.write("disable", req, in)
}

func (s *accountShelf) Enable(_ context.Context, req domain.WriteRequest, in domain.AccountWrite) error {
	return s.write("enable", req, in)
}

func (s *accountShelf) SignOut(_ context.Context, req domain.WriteRequest, in domain.AccountWrite) error {
	return s.write("sign-out", req, in)
}

func (s *accountShelf) ResetPassword(_ context.Context, req domain.WriteRequest, in domain.AccountPasswordReset) error {
	if err := s.write("reset-"+string(in.Delivery), req, in.AccountWrite); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resets = append(s.resets, in)
	return nil
}

func (s *accountShelf) written() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.writes)
}

type writeRig struct {
	app       *application.Application
	shelf     *accountShelf
	principal *forgetLog
	passwords *passwordSource
	policy    organizationmodel.SignInPolicy
	logs      *strings.Builder
}

var (
	teacherKeys = access.NewSet(access.TeachingGrading)
	adminKeys   = access.NewSet(access.All()...)
	theAdmin    = domain.WriteRequest{ActorID: "admin", All: true, Grants: adminKeys, IP: "203.0.113.7", UserAgent: "writes-test"}
)

func newWriteRig(ids ...string) *writeRig {
	r := &writeRig{shelf: newShelf(ids...), passwords: &passwordSource{}, policy: openPolicy(), logs: &strings.Builder{}}
	sets := map[string]access.Set{"admin": adminKeys, "other-admin": adminKeys}
	for _, id := range ids {
		sets[id] = teacherKeys
	}
	r.principal = &forgetLog{heldPermissions: heldPermissions{sets: sets}}
	r.app = application.New(nil, nil, time.Hour, nil, nil)
	r.app.SetPrincipals(r.principal)
	r.app.SetTemporaryPasswords(r.passwords.make)
	r.app.SetLogger(slog.New(slog.NewTextHandler(r.logs, nil)))
	r.app.SetSignInPolicy(switchablePolicy(func() organizationmodel.SignInPolicy { return r.policy }))
	r.app.SetAdminUsers(r.shelf, rolesReturning(accessmodel.AssignableRole{ID: "teacher-role", Name: "Giáo viên", Revision: 4, OpensTeacher: true}))
	return r
}

func rolesReturning(role accessmodel.AssignableRole) cqrs.QueryHandler[accessquery.AssignableRole, accessmodel.AssignableRole] {
	return cqrs.HandlerFunc[accessquery.AssignableRole, accessmodel.AssignableRole](
		func(_ context.Context, q accessquery.AssignableRole) (accessmodel.AssignableRole, error) {
			if q.RoleID != role.ID {
				return accessmodel.AssignableRole{}, accessdomain.ErrRoleNotFound
			}
			return role, nil
		})
}

type accountAction struct {
	name string
	self bool
	run  func(r *writeRig, req domain.WriteRequest, id string) error
}

var accountActions = []accountAction{
	{"change role", false, func(r *writeRig, req domain.WriteRequest, id string) error {
		_, err := r.app.Commands.ChangeUserRole.Handle(context.Background(), command.ChangeUserRole{Request: req, ID: id, RoleID: "teacher-role"})
		return err
	}},
	{"disable", false, func(r *writeRig, req domain.WriteRequest, id string) error {
		_, err := r.app.Commands.DisableUser.Handle(context.Background(), command.DisableUser{Request: req, ID: id})
		return err
	}},
	{"enable", true, func(r *writeRig, req domain.WriteRequest, id string) error {
		_, err := r.app.Commands.EnableUser.Handle(context.Background(), command.EnableUser{Request: req, ID: id})
		return err
	}},
	{"sign out", true, func(r *writeRig, req domain.WriteRequest, id string) error {
		_, err := r.app.Commands.SignOutUser.Handle(context.Background(), command.SignOutUser{Request: req, ID: id})
		return err
	}},
	{"reset to a temporary password", false, func(r *writeRig, req domain.WriteRequest, id string) error {
		_, err := r.app.Commands.ResetUserPassword.Handle(context.Background(), command.ResetUserPassword{Request: req, ID: id, Delivery: domain.DeliveryTemporaryPassword})
		return err
	}},
}

func TestEveryAccountWriteRefusesAnUnknownAccountAndOneBeyondTheCaller(t *testing.T) {
	for _, a := range accountActions {
		t.Run(a.name, func(t *testing.T) {
			r := newWriteRig("pupil")
			r.principal.sets["strong"] = adminKeys
			manager := domain.WriteRequest{ActorID: "manager", Grants: access.NewSet(access.PeopleUsersManage, access.PeopleRolesManage)}

			if err := a.run(r, theAdmin, "ghost"); !errors.Is(err, domain.ErrUserNotFound) {
				t.Errorf("an unknown account: %v, want ErrUserNotFound", err)
			}
			if err := a.run(r, manager, "pupil"); !errors.Is(err, accessdomain.ErrTargetRoleNotAllowed) {
				t.Errorf("an account holding a key the caller lacks: %v, want ErrTargetRoleNotAllowed", err)
			}
			if err := a.run(r, manager, "strong"); !errors.Is(err, accessdomain.ErrTargetRoleNotAllowed) {
				t.Errorf("an Admin acted on by a manager: %v, want ErrTargetRoleNotAllowed", err)
			}
			if got := r.shelf.written(); len(got) != 0 {
				t.Errorf("a refusal wrote %v", got)
			}
			if got := r.principal.forgotten(); len(got) != 0 {
				t.Errorf("a refusal forgot %v, want nothing: nothing changed", got)
			}
			if r.passwords.calls.Load() != 0 {
				t.Errorf("a refusal made %d temporary passwords", r.passwords.calls.Load())
			}
		})
	}
}

func TestOnlySigningOutAndEnablingAreAllowedOnTheCallersOwnAccount(t *testing.T) {
	for _, a := range accountActions {
		t.Run(a.name, func(t *testing.T) {
			r := newWriteRig()
			r.shelf.users["admin"] = domain.AdminUser{ID: "admin", Status: domain.AccountActive}

			err := a.run(r, theAdmin, "admin")

			if a.self {
				if err != nil || len(r.shelf.written()) != 1 {
					t.Errorf("on oneself: %v after writing %v, want one write", err, r.shelf.written())
				}
				return
			}
			if !errors.Is(err, domain.ErrCannotActOnSelf) {
				t.Errorf("on oneself: %v, want ErrCannotActOnSelf", err)
			}
			if len(r.shelf.written()) != 0 || r.passwords.calls.Load() != 0 {
				t.Errorf("a refusal wrote %v and made %d passwords", r.shelf.written(), r.passwords.calls.Load())
			}
		})
	}
}

func TestAWriteThatSucceedsForgetsTheAccountOnceAndCarriesTheRequestsOrigin(t *testing.T) {
	for _, a := range accountActions {
		t.Run(a.name, func(t *testing.T) {
			r := newWriteRig("pupil")

			if err := a.run(r, theAdmin, "pupil"); err != nil {
				t.Fatal(err)
			}

			if got := r.principal.forgotten(); !slices.Equal(got, []string{"pupil"}) {
				t.Errorf("forgot %v, want the account once", got)
			}
			if len(r.shelf.reqs) != 1 || r.shelf.reqs[0].IP != "203.0.113.7" || r.shelf.reqs[0].UserAgent != "writes-test" || r.shelf.reqs[0].ActorID != "admin" {
				t.Errorf("the repository was given %+v, want the request's actor and origin", r.shelf.reqs)
			}
		})
	}
}

func TestAFaultFromTheRepositoryIsNotAnAnswerAndForgetsNothing(t *testing.T) {
	for _, a := range accountActions {
		t.Run(a.name, func(t *testing.T) {
			r := newWriteRig("pupil")
			r.shelf.refuse["pupil"] = errors.New("pq: deadlock detected")

			if err := a.run(r, theAdmin, "pupil"); err == nil || !strings.Contains(err.Error(), "deadlock") {
				t.Errorf("answered %v, want the fault", err)
			}
			if got := r.principal.forgotten(); len(got) != 0 {
				t.Errorf("forgot %v after a fault that wrote nothing", got)
			}
		})
	}
}

func TestATargetWhoseRoleChangedIsReadAgainAndAThirdChangeIsRefused(t *testing.T) {
	for _, a := range accountActions {
		t.Run(a.name, func(t *testing.T) {
			r := newWriteRig("pupil")
			r.shelf.changed["pupil"] = 2

			if err := a.run(r, theAdmin, "pupil"); err != nil {
				t.Fatalf("after two changes: %v, want the third read to go through", err)
			}
			if got := r.principal.forgotten(); !slices.Equal(got, []string{"pupil", "pupil", "pupil"}) {
				t.Errorf("forgot %v, want the stale principal dropped after each change and the account after the write", got)
			}

			r = newWriteRig("pupil")
			r.shelf.changed["pupil"] = 3
			if err := a.run(r, theAdmin, "pupil"); !errors.Is(err, domain.ErrRoleChanged) {
				t.Errorf("after three changes: %v, want ErrRoleChanged", err)
			}
			if got := r.shelf.written(); len(got) != 0 {
				t.Errorf("wrote %v", got)
			}
		})
	}
}

func TestAWriteTheDatabaseEndedOverAContendedLockIsMadeAgainAndAThirdIsRefused(t *testing.T) {
	for _, a := range accountActions {
		t.Run(a.name, func(t *testing.T) {
			r := newWriteRig("pupil")
			r.shelf.held["pupil"] = 2

			if err := a.run(r, theAdmin, "pupil"); err != nil {
				t.Fatalf("after two contended writes: %v, want the third to go through", err)
			}
			if got := r.shelf.written(); len(got) != 1 {
				t.Errorf("wrote %v, want one write", got)
			}

			r = newWriteRig("pupil")
			r.shelf.held["pupil"] = 3
			if err := a.run(r, theAdmin, "pupil"); !errors.Is(err, domain.ErrWriteContended) {
				t.Errorf("after three contended writes: %v, want ErrWriteContended", err)
			}
			if got := r.shelf.written(); len(got) != 0 {
				t.Errorf("wrote %v", got)
			}
			if got := r.principal.forgotten(); len(got) != 0 {
				t.Errorf("forgot %v after writes that wrote nothing", got)
			}
		})
	}
}

func TestChangingARoleNamesTheRoleItsRevisionAndWhetherItOpensTheTeacherWorkspace(t *testing.T) {
	r := newWriteRig("pupil")
	var given domain.RoleChange
	r.app.SetAdminUsers(roleSpy{AdminUsers: r.shelf, into: &given},
		rolesReturning(accessmodel.AssignableRole{ID: "scribe", Builtin: access.BuiltinAdmin, Name: "Người ghi", Revision: 9}))

	_, err := r.app.Commands.ChangeUserRole.Handle(context.Background(), command.ChangeUserRole{Request: theAdmin, ID: "pupil", RoleID: "scribe"})
	if err != nil {
		t.Fatal(err)
	}

	if given.UserID != "pupil" || given.ToRoleID != "scribe" || given.ToRoleName != "Người ghi" || given.ToRoleRevision != 9 || !given.ToAdmin || given.ToOpensTeacher {
		t.Errorf("repository was given %+v", given)
	}

	r = newWriteRig("pupil")
	for _, refused := range []error{accessdomain.ErrRoleNotFound, accessdomain.ErrNotAssignable, accessdomain.ErrTargetRoleNotAllowed} {
		r.app.SetAdminUsers(r.shelf, cqrs.HandlerFunc[accessquery.AssignableRole, accessmodel.AssignableRole](
			func(context.Context, accessquery.AssignableRole) (accessmodel.AssignableRole, error) {
				return accessmodel.AssignableRole{}, refused
			}))
		_, err := r.app.Commands.ChangeUserRole.Handle(context.Background(), command.ChangeUserRole{Request: theAdmin, ID: "pupil", RoleID: "x"})
		if !errors.Is(err, refused) {
			t.Errorf("a role the access module refuses with %v: %v", refused, err)
		}
	}
	if got := r.shelf.written(); len(got) != 0 {
		t.Errorf("a refused role wrote %v", got)
	}
}

type roleSpy struct {
	domain.AdminUsers
	into *domain.RoleChange
}

func (s roleSpy) ChangeRole(_ context.Context, _ domain.WriteRequest, in domain.RoleChange) error {
	*s.into = in
	return nil
}

func (s roleSpy) Get(_ context.Context, id string) (domain.AdminUser, error) {
	return domain.AdminUser{ID: id}, nil
}

func TestAnAccountThatOwnsContentIsRefusedWithWhatItOwns(t *testing.T) {
	r := newWriteRig("owner")
	owned := domain.OwnedContent{Classes: 2, Tests: 3}
	r.shelf.refuse["owner"] = &domain.OwnsContentError{Owned: owned}

	_, err := r.app.Commands.ChangeUserRole.Handle(context.Background(), command.ChangeUserRole{Request: theAdmin, ID: "owner", RoleID: "teacher-role"})

	var got *domain.OwnsContentError
	if !errors.As(err, &got) || got.Owned != owned {
		t.Fatalf("answered %v, want the counts of what the account owns", err)
	}
	if forgotten := r.principal.forgotten(); len(forgotten) != 0 {
		t.Errorf("forgot %v after a refusal", forgotten)
	}
}

func TestADisabledAccountIsNotResetAndAnEnabledOneIsNotEnabledTwice(t *testing.T) {
	r := newWriteRig("gone")
	user := r.shelf.users["gone"]
	user.Status = domain.AccountDisabled
	r.shelf.users["gone"] = user

	for _, delivery := range []domain.Delivery{domain.DeliveryTemporaryPassword, domain.DeliveryEmailLink} {
		_, err := r.app.Commands.ResetUserPassword.Handle(context.Background(), command.ResetUserPassword{Request: theAdmin, ID: "gone", Delivery: delivery})
		if delivery == domain.DeliveryEmailLink && errors.Is(err, domain.ErrSetPasswordUnavailable) {
			continue
		}
		if !errors.Is(err, domain.ErrAccountDisabled) {
			t.Errorf("a %s reset of a disabled account: %v, want ErrAccountDisabled", delivery, err)
		}
	}
	if r.passwords.calls.Load() != 0 || len(r.shelf.written()) != 0 {
		t.Errorf("a refused reset made %d passwords and wrote %v", r.passwords.calls.Load(), r.shelf.written())
	}
}

func TestAPasswordResetNeedsPasswordSignInAndAKnownDelivery(t *testing.T) {
	r := newWriteRig("pupil")
	r.policy = policyWith(passwordOff)

	_, err := r.app.Commands.ResetUserPassword.Handle(context.Background(), command.ResetUserPassword{Request: theAdmin, ID: "ghost", Delivery: domain.DeliveryTemporaryPassword})
	if !errors.Is(err, domain.ErrPasswordSignInDisabled) {
		t.Errorf("single reset with password sign-in off: %v, want ErrPasswordSignInDisabled before the account is looked up", err)
	}
	_, err = r.app.Commands.ResetUsersPasswords.Handle(context.Background(), command.ResetUsersPasswords{Request: theAdmin, IDs: []string{"pupil"}, Delivery: domain.DeliveryTemporaryPassword})
	if !errors.Is(err, domain.ErrPasswordSignInDisabled) {
		t.Errorf("bulk reset with password sign-in off: %v, want ErrPasswordSignInDisabled", err)
	}

	r.policy = openPolicy()
	_, err = r.app.Commands.ResetUserPassword.Handle(context.Background(), command.ResetUserPassword{Request: theAdmin, ID: "pupil", Delivery: domain.Delivery("carrier_pigeon")})
	if err == nil {
		t.Error("an unknown delivery was accepted")
	}
	_, err = r.app.Commands.ResetUserPassword.Handle(context.Background(), command.ResetUserPassword{Request: theAdmin, ID: "pupil", Delivery: domain.DeliveryEmailLink})
	if !errors.Is(err, domain.ErrSetPasswordUnavailable) {
		t.Errorf("a link on a server with no way to send one: %v, want ErrSetPasswordUnavailable", err)
	}
	if r.passwords.calls.Load() != 0 || len(r.shelf.written()) != 0 {
		t.Errorf("refusals made %d passwords and wrote %v", r.passwords.calls.Load(), r.shelf.written())
	}
}

func TestATemporaryPasswordIsKeptOnlyWhereTheCentreAllowsIt(t *testing.T) {
	for name, c := range map[string]struct {
		policy organizationmodel.SignInPolicy
		keep   bool
	}{
		"the centre forces a change": {openPolicy(), false},
		"the centre lets it stay":    {policyWith(keepTemporary), true},
	} {
		t.Run(name, func(t *testing.T) {
			r := newWriteRig("pupil")
			r.policy = c.policy

			result, err := r.app.Commands.ResetUserPassword.Handle(context.Background(), command.ResetUserPassword{Request: theAdmin, ID: "pupil", Delivery: domain.DeliveryTemporaryPassword})
			if err != nil {
				t.Fatal(err)
			}

			if result.TemporaryPassword == "" || len(r.shelf.resets) != 1 || r.shelf.resets[0].Hash == "" || r.shelf.reqs[0].KeepTemporaryPassword != c.keep {
				t.Errorf("password %q, resets %+v, keep %v, want a password with its hash and keep %v", result.TemporaryPassword, r.shelf.resets, r.shelf.reqs[0].KeepTemporaryPassword, c.keep)
			}
		})
	}
}

func TestABulkDisableReportsEachRefusalAndDisablesTheRest(t *testing.T) {
	r := newWriteRig("one", "two", "last", "broken", "boom", "outside")
	r.principal.sets["outside"] = adminKeys
	r.shelf.refuse["last"] = domain.ErrLastAdmin
	r.shelf.refuse["broken"] = errors.New("pq: connection reset by peer, password=hunter2")
	r.shelf.panics["boom"] = true
	by := domain.WriteRequest{ActorID: "manager", Grants: access.NewSet(access.PeopleUsersManage, access.TeachingGrading)}
	r.principal.sets["manager"] = by.Grants

	result, err := r.app.Commands.DisableUsers.Handle(context.Background(), command.DisableUsers{
		Request: by, IDs: []string{"one", "ghost", "manager", "last", "broken", "outside", "boom", "two"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(result.Disabled, []string{"one", "two"}) {
		t.Errorf("disabled %v, want the two that could be", result.Disabled)
	}
	want := []domain.UserFailure{
		{UserID: "ghost", Reason: domain.ErrUserNotFound},
		{UserID: "manager", Reason: domain.ErrCannotActOnSelf},
		{UserID: "last", Reason: domain.ErrLastAdmin},
		{UserID: "broken", Reason: domain.ErrActionFailed},
		{UserID: "outside", Reason: accessdomain.ErrTargetRoleNotAllowed},
		{UserID: "boom", Reason: domain.ErrActionFailed},
	}
	if len(result.Failed) != len(want) {
		t.Fatalf("failed %+v, want %+v in the order named", result.Failed, want)
	}
	for i, w := range want {
		if result.Failed[i].UserID != w.UserID || !errors.Is(result.Failed[i].Reason, w.Reason) {
			t.Errorf("failure %d = %+v, want %+v", i, result.Failed[i], w)
		}
	}
	for _, failure := range result.Failed {
		if failure.Reason != domain.ErrActionFailed && strings.Contains(fmt.Sprint(failure.Reason), "hunter2") {
			t.Errorf("a failure carries the fault's text: %v", failure.Reason)
		}
	}
	if strings.Contains(fmt.Sprint(result.Failed), "hunter2") {
		t.Errorf("the answer carries the fault's text: %+v", result.Failed)
	}
	if !strings.Contains(r.logs.String(), "broken") || !strings.Contains(r.logs.String(), "boom") {
		t.Errorf("the faults were not logged: %q", r.logs.String())
	}
	batches := map[string]bool{}
	for _, req := range r.shelf.reqs {
		batches[req.BatchID] = true
	}
	if len(batches) != 1 || batches[""] {
		t.Errorf("audit rows carry batch ids %v, want one id for the call", batches)
	}
	if got := r.principal.forgotten(); !slices.Equal(got, []string{"one", "two"}) {
		t.Errorf("forgot %v, want the two accounts that were disabled", got)
	}
}

func TestTwoBulkCallsHaveTwoBatchIds(t *testing.T) {
	r := newWriteRig("one", "two")
	for _, id := range []string{"one", "two"} {
		if _, err := r.app.Commands.DisableUsers.Handle(context.Background(), command.DisableUsers{Request: theAdmin, IDs: []string{id}}); err != nil {
			t.Fatal(err)
		}
	}
	if len(r.shelf.reqs) != 2 || r.shelf.reqs[0].BatchID == r.shelf.reqs[1].BatchID {
		t.Errorf("batch ids = %v, want one per call", []string{r.shelf.reqs[0].BatchID, r.shelf.reqs[1].BatchID})
	}
}

func TestABulkResetKeepsEveryPasswordItMadeAndSaysWhoItCouldNotReset(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	r := newWriteRig(ids...)
	r.shelf.refuse["c"] = errors.New("pq: connection lost")
	r.shelf.panics["f"] = true
	r.shelf.users["g"] = domain.AdminUser{ID: "g", Status: domain.AccountDisabled}

	result, err := r.app.Commands.ResetUsersPasswords.Handle(context.Background(), command.ResetUsersPasswords{
		Request: theAdmin, IDs: append(slices.Clone(ids), "ghost", "admin"), Delivery: domain.DeliveryTemporaryPassword,
	})
	if err != nil {
		t.Fatalf("a bulk reset that reset accounts answered %v: the passwords would be lost", err)
	}

	got := make([]string, len(result.Reset))
	for i, reset := range result.Reset {
		got[i] = reset.UserID
		if reset.TemporaryPassword == "" || reset.Email != reset.UserID+"@example.test" || reset.FullName != "Tên "+reset.UserID {
			t.Errorf("reset %+v, want a password, the email and the name", reset)
		}
	}
	if !slices.Equal(got, []string{"a", "b", "d", "e", "h"}) {
		t.Errorf("reset %v, want the five that could be, in the order named", got)
	}
	wantFailed := map[string]error{"c": domain.ErrActionFailed, "f": domain.ErrActionFailed, "g": domain.ErrAccountDisabled, "ghost": domain.ErrUserNotFound, "admin": domain.ErrCannotActOnSelf}
	if len(result.Failed) != len(wantFailed) {
		t.Fatalf("failed %+v", result.Failed)
	}
	for _, failure := range result.Failed {
		if want, ok := wantFailed[failure.UserID]; !ok || !errors.Is(failure.Reason, want) {
			t.Errorf("%s failed with %v, want %v", failure.UserID, failure.Reason, want)
		}
	}
	if peak := r.passwords.peak.Load(); peak > 2 {
		t.Errorf("%d passwords were being hashed at once, want at most two", peak)
	}
}

func TestABulkResetNamesNoMoreAccountsThanItsDeliveryAllows(t *testing.T) {
	many := func(n int) []string {
		ids := make([]string, n)
		for i := range ids {
			ids[i] = fmt.Sprintf("u%03d", i)
		}
		return ids
	}
	for name, c := range map[string]struct {
		delivery domain.Delivery
		count    int
		refused  bool
	}{
		"forty temporary passwords":          {domain.DeliveryTemporaryPassword, 40, false},
		"forty-one temporary passwords":      {domain.DeliveryTemporaryPassword, 41, true},
		"a hundred links":                    {domain.DeliveryEmailLink, 100, false},
		"a hundred and one links":            {domain.DeliveryEmailLink, 101, true},
		"forty-one links are still allowed":  {domain.DeliveryEmailLink, 41, false},
		"one hundred temporary passwords no": {domain.DeliveryTemporaryPassword, 100, true},
	} {
		t.Run(name, func(t *testing.T) {
			r := newWriteRig(many(c.count)...)
			r.app.SetPasswordLinks(&linkStore{})

			result, err := r.app.Commands.ResetUsersPasswords.Handle(context.Background(), command.ResetUsersPasswords{Request: theAdmin, IDs: many(c.count), Delivery: c.delivery})

			var invalid *validation.Error
			if c.refused {
				if !errors.As(err, &invalid) || len(invalid.Fields) != 1 || invalid.Fields[0].Field != "userIds" {
					t.Fatalf("answered %v, want a validation error naming userIds", err)
				}
				if field := invalid.Fields[0]; field.English == "" || field.English == field.Message {
					t.Errorf("field = %+v, want a Vietnamese and an English sentence", field)
				}
				if r.passwords.calls.Load() != 0 || len(r.shelf.written()) != 0 {
					t.Errorf("a refused batch made %d passwords and wrote %d", r.passwords.calls.Load(), len(r.shelf.written()))
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Reset)+len(result.Failed) != c.count {
				t.Errorf("%d reset and %d failed for %d named", len(result.Reset), len(result.Failed), c.count)
			}
		})
	}
}

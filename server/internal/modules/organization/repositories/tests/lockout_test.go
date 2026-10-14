//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"quizzivy/internal/modules/organization/domain"
)

func (s *session) policyDocument(policy domain.SignIn) domain.Settings {
	doc := s.fullDocument()
	doc.SignIn = policy
	return doc
}

func (s *session) mustRefuse(in domain.Save) *domain.LockedOut {
	s.t.Helper()
	before := s.raw()

	_, err := s.repo.Save(context.Background(), in)

	if !errors.Is(err, domain.ErrLockout) {
		s.t.Fatalf("save = %v, want it refused as a lockout", err)
	}
	var locked *domain.LockedOut
	if !errors.As(err, &locked) {
		s.t.Fatalf("error %v does not carry the impact", err)
	}
	if !locked.Impact.Lockout() {
		s.t.Errorf("refused with an impact that is not a lockout: %+v", locked.Impact)
	}
	if after := s.raw(); !after.same(before) {
		s.t.Errorf("a refused save changed the row: %+v, want %+v", after, before)
	}
	if audits := s.audits(); len(audits) != 0 {
		s.t.Errorf("a refused save wrote %d audit rows", len(audits))
	}
	return locked
}

func (s *session) mustSave(in domain.Save) domain.Settings {
	s.t.Helper()
	got, err := s.repo.Save(context.Background(), in)
	if err != nil {
		s.t.Fatalf("save: %v", err)
	}
	return got
}

func TestSaveRefusesAPolicyThatLocksTheActorOutEvenWhenAnotherAdminKeepsAWay(t *testing.T) {
	s := lockedSession(t)
	d := s.domainIn()
	actor := s.addAccount(account{role: "admin", password: true})
	s.addAccount(account{role: "admin", google: "giam.doc@" + d})
	policy := domain.SignIn{Google: true, AllowedGoogleDomains: []string{d}}

	locked := s.mustRefuse(s.save(s.policyDocument(policy), s.stored().Revision, actor))

	if !locked.Impact.ActorLocked {
		t.Errorf("impact = %+v, want the actor locked out", locked.Impact)
	}
	if locked.Impact.AffectedAdmins >= locked.Impact.ActiveAdmins {
		t.Errorf("impact = %+v, want an Admin who keeps a way, so that only the actor rule refuses", locked.Impact)
	}
	if locked.Impact.AffectedUsers < 1 {
		t.Errorf("impact = %+v, want the actor counted among the affected", locked.Impact)
	}
}

func TestSaveRefusesAPolicyThatLeavesEveryAdminWithoutAWayWhoeverAsks(t *testing.T) {
	s := lockedSession(t)
	d := s.domainIn()
	actor := s.addAccount(account{role: "teacher", google: "giao.vien@" + d})
	s.addAccount(account{role: "admin", password: true})
	s.addAccount(account{role: "admin"})
	s.addAccount(account{role: "admin", password: true, google: "da.nghi@" + d, disabled: true})
	policy := domain.SignIn{Google: true, AllowedGoogleDomains: []string{d}}

	locked := s.mustRefuse(s.save(s.policyDocument(policy), s.stored().Revision, actor))

	if locked.Impact.ActorLocked {
		t.Errorf("impact = %+v, want the actor to keep their way", locked.Impact)
	}
	if locked.Impact.ActiveAdmins < 2 || locked.Impact.AffectedAdmins != locked.Impact.ActiveAdmins {
		t.Errorf("impact = %+v, want every active Admin affected, the disabled one not rescuing them", locked.Impact)
	}
}

func TestSaveRefusesAPolicyThatLeavesEveryAdminWithoutAWayWhenNobodyAsks(t *testing.T) {
	s := lockedSession(t)
	d := s.domainIn()
	s.addAccount(account{role: "admin", password: true})
	policy := domain.SignIn{Google: true, AllowedGoogleDomains: []string{d}}

	locked := s.mustRefuse(s.save(s.policyDocument(policy), s.stored().Revision, ""))

	if locked.Impact.ActorLocked || locked.Impact.AffectedAdmins != locked.Impact.ActiveAdmins {
		t.Errorf("impact = %+v, want no locked actor and every Admin affected", locked.Impact)
	}
}

func TestSaveAllowsAPolicyThatKeepsTheActorAWayWhileOtherAdminsLoseTheirs(t *testing.T) {
	s := lockedSession(t)
	d := s.domainIn()
	actor := s.addAccount(account{role: "admin", google: "hieu.truong@" + d})
	s.addAccount(account{role: "admin", password: true})
	s.addAccount(account{role: "admin"})
	policy := domain.SignIn{Google: true, ForcePasswordChange: true, JoinCodes: true, AllowedGoogleDomains: []string{d}}
	before := s.stored()

	got := s.mustSave(s.save(s.policyDocument(policy), before.Revision, actor))

	if !reflect.DeepEqual(got.SignIn, policy) {
		t.Errorf("stored sign-in = %+v, want %+v", got.SignIn, policy)
	}
	if got.Revision != before.Revision+1 {
		t.Errorf("revision = %d, want %d", got.Revision, before.Revision+1)
	}
	if audits := s.audits(); len(audits) != 1 {
		t.Errorf("audit rows = %d, want 1", len(audits))
	}
	impact, err := s.repo.Impact(context.Background(), policy, actor, true)
	if err != nil {
		t.Fatal(err)
	}
	if impact.ActorLocked || impact.AffectedAdmins < 2 {
		t.Errorf("impact of the saved policy = %+v, want the actor spared and the two other Admins affected", impact)
	}
}

func TestSaveAllowsAPasswordPolicyForAnAdminWithAPassword(t *testing.T) {
	s := lockedSession(t)
	actor := s.addAccount(account{role: "admin", password: true})
	s.addAccount(account{role: "admin", google: "chi.google@" + s.domainIn()})
	policy := domain.SignIn{Password: true, ForcePasswordChange: true, JoinCodes: true, AllowedGoogleDomains: []string{}}

	got := s.mustSave(s.save(s.policyDocument(policy), s.stored().Revision, actor))

	if !reflect.DeepEqual(got.SignIn, policy) {
		t.Errorf("stored sign-in = %+v, want %+v", got.SignIn, policy)
	}
}

func TestSaveAllowsAPolicyWithNoActorWhenAnAdminKeepsAWay(t *testing.T) {
	s := lockedSession(t)
	s.addAccount(account{role: "admin", password: true})
	policy := domain.SignIn{Password: true, AllowedGoogleDomains: []string{}}

	got := s.mustSave(s.save(s.policyDocument(policy), s.stored().Revision, ""))

	if got.UpdatedBy != "" || !reflect.DeepEqual(got.SignIn, policy) {
		t.Errorf("stored = %+v, want the policy with no author", got)
	}
}

func TestSaveCountsGoogleOnlyWhenTheServerCanSignInWithIt(t *testing.T) {
	cases := []struct {
		name       string
		policy     func(d string) domain.SignIn
		everyAdmin bool
	}{
		{
			"a Google policy and an admin with only Google",
			func(d string) domain.SignIn { return domain.SignIn{Google: true, AllowedGoogleDomains: []string{d}} },
			true,
		},
		{
			"both methods and an admin with only Google",
			func(d string) domain.SignIn {
				return domain.SignIn{Google: true, Password: true, AllowedGoogleDomains: []string{d}}
			},
			false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := lockedSession(t)
			d := s.domainIn()
			actor := s.addAccount(account{role: "admin", google: "quan.tri@" + d})
			doc := s.policyDocument(c.policy(d))

			notConfigured := s.save(doc, s.stored().Revision, actor)
			notConfigured.GoogleConfigured = false
			locked := s.mustRefuse(notConfigured)
			if !locked.Impact.ActorLocked {
				t.Errorf("impact without Google = %+v, want the actor locked out", locked.Impact)
			}
			if c.everyAdmin && locked.Impact.AffectedAdmins != locked.Impact.ActiveAdmins {
				t.Errorf("impact without Google = %+v, want every Admin affected", locked.Impact)
			}

			configured := s.save(doc, s.stored().Revision, actor)
			got := s.mustSave(configured)
			if !reflect.DeepEqual(got.SignIn, doc.SignIn) {
				t.Errorf("stored sign-in = %+v, want %+v", got.SignIn, doc.SignIn)
			}
		})
	}
}

func TestSaveKeepsPasswordsCountingWhenTheServerCannotSignInWithGoogle(t *testing.T) {
	s := lockedSession(t)
	d := s.domainIn()
	actor := s.addAccount(account{role: "admin", password: true, google: "quan.tri@" + d})
	policy := domain.SignIn{Google: true, Password: true, AllowedGoogleDomains: []string{d}}
	in := s.save(s.policyDocument(policy), s.stored().Revision, actor)
	in.GoogleConfigured = false

	got := s.mustSave(in)

	if !reflect.DeepEqual(got.SignIn, policy) {
		t.Errorf("stored sign-in = %+v, want %+v", got.SignIn, policy)
	}
}

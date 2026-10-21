//go:build integration

package repositories_test

import (
	"context"
	"strings"
	"testing"

	"quizzivy/internal/modules/organization/domain"
)

func (s *session) domainIn() string  { return "dai-hoc-" + s.nonce + ".example" }
func (s *session) domainOut() string { return "noi-khac-" + s.nonce + ".example" }

type census struct {
	adminPassword      string
	adminGoogleIn      string
	adminGoogleOut     string
	adminBoth          string
	adminNone          string
	adminDisabledBoth  string
	adminDisabledNone  string
	teacherPassword    string
	assistantNone      string
	customGoogleIn     string
	studentGoogleUpper string
	studentSubdomain   string
	studentNone        string
	studentBoth        string
	studentDisabled    string
}

func (s *session) addCensus() census {
	s.t.Helper()
	in, out := s.domainIn(), s.domainOut()
	return census{
		adminPassword:      s.addAccount(account{role: "admin", password: true}),
		adminGoogleIn:      s.addAccount(account{role: "admin", google: "gv@" + in}),
		adminGoogleOut:     s.addAccount(account{role: "admin", google: "gv@" + out}),
		adminBoth:          s.addAccount(account{role: "admin", password: true, google: "gv.hai@" + out}),
		adminNone:          s.addAccount(account{role: "admin"}),
		adminDisabledBoth:  s.addAccount(account{role: "admin", password: true, google: "cu@" + in, disabled: true}),
		adminDisabledNone:  s.addAccount(account{role: "admin", disabled: true}),
		teacherPassword:    s.addAccount(account{role: "teacher", password: true}),
		assistantNone:      s.addAccount(account{role: "assistant"}),
		customGoogleIn:     s.addAccount(account{role: "custom", google: "tro.giang@" + in}),
		studentGoogleUpper: s.addAccount(account{role: "student", google: "Em.Hoc@" + strings.ToUpper(in)}),
		studentSubdomain:   s.addAccount(account{role: "student", google: "em@khoa." + in}),
		studentNone:        s.addAccount(account{role: "student"}),
		studentBoth:        s.addAccount(account{role: "student", password: true, google: "em.hai@" + out}),
		studentDisabled:    s.addAccount(account{role: "student", disabled: true}),
	}
}

const (
	activeCensusAdmins = 5
	unknownUser        = "00000000-0000-7000-8000-0000000000bb"
)

func TestImpactCountsTheActiveAccountsLeftWithoutAWay(t *testing.T) {
	s := snapshotSession(t)
	in, out := s.domainIn(), s.domainOut()
	cases := []struct {
		name       string
		policy     domain.SignIn
		configured bool
		users      int
		admins     int
	}{
		{"password only", domain.SignIn{Password: true}, true, 8, 3},
		{"password only, whatever the domain list holds", domain.SignIn{Password: true, AllowedGoogleDomains: []string{in}}, true, 8, 3},
		{"password only with Google switched on but not configured", domain.SignIn{Password: true, Google: true}, false, 8, 3},
		{"Google only, any domain", domain.SignIn{Google: true}, true, 5, 2},
		{"Google only, one domain", domain.SignIn{Google: true, AllowedGoogleDomains: []string{in}}, true, 9, 4},
		{"Google only, two domains", domain.SignIn{Google: true, AllowedGoogleDomains: []string{in, out}}, true, 6, 2},
		{"both, any domain", domain.SignIn{Google: true, Password: true}, true, 3, 1},
		{"both, one domain", domain.SignIn{Google: true, Password: true, AllowedGoogleDomains: []string{in}}, true, 5, 2},
		{"both, two domains", domain.SignIn{Google: true, Password: true, AllowedGoogleDomains: []string{in, out}}, true, 4, 1},
		{"Google only but not configured", domain.SignIn{Google: true}, false, 12, activeCensusAdmins},
		{"Google only with a domain list but not configured", domain.SignIn{Google: true, AllowedGoogleDomains: []string{in}}, false, 12, activeCensusAdmins},
	}
	before := make([]domain.Impact, len(cases))
	for i, c := range cases {
		got, err := s.repo.Impact(context.Background(), c.policy, "", c.configured)
		if err != nil {
			t.Fatalf("baseline of %q: %v", c.name, err)
		}
		before[i] = got
	}
	s.addCensus()

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			after, err := s.repo.Impact(context.Background(), c.policy, "", c.configured)
			if err != nil {
				t.Fatal(err)
			}
			got := domain.Impact{
				AffectedUsers:  after.AffectedUsers - before[i].AffectedUsers,
				AffectedAdmins: after.AffectedAdmins - before[i].AffectedAdmins,
				ActiveAdmins:   after.ActiveAdmins - before[i].ActiveAdmins,
			}
			want := domain.Impact{AffectedUsers: c.users, AffectedAdmins: c.admins, ActiveAdmins: activeCensusAdmins}
			if got != want {
				t.Errorf("accounts the policy adds to the baseline = %+v, want %+v", got, want)
			}
			if after.ActorLocked {
				t.Error("an impact asked for no actor says the actor is locked out")
			}
		})
	}
}

func TestImpactSaysWhetherTheActorIsLeftWithoutAWay(t *testing.T) {
	s := snapshotSession(t)
	c := s.addCensus()
	in := s.domainIn()
	passwordOnly := domain.SignIn{Password: true}
	googleOnly := domain.SignIn{Google: true}
	googleIn := domain.SignIn{Google: true, AllowedGoogleDomains: []string{in}}
	both := domain.SignIn{Google: true, Password: true}
	cases := []struct {
		name       string
		policy     domain.SignIn
		configured bool
		actor      string
		locked     bool
	}{
		{"password policy, admin with only Google", passwordOnly, true, c.adminGoogleIn, true},
		{"password policy, admin with a password", passwordOnly, true, c.adminPassword, false},
		{"password policy, admin with both", passwordOnly, true, c.adminBoth, false},
		{"Google policy, admin with only a password", googleOnly, true, c.adminPassword, true},
		{"Google policy, admin with Google elsewhere", googleOnly, true, c.adminGoogleOut, false},
		{"Google policy not configured, admin with Google", googleOnly, false, c.adminGoogleIn, true},
		{"Google policy not configured, admin with a password too", domain.SignIn{Google: true, Password: true}, false, c.adminBoth, false},
		{"domain policy, admin with Google in the domain", googleIn, true, c.adminGoogleIn, false},
		{"domain policy, admin with Google in another domain", googleIn, true, c.adminGoogleOut, true},
		{"domain policy, the domain in upper case on the identity", googleIn, true, c.studentGoogleUpper, false},
		{"domain policy, a sub-domain of the domain", googleIn, true, c.studentSubdomain, true},
		{"domain policy, a custom role", googleIn, true, c.customGoogleIn, false},
		{"both policy, an account with neither", both, true, c.adminNone, true},
		{"both policy, an account with Google only", both, true, c.adminGoogleOut, false},
		{"both policy, an account with a password only", both, true, c.teacherPassword, false},
		{"no way, an assistant", both, true, c.assistantNone, true},
		{"no way, a student", both, true, c.studentNone, true},
		{"a disabled account is never locked out", passwordOnly, true, c.adminDisabledNone, false},
		{"a disabled student is never locked out", passwordOnly, true, c.studentDisabled, false},
		{"no actor", googleIn, true, "", false},
		{"an actor who is not a user", googleIn, true, unknownUser, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.repo.Impact(context.Background(), tc.policy, tc.actor, tc.configured)
			if err != nil {
				t.Fatal(err)
			}
			if got.ActorLocked != tc.locked {
				t.Errorf("actor locked = %v, want %v (impact %+v)", got.ActorLocked, tc.locked, got)
			}
		})
	}
}

func TestImpactNeverCountsADisabledAccount(t *testing.T) {
	s := snapshotSession(t)
	policy := domain.SignIn{Password: true}
	before, err := s.repo.Impact(context.Background(), policy, "", true)
	if err != nil {
		t.Fatal(err)
	}
	s.addAccount(account{role: "admin", disabled: true})
	s.addAccount(account{role: "admin", password: true, disabled: true})
	s.addAccount(account{role: "teacher", disabled: true})
	s.addAccount(account{role: "student", disabled: true, google: "ai-do@" + s.domainIn()})

	after, err := s.repo.Impact(context.Background(), policy, "", true)
	if err != nil {
		t.Fatal(err)
	}

	if after != before {
		t.Errorf("impact with four disabled accounts added = %+v, want the baseline %+v", after, before)
	}
}

func TestImpactCountsAnAdminByTheBuiltInRoleOnly(t *testing.T) {
	s := snapshotSession(t)
	policy := domain.SignIn{Password: true}
	before, err := s.repo.Impact(context.Background(), policy, "", true)
	if err != nil {
		t.Fatal(err)
	}
	s.addAccount(account{role: "teacher"})
	s.addAccount(account{role: "assistant"})
	s.addAccount(account{role: "custom"})
	s.addAccount(account{role: "student"})

	after, err := s.repo.Impact(context.Background(), policy, "", true)
	if err != nil {
		t.Fatal(err)
	}

	want := domain.Impact{
		AffectedUsers:  before.AffectedUsers + 4,
		AffectedAdmins: before.AffectedAdmins,
		ActiveAdmins:   before.ActiveAdmins,
	}
	if after != want {
		t.Errorf("impact with four non-Admin accounts added = %+v, want %+v", after, want)
	}
}

func TestImpactTreatsAnAbsentDomainListAsEveryDomain(t *testing.T) {
	s := snapshotSession(t)
	student := s.addAccount(account{role: "student", google: "em@" + s.domainOut()})
	googleOnly := func(domains []string) domain.Impact {
		got, err := s.repo.Impact(context.Background(), domain.SignIn{Google: true, AllowedGoogleDomains: domains}, student, true)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	if got := googleOnly(nil); got.ActorLocked {
		t.Error("a nil domain list refuses a Google account")
	}
	if got := googleOnly([]string{}); got.ActorLocked {
		t.Error("an empty domain list refuses a Google account")
	}
	if got := googleOnly([]string{s.domainIn()}); !got.ActorLocked {
		t.Error("a domain list that omits the account's domain leaves it a way")
	}
	if got := googleOnly([]string{s.domainIn(), s.domainOut()}); got.ActorLocked {
		t.Error("a domain list that includes the account's domain locks it out")
	}
}

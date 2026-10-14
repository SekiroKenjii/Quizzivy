package tests

import (
	"errors"
	"strings"
	"testing"

	"quizzivy/internal/modules/organization/domain"
	"quizzivy/internal/shared/validation"
)

func valid() domain.Settings {
	return domain.Settings{
		Profile:   domain.Profile{Name: "Quizzivy English Centre"},
		SignIn:    domain.SignIn{Google: true, Password: true, ForcePasswordChange: true, JoinCodes: true},
		Retention: domain.Retention{KeepFocusEventsMonths: 13},
	}
}

func refusedFields(t *testing.T, s domain.Settings) map[string]string {
	t.Helper()
	_, err := s.Normalised()
	var invalid *validation.Error
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %v, want a validation error", err)
	}
	out := map[string]string{}
	for _, f := range invalid.Fields {
		out[f.Field] = f.Message
	}
	return out
}

func TestAValidDocumentIsStoredTrimmedAndLowerCased(t *testing.T) {
	s := valid()
	s.Profile.Name = "  Trung tâm Quizzivy  "
	s.Profile.Website = " https://quizzivy.com/lien-he "
	s.Profile.ContactEmail = " Hello@Quizzivy.com "
	s.Profile.FrontDeskPhone = " +84 (28) 3822-1234 "
	s.SignIn.AllowedGoogleDomains = []string{" Quizzivy.COM ", "school.edu.vn"}
	got, err := s.Normalised()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile.Name != "Trung tâm Quizzivy" || got.Profile.Website != "https://quizzivy.com/lien-he" ||
		got.Profile.ContactEmail != "Hello@Quizzivy.com" || got.Profile.FrontDeskPhone != "+84 (28) 3822-1234" {
		t.Fatalf("profile = %+v", got.Profile)
	}
	if strings.Join(got.SignIn.AllowedGoogleDomains, ",") != "quizzivy.com,school.edu.vn" {
		t.Fatalf("domains = %v", got.SignIn.AllowedGoogleDomains)
	}
}

func TestEachBadFieldIsNamedByItsContractPath(t *testing.T) {
	cases := []struct {
		name   string
		change func(*domain.Settings)
		field  string
		reason string
	}{
		{"blank name", func(s *domain.Settings) { s.Profile.Name = "   " }, "organization.name", domain.ReasonRequired},
		{"name of 121", func(s *domain.Settings) { s.Profile.Name = strings.Repeat("a", 121) }, "organization.name", domain.ReasonTooLong},
		{"short name of 41", func(s *domain.Settings) { s.Profile.ShortName = strings.Repeat("a", 41) }, "organization.shortName", domain.ReasonTooLong},
		{"website with a space", func(s *domain.Settings) { s.Profile.Website = "quizzivy .com" }, "organization.website", domain.ReasonWebsite},
		{"website with another scheme", func(s *domain.Settings) { s.Profile.Website = "javascript:alert(1)" }, "organization.website", domain.ReasonWebsite},
		{"website of ftp", func(s *domain.Settings) { s.Profile.Website = "ftp://quizzivy.com" }, "organization.website", domain.ReasonWebsite},
		{"website of 201", func(s *domain.Settings) { s.Profile.Website = strings.Repeat("a", 201) }, "organization.website", domain.ReasonTooLong},
		{"email without a domain dot", func(s *domain.Settings) { s.Profile.ContactEmail = "a@b" }, "organization.contactEmail", domain.ReasonEmail},
		{"email with two ats", func(s *domain.Settings) { s.Profile.ContactEmail = "a@b@c.com" }, "organization.contactEmail", domain.ReasonEmail},
		{"phone with letters", func(s *domain.Settings) { s.Profile.FrontDeskPhone = "call me" }, "organization.frontDeskPhone", domain.ReasonPhone},
		{"phone too short", func(s *domain.Settings) { s.Profile.FrontDeskPhone = "12345" }, "organization.frontDeskPhone", domain.ReasonPhone},
		{"hours of 121", func(s *domain.Settings) { s.Profile.FrontDeskHours = strings.Repeat("a", 121) }, "organization.frontDeskHours", domain.ReasonTooLong},
		{"address of 301", func(s *domain.Settings) { s.Profile.Address = strings.Repeat("a", 301) }, "organization.address", domain.ReasonTooLong},
		{"logo hash of 31 bytes", func(s *domain.Settings) {
			s.Profile.LightLogo = &domain.Logo{SHA256: make([]byte, 31), Type: domain.LogoPNG}
		}, "organization.logo.light", domain.ReasonLogo},
		{"logo of another type", func(s *domain.Settings) {
			s.Profile.DarkLogo = &domain.Logo{SHA256: make([]byte, 32), Type: "image/gif"}
		}, "organization.logo.dark", domain.ReasonLogo},
		{"no way to sign in", func(s *domain.Settings) { s.SignIn.Google, s.SignIn.Password = false, false }, "signIn", domain.ReasonNoMethod},
		{"a malformed domain", func(s *domain.Settings) { s.SignIn.AllowedGoogleDomains = []string{"not a domain"} }, "signIn.allowedGoogleDomains", domain.ReasonDomain},
		{"a domain without a dot", func(s *domain.Settings) { s.SignIn.AllowedGoogleDomains = []string{"localhost"} }, "signIn.allowedGoogleDomains", domain.ReasonDomain},
		{"a repeated domain", func(s *domain.Settings) {
			s.SignIn.AllowedGoogleDomains = []string{"a.com", "A.com"}
		}, "signIn.allowedGoogleDomains", domain.ReasonRepeated},
		{"fifty-one domains", func(s *domain.Settings) {
			for i := range 51 {
				s.SignIn.AllowedGoogleDomains = append(s.SignIn.AllowedGoogleDomains, strings.Repeat("a", i+1)+".com")
			}
		}, "signIn.allowedGoogleDomains", domain.ReasonTooMany},
		{"answers kept six months", func(s *domain.Settings) { six := 6; s.Retention.KeepAnswersMonths = &six }, "retention.keepAnswersMonths", domain.ReasonMonths},
		{"focus events kept seven months", func(s *domain.Settings) { s.Retention.KeepFocusEventsMonths = 7 }, "retention.keepFocusEventsMonths", domain.ReasonMonths},
		{"anonymising after 13 months", func(s *domain.Settings) { m := 13; s.Retention.AnonymiseDisabledAfterMonths = &m }, "retention.anonymiseDisabledAfterMonths", domain.ReasonMonths},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := valid()
			c.change(&s)
			if got := refusedFields(t, s)[c.field]; got != c.reason {
				t.Fatalf("%s = %q, want %q", c.field, got, c.reason)
			}
		})
	}
}

func TestEveryRefusedFieldIsNamedAtOnce(t *testing.T) {
	s := valid()
	s.Profile.Name = ""
	s.Profile.ContactEmail = "nope"
	s.SignIn.Google, s.SignIn.Password = false, false
	fields := refusedFields(t, s)
	for _, want := range []string{"organization.name", "organization.contactEmail", "signIn"} {
		if _, ok := fields[want]; !ok {
			t.Errorf("%s is not named in %v", want, fields)
		}
	}
}

func TestTheAcceptedBoundsAreAccepted(t *testing.T) {
	s := valid()
	s.Profile.Name = strings.Repeat("a", 120)
	s.Profile.ShortName = strings.Repeat("a", 40)
	s.Profile.Address = strings.Repeat("a", 300)
	s.Profile.FrontDeskHours = strings.Repeat("a", 120)
	s.Profile.Website = "quizzivy.com"
	s.Profile.FrontDeskPhone = "028 3822 1234"
	twelve, twentyFour := 12, 24
	s.Retention = domain.Retention{KeepAnswersMonths: &twelve, KeepFocusEventsMonths: 6, AnonymiseDisabledAfterMonths: &twentyFour}
	s.SignIn.Google, s.SignIn.Password = false, true
	s.Profile.LightLogo = &domain.Logo{SHA256: make([]byte, 32), Type: domain.LogoSVG}
	if _, err := s.Normalised(); err != nil {
		t.Fatal(err)
	}
}

func TestChangedFieldsNamesOnlyWhatDiffers(t *testing.T) {
	a := valid()
	if got := a.ChangedFields(a); len(got) != 0 {
		t.Fatalf("an unchanged document changed %v", got)
	}
	b := valid()
	b.Profile.Name = "Other"
	b.Profile.LightLogo = &domain.Logo{SHA256: make([]byte, 32), Type: domain.LogoPNG}
	b.SignIn.AllowedGoogleDomains = []string{"a.com"}
	b.SignIn.JoinCodes = false
	nine := 12
	b.Retention.KeepAnswersMonths = &nine
	want := "organization.name,organization.logo.light,signIn.joinCodes,signIn.allowedGoogleDomains,retention.keepAnswersMonths"
	if got := strings.Join(a.ChangedFields(b), ","); got != want {
		t.Fatalf("changed = %s, want %s", got, want)
	}
}

func TestALogoIsEqualToItselfOnly(t *testing.T) {
	digest := make([]byte, 32)
	a := &domain.Logo{SHA256: digest, Type: domain.LogoPNG}
	other := make([]byte, 32)
	other[0] = 1
	if !a.Equal(&domain.Logo{SHA256: append([]byte{}, digest...), Type: domain.LogoPNG}) {
		t.Error("the same bytes and type differ")
	}
	if a.Equal(&domain.Logo{SHA256: other, Type: domain.LogoPNG}) || a.Equal(&domain.Logo{SHA256: digest, Type: domain.LogoSVG}) || a.Equal(nil) {
		t.Error("a different logo is equal")
	}
	var none *domain.Logo
	if !none.Equal(nil) {
		t.Error("no logo differs from no logo")
	}
}

func TestALockoutIsTheActorOrEveryAdmin(t *testing.T) {
	cases := []struct {
		name   string
		impact domain.Impact
		want   bool
	}{
		{"nobody affected", domain.Impact{ActiveAdmins: 2}, false},
		{"one of two admins affected", domain.Impact{AffectedUsers: 1, AffectedAdmins: 1, ActiveAdmins: 2}, false},
		{"every admin affected", domain.Impact{AffectedUsers: 2, AffectedAdmins: 2, ActiveAdmins: 2}, true},
		{"the actor is affected", domain.Impact{AffectedUsers: 1, ActorLocked: true, ActiveAdmins: 2}, true},
		{"students affected only", domain.Impact{AffectedUsers: 40, ActiveAdmins: 1}, false},
	}
	for _, c := range cases {
		if got := c.impact.Lockout(); got != c.want {
			t.Errorf("%s: Lockout = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLockedOutSatisfiesErrLockout(t *testing.T) {
	var err error = &domain.LockedOut{Impact: domain.Impact{AffectedUsers: 3}}
	if !errors.Is(err, domain.ErrLockout) {
		t.Fatal("LockedOut is not ErrLockout")
	}
}

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
	s.Profile.LightLogo = &domain.Logo{SHA256: make([]byte, 32), Type: domain.LogoPNG}
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
	if a.Equal(&domain.Logo{SHA256: other, Type: domain.LogoPNG}) || a.Equal(&domain.Logo{SHA256: digest, Type: "image/svg+xml"}) || a.Equal(nil) {
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

func TestAWebsiteIsAHostOrAnHTTPSAddressAndNothingElse(t *testing.T) {
	refused := []string{
		"javascript:1", "JavaScript:0%0aalert(1)", "vbscript:1", "javascript:alert(1)", "data:text/html,x",
		"//x.y", "https:///x", "https://user:pass@evil.com", "https://user@evil.com", "x", "localhost",
		"http://quizzivy.com", "HTTP://quizzivy.com", "ftp://quizzivy.com", "https://quizzivy.com:abc", "https:quizzivy.com",
		"https://quiz zivy.com", "https://-bad.com", "https://a..b.com", "quizzivy.com\\evil",
	}
	for _, site := range refused {
		s := valid()
		s.Profile.Website = site
		if got := refusedFields(t, s)["organization.website"]; got != domain.ReasonWebsite {
			t.Errorf("%q: website = %q, want %q", site, got, domain.ReasonWebsite)
		}
	}
	accepted := []string{
		"quizzivy.com", "www.quizzivy.com", "quizzivy.com/lien-he", "https://quizzivy.com", "HTTPS://Quizzivy.com/a?b=c#d",
		"https://quizzivy.com:8443/x", "quizzivy.com:8443", "trungtam.edu.vn", "192.168.1.1",
	}
	for _, site := range accepted {
		s := valid()
		s.Profile.Website = site
		if _, err := s.Normalised(); err != nil {
			t.Errorf("%q refused: %v", site, err)
		}
	}
}

func TestAContactEmailIsAnAddressWithADomain(t *testing.T) {
	refused := []string{
		"a@.c", "a@b.", "a@b..c", "<a@b.c>", "a@[1.2.3.4]", "javascript:a@b.c", "a@b", "@b.com", "a@@b.com",
		".a@b.com", "a.@b.com", "a..b@b.com", "a b@b.com", "a@b c.com", "a@-b.com", strings.Repeat("a", 65) + "@b.com",
		"a,b@b.com", "a(b)@b.com", "\"a\"@b.com", "tên@b.com",
	}
	for _, email := range refused {
		s := valid()
		s.Profile.ContactEmail = email
		if got := refusedFields(t, s)["organization.contactEmail"]; got != domain.ReasonEmail {
			t.Errorf("%q: contactEmail = %q, want %q", email, got, domain.ReasonEmail)
		}
	}
	accepted := []string{"a@b.c", "hello@quizzivy.com", "first.last+tag@mail.school.edu.vn", "o'brien@quizzivy.com", strings.Repeat("a", 64) + "@b.com"}
	for _, email := range accepted {
		s := valid()
		s.Profile.ContactEmail = email
		if _, err := s.Normalised(); err != nil {
			t.Errorf("%q refused: %v", email, err)
		}
	}
}

func TestANameHoldsNoLineBreakControlOrInvisibleCharacter(t *testing.T) {
	cases := map[string]string{
		"a zero width space":   "\u200b",
		"only a zero width":    "\u200b\u200d",
		"a line break":         "a\nb",
		"a carriage return":    "a\rb",
		"a tab":                "a\tb",
		"a NUL":                "a\x00b",
		"a zero width joiner":  "Quiz\u200dzivy",
		"a BOM":                "\xef\xbb\xbfQuizzivy",
		"a right-to-left mark": "Quizzivy\u200f ",
		"a line separator":     "a\u2028b",
		"a Hangul filler":      "\u3164",
		"a Braille blank":      "\u2800",
		"a Hangul choseong":    "\u115f\u1160",
		"a halfwidth filler":   "\uffa0",
		"a filler inside text": "Quiz\u3164zivy",
		"a lone accent":        "\u0301",
		"a variation selector": "\ufe0f",
	}
	for name, text := range cases {
		s := valid()
		s.Profile.Name = text
		_, err := s.Normalised()
		if err == nil {
			t.Errorf("name with %s was accepted", name)
			continue
		}
		fields := refusedFields(t, s)
		if fields["organization.name"] != domain.ReasonCharacters && fields["organization.name"] != domain.ReasonRequired {
			t.Errorf("name with %s: reason = %q", name, fields["organization.name"])
		}
		s = valid()
		s.Profile.ShortName = text
		if got := refusedFields(t, s)["organization.shortName"]; got != domain.ReasonCharacters {
			t.Errorf("short name with %s: reason = %q, want %q", name, got, domain.ReasonCharacters)
		}
	}
}

func TestAVietnameseNameWithMarksAndInnerSpacesIsAccepted(t *testing.T) {
	for _, name := range []string{"Trung tâm Anh ngữ Việt Úc", "Quizzivy  Centre", "Trung tâm Anh ngữ (cơ sở 2)"} {
		s := valid()
		s.Profile.Name = name
		s.Profile.ShortName = "Trung tâm"
		if _, err := s.Normalised(); err != nil {
			t.Errorf("%q refused: %v", name, err)
		}
	}
}

func TestANameStaysAsOneValueWhicheverFormTheClientSends(t *testing.T) {
	composed, decomposed := "Trung t\u00e2m Quizzivy", "Trung ta\u0302m Quizzivy"
	stored := valid()
	stored.Profile.Name, stored.Profile.ShortName = composed, "T\u00e2m"
	next := valid()
	next.Profile.Name, next.Profile.ShortName = decomposed, "Ta\u0302m"
	got, err := next.Normalised()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile.Name != composed || got.Profile.ShortName != "T\u00e2m" {
		t.Fatalf("name = %q, short name = %q, want the composed forms", got.Profile.Name, got.Profile.ShortName)
	}
	if changed := stored.ChangedFields(got); len(changed) != 0 {
		t.Fatalf("changed = %v, want none for the same name in another form", changed)
	}
}

func TestANameIsCountedAfterItIsComposed(t *testing.T) {
	s := valid()
	s.Profile.Name = strings.Repeat("a\u0302", domain.MaxName)
	got, err := s.Normalised()
	if err != nil {
		t.Fatalf("%d composed characters refused: %v", domain.MaxName, err)
	}
	if want := strings.Repeat("\u00e2", domain.MaxName); got.Profile.Name != want {
		t.Fatalf("name = %q, want %q", got.Profile.Name, want)
	}
}

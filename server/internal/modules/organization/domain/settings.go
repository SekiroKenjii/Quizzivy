// Package domain is the organization model: the centre's one settings document (who it is and how it is reached, its logos, how people sign in and how long it keeps what it collects), the rules a document must keep to be stored, and what a sign-in policy would do to the accounts that sign in under it.
package domain

import (
	"bytes"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"quizzivy/internal/shared/validation"
)

// LogoType is the media type a logo is stored and served with.
type LogoType string

// LogoPNG is the one type a logo has. The database also admits image/svg+xml,
// a value the application never writes and reads as no logo (D-019).
const LogoPNG LogoType = "image/png"

// LogoExtension is the file extension a logo's bytes are stored and downloaded under.
const LogoExtension = "png"

// LogoGrace is how long a stored logo that no setting names is kept: an upload
// is stored before the save that names it.
const LogoGrace = 24 * time.Hour

// Valid reports whether t is the type a logo may have.
func (t LogoType) Valid() bool { return t == LogoPNG }

// Variant names the background a logo is drawn for.
type Variant string

// The two variants.
const (
	Light Variant = "light"
	Dark  Variant = "dark"
)

// Valid reports whether v is one of the two variants.
func (v Variant) Valid() bool { return v == Light || v == Dark }

// Logo is a stored logo, named by the SHA-256 of its bytes.
type Logo struct {
	SHA256 []byte
	Type   LogoType
}

// Equal reports whether two logos, either of which may be nil, name the same bytes.
func (l *Logo) Equal(other *Logo) bool {
	if l == nil || other == nil {
		return l == nil && other == nil
	}
	return l.Type == other.Type && bytes.Equal(l.SHA256, other.SHA256)
}

// Profile is who the centre is and how it is reached. Every field but Name is
// optional, and an absent one is the empty string.
type Profile struct {
	Name           string
	ShortName      string
	Website        string
	ContactEmail   string
	FrontDeskPhone string
	FrontDeskHours string
	Address        string
	LightLogo      *Logo
	DarkLogo       *Logo
}

// Logo returns the logo of variant v, or nil.
func (p Profile) Logo(v Variant) *Logo {
	if v == Dark {
		return p.DarkLogo
	}
	return p.LightLogo
}

// SignIn is how people sign in. AllowedGoogleDomains, lower-case, bounds the
// Google accounts that may sign in, link or join; empty allows every domain.
type SignIn struct {
	Google               bool
	Password             bool
	ForcePasswordChange  bool
	JoinCodes            bool
	AllowedGoogleDomains []string
}

// Retention is how long the centre keeps what it collects. A nil month count
// means for ever (answers) or never (anonymising disabled accounts).
type Retention struct {
	KeepAnswersMonths            *int
	KeepFocusEventsMonths        int
	AnonymiseDisabledAfterMonths *int
}

// Settings is the whole document as stored: the fields an Admin writes, and
// the revision, time and author of the last save. UpdatedBy is empty before
// the first save and once that account is gone.
type Settings struct {
	Profile   Profile
	SignIn    SignIn
	Retention Retention
	Revision  int64
	UpdatedAt time.Time
	UpdatedBy string
}

// The bounds the stored document keeps, as the database checks them.
const (
	MaxName             = 120
	MaxShortName        = 40
	MaxWebsite          = 200
	MaxContactEmail     = 254
	MaxFrontDeskHours   = 120
	MaxAddress          = 300
	MaxAllowedDomains   = 50
	DefaultFocusMonths  = 13
	logoDigestBytes     = 32
	maxLocalPart        = 64
	maxDomainCharacters = 253
)

// The reasons a field is refused, which the transport words.
const (
	ReasonRequired   = "required"
	ReasonCharacters = "characters"
	ReasonTooLong    = "too_long"
	ReasonWebsite    = "website"
	ReasonEmail      = "email"
	ReasonPhone      = "phone"
	ReasonDomain     = "domain"
	ReasonRepeated   = "repeated"
	ReasonTooMany    = "too_many"
	ReasonNoMethod   = "no_method"
	ReasonMonths     = "months"
	ReasonLogo       = "logo"
	ReasonLogoGone   = "logo_missing"
	ReasonLogoStore  = "logo_store_off"
)

var (
	phonePattern  = regexp.MustCompile(`^[0-9+ ().-]{6,30}$`)
	localPattern  = regexp.MustCompile("^[A-Za-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\\.[A-Za-z0-9!#$%&'*+/=?^_`{|}~-]+)*$")
	domainPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
)

// Normalised returns the document an Admin wrote as it would be stored: every
// text trimmed, the domains lower-cased. It refuses, naming every field at
// once as the contract's paths (organization.name, signIn.allowedGoogleDomains,
// …) with a Reason* message, a document the database would refuse or that
// breaks a rule of its own.
func (s Settings) Normalised() (Settings, error) {
	var refused validation.Error
	out := s
	out.Profile = normaliseProfile(s.Profile, &refused)
	out.SignIn = normaliseSignIn(s.SignIn, &refused)
	checkRetention(s.Retention, &refused)
	if err := refused.OrNil(); err != nil {
		return Settings{}, err
	}
	return out, nil
}

func normaliseProfile(p Profile, refused *validation.Error) Profile {
	out := p
	out.Name = strings.TrimSpace(p.Name)
	switch {
	case out.Name == "":
		refused.Add("organization.name", ReasonRequired)
	case utf8.RuneCountInString(out.Name) > MaxName:
		refused.Add("organization.name", ReasonTooLong)
	case !singleLine(out.Name):
		refused.Add("organization.name", ReasonCharacters)
	}
	out.ShortName = bounded(p.ShortName, MaxShortName, "organization.shortName", refused)
	if !singleLine(out.ShortName) {
		refused.Add("organization.shortName", ReasonCharacters)
	}
	out.Website = bounded(p.Website, MaxWebsite, "organization.website", refused)
	if out.Website != "" && utf8.RuneCountInString(out.Website) <= MaxWebsite && !validWebsite(out.Website) {
		refused.Add("organization.website", ReasonWebsite)
	}
	out.ContactEmail = bounded(p.ContactEmail, MaxContactEmail, "organization.contactEmail", refused)
	if out.ContactEmail != "" && utf8.RuneCountInString(out.ContactEmail) <= MaxContactEmail && !validEmail(out.ContactEmail) {
		refused.Add("organization.contactEmail", ReasonEmail)
	}
	out.FrontDeskPhone = strings.TrimSpace(p.FrontDeskPhone)
	if out.FrontDeskPhone != "" && !phonePattern.MatchString(out.FrontDeskPhone) {
		refused.Add("organization.frontDeskPhone", ReasonPhone)
	}
	out.FrontDeskHours = bounded(p.FrontDeskHours, MaxFrontDeskHours, "organization.frontDeskHours", refused)
	out.Address = bounded(p.Address, MaxAddress, "organization.address", refused)
	checkLogo(p.LightLogo, "organization.logo.light", refused)
	checkLogo(p.DarkLogo, "organization.logo.dark", refused)
	return out
}

func bounded(value string, limit int, field string, refused *validation.Error) string {
	trimmed := strings.TrimSpace(value)
	if utf8.RuneCountInString(trimmed) > limit {
		refused.Add(field, ReasonTooLong)
	}
	return trimmed
}

func checkLogo(l *Logo, field string, refused *validation.Error) {
	if l != nil && (len(l.SHA256) != logoDigestBytes || !l.Type.Valid()) {
		refused.Add(field, ReasonLogo)
	}
}

func singleLine(text string) bool {
	return strings.IndexFunc(text, func(r rune) bool { return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) }) < 0
}

func validWebsite(site string) bool {
	if strings.IndexFunc(site, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return false
	}
	candidate := site
	if !strings.Contains(site, "://") {
		candidate = "https://" + site
	}
	u, err := url.Parse(candidate)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" {
		return false
	}
	return domainPattern.MatchString(strings.ToLower(u.Hostname()))
}

func validEmail(email string) bool {
	local, host, found := strings.Cut(email, "@")
	if !found || len(local) > maxLocalPart || !localPattern.MatchString(local) {
		return false
	}
	return domainPattern.MatchString(strings.ToLower(host))
}

func normaliseSignIn(s SignIn, refused *validation.Error) SignIn {
	out := s
	if !s.Google && !s.Password {
		refused.Add("signIn", ReasonNoMethod)
	}
	if len(s.AllowedGoogleDomains) > MaxAllowedDomains {
		refused.Add("signIn.allowedGoogleDomains", ReasonTooMany)
	}
	out.AllowedGoogleDomains = make([]string, 0, len(s.AllowedGoogleDomains))
	for _, raw := range s.AllowedGoogleDomains {
		domain := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case len(domain) > maxDomainCharacters || !domainPattern.MatchString(domain):
			refused.Add("signIn.allowedGoogleDomains", ReasonDomain)
		case slices.Contains(out.AllowedGoogleDomains, domain):
			refused.Add("signIn.allowedGoogleDomains", ReasonRepeated)
		default:
			out.AllowedGoogleDomains = append(out.AllowedGoogleDomains, domain)
		}
	}
	return out
}

func checkRetention(r Retention, refused *validation.Error) {
	if m := r.KeepAnswersMonths; m != nil && *m != 12 && *m != 24 {
		refused.Add("retention.keepAnswersMonths", ReasonMonths)
	}
	if !slices.Contains([]int{6, 12, 13, 24}, r.KeepFocusEventsMonths) {
		refused.Add("retention.keepFocusEventsMonths", ReasonMonths)
	}
	if m := r.AnonymiseDisabledAfterMonths; m != nil && *m != 12 && *m != 24 {
		refused.Add("retention.anonymiseDisabledAfterMonths", ReasonMonths)
	}
}

// ChangedFields names, as the contract's paths and in the document's order,
// the fields of next that differ from s. The revision and the save's time and
// author are not fields.
func (s Settings) ChangedFields(next Settings) []string {
	var out []string
	add := func(field string, changed bool) {
		if changed {
			out = append(out, field)
		}
	}
	a, b := s.Profile, next.Profile
	add("organization.name", a.Name != b.Name)
	add("organization.shortName", a.ShortName != b.ShortName)
	add("organization.website", a.Website != b.Website)
	add("organization.contactEmail", a.ContactEmail != b.ContactEmail)
	add("organization.frontDeskPhone", a.FrontDeskPhone != b.FrontDeskPhone)
	add("organization.frontDeskHours", a.FrontDeskHours != b.FrontDeskHours)
	add("organization.address", a.Address != b.Address)
	add("organization.logo.light", !a.LightLogo.Equal(b.LightLogo))
	add("organization.logo.dark", !a.DarkLogo.Equal(b.DarkLogo))
	add("signIn.google", s.SignIn.Google != next.SignIn.Google)
	add("signIn.password", s.SignIn.Password != next.SignIn.Password)
	add("signIn.forcePasswordChange", s.SignIn.ForcePasswordChange != next.SignIn.ForcePasswordChange)
	add("signIn.joinCodes", s.SignIn.JoinCodes != next.SignIn.JoinCodes)
	add("signIn.allowedGoogleDomains", !slices.Equal(s.SignIn.AllowedGoogleDomains, next.SignIn.AllowedGoogleDomains))
	add("retention.keepAnswersMonths", !equalMonths(s.Retention.KeepAnswersMonths, next.Retention.KeepAnswersMonths))
	add("retention.keepFocusEventsMonths", s.Retention.KeepFocusEventsMonths != next.Retention.KeepFocusEventsMonths)
	add("retention.anonymiseDisabledAfterMonths", !equalMonths(s.Retention.AnonymiseDisabledAfterMonths, next.Retention.AnonymiseDisabledAfterMonths))
	return out
}

func equalMonths(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

package command

import (
	"context"
	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/opt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// UpdateProfile changes only the caller's supplied profile fields.
type UpdateProfile struct {
	UserID    string
	Patch     domain.ProfilePatch
	IP        string
	UserAgent string
}

// UpdateProfileHandler validates normalized profile values before the atomic write.
type UpdateProfileHandler struct{ *support.Service }

var phonePattern = regexp.MustCompile(`^[0-9+ ]{6,20}$`)

func (s UpdateProfileHandler) Handle(ctx context.Context, cmd UpdateProfile) (domain.User, error) {
	patch, err := normalizedProfile(cmd.Patch)
	if err != nil {
		return domain.User{}, err
	}
	user, err := s.Users.FindUserByID(ctx, cmd.UserID)
	if err != nil {
		return domain.User{}, err
	}
	if user.Disabled() {
		return domain.User{}, domain.ErrAccountDisabled
	}
	return s.Users.UpdateProfile(ctx, domain.ProfileRecord{UserID: user.ID, Patch: patch, Now: s.Now(), IP: opt.String(cmd.IP), UserAgent: opt.String(cmd.UserAgent)})
}

func normalizedProfile(p domain.ProfilePatch) (domain.ProfilePatch, error) {
	if p.FullName == nil && !p.DisplayNameSet && !p.PhoneSet && p.Locale == nil && p.TimeZone == nil {
		return p, domain.ErrProfileEmpty
	}
	if p.FullName != nil {
		name, err := normalizedFullName(*p.FullName)
		if err != nil {
			return p, err
		}
		p.FullName = &name
	}
	if p.DisplayNameSet {
		name, err := normalizedDisplayName(p.DisplayName)
		if err != nil {
			return p, err
		}
		p.DisplayName = name
	}
	if p.PhoneSet && p.Phone != nil && !phonePattern.MatchString(*p.Phone) {
		return p, domain.ErrPhoneInvalid
	}
	if p.Locale != nil && *p.Locale != "vi" && *p.Locale != "en" {
		return p, domain.ErrLocaleInvalid
	}
	if p.TimeZone != nil && !validZone(*p.TimeZone) {
		return p, domain.ErrTimeZoneInvalid
	}
	return p, nil
}

func validZone(zone string) bool {
	if zone == "" || zone == "Local" || utf8.RuneCountInString(zone) > 64 {
		return false
	}
	_, err := time.LoadLocation(zone)
	return err == nil
}

func normalizedDisplayName(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	name := strings.TrimSpace(*value)
	if n := utf8.RuneCountInString(name); n < 1 || n > 80 {
		return nil, domain.ErrDisplayNameInvalid
	}
	return &name, nil
}

func normalizedFullName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", domain.ErrNameRequired
	}
	if utf8.RuneCountInString(name) > domain.MaxFullNameLength {
		return "", domain.ErrNameTooLong
	}
	return name, nil
}

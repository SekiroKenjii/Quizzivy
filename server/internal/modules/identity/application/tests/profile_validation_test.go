package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"reflect"
	"strings"
	"testing"
	"time"
)

func profileName(value string) *string { return &value }
func profileBool(value bool) *bool     { return &value }

type profileUsers struct {
	domain.Users
	user  domain.User
	got   domain.ProfileRecord
	calls int
	err   error
	prefs domain.PreferencesRecord
}

func (u *profileUsers) FindUserByID(context.Context, string) (domain.User, error) {
	return u.user, u.err
}
func (u *profileUsers) UpdateProfile(_ context.Context, p domain.ProfileRecord) (domain.User, error) {
	u.got = p
	u.calls++
	return u.user, u.err
}
func (u *profileUsers) UpdatePreferences(_ context.Context, p domain.PreferencesRecord) (domain.Preferences, error) {
	u.prefs = p
	u.calls++
	return domain.Preferences{}, u.err
}

func TestProfilePatchValidatesNormalizedNamesAndPresence(t *testing.T) {
	cases := []struct {
		name  string
		patch domain.ProfilePatch
		want  error
	}{
		{"empty", domain.ProfilePatch{}, domain.ErrProfileEmpty},
		{"empty full name", domain.ProfilePatch{FullName: profileName(" \t ")}, domain.ErrNameRequired},
		{"full bound", domain.ProfilePatch{FullName: profileName(strings.Repeat("ữ", 200))}, nil},
		{"full over", domain.ProfilePatch{FullName: profileName(strings.Repeat("ữ", 201))}, domain.ErrNameTooLong},
		{"display clear", domain.ProfilePatch{DisplayNameSet: true}, nil},
		{"display bound", domain.ProfilePatch{DisplayNameSet: true, DisplayName: profileName(strings.Repeat("😀", 80))}, nil},
		{"display over", domain.ProfilePatch{DisplayNameSet: true, DisplayName: profileName(strings.Repeat("ữ", 81))}, domain.ErrDisplayNameInvalid},
		{"display whitespace", domain.ProfilePatch{DisplayNameSet: true, DisplayName: profileName(" \t ")}, domain.ErrDisplayNameInvalid},
		{"phone clear", domain.ProfilePatch{PhoneSet: true}, nil},
		{"phone valid", domain.ProfilePatch{PhoneSet: true, Phone: profileName("+84 123456")}, nil},
		{"phone literal spaces", domain.ProfilePatch{PhoneSet: true, Phone: profileName("      ")}, nil},
		{"phone invalid", domain.ProfilePatch{PhoneSet: true, Phone: profileName("123-456")}, domain.ErrPhoneInvalid},
		{"locale valid", domain.ProfilePatch{Locale: profileName("en")}, nil},
		{"locale invalid", domain.ProfilePatch{Locale: profileName("fr")}, domain.ErrLocaleInvalid},
		{"zone UTC", domain.ProfilePatch{TimeZone: profileName("UTC")}, nil},
		{"zone DST", domain.ProfilePatch{TimeZone: profileName("America/New_York")}, nil},
		{"zone blank", domain.ProfilePatch{TimeZone: profileName("")}, domain.ErrTimeZoneInvalid},
		{"zone whitespace", domain.ProfilePatch{TimeZone: profileName(" UTC ")}, domain.ErrTimeZoneInvalid},
		{"zone Local", domain.ProfilePatch{TimeZone: profileName("Local")}, domain.ErrTimeZoneInvalid},
		{"zone unknown", domain.ProfilePatch{TimeZone: profileName("Mars/Olympus")}, domain.ErrTimeZoneInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &profileUsers{user: domain.User{ID: "actor"}}
			app := application.New(repo, nil, time.Hour, nil, nil)
			_, err := app.Commands.UpdateProfile.Handle(context.Background(), command.UpdateProfile{UserID: "actor", Patch: tc.patch})
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v want%v", err, tc.want)
			}
			if tc.want != nil && repo.calls != 0 {
				t.Fatal("invalid patch reached repository")
			}
		})
	}
	repo := &profileUsers{user: domain.User{ID: "actor"}}
	app := application.New(repo, nil, time.Hour, nil, nil)
	patch := domain.ProfilePatch{FullName: profileName("  Nguyễn An  "), DisplayNameSet: true, DisplayName: profileName("  Cô An  ")}
	if _, err := app.Commands.UpdateProfile.Handle(context.Background(), command.UpdateProfile{UserID: "actor", Patch: patch}); err != nil {
		t.Fatal(err)
	}
	if *repo.got.Patch.FullName != "Nguyễn An" || *repo.got.Patch.DisplayName != "Cô An" || repo.got.Patch.PhoneSet {
		t.Fatalf("patch=%+v", repo.got.Patch)
	}
	if *patch.FullName != "  Nguyễn An  " {
		t.Fatal("input pointer mutated")
	}
}

func TestProfilePatchRefusesDisabledAndPropagatesRepositoryFailure(t *testing.T) {
	disabled := time.Now()
	boom := errors.New("database unavailable")
	for _, tc := range []struct {
		user      domain.User
		err, want error
	}{{domain.User{ID: "a", DisabledAt: &disabled}, nil, domain.ErrAccountDisabled}, {domain.User{}, domain.ErrUserNotFound, domain.ErrUserNotFound}, {domain.User{}, boom, boom}} {
		repo := &profileUsers{user: tc.user, err: tc.err}
		app := application.New(repo, nil, time.Hour, nil, nil)
		_, err := app.Commands.UpdateProfile.Handle(context.Background(), command.UpdateProfile{UserID: "a", Patch: domain.ProfilePatch{Locale: profileName("vi")}})
		if !errors.Is(err, tc.want) || repo.calls != 0 {
			t.Fatalf("error=%v calls=%d", err, repo.calls)
		}
	}
}

func TestProfileMetadataUsesTheApplicationClock(t *testing.T) {
	repo := &profileUsers{user: domain.User{ID: "actor"}}
	app := application.New(repo, nil, time.Hour, nil, nil)
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	app.SetClock(func() time.Time { return now })
	_, err := app.Commands.UpdateProfile.Handle(context.Background(), command.UpdateProfile{UserID: "actor", Patch: domain.ProfilePatch{DisplayNameSet: true}, IP: "203.0.113.7", UserAgent: "profile-test"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.got.UserID != "actor" || repo.got.Now != now || !reflect.DeepEqual(repo.got.IP, profileName("203.0.113.7")) {
		t.Fatalf("metadata=%+v", repo.got)
	}
}

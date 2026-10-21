//go:build integration

package repositories_test

import (
	"reflect"
	"testing"

	"quizzivy/internal/modules/organization/domain"
	"quizzivy/internal/shared/limits"
)

func TestReadMapsAbsentColumnsToEmptyValues(t *testing.T) {
	s := lockedSession(t)
	s.exec(`
		UPDATE app.organization_settings
		   SET name = 'Chỉ có tên', short_name = NULL, website = NULL, contact_email = NULL,
		       front_desk_phone = NULL, front_desk_hours = NULL, address = NULL,
		       logo_sha256 = NULL, logo_type = NULL, logo_dark_sha256 = NULL, logo_dark_type = NULL,
		       allowed_google_domains = '{}',
		       keep_answers_months = NULL, anonymise_disabled_after_months = NULL,
		       updated_by = NULL`)

	got := s.stored()

	if !reflect.DeepEqual(got.Profile, domain.Profile{Name: "Chỉ có tên"}) {
		t.Errorf("profile = %+v, want only the name", got.Profile)
	}
	if got.Profile.LightLogo != nil || got.Profile.DarkLogo != nil {
		t.Errorf("logos = %v and %v, want nil", got.Profile.LightLogo, got.Profile.DarkLogo)
	}
	if got.SignIn.AllowedGoogleDomains == nil || len(got.SignIn.AllowedGoogleDomains) != 0 {
		t.Errorf("allowed domains = %#v, want an empty non-nil slice", got.SignIn.AllowedGoogleDomains)
	}
	if got.Retention.KeepAnswersMonths != nil || got.Retention.AnonymiseDisabledAfterMonths != nil {
		t.Errorf("retention = %+v, want nil month counts", got.Retention)
	}
	if got.UpdatedBy != "" {
		t.Errorf("updated by = %q, want empty", got.UpdatedBy)
	}
}

func TestAReservedSVGLogoTypeReadsAsNoLogo(t *testing.T) {
	s := lockedSession(t)
	s.exec(`
		UPDATE app.organization_settings
		   SET logo_sha256 = decode(repeat('01', 32), 'hex'), logo_type = 'image/svg+xml',
		       logo_dark_sha256 = decode(repeat('02', 32), 'hex'), logo_dark_type = 'image/png'`)

	got := s.stored()

	if got.Profile.LightLogo != nil {
		t.Errorf("light logo = %+v, want none for the reserved type", got.Profile.LightLogo)
	}
	if got.Profile.DarkLogo == nil || got.Profile.DarkLogo.Type != domain.LogoPNG {
		t.Errorf("dark logo = %+v, want the PNG", got.Profile.DarkLogo)
	}
}

func TestReadMapsEveryColumnToItsField(t *testing.T) {
	s := lockedSession(t)
	author := s.addAccount(account{role: "admin", password: true})
	s.exec(`
		UPDATE app.organization_settings
		   SET name = 'Trung tâm A', short_name = 'TTA', website = 'https://tta.example', contact_email = 'a@tta.example',
		       front_desk_phone = '028 3822 1111', front_desk_hours = 'Cả tuần', address = '1 Lê Lợi',
		       logo_sha256 = decode(repeat('01', 32), 'hex'), logo_type = 'image/png',
		       logo_dark_sha256 = decode(repeat('02', 32), 'hex'), logo_dark_type = 'image/png',
		       google_sign_in = false, password_sign_in = true, force_password_change = false, join_codes_enabled = false,
		       allowed_google_domains = ARRAY['b.example', 'a.example'],
		       keep_answers_months = 12, keep_focus_events_months = 24, anonymise_disabled_after_months = 24,
		       limit_seats = 300, limit_storage_bytes = 21474836480, limit_emails_per_month = 50000, warn_at_percent = 90,
		       revision = 41, updated_by = $1::uuid`, author)

	got := s.stored()

	want := domain.Settings{
		Profile: domain.Profile{
			Name: "Trung tâm A", ShortName: "TTA", Website: "https://tta.example", ContactEmail: "a@tta.example",
			FrontDeskPhone: "028 3822 1111", FrontDeskHours: "Cả tuần", Address: "1 Lê Lợi",
			LightLogo: &domain.Logo{SHA256: bytesOf(0x01, 32), Type: domain.LogoPNG},
			DarkLogo:  &domain.Logo{SHA256: bytesOf(0x02, 32), Type: domain.LogoPNG},
		},
		SignIn: domain.SignIn{
			Google: false, Password: true, ForcePasswordChange: false, JoinCodes: false,
			AllowedGoogleDomains: []string{"b.example", "a.example"},
		},
		Retention: domain.Retention{KeepAnswersMonths: intp(12), KeepFocusEventsMonths: 24, AnonymiseDisabledAfterMonths: intp(24)},
		Limits: limits.Stored{
			limits.Seats: 300, limits.StorageBytes: 20 * limits.GiB, limits.EmailsPerMonth: 50000, limits.WarnAtPercent: 90,
		},
		Revision:  41,
		UpdatedBy: author,
		UpdatedAt: got.UpdatedAt,
	}
	if !sameSettings(got, want) {
		t.Errorf("document = %+v, want %+v", got, want)
	}
	if got.UpdatedAt.IsZero() || !got.UpdatedAt.Equal(s.dbNow()) {
		t.Errorf("updated at = %v, want the time of the change %v", got.UpdatedAt, s.dbNow())
	}
}

func bytesOf(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

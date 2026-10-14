package http

import (
	"context"
	"encoding/hex"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/organization/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/validation"
)

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func text(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func months(m *int) *int {
	if m == nil {
		return nil
	}
	v := *m
	return &v
}

func settingsBody(s domain.Settings) openapi.OrganizationSettings {
	p := s.Profile
	out := openapi.OrganizationSettings{
		Organization: openapi.OrganizationProfile{
			Name: p.Name, ShortName: nullable(p.ShortName), Website: nullable(p.Website), ContactEmail: nullable(p.ContactEmail),
			FrontDeskPhone: nullable(p.FrontDeskPhone), FrontDeskHours: nullable(p.FrontDeskHours), Address: nullable(p.Address),
		},
		SignIn: openapi.SignInPolicy{
			Google: s.SignIn.Google, Password: s.SignIn.Password, ForcePasswordChange: s.SignIn.ForcePasswordChange,
			JoinCodes: s.SignIn.JoinCodes, AllowedGoogleDomains: append([]string{}, s.SignIn.AllowedGoogleDomains...),
		},
		Retention: openapi.RetentionPolicy{KeepFocusEventsMonths: openapi.RetentionPolicyKeepFocusEventsMonths(s.Retention.KeepFocusEventsMonths)},
		Revision:  s.Revision,
		UpdatedAt: s.UpdatedAt.UTC(),
	}
	if m := s.Retention.KeepAnswersMonths; m != nil {
		out.Retention.KeepAnswersMonths = new(openapi.RetentionPolicyKeepAnswersMonths(*m))
	}
	if m := s.Retention.AnonymiseDisabledAfterMonths; m != nil {
		out.Retention.AnonymiseDisabledAfterMonths = new(openapi.RetentionPolicyAnonymiseDisabledAfterMonths(*m))
	}
	out.Organization.Logo.Light = logoBody(p.LightLogo)
	out.Organization.Logo.Dark = logoBody(p.DarkLogo)
	if s.UpdatedBy != "" {
		id := httpapi.ParseUUID(s.UpdatedBy)
		out.UpdatedBy = &id
	}
	return out
}

func logoBody(l *domain.Logo) *openapi.OrganizationLogo {
	if l == nil {
		return nil
	}
	return &openapi.OrganizationLogo{Sha256: hex.EncodeToString(l.SHA256), Type: openapi.OrganizationLogoType(l.Type)}
}

func settingsOf(in openapi.OrganizationSettingsInput) domain.Settings {
	p := in.Organization
	profile := domain.Profile{
		Name: p.Name, ShortName: text(p.ShortName), Website: text(p.Website), ContactEmail: text(p.ContactEmail),
		FrontDeskPhone: text(p.FrontDeskPhone), FrontDeskHours: text(p.FrontDeskHours), Address: text(p.Address),
		LightLogo: logoOf(p.Logo.Light), DarkLogo: logoOf(p.Logo.Dark),
	}
	r := in.Retention
	retention := domain.Retention{KeepFocusEventsMonths: int(r.KeepFocusEventsMonths)}
	if r.KeepAnswersMonths != nil {
		retention.KeepAnswersMonths = months(new(int(*r.KeepAnswersMonths)))
	}
	if r.AnonymiseDisabledAfterMonths != nil {
		retention.AnonymiseDisabledAfterMonths = months(new(int(*r.AnonymiseDisabledAfterMonths)))
	}
	return domain.Settings{Profile: profile, SignIn: signInOf(in.SignIn), Retention: retention}
}

func signInOf(s openapi.SignInPolicy) domain.SignIn {
	return domain.SignIn{
		Google: s.Google, Password: s.Password, ForcePasswordChange: s.ForcePasswordChange, JoinCodes: s.JoinCodes,
		AllowedGoogleDomains: append([]string{}, s.AllowedGoogleDomains...),
	}
}

func logoOf(l *openapi.OrganizationLogo) *domain.Logo {
	if l == nil {
		return nil
	}
	digest, err := hex.DecodeString(l.Sha256)
	if err != nil {
		digest = nil
	}
	return &domain.Logo{SHA256: digest, Type: domain.LogoType(l.Type)}
}

func lockoutBody(ctx context.Context, impact domain.Impact) openapi.ErrorResponse {
	message := httpx.Text(ctx,
		"Chính sách đăng nhập này sẽ khoá những quản trị viên ra ngoài. Hãy giữ lại ít nhất một cách đăng nhập cho mỗi người.",
		"This sign-in policy would lock the Admins out. Keep at least one way in for each of them.")
	if impact.ActorLocked {
		message = httpx.Text(ctx,
			"Chính sách đăng nhập này sẽ khoá chính bạn ra ngoài. Hãy giữ lại một cách đăng nhập cho bạn.",
			"This sign-in policy would lock you out. Keep a way in for yourself.")
	}
	return httpapi.ErrorWithDetails(ctx, openapi.SIGNINPOLICYLOCKOUT, message, map[string]interface{}{
		"affectedUsers": impact.AffectedUsers, "affectedAdmins": impact.AffectedAdmins,
	})
}

func reasonText(ctx context.Context, reason string) (string, bool) {
	switch reason {
	case domain.ReasonRequired:
		return httpx.Text(ctx, "Không được để trống.", "Cannot be empty."), true
	case domain.ReasonTooLong:
		return httpx.Text(ctx, "Quá dài.", "Too long."), true
	case domain.ReasonWebsite:
		return httpx.Text(ctx, "Nhập tên miền hoặc địa chỉ http/https, không có khoảng trắng.", "Enter a domain or an http/https address with no spaces."), true
	case domain.ReasonEmail:
		return httpx.Text(ctx, "Địa chỉ email không hợp lệ.", "Not a valid email address."), true
	case domain.ReasonPhone:
		return httpx.Text(ctx, "Chỉ gồm chữ số, khoảng trắng và + ( ) . -, từ 6 đến 30 ký tự.", "Digits, spaces and + ( ) . - only, 6 to 30 characters."), true
	case domain.ReasonDomain:
		return httpx.Text(ctx, "Tên miền không hợp lệ.", "Not a valid domain."), true
	case domain.ReasonRepeated:
		return httpx.Text(ctx, "Tên miền bị lặp.", "Repeated domain."), true
	case domain.ReasonTooMany:
		return httpx.Text(ctx, "Tối đa 50 tên miền.", "At most 50 domains."), true
	case domain.ReasonNoMethod:
		return httpx.Text(ctx, "Phải bật ít nhất một cách đăng nhập.", "Keep at least one way to sign in on."), true
	case domain.ReasonMonths:
		return httpx.Text(ctx, "Số tháng không nằm trong các lựa chọn.", "Not one of the choices."), true
	case domain.ReasonLogo:
		return httpx.Text(ctx, "Logo không hợp lệ.", "Not a valid logo."), true
	case domain.ReasonLogoGone:
		return httpx.Text(ctx, "Máy chủ không có logo này. Hãy tải lên lại.", "The server holds no such logo. Upload it again."), true
	case domain.ReasonLogoStore:
		return httpx.Text(ctx, "Máy chủ chưa có kho lưu trữ nên chưa thể lưu logo.", "This server has no object storage, so a logo cannot be kept."), true
	}
	return "", false
}

func worded(ctx context.Context, invalid *validation.Error) *validation.Error {
	out := &validation.Error{}
	for _, f := range invalid.Fields {
		message, ok := reasonText(ctx, f.Message)
		if !ok {
			message = f.Message
		}
		out.Add(f.Field, message)
	}
	return out
}

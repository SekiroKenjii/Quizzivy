// Package http is the organization module's transport: the settings and the logo upload under /admin/settings, and the public read of the centre and its logo.
package http

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"

	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/organization/application"
	"quizzivy/internal/modules/organization/application/command"
	"quizzivy/internal/modules/organization/application/query"
	"quizzivy/internal/modules/organization/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"quizzivy/internal/shared/validation"
)

const (
	publicCacheControl    = "public, max-age=300"
	immutableCacheControl = "public, max-age=31536000, immutable"
	logoSandbox           = "default-src 'none'; style-src 'unsafe-inline'; sandbox"
)

// Organization serves the centre's settings and its public face.
type Organization struct {
	app *application.Application
}

// NewOrganization builds the transport. A nil application answers 501.
func NewOrganization(app *application.Application) Organization {
	return Organization{app: app}
}

// GetOrganizationSettings implements GET /admin/settings.
func (h Organization) GetOrganizationSettings(ctx context.Context, _ openapi.GetOrganizationSettingsRequestObject) (openapi.GetOrganizationSettingsResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	settings, err := h.app.Queries.Settings.Handle(ctx, query.Settings{})
	if err != nil {
		return nil, err
	}
	return openapi.GetOrganizationSettings200JSONResponse{
		Body:    settingsBody(settings),
		Headers: openapi.GetOrganizationSettings200ResponseHeaders{CacheControl: "private, no-store"},
	}, nil
}

// UpdateOrganizationSettings implements PUT /admin/settings.
func (h Organization) UpdateOrganizationSettings(ctx context.Context, request openapi.UpdateOrganizationSettingsRequestObject) (openapi.UpdateOrganizationSettingsResponseObject, error) {
	who, ok := httpapi.ActorFromContext(ctx)
	if h.app == nil || !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	saved, err := h.app.Commands.UpdateSettings.Handle(ctx, command.UpdateSettings{
		Actor: who, Settings: settingsOf(*request.Body), ExpectedRevision: request.Body.ExpectedRevision,
	})
	var invalid *validation.Error
	var locked *domain.LockedOut
	switch {
	case err == nil:
		return openapi.UpdateOrganizationSettings200JSONResponse(settingsBody(saved)), nil
	case errors.As(err, &invalid):
		return openapi.UpdateOrganizationSettings400JSONResponse(httpapi.Invalid(ctx,
			httpx.Text(ctx, "Cài đặt không hợp lệ.", "The settings are not valid."), worded(ctx, invalid))), nil
	case errors.Is(err, domain.ErrStale):
		return openapi.UpdateOrganizationSettings409JSONResponse(httpapi.Error(ctx, openapi.STALEWRITE,
			httpx.Text(ctx, "Cài đặt đã được lưu ở nơi khác. Hãy tải lại rồi sửa tiếp.",
				"The settings were saved elsewhere. Reload them and edit again."))), nil
	case errors.As(err, &locked):
		return openapi.UpdateOrganizationSettings422JSONResponse(lockoutBody(ctx, locked.Impact)), nil
	default:
		return nil, err
	}
}

// GetSignInImpact implements POST /admin/settings/sign-in-impact.
func (h Organization) GetSignInImpact(ctx context.Context, request openapi.GetSignInImpactRequestObject) (openapi.GetSignInImpactResponseObject, error) {
	who, ok := httpapi.ActorFromContext(ctx)
	if h.app == nil || !ok || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	impact, err := h.app.Queries.SignInImpact.Handle(ctx, query.SignInImpact{ActorID: who.ID, Policy: signInOf(request.Body.SignIn)})
	if err != nil {
		return nil, err
	}
	return openapi.GetSignInImpact200JSONResponse(openapi.SignInImpact{
		AffectedUsers: impact.AffectedUsers, AffectedAdmins: impact.AffectedAdmins,
		ActorLocked: impact.ActorLocked, Lockout: impact.Lockout(),
	}), nil
}

// UploadOrganizationLogo implements POST /admin/settings/logo.
func (h Organization) UploadOrganizationLogo(ctx context.Context, request openapi.UploadOrganizationLogoRequestObject) (openapi.UploadOrganizationLogoResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	if request.Body == nil {
		return noLogo(ctx), nil
	}
	file, variant, err := readUpload(request.Body)
	if overBodyLimit(err) {
		return logoTooLarge(ctx), nil
	}
	if err != nil || !variant.Valid() {
		return noLogo(ctx), nil
	}
	stored, err := h.app.Commands.UploadLogo.Handle(ctx, command.UploadLogo{Body: bytes.NewReader(file)})
	switch {
	case err == nil:
		return openapi.UploadOrganizationLogo200JSONResponse(openapi.OrganizationLogoUpload{
			Sha256: hex.EncodeToString(stored.Logo.SHA256), Type: openapi.OrganizationLogoUploadType(stored.Logo.Type),
			Width: stored.Width, Height: stored.Height,
		}), nil
	case errors.Is(err, domain.ErrStorageOff):
		return openapi.UploadOrganizationLogo501JSONResponse(httpapi.Error(ctx, openapi.INTERNAL,
			httpx.Text(ctx, "Máy chủ chưa có kho lưu trữ nên chưa thể lưu logo.", "This server has no object storage, so a logo cannot be kept."))), nil
	case errors.Is(err, domain.ErrLogoTooLarge):
		return logoTooLarge(ctx), nil
	default:
		if refusal, ok := unsupported(ctx, err); ok {
			return refusal, nil
		}
		return nil, err
	}
}

func unsupported(ctx context.Context, err error) (openapi.UploadOrganizationLogo415JSONResponse, bool) {
	refuse := func(code openapi.ErrorCode, message string) (openapi.UploadOrganizationLogo415JSONResponse, bool) {
		return openapi.UploadOrganizationLogo415JSONResponse(httpapi.Error(ctx, code, message)), true
	}
	switch {
	case errors.Is(err, domain.ErrLogoUnsupported):
		return refuse(openapi.MEDIATYPEUNSUPPORTED, httpx.Text(ctx, "Logo phải là ảnh PNG hoặc SVG.", "The logo must be a PNG or an SVG."))
	case errors.Is(err, domain.ErrLogoUnreadable):
		return refuse(openapi.MEDIAUNREADABLE, httpx.Text(ctx, "Không đọc được logo này. Tệp có thể bị lỗi hoặc chưa tải lên hết.", "This logo cannot be read. It may be damaged or not fully uploaded."))
	case errors.Is(err, domain.ErrLogoDimensions):
		return refuse(openapi.IMAGEDIMENSIONS, httpx.Text(ctx, "Logo phải là hình vuông, mỗi cạnh từ 256 đến 2048 px.", "The logo must be square, each side 256 to 2048 pixels."))
	case errors.Is(err, domain.ErrSVGNotAllowed):
		return refuse(openapi.SVGNOTALLOWED, httpx.Text(ctx, "Tệp SVG chứa thành phần không được phép. Hãy xuất lại logo chỉ gồm hình vẽ.", "The SVG holds something that is not allowed. Export the logo again with drawing elements only."))
	}
	return openapi.UploadOrganizationLogo415JSONResponse{}, false
}

func readUpload(reader *multipart.Reader) ([]byte, domain.Variant, error) {
	var file []byte
	var variant domain.Variant
	haveFile := false
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", err
		}
		switch {
		case part.FileName() != "" && !haveFile:
			file, err = io.ReadAll(io.LimitReader(part, domain.MaxPNGBytes+1))
			haveFile = err == nil
		case part.FormName() == "variant":
			var value []byte
			value, err = io.ReadAll(io.LimitReader(part, 16))
			variant = domain.Variant(value)
		}
		_ = part.Close()
		if err != nil {
			return nil, "", err
		}
	}
	if !haveFile {
		return nil, "", errors.New("organization: the upload holds no file")
	}
	return file, variant, nil
}

func overBodyLimit(err error) bool {
	var tooLarge *http.MaxBytesError
	return errors.As(err, &tooLarge)
}

func noLogo(ctx context.Context) openapi.UploadOrganizationLogo400JSONResponse {
	return openapi.UploadOrganizationLogo400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(httpapi.FieldError(ctx, "file",
		httpx.Text(ctx, "Yêu cầu tải lên cần một tệp logo và variant là light hoặc dark.", "The upload needs a logo file and a variant of light or dark.")))}
}

func logoTooLarge(ctx context.Context) openapi.UploadOrganizationLogo413JSONResponse {
	return openapi.UploadOrganizationLogo413JSONResponse(httpapi.Error(ctx, openapi.MEDIATOOLARGE,
		httpx.Text(ctx, "Logo vượt quá giới hạn: PNG tối đa 1 MB, SVG tối đa 256 KB.", "The logo is over the limit: 1 MB for a PNG, 256 KB for an SVG.")))
}

// GetPublicOrganization implements GET /public/organization.
func (h Organization) GetPublicOrganization(ctx context.Context, _ openapi.GetPublicOrganizationRequestObject) (openapi.GetPublicOrganizationResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	s, err := h.app.Queries.Current.Handle(ctx, query.Current{})
	if err != nil {
		return nil, err
	}
	p := s.Profile
	body := openapi.PublicOrganization{
		Name: p.Name, ShortName: nullable(p.ShortName), Website: nullable(p.Website), ContactEmail: nullable(p.ContactEmail),
		FrontDeskPhone: nullable(p.FrontDeskPhone), FrontDeskHours: nullable(p.FrontDeskHours), Address: nullable(p.Address),
		JoinCodes: s.SignIn.JoinCodes,
	}
	body.Logo.LightUrl, body.Logo.DarkUrl = logoURL(domain.Light, p.LightLogo), logoURL(domain.Dark, p.DarkLogo)
	body.SignIn.Google, body.SignIn.Password = s.SignIn.Google, s.SignIn.Password
	return openapi.GetPublicOrganization200JSONResponse{
		Body:    body,
		Headers: openapi.GetPublicOrganization200ResponseHeaders{CacheControl: publicCacheControl},
	}, nil
}

func logoURL(variant domain.Variant, logo *domain.Logo) *string {
	if logo == nil {
		return nil
	}
	q := url.Values{"variant": {string(variant)}, "v": {hex.EncodeToString(logo.SHA256)}}
	out := "/public/organization/logo?" + q.Encode()
	return &out
}

// GetOrganizationLogo implements GET /public/organization/logo.
func (h Organization) GetOrganizationLogo(ctx context.Context, request openapi.GetOrganizationLogoRequestObject) (openapi.GetOrganizationLogoResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	logo, err := h.app.Queries.Logo.Handle(ctx, query.Logo{Variant: domain.Variant(request.Params.Variant)})
	switch {
	case errors.Is(err, domain.ErrNoLogo), errors.Is(err, domain.ErrStorageOff):
		return openapi.GetOrganizationLogo404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx,
			httpx.Text(ctx, "Trung tâm chưa có logo.", "The centre has no logo.")))}, nil
	case err != nil:
		return nil, err
	}
	cache := publicCacheControl
	if request.Params.V != nil && *request.Params.V == hex.EncodeToString(logo.Logo.SHA256) {
		cache = immutableCacheControl
	}
	headers := openapi.GetOrganizationLogo200ResponseHeaders{
		CacheControl: cache, ContentDisposition: `attachment; filename="logo.` + logo.Logo.Type.Extension() + `"`,
		ContentSecurityPolicy: logoSandbox, XContentTypeOptions: "nosniff",
	}
	if logo.Logo.Type == domain.LogoSVG {
		return openapi.GetOrganizationLogo200ImagesvgXmlResponse{Body: bytes.NewReader(logo.Bytes), Headers: headers, ContentLength: int64(len(logo.Bytes))}, nil
	}
	return openapi.GetOrganizationLogo200ImagepngResponse{Body: bytes.NewReader(logo.Bytes), Headers: headers, ContentLength: int64(len(logo.Bytes))}, nil
}

package email

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"net/url"
	"strings"
	texttemplate "text/template"
)

// Template names a message the app sends. It is stored in the outbox, so a
// name never changes once it has shipped.
type Template string

// The templates the app sends.
const (
	AccountInvite     Template = "account_invite"
	PasswordResetLink Template = "password_reset_link"
	SignInInvite      Template = "sign_in_invite"
	DataExportReady   Template = "data_export_ready"
)

// Locales are the languages every template is written in, Vietnamese first.
var Locales = []string{"vi", "en"}

// DefaultLocale is the language of a message whose recipient has chosen none.
const DefaultLocale = "vi"

// Params are one template's parameters, stored as the outbox row's params.
// Each template has its own type, and Template says which.
type Params interface {
	Template() Template
}

// AccountInviteParams invite a new account to set its first password through
// a link that carries the outbox secret.
type AccountInviteParams struct {
	FullName string `json:"fullName"`
}

// PasswordResetLinkParams send an account a link, carrying the outbox
// secret, to set a new password.
type PasswordResetLinkParams struct {
	FullName string `json:"fullName"`
}

// SignInInviteParams tell a new account to sign in with Google, when the
// organization has password sign-in off.
type SignInInviteParams struct {
	FullName string `json:"fullName"`
}

// DataExportReadyParams tell the requester of a data export that its app page
// can download it.
type DataExportReadyParams struct {
	FullName string `json:"fullName"`
	ExportID string `json:"exportId"`
}

func (AccountInviteParams) Template() Template { return AccountInvite }

func (PasswordResetLinkParams) Template() Template { return PasswordResetLink }

func (SignInInviteParams) Template() Template { return SignInInvite }

func (DataExportReadyParams) Template() Template { return DataExportReady }

type definition struct {
	secret bool
	decode func([]byte) (Params, error)
	link   func(appURL string, p Params, secret string) string
}

func decoder[P Params]() func([]byte) (Params, error) {
	return func(raw []byte) (Params, error) {
		var p P
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&p); err != nil {
			return nil, err
		}
		return p, nil
	}
}

func setPasswordLink(appURL string, _ Params, secret string) string {
	return appURL + "/set-password#token=" + secret
}

var definitions = map[Template]definition{
	AccountInvite:     {secret: true, decode: decoder[AccountInviteParams](), link: setPasswordLink},
	PasswordResetLink: {secret: true, decode: decoder[PasswordResetLinkParams](), link: setPasswordLink},
	SignInInvite: {decode: decoder[SignInInviteParams](), link: func(appURL string, _ Params, _ string) string {
		return appURL + "/login"
	}},
	DataExportReady: {decode: decoder[DataExportReadyParams](), link: func(appURL string, p Params, _ string) string {
		return appURL + "/admin/exports/" + url.PathEscape(p.(DataExportReadyParams).ExportID)
	}},
}

// Templates lists every template the app sends.
func Templates() []Template {
	return []Template{AccountInvite, PasswordResetLink, SignInInvite, DataExportReady}
}

// NeedsSecret reports whether t's link carries the outbox secret, so that a
// message of it cannot be enqueued without one and no other message with one.
func NeedsSecret(t Template) bool { return definitions[t].secret }

// ErrUnknownTemplate is a template name the app does not send.
var ErrUnknownTemplate = errors.New("email: unknown template")

// DecodeParams reads the stored params of a t message into t's own type.
func DecodeParams(t Template, raw []byte) (Params, error) {
	def, ok := definitions[t]
	if !ok {
		return nil, ErrUnknownTemplate
	}
	return def.decode(raw)
}

// Letter is what one message is rendered from: the recipient's locale, the
// organization's short name for the header, the app's origin its link points
// into, the template's params and, for a template that needs one, the secret
// its link carries.
type Letter struct {
	Locale string
	Org    string
	AppURL string
	Params Params
	Secret string
}

// Rendered is a rendered message's subject on one line, and its text and HTML
// parts.
type Rendered struct {
	Subject string
	Text    string
	HTML    string
}

type view struct {
	Org    string
	Link   string
	Params Params
}

type pair struct {
	text *texttemplate.Template
	html *htmltemplate.Template
}

//go:embed templates
var files embed.FS

// Renderer renders the embedded templates. Each file defines "subject",
// "text" and "html": the first two are executed as text/template and the
// last as html/template, all with missingkey=error.
type Renderer struct {
	pairs map[string]pair
}

// NewRenderer parses every template in every locale and refuses a missing
// file or a file that does not define all three parts.
func NewRenderer() (*Renderer, error) {
	r := &Renderer{pairs: map[string]pair{}}
	for _, locale := range Locales {
		for _, t := range Templates() {
			p, err := parseTemplate(locale, t)
			if err != nil {
				return nil, err
			}
			r.pairs[locale+"/"+string(t)] = p
		}
	}
	return r, nil
}

func parseTemplate(locale string, t Template) (pair, error) {
	name := locale + "/" + string(t) + ".tmpl"
	src, err := files.ReadFile("templates/" + name)
	if err != nil {
		return pair{}, fmt.Errorf("email template %s: %w", name, err)
	}
	text, err := texttemplate.New(name).Option("missingkey=error").Parse(string(src))
	if err != nil {
		return pair{}, fmt.Errorf("email template %s: %w", name, err)
	}
	html, err := htmltemplate.New(name).Option("missingkey=error").Parse(string(src))
	if err != nil {
		return pair{}, fmt.Errorf("email template %s: %w", name, err)
	}
	for _, part := range []string{"subject", "text", "html"} {
		if text.Lookup(part) == nil {
			return pair{}, fmt.Errorf("email template %s defines no %q", name, part)
		}
	}
	return pair{text: text, html: html}, nil
}

// Render renders l. It refuses an unknown locale or template, a template that
// needs a secret without one, and any part that renders empty.
func (r *Renderer) Render(l Letter) (Rendered, error) {
	if l.Params == nil {
		return Rendered{}, ErrUnknownTemplate
	}
	t := l.Params.Template()
	def, ok := definitions[t]
	p, found := r.pairs[l.Locale+"/"+string(t)]
	if !ok || !found {
		return Rendered{}, fmt.Errorf("email: no %s template in %q", t, l.Locale)
	}
	if def.secret && l.Secret == "" {
		return Rendered{}, fmt.Errorf("email: %s needs a secret", t)
	}
	v := view{Org: l.Org, Link: def.link(strings.TrimRight(l.AppURL, "/"), l.Params, l.Secret), Params: l.Params}
	var subject, text, html bytes.Buffer
	if err := p.text.ExecuteTemplate(&subject, "subject", v); err != nil {
		return Rendered{}, err
	}
	if err := p.text.ExecuteTemplate(&text, "text", v); err != nil {
		return Rendered{}, err
	}
	if err := p.html.ExecuteTemplate(&html, "html", v); err != nil {
		return Rendered{}, err
	}
	out := Rendered{Subject: strings.Join(strings.Fields(subject.String()), " "), Text: strings.TrimSpace(text.String()) + "\n", HTML: strings.TrimSpace(html.String()) + "\n"}
	if out.Subject == "" || strings.TrimSpace(out.Text) == "" || strings.TrimSpace(out.HTML) == "" {
		return Rendered{}, fmt.Errorf("email: %s in %q renders an empty part", t, l.Locale)
	}
	return out, nil
}
